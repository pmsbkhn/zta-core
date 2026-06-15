// Command zta-pep is a configuration-driven PEP sidecar: a reverse proxy that
// enforces authorization in front of ANY upstream workload — including non-Go
// services that cannot embed the pep library. Put it in the request path (same
// pod / next to the service), point it at the protected upstream and a PDP, and
// declare the guarded routes in a YAML file.
//
//	zta-pep -config /etc/zta/pep.yaml
//
// It wires the same L0/L1/L2 ladder, decision-token fast-path, CAEP revocation
// and bubble-up step-up as the reference VSP workloads, but with everything
// taken from config instead of code. Fail-closed: any PDP error denies.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pmsbkhn/zta-core/authz/authzen"
	"github.com/pmsbkhn/zta-core/authz/pdpclient"
	"github.com/pmsbkhn/zta-core/authz/pep"
	"github.com/pmsbkhn/zta-core/authz/token"
	"github.com/pmsbkhn/zta-core/ports/pip"
	"github.com/pmsbkhn/zta-core/services"
	"github.com/pmsbkhn/zta-core/signals/caep"
	"sigs.k8s.io/yaml"
)

// Config is the sidecar's YAML configuration (struct tags are JSON because
// sigs.k8s.io/yaml decodes YAML via JSON).
type Config struct {
	Listen          string `json:"listen"`            // e.g. ":8080"
	Profile         string `json:"profile"`           // edge | east_west | partner
	PEPID           string `json:"pep_id"`            // identifier surfaced to the PDP
	Upstream        string `json:"upstream"`          // protected service, e.g. http://127.0.0.1:9000
	RequirePeerSVID bool   `json:"require_peer_svid"` // demand a verified mTLS peer cert (serve mTLS)
	TokenSecret     string `json:"token_secret"`      // optional: enable decision-token fast-path (HS256)
	CAEPSecret      string `json:"caep_secret"`       // optional: enable CAEP receiver at POST /events
	PDP             struct {
		HTTPURL  string `json:"http_url"`  // AuthZEN HTTP endpoint, e.g. http://pdp:8080
		GRPCAddr string `json:"grpc_addr"` // gRPC endpoint over mTLS, e.g. pdp:9090 (preferred)
	} `json:"pdp"`
	Routes []struct {
		Method        string   `json:"method"`
		Path          string   `json:"path"`
		Action        string   `json:"action"`
		ResourceType  string   `json:"resource_type"`
		ResourceProps []string `json:"resource_props"`
	} `json:"routes"`
}

func main() {
	cfgPath := flag.String("config", envOr("ZTA_PEP_CONFIG", "/etc/zta/pep.yaml"), "path to the YAML config")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).With("svc", "zta-pep")
	if err := run(*cfgPath, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath string, log *slog.Logger) error {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return err
	}
	pdp, err := buildPDP(cfg, log)
	if err != nil {
		return err
	}
	h, err := handler(cfg, pdp, log)
	if err != nil {
		return err
	}

	srv := &http.Server{Addr: cfg.Listen, Handler: h, ReadHeaderTimeout: 5 * time.Second}

	// Serve mTLS when a peer SVID is required (SVID material from SPIRE Workload
	// API or SVID_* env, via services.LoadServerTLS). Otherwise plain HTTP (edge).
	if cfg.RequirePeerSVID {
		tlsCfg, ok, terr := services.LoadServerTLS()
		if terr != nil {
			return terr
		}
		if !ok {
			return errors.New("require_peer_svid=true but no SVID configured (SPIFFE_ENDPOINT_SOCKET / SVID_*)")
		}
		srv.TLSConfig = tlsCfg
		log.Info("zta-pep listening (mTLS)", "addr", cfg.Listen, "profile", cfg.Profile, "upstream", cfg.Upstream)
		if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	log.Info("zta-pep listening (HTTP)", "addr", cfg.Listen, "profile", cfg.Profile, "upstream", cfg.Upstream)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// buildPDP selects the PDP transport: gRPC-over-mTLS preferred, else AuthZEN HTTP.
func buildPDP(cfg Config, log *slog.Logger) (pep.PDP, error) {
	switch {
	case cfg.PDP.GRPCAddr != "":
		c, err := services.PDPGRPCClient(cfg.PDP.GRPCAddr)
		if err != nil {
			return nil, err
		}
		log.Info("PDP over gRPC/mTLS", "addr", cfg.PDP.GRPCAddr)
		return c, nil
	case cfg.PDP.HTTPURL != "":
		log.Info("PDP over HTTP", "url", cfg.PDP.HTTPURL)
		return pdpclient.New(cfg.PDP.HTTPURL), nil
	default:
		return nil, errors.New("config: pdp.grpc_addr or pdp.http_url is required")
	}
}

// handler builds the sidecar's HTTP handler: the PEP ladder in front of a reverse
// proxy to the upstream, plus /healthz and (optionally) a CAEP receiver.
func handler(cfg Config, pdp pep.PDP, log *slog.Logger) (http.Handler, error) {
	target, err := url.Parse(cfg.Upstream)
	if err != nil {
		return nil, errors.New("config: invalid upstream URL: " + err.Error())
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	// Edge profile owns the user session: translate a bubbled-up X-Step-Up-Required
	// from a deeper service into a 401 MFA challenge so the client can re-auth.
	if cfg.Profile == string(authzen.ProfileEdge) {
		proxy.ModifyResponse = func(resp *http.Response) error {
			acr := resp.Header.Get(pep.HeaderStepUpRequired)
			if acr == "" {
				return nil
			}
			challenge, _ := json.Marshal(map[string]any{"error": "step_up_required", "required_acr": acr, "method": "mfa"})
			resp.StatusCode = http.StatusUnauthorized
			resp.Status = http.StatusText(http.StatusUnauthorized)
			resp.Body = io.NopCloser(bytes.NewReader(challenge))
			resp.ContentLength = int64(len(challenge))
			resp.Header.Set("Content-Type", "application/json")
			resp.Header.Set("Content-Length", strconv.Itoa(len(challenge)))
			return nil
		}
	}

	var verifier pep.TokenVerifier
	if cfg.TokenSecret != "" {
		verifier = token.NewIssuer([]byte(cfg.TokenSecret), time.Minute) // TTL unused on verify
	}

	mux := http.NewServeMux()
	var revocations pep.RevocationChecker
	if cfg.CAEPSecret != "" {
		cache := caep.NewRevocationCache()
		mux.HandleFunc("POST /events", caep.NewReceiver(caep.NewSigner([]byte(cfg.CAEPSecret)), cache).Handler())
		revocations = cache
		log.Info("CAEP receiver enabled at POST /events")
	}

	guard := pep.New(pep.Config{
		Profile:         authzen.Profile(cfg.Profile),
		PEPID:           cfg.PEPID,
		PDP:             pdp,
		Attestor:        attestedInTrustDomain{}, // cryptographic check is the mTLS handshake; this is the SPI seam
		Logger:          log,
		RequirePeerSVID: cfg.RequirePeerSVID,
		TokenVerifier:   verifier,
		Revocations:     revocations,
		Routes:          routes(cfg),
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.Handle("/", guard.Middleware(proxy)) // L1 enforces the allow-listed routes
	return mux, nil
}

func routes(cfg Config) []pep.Route {
	out := make([]pep.Route, 0, len(cfg.Routes))
	for _, r := range cfg.Routes {
		out = append(out, pep.Route{
			Method:        r.Method,
			Path:          r.Path,
			Action:        r.Action,
			ResourceType:  r.ResourceType,
			ResourceProps: r.ResourceProps,
		})
	}
	return out
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	return parseConfig(b)
}

func parseConfig(b []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, errors.New("config: parse: " + err.Error())
	}
	switch authzen.Profile(cfg.Profile) {
	case authzen.ProfileEdge, authzen.ProfileEastWest, authzen.ProfilePartner:
	default:
		return cfg, errors.New("config: profile must be edge|east_west|partner, got " + strconv.Quote(cfg.Profile))
	}
	if cfg.Listen == "" || cfg.Upstream == "" || cfg.PEPID == "" {
		return cfg, errors.New("config: listen, upstream and pep_id are required")
	}
	if len(cfg.Routes) == 0 {
		return cfg, errors.New("config: at least one route is required")
	}
	return cfg, nil
}

// attestedInTrustDomain accepts any SPIFFE id — the cryptographic guarantee is
// the mTLS handshake (the channel already verified the peer cert chains to the
// trust domain). It is the default pip.WorkloadAttestor; swap in a stricter
// implementation (allow-list, SPIRE registration lookup) by embedding the pep
// library directly.
type attestedInTrustDomain struct{}

var _ pip.WorkloadAttestor = attestedInTrustDomain{}

func (attestedInTrustDomain) ValidateSVID(_ context.Context, spiffeID string) (bool, error) {
	return strings.HasPrefix(spiffeID, "spiffe://"), nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
