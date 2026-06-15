package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pmsbkhn/zta-core/authz/authzen"
	"github.com/pmsbkhn/zta-core/authz/pep"
)

// fakePDP returns a canned decision, standing in for a real PDP.
type fakePDP struct {
	resp authzen.Response
	err  error
}

func (f fakePDP) Evaluate(context.Context, authzen.Request) (authzen.Response, error) {
	return f.resp, f.err
}

func eastWestCfg(upstream string) Config {
	return Config{
		Listen: ":0", Profile: "east_west", PEPID: "test-sidecar", Upstream: upstream,
		Routes: []struct {
			Method        string   `json:"method"`
			Path          string   `json:"path"`
			Action        string   `json:"action"`
			ResourceType  string   `json:"resource_type"`
			ResourceProps []string `json:"resource_props"`
		}{{Method: "POST", Path: "/settle", Action: "wallet:settle", ResourceType: "wallet:account", ResourceProps: []string{"amount", "currency"}}},
	}
}

func settle(t *testing.T, base, aal string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, base+"/settle", bytes.NewReader([]byte(`{"amount":1,"currency":"VND"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(pep.HeaderSubjectID, "u-1")
	req.Header.Set(pep.HeaderAAL, aal)
	req.Header.Set(pep.HeaderCallerSpiffe, "spiffe://vsp.local/ns/billing/sa/multi-bill-svc") // L0 dev fallback
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestParseConfig(t *testing.T) {
	ok := []byte("listen: \":8080\"\nprofile: east_west\npep_id: x\nupstream: http://u:9000\nroutes:\n  - {method: POST, path: /p, action: \"a:b\", resource_type: \"a:c\"}\n")
	if _, err := parseConfig(ok); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	bad := []byte("listen: \":8080\"\nprofile: bogus\npep_id: x\nupstream: http://u:9000\nroutes: [{method: GET, path: /, action: a:b, resource_type: a:c}]\n")
	if _, err := parseConfig(bad); err == nil {
		t.Fatal("expected error for invalid profile")
	}
	missing := []byte("profile: edge\nroutes: [{method: GET, path: /, action: a:b, resource_type: a:c}]\n")
	if _, err := parseConfig(missing); err == nil {
		t.Fatal("expected error for missing listen/upstream/pep_id")
	}
}

func TestHandler_AllowProxiesToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("upstream-ok"))
	}))
	defer upstream.Close()

	h, err := handler(eastWestCfg(upstream.URL), fakePDP{resp: authzen.Response{Decision: true, Context: &authzen.ResponseContext{}}}, slog.Default())
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp := settle(t, srv.URL, "AAL3")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("allow: expected 200, got %d", resp.StatusCode)
	}
	if b, _ := io.ReadAll(resp.Body); string(b) != "upstream-ok" {
		t.Fatalf("expected proxied upstream body, got %q", b)
	}
}

func TestHandler_StepUpBubblesUp403(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("upstream must not be reached on a denied request")
	}))
	defer upstream.Close()

	deny := authzen.Response{Decision: false, Context: &authzen.ResponseContext{
		ReasonCode:  "step_up_required",
		Obligations: []authzen.Obligation{{Type: authzen.ObligationStepUp, Details: map[string]any{"required_acr": "AAL3"}}},
	}}
	h, _ := handler(eastWestCfg(upstream.URL), fakePDP{resp: deny}, slog.Default())
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp := settle(t, srv.URL, "AAL2")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("east_west step-up: expected 403, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get(pep.HeaderStepUpRequired); got != "AAL3" {
		t.Fatalf("X-Step-Up-Required = %q, want AAL3", got)
	}
}

func TestHandler_EdgeTranslatesUpstreamStepUpTo401(t *testing.T) {
	// The deeper service bubbled a step-up up in its response header; the edge
	// sidecar must translate that into a 401 MFA challenge.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(pep.HeaderStepUpRequired, "AAL3")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer upstream.Close()

	cfg := Config{
		Listen: ":0", Profile: "edge", PEPID: "edge-sidecar", Upstream: upstream.URL,
		Routes: []struct {
			Method        string   `json:"method"`
			Path          string   `json:"path"`
			Action        string   `json:"action"`
			ResourceType  string   `json:"resource_type"`
			ResourceProps []string `json:"resource_props"`
		}{{Method: "POST", Path: "/pay", Action: "bill:pay", ResourceType: "bill:invoice", ResourceProps: []string{"amount", "currency"}}},
	}
	h, _ := handler(cfg, fakePDP{resp: authzen.Response{Decision: true, Context: &authzen.ResponseContext{}}}, slog.Default())
	srv := httptest.NewServer(h)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/pay", bytes.NewReader([]byte(`{"amount":9000000,"currency":"VND"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(pep.HeaderSubjectID, "u-1")
	req.Header.Set(pep.HeaderAAL, "AAL2")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("edge translate: expected 401, got %d", resp.StatusCode)
	}
}
