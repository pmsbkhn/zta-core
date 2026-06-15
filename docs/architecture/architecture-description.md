# Mô tả Kiến trúc — Nền tảng Authorization ZTA

> **Tiêu chuẩn:** ISO/IEC/IEEE 42010:2011 — *Systems and software engineering — Architecture description*
> **Hệ thống (System-of-Interest):** `github.com/pmsbkhn/zta-core` — **nền tảng (platform) Zero Trust
> Authorization tái sử dụng được**, đóng gói thành repo/Go module riêng để import & triển khai vào bất kỳ
> hệ thống nào (Go: thư viện; non-Go: PEP sidecar).
> **Reference adopter:** `github.com/pmsbkhn/authorization-zta` (`examples/vsp` — demo VSP đầu cuối).
> **Phạm vi mã nguồn:** sau mốc M14 + tách lõi (zta-core `v0.1.0`) + PEP sidecar (`v0.2.0`).
> **Trạng thái:** Mô tả kiến trúc *as-built*.

---

## 1. Giới thiệu và phạm vi (Identification)

> **Định vị (đã hiện thực):** Hệ thống mô tả ở đây **là nền tảng ZTA `zta-core`** — repo/Go module riêng
> chứa toàn bộ thành phần tái sử dụng (PEP library, PDP, engine, contract, identity, continuous-evaluation,
> policy store, **PEP sidecar**). Các service nghiệp vụ **VSP (Gateway / Multi-Bill / VSP Wallet)** sống ở
> repo **`authorization-zta/examples/vsp`** như một **reference adopter** chứng minh nền tảng chạy đầu-cuối;
> chúng **không** thuộc lõi. Mục tiêu kiến trúc hàng đầu là **tính tái sử dụng & khả chuyển** sang hệ thống
> khác — đã đạt: `go get github.com/pmsbkhn/zta-core` (Go) hoặc đặt `zta-pep` sidecar (non-Go).

`zta-core` là một **nền tảng** hiện thực hóa các nguyên tắc **NIST Zero Trust Architecture
(SP 800-207)**, phơi bày ra ngoài một facade chuẩn **OpenID AuthZEN 1.0**. Nền tảng định nghĩa và cung
cấp ba mặt phẳng để một hệ thống bất kỳ lắp vào:

- **Data Plane** — caller & workload nghiệp vụ *của hệ thống áp dụng* (trong demo: user, Multi-Bill,
  VSP Wallet, partner). Nền tảng **không** sở hữu các workload này.
- **PEP Layer** — **library/màng thực thi** mà adopter nhúng vào trước mỗi workload (bề mặt áp dụng
  chính). Đây là sản phẩm phân phối quan trọng nhất.
- **Control Plane** — PDP dùng chung, ẩn sau facade AuthZEN, nhúng/kết nối policy engine (OPA, ReBAC) và
  nguồn thông tin (PIP: identity, workload attestation, policy store). Adopter cung cấp **bundle policy
  riêng**, **trust domain riêng**, **triển khai PIP riêng**.

Nền tảng viết bằng **Go 1.26**, phụ thuộc trực tiếp: `open-policy-agent/opa v1.17.1`,
`spiffe/go-spiffe/v2 v2.7.0`, `google.golang.org/grpc v1.81.0`, `minio-go/v7 v7.2.0` (xem `go.mod`).

### 1.0. Lõi nền tảng (`zta-core`) vs Reference adopter (`authorization-zta`)

Lõi đã **tách hẳn sang repo riêng** `github.com/pmsbkhn/zta-core`, mọi package **import được** (không còn
`internal/`). Demo VSP ở repo `authorization-zta` consume qua versioned require.

| Lớp | Phân loại | Repo / gói |
|---|---|---|
| Contract, facade, router, engine adapter, token, PEP, transport, identity, CAEP, policy store, PIP seam | **Lõi nền tảng** (tái sử dụng) | `zta-core`: `authz/{authzen,api,pdp,engine,rebac,token,pep,pdpclient,grpcpdp}`, `identity/spiffe`, `signals/caep`, `policystore/bundlestore`, `ports/pip`, `services` |
| Khung policy (router/schema/lib/profiles) + cơ chế data-driven | **Lõi nền tảng** (Rego framework) | `zta-core`: `policies/{main,global,lib,profiles}.rego` |
| **PEP sidecar** cấu hình bằng YAML (non-Go adopter) | **Lõi nền tảng** (binary) | `zta-core`: `cmd/zta-pep` |
| PDP generic + ops tools | **Lõi nền tảng** (binary) | `zta-core`: `cmd/{pdp,caepemit,bundlepush}` |
| Workload demo + wiring + business policy | **Reference adopter** (thay được) | `authorization-zta`: `examples/vsp/{cmd,app,policies,deploy,scripts}` |
| Giá trị opinionated của demo | **Cấu hình mẫu** (tham số hóa) | `vsp.local`, IDs `ns/<x>/sa/<y>`, profiles `edge/east_west/partner`, ngưỡng 5.000.000đ — đều ở repo demo |

> **Packaging (đã giải quyết):** lõi là Go module độc lập `github.com/pmsbkhn/zta-core`, publish + tag
> (`v0.1.0` lõi, `v0.2.0` thêm sidecar). Hệ thống Go: `go get github.com/pmsbkhn/zta-core` rồi nhúng
> `authz/pep`. Hệ thống non-Go: đặt `cmd/zta-pep` sidecar trước service (xem
> [spec 14](specs/14-pep-sidecar.md)). Rào cản `internal/` của bản PoC đã được gỡ.

### 1.1. Mục đích của tài liệu này

Tài liệu Mô tả Kiến trúc (Architecture Description — AD) này:

1. Xác định **stakeholders** và **concerns** mà kiến trúc phải giải quyết.
2. Định nghĩa các **viewpoint** dùng để khung hóa concerns, và các **view** tương ứng mô tả hệ thống.
3. Ghi lại các **quyết định kiến trúc** (architecture decisions) và lý do (rationale).
4. Thiết lập **correspondence rules** đảm bảo nhất quán giữa các view.
5. Nêu các **concern còn mở / hướng tối ưu**.

### 1.2. Những gì là "thật" và những gì còn "mock" (mức trưởng thành)

| Năng lực | Trạng thái | Ghi chú |
|---|---|---|
| OPA engine nhúng + Rego phân cấp | **Thật** | compile 1 lần, eval in-process |
| Facade AuthZEN 1.0 (HTTP + gRPC) | **Thật** | `POST /access/v1/evaluation`; gRPC `AccessEvaluation.Evaluate` |
| PEP L0/L1/L2 + bubble-up step-up | **Thật** | edge→401, east_west→403 |
| mTLS + SPIFFE X509-SVID mọi chặng nội bộ | **Thật** | lib `go-spiffe/v2` |
| SPIRE control-plane (server/agent, Workload API) | **Thật** | docker-compose; x509pop + k8s_psat + Vault upstream |
| Decision token (HS256, ràng tuple + digest) | **Thật** | fast-path tại PEP, sống sót PDP outage |
| CAEP push thu hồi session | **Thật** | SET RFC 8417 (HS256), PEP deny tức thì |
| Policy bundle store S3 bất biến (WORM) | **Thật** | MinIO object-lock + versioning |
| ReBAC engine (OpenFGA) sau `pdp.Engine` | **Thật** | drop-in cùng interface với OPA |
| In-process CA `svidmint` (thay SPIRE ở demo dev) | **Mock** | chỉ phần *cấp phát* CA; mTLS/SVID là thật |
| IdP enrich subject | **Mock** (`testsupport/mock`) | seam `pip.IdentityProvider` chưa nối hot path |
| Asymmetric signing cho decision token | **Chưa làm** | hiện HS256 đối xứng |

---

## 2. Stakeholders và mối quan tâm (Stakeholders & Concerns)

ISO 42010 yêu cầu AD xác định stakeholders và các concern của họ. Bảng dưới là cơ sở để chọn viewpoint.

### 2.1. Stakeholders

| ID | Stakeholder | Vai trò liên quan kiến trúc |
|---|---|---|
| S1 | **Kỹ sư nền tảng ZTA** (chủ sản phẩm) | Sở hữu lõi nền tảng (PDP, PEP, engine, contract); giữ tính ổn định API và khả tái sử dụng. **Chủ thể chính.** |
| **S7** | **Đội áp dụng / Integrator** (hệ thống khác) | **Stakeholder trọng tâm mới.** Nhúng PEP vào service của họ, dựng PDP, cung cấp bundle policy + trust domain + PIP riêng. Quan tâm: dễ tích hợp, ít phải sửa core, API ổn định, tài liệu rõ. |
| S2 | **Kỹ sư workload demo** (Multi-Bill, Wallet, Gateway) | Reference adopter — minh họa cách S7 tích hợp PEP, propagate identity, xử lý bubble-up. |
| S3 | **Tác giả Policy / Security Analyst** (của adopter) | Viết Rego/ReBAC model **domain riêng**, quản lý `data.json`, vận hành thu hồi session (CAEP). |
| S4 | **Kỹ sư Platform/SRE** (của adopter) | Triển khai SPIRE, mTLS mesh, S3 bundle store, scheduler/lock, observability vào hạ tầng của họ. |
| S5 | **Security Architect / Auditor** | Bảo đảm fail-closed, không giả mạo danh tính, toàn vẹn lịch sử policy, audit trail. |
| S6 | **Đối tác bên ngoài (Partner)** | Gọi qua profile `partner`; quan tâm hợp đồng và biên giới tin cậy. |

### 2.2. Concerns (mối quan tâm)

| ID | Concern | Stakeholder | View khung hóa |
|---|---|---|---|
| C1 | **Tách quyết định khỏi thực thi** (PDP vs PEP) | S1,S2,S5 | Logic, Process |
| C2 | **Fail-closed tuyệt đối** — thiếu allow tường minh = deny | S1,S3,S5 | Logic, Policy-Lifecycle |
| C3 | **Hợp đồng dữ liệu ổn định & engine-agnostic** | S1,S2,S6 | Information, Logic |
| C4 | **Danh tính workload không giả mạo được** (mTLS/SVID) | S2,S4,S5 | Security/Trust, Deployment |
| C5 | **Xử lý step-up xuyên chặng** khi service sâu không có session user | S2,S3 | Process |
| C6 | **Hiệu năng & độ trễ** — không phải request nào cũng chạm PDP | S1,S2,S4 | Process, Logic |
| C7 | **Khả dụng** — PDP outage không làm tê liệt mọi quyết định an toàn | S4,S5 | Process |
| C8 | **Vòng đời policy an toàn** — GitOps, fitness test, version bất biến, rollback | S3,S4,S5 | Policy-Lifecycle |
| C9 | **Đánh giá liên tục** — thu hồi session/posture đè quyết định đã cache | S3,S5 | Process, Security/Trust |
| C10 | **Khả năng mở rộng mô hình quyền** — ABAC/RBAC (OPA) và ReBAC (Zanzibar) cùng tồn tại | S1,S3 | Logic |
| C11 | **Khả năng vận hành** — triển khai SPIRE thật, k8s, secret management | S4 | Deployment |
| C12 | **Khả năng kiểm toán & truy vết** — correlation id, obligation log, audit | S5 | Information, Process |
| **C13** | **Tính tái sử dụng & khả chuyển** — lõi nền tảng import/nhúng được từ hệ thống khác mà không sửa core; lõi tách khỏi demo | **S1,S7** | **Adoption/Packaging**, Logical |
| **C14** | **Dễ áp dụng (onboarding)** — adopter cấu hình bằng tham số/bundle, không phải code; API ổn định, có versioning; tài liệu tích hợp | **S7,S3** | **Adoption/Packaging** |
| **C15** | **Phi-VSP hóa** — trust domain, naming-convention, profiles, ngưỡng nghiệp vụ là tham số chứ không hằng số | **S1,S7** | **Adoption/Packaging**, Information |

---

## 3. Khung kiến trúc & các Viewpoint (Architecture Viewpoints)

AD này định nghĩa **bảy viewpoint**. Mỗi viewpoint khung hóa một tập concern và quy định "model kind"
(loại mô hình) dùng trong view tương ứng.

| Viewpoint | Khung hóa concern | Model kinds |
|---|---|---|
| **VP-1 Context** | C1, C3, C6 | Sơ đồ ngữ cảnh hệ thống; ranh giới tin cậy |
| **VP-2 Logical/Functional** | C1, C2, C3, C10 | Sơ đồ thành phần & interface (port/adapter); bảng trách nhiệm |
| **VP-3 Information (Data Contract)** | C3, C12 | Lược đồ kiểu dữ liệu request/response; naming convention; token claims |
| **VP-4 Process/Runtime** | C1, C5, C6, C7, C9, C12 | Sơ đồ tuần tự (sequence); máy trạng thái phễu L0/L1/L2; bảng outcome→HTTP |
| **VP-5 Security/Trust** | C2, C4, C9 | Mô hình tin cậy SPIFFE; chuỗi PKI; mô hình mối đe dọa rút gọn |
| **VP-6 Policy Lifecycle** | C2, C8, C10 | Pipeline GitOps; cây phân cấp Rego; dòng dữ liệu data-driven |
| **VP-7 Deployment** | C4, C7, C11 | Sơ đồ triển khai (process, container, k8s); bảng cổng/biến môi trường |
| **VP-8 Adoption/Packaging** | C13, C14, C15 | Bản đồ ranh giới core↔demo; danh mục **extension point (SPI)**; ma trận tham số hóa; mô hình phân phối (library/sidecar/daemon) |

---

## 4. Các View kiến trúc (Architecture Views)

### 4.1. View Context (VP-1)

```
                 ┌─────────────────────── Trust Domain: vsp.local ───────────────────────┐
                 │                                                                        │
  User/Device ──http──▶  Gateway          Multi-Bill            VSP Wallet                │
                 │       (Edge PEP) ─mTLS▶ (delegation) ─mTLS▶  (East-West PEP)           │
                 │          :8088            :8081                 :8082                   │
                 │            │                │                     │                     │
                 │            │ AuthZEN        │ AuthZEN             │ AuthZEN             │
  Partner ───────┼──────▶     ▼ (gRPC/mTLS hoặc HTTP)               ▼                     │
                 │        ┌──────────────────────────────────────────────┐               │
                 │        │  Control Plane — PDP (:8080 HTTP / :9090 gRPC) │               │
                 │        │   AuthZEN Facade → Unified Router → Engine     │               │
                 │        │   (OPA nhúng  |  ReBAC/OpenFGA)                 │               │
                 │        └───────┬───────────────┬──────────────┬────────┘               │
                 │   pull bundle  │      PIP seams │   mint token │                        │
                 │                ▼                ▼              ▼                        │
                 │      S3 immutable store   IdP / SPIRE     decision_token                │
                 └────────────────────────────────────────────────────────────────────────┘
   CAEP admin (caepemit) ──SET (mTLS)──▶ PEP /events  (thu hồi/khôi phục session)
```

Ranh giới hệ thống: tất cả nằm trong một **trust domain SPIFFE `vsp.local`**. Mọi chặng nội bộ
(`gateway→multibill`, `multibill→wallet`, `PEP→PDP` gRPC) đều mTLS với SVID do SPIRE cấp. Biên giới
ngoài: User/Device (HTTP, có session) và Partner (profile `partner`).

### 4.2. View Logical/Functional (VP-2)

Hệ thống tuân thủ **Ports & Adapters**: PDP phụ thuộc các *port* (interface), các *adapter* hiện thực
chúng. Điều này là chốt cho C10 (engine-agnostic) và C3 (contract ổn định).

**Bản đồ thành phần ↔ trách nhiệm ↔ port chính:**

| Thành phần (gói) | Trách nhiệm | Port nó phụ thuộc / cung cấp |
|---|---|---|
| `authz/authzen` | VSP Standard Contract: kiểu dữ liệu + validate naming-convention | — (kiểu dùng chung) |
| `authz/api` | Facade AuthZEN 1.0 HTTP; decode/validate→map HTTP status | dùng `Evaluator` (= `pdp.Service`) |
| `authz/grpcpdp` | Facade AuthZEN qua gRPC; client gRPC cho PEP | dùng `Evaluator`; client impl `pep.PDP` |
| `authz/pdp` | **Unified Router**: validate→eval→mint token→assemble response | định nghĩa **port `pdp.Engine`**; dùng `token.Issuer` |
| `authz/engine` | Adapter OPA: compile bundle, eval, fail-closed; `engine.Decision` | impl `pdp.Engine` |
| `authz/rebac` | Adapter ReBAC: gọi OpenFGA `/check`, map quan hệ | impl `pdp.Engine` |
| `authz/token` | Cấp/verify decision token HS256, ràng tuple + digest | cung cấp `Issuer` (impl `pep.TokenVerifier`) |
| `authz/pep` | Phễu L0/L1/L2, fast-path token, revocation, outcome→HTTP | định nghĩa port `PDP`, `TokenVerifier`, `RevocationChecker` |
| `authz/pdpclient` | Adapter HTTP client AuthZEN cho PEP | impl `pep.PDP` |
| `identity/spiffe` | CA in-process, mint SVID, rotation, mTLS `tls.Config`, Workload API | cung cấp `Source` (impl `x509svid/x509bundle.Source`) |
| `signals/caep` | SET ký HS256, transmitter/receiver, `RevocationCache` | `RevocationCache` impl `pep.RevocationChecker` |
| `policystore/bundlestore` | Adapter S3 (MinIO) cho policy bundle | impl `pip.PolicyStore` |
| `ports/pip` | Các seam: `IdentityProvider`, `WorkloadAttestor`, `PolicyStore` | định nghĩa port |
| `testsupport/mock` | Mock của các seam pip (M1) | impl các port pip |
| `services` | Wiring từng process thành `http.Handler` + dựng mTLS/gRPC | tổ hợp tất cả |
| `cmd/*` | Entrypoint mỏng: đọc env → gọi `services.*` | — |

**Bất biến kiến trúc cốt lõi (architectural invariants):**

1. **Một interface quyết định duy nhất** — `pdp.Engine { Eval(ctx, input any) (engine.Decision, error) }`
   (`authz/pdp/pdp.go:22`). Cả OPA (`engine.Engine`) và ReBAC (`rebac.Engine`) thỏa nó →
   thay/ghép engine **không đụng facade hay PEP** (C10).
2. **PEP không tin gì ngầm** — mọi nhánh không-allow-tường-minh đều deny (C2). Token sai → fallback PDP,
   không bao giờ allow ngầm.
3. **Hợp đồng đi qua biên là `authzen.Request/Response`** — HTTP và gRPC chỉ là transport; cùng một
   `pdp.Service.Evaluate` phục vụ cả hai (C3).

### 4.3. View Information / Data Contract (VP-3)

**VSP Standard Contract** (design-v3 §3) ràng buộc naming-convention trên AuthZEN 1.0 để engine phân
loại chính xác. Chi tiết kiểu dữ liệu: [spec 01](specs/01-authzen-contract.md).

- **Subject.Type** ∈ {`user`, `workload`}; **Action.Name** = `<domain>:<action>`;
  **Resource.Type** = `<domain>:<entity>`; **action và resource phải cùng domain**.
- **Context.authz_profile** ∈ {`edge`, `east_west`, `partner`} — khóa định tuyến profile.
- **AAL** (NIST 800-63) ∈ {`AAL1`,`AAL2`,`AAL3`} tại `subject.properties.auth_assurance_level`.
- **Delegation actor** `subject.properties.act` = `{type: workload, id: spiffe://…}` (bắt buộc ở
  `east_west`).
- **Response**: `decision` + `context.{decision_token, obligations[], reason_code}`. Obligation ∈
  {`step_up`, `log`}.

**Decision token claims** (ràng buộc an toàn, [spec 06](specs/06-decision-token.md)):
`sub, act, res(=Type/ID), aal, rd(=digest(resource.properties)), cid, iat, exp`. Digest SHA-256 trên
JSON đã sort khóa → token "low-value" không dùng được cho "high-value" (C6 an toàn).

**Mô hình kép HTTP↔Protobuf**: tại biên nội bộ gRPC, `properties`/`context`/`details` dùng
`google.protobuf.Struct` → không cần schema cứng cho thuộc tính nghiệp vụ (xem
[spec 08](specs/08-grpc-transport.md)).

### 4.4. View Process / Runtime (VP-4)

#### 4.4.1. Phễu đánh giá tại PEP (L0/L1/L2) — design-v3 §2

```
inbound request
   │
   ▼ L0  Channel/Peer  (chỉ east_west): lấy SPIFFE id từ peer cert mTLS đã verify
   │        └─ no peer SVID → DropL0 ("l0_no_peer_svid")  |  not attested → "l0_peer_not_attested"
   ▼ L1  Route guard: (method,path) khớp Route? không → DenyRoute ("l1_route_not_permitted")
   │
   ▼ (M11) Revocation: subject bị thu hồi? → DenyForbidden ("session_revoked")  ◀── đè cả fast-path
   │
   ▼ L2  Resource/Action:
   │        ├─ (M5) fast-path: X-Decision-Token verify + khớp tuple+digest + AAL≥ → Allow (bỏ PDP)
   │        └─ else: gọi PDP.Evaluate  (lỗi/non-200 → DenyForbidden "l2_pdp_unavailable", fail-closed)
   ▼ classify(resp): allow → Allow(+token) | step_up obligation → DenyStepUp(acr) | else DenyForbidden
```

Thứ tự **revocation trước fast-path** là điểm mấu chốt cho C9: quyết định đã cache (token còn hạn) vẫn
bị thu hồi đè (`authz/pep/enforce.go`).

#### 4.4.2. Outcome → HTTP, phân biệt theo profile (bubble-up §4)

| Outcome | edge | east_west / partner |
|---|---|---|
| `Allow` | 200, forward; echo `X-Decision-Token` | 200, forward; echo token |
| `DenyStepUp` | **401** challenge JSON `{step_up_required, required_acr, method:mfa}` + header `X-Step-Up-Required` | **403** + header `X-Step-Up-Required` (không challenge — không có session user) |
| `DropL0` | 403 `{error}` | 403 `{error}` |
| `DenyRoute`/`DenyForbidden` | 403 `{error}` | 403 `{error}` |

**Bubble-up đầu cuối** (đã verify live + test): Wallet (sâu) deny step-up → **403 + `X-Step-Up-Required:
AAL3`** → Multi-Bill dội ngược → Gateway (Edge) dịch thành **401** để UI bật MFA → user lên AAL3 →
retry → 200. (C5)

#### 4.4.3. Đường ra quyết định ở Control Plane

```
api/grpcpdp facade
   └─ pdp.Service.Evaluate(req)
        1. req.Validate()          → ValidationError ⇒ HTTP 400 / gRPC InvalidArgument
        2. toInput(req)            → map[string]any (JSON roundtrip)
        3. engine.Eval(input)      → engine.Decision  (OPA: funnel Rego;  ReBAC: OpenFGA /check)
        4. assemble: nếu Allow ⇒ token.Issue(claims) gắn decision_token; map obligations; reason_code
```

#### 4.4.4. Các thuộc tính runtime cho C6/C7 (hiệu năng & khả dụng)

- **Compile-once**: OPA bundle compile thành `rego.PreparedEvalQuery` lúc khởi động; eval in-process,
  an toàn concurrency (`authz/engine/opa.go`).
- **Fast-path token**: PEP bỏ hẳn round-trip PDP cho request giống hệt trong TTL (C6).
- **Sống sót PDP outage**: token còn hạn → PEP vẫn allow đúng request (đã verify: tắt PDP, settle giống
  hệt vẫn 200) (C7).
- **Token caching ở Multi-Bill**: cache theo tuple `subject|aal|resource|body` để replay token tới
  Wallet (`services/multibill.go`).

### 4.5. View Security / Trust (VP-5)

**Mô hình danh tính kép:**

- **Danh tính workload** = mật mã: X509-SVID (URI SAN `spiffe://vsp.local/ns/<ns>/sa/<sa>`), xác lập
  qua **mTLS handshake**. Delegation actor `act` lấy từ **peer cert đã verify**, *không* từ header
  (C4). Header `X-Vsp-Caller-Spiffe` chỉ dùng ở dev-mode (`RequirePeerSVID=false`).
- **Danh tính user** = `subject`/`aal` propagate qua header (production sẽ là signed token).

**Chuỗi tin cậy PKI (qua các mốc):**

```
Org/Vault Root CA ──(UpstreamAuthority)──▶ SPIRE intermediate ──▶ X509-SVID mỗi workload
  M8: UpstreamAuthority "disk" (nodecert sinh upstream-root)
  M12: UpstreamAuthority "vault" (Vault PKI sign-intermediate)  → SVID chain về Vault root
Node attestation: M8 x509pop (node cert)  |  M13 k8s_psat (projected SA token → TokenReview)
```

**Bảo đảm đã verify:** client không SVID → rớt ngay tại **TLS handshake (L0)**, không chạm PEP/PDP;
cert ký bởi CA lạ → reject ở handshake (foreign-CA test, gRPC mTLS test).

**Đánh giá liên tục (C9):** CAEP/SSF — Control Plane ký SET (HS256, kiểu RFC 8417) `session-revoked`/
`session-restored`, push tới PEP `/events` (mTLS); PEP cập nhật `RevocationCache` và **deny ngay**, đè
token còn hạn. Tamper/forge SET bị từ chối (constant-time HMAC compare).

**Fail-closed (C2)** ở mọi tầng: validate lỗi, policy undefined, domain lạ, schema sai, input không
phải map, PDP unavailable, token mismatch — tất cả → deny.

### 4.6. View Policy Lifecycle (VP-6)

**Cây Rego phân cấp** (`policies/`, [spec 04](specs/04-engine-opa-policies.md)) — KHÔNG viết policy theo
từng PEP, mà định tuyến theo payload:

```
main.rego (vsp.authz.decision)  — fail-closed default {allow:false, reason_code:"default_deny"}
  ├─ Gate 1  global/schema.rego     — validate naming-convention (defense-in-depth)
  ├─ Gate 2  lib + data.json        — required_attributes theo action (data-driven §5.2)
  ├─ Gate 3  profiles/profiles.rego — invariant theo chặng (edge cần source_ip; east_west cần act spiffe; partner cần partner_id)
  └─ Gate 4  domain/<domain>.rego   — dynamic dispatch: data.vsp.domain[split(resource.type,":")[0]].verdict
```

Vi phạm ở bất kỳ gate nào → chặn trước khi vào nghiệp vụ. Thêm domain mới = thêm 1 file Rego (router
không đổi). Thêm yêu cầu thuộc tính = sửa `data.json` (không chạm code).

**Logic nghiệp vụ then chốt** (ví dụ tham chiếu): `wallet:settle` > **5.000.000 VND** đòi **AAL3**;
chưa đủ → obligation `step_up(AAL3)`; ≤ ngưỡng → AAL2 đủ. `bill:pay` đòi AAL2.

**GitOps + immutable store (§5.3, C8):**

```
Git (Rego + data.json) ──PR──▶ opa test (fitness, gate) ──▶ opa build (bundle.tar.gz)
   ──▶ bundlepush → S3 (object-lock GOVERNANCE + versioning, WORM)  ──▶ PDP pull (NewFromBundle)
```

Republish = **version mới**, bản cũ giữ nguyên → rollback/retry an toàn. PDP chọn nguồn theo
`S3_ENDPOINT`: có → pull bundle S3; không → dùng bundle embed trong binary.

### 4.7. View Deployment (VP-7)

Chi tiết: [spec 12](specs/12-services-and-cmd.md), [spec 13](specs/13-deployment-topology.md).

**Tô-pô tham chiếu (docker-compose, trust domain `vsp.local`):**

| Process | Cổng | UID | SPIFFE ID | mTLS |
|---|---|---|---|---|
| `pdp` | 8080 HTTP, 9090 gRPC | 10004 | `…/ns/pdp/sa/pdp-svc` | gRPC mTLS in |
| `wallet` (East-West PEP) | 8082 | 10001 | `…/ns/wallet/sa/vsp-wallet-svc` | server in, gRPC→PDP |
| `multibill` (delegation) | 8081 | 10002 | `…/ns/billing/sa/multi-bill-svc` | server in + client out |
| `gateway` (Edge PEP) | 8088 (public) | 10003 | `…/ns/edge/sa/api-gateway` | client out, gRPC→PDP |
| `spire-server` / `spire-agent` | — | 0 | trust anchor + Workload API socket | — |

**Biến môi trường chính** (đầy đủ ở spec 12): `PDP_ADDR`, `PDP_GRPC_ADDR`, `PDP_TOKEN_SECRET`,
`PDP_TOKEN_TTL` (300s), `PDP_URL`, `MULTIBILL_URL`, `WALLET_URL`, `CAEP_SECRET`,
`SPIFFE_ENDPOINT_SOCKET`, `SVID_{CERT,KEY,BUNDLE,TRUST_DOMAIN}`, `S3_{ENDPOINT,ACCESS_KEY,SECRET_KEY,
BUCKET,OBJECT}`.

**Biến thể triển khai:** docker-compose (x509pop + UpstreamAuthority disk); `deploy/vault` (Vault PKI
upstream); `deploy/k8s` (k3d + k8s_psat); `deploy/rebac` (OpenFGA). Image: distroless static, 5 binary
daemon (pdp/gateway/multibill/wallet/caepemit).

### 4.8. View Adoption / Packaging (VP-8)

View này mô tả nền tảng *từ góc nhìn đội áp dụng (S7)*: họ lấy gì, cắm gì, cấu hình gì.

#### 4.8.1. Mô hình phân phối (đề xuất, theo bề mặt áp dụng)

```
Adopter system
  ├─ [import library]  PEP  ──────────▶ nhúng middleware vào HTTP handler của họ (bề mặt #1)
  │                     pdpclient/grpcpdp ──▶ trỏ tới PDP dùng chung
  ├─ [run daemon]      PDP generic ────▶ nạp bundle policy CỦA HỌ từ S3 (không embed)
  ├─ [provide]         bundle Rego domain + data.json + OpenFGA model  (policy nghiệp vụ của họ)
  ├─ [provide]         trust domain + SPIRE entries + PIP impl (IdentityProvider/WorkloadAttestor)
  └─ [configure]       profiles, naming-convention, token TTL/secret, ngưỡng — qua tham số
```

Nền tảng cung cấp: **lõi tái sử dụng** (mục [§1.0](#10-lõi-nền-tảng-vs-reference-adopter-ranh-giới)) +
**khung Rego** (`main/global/lib/profiles`). Adopter cung cấp: **domain policy + danh tính + hạ tầng +
cấu hình**.

#### 4.8.2. Danh mục Extension Point (SPI) — hợp đồng adopter cắm vào

| SPI (port) | Vị trí | Adopter cung cấp gì | Bản mẫu sẵn |
|---|---|---|---|
| `pdp.Engine` | `authz/pdp` | Engine quyết định (ABAC/ReBAC/khác) | OPA (`engine`), ReBAC (`rebac`) |
| `pep.PDP` | `authz/pep` | Cách PEP gọi PDP (transport) | HTTP (`pdpclient`), gRPC (`grpcpdp`) |
| `pep.TokenVerifier` | `authz/pep` | Xác minh decision token | `token.Issuer` |
| `pep.RevocationChecker` | `authz/pep` | Nguồn trạng thái thu hồi | `caep.RevocationCache` |
| `pip.IdentityProvider` | `ports/pip` | Enrich subject (roles/posture) | `mock` (cần bản thật) |
| `pip.WorkloadAttestor` | `ports/pip` | Validate SVID workload | `mock` (cần SPIRE thật) |
| `pip.PolicyStore` | `ports/pip` | Nguồn bundle policy | S3 (`bundlestore`), `mock` |
| `caep.Sink` | `signals/caep` | Tiêu thụ sự kiện liên tục | `RevocationCache` |
| Identity `Source` | `identity/spiffe` | Nguồn SVID/bundle | Workload API (SPIRE), Rotating (dev) |

> Các interface này nay **public và import được** từ `github.com/pmsbkhn/zta-core/...` (đã ra khỏi
> `internal/`, versioned từ `v0.1.0`) — là API mở rộng chính thức để adopter cắm triển khai riêng.

#### 4.8.3. Ma trận tham số hóa (cần "phi-VSP hóa" — C15)

| Hạng mục opinionated | Hiện trạng | Cần thành |
|---|---|---|
| Trust domain `vsp.local` | hằng số rải rác / env `SVID_TRUST_DOMAIN` | tham số nền tảng nhất quán |
| Profiles `edge/east_west/partner` + map outcome→HTTP | hằng số trong `authzen`/`pep` | cấu hình mở rộng được (adopter thêm profile) |
| Naming-convention `<domain>:<action>` | regex cứng (Go + Rego) | policy/khung có thể nới hoặc thay |
| Routes/ResourceProps của PEP | khai báo trong Go (`services`) | khai báo bằng file cấu hình (PEP generic) |
| Ngưỡng nghiệp vụ (5.000.000đ) | hằng số trong `domain/wallet.rego` (demo) | thuộc bundle adopter / `data.json` |
| Decision token secret/TTL | env, mặc định dev | bắt buộc cấu hình + (backlog) asymmetric |

#### 4.8.4. Mức độ sẵn sàng nền tảng — *cập nhật sau khi tách repo + sidecar*

- **Tách concern lân cận**: ✓ gom nhóm tường minh trong `zta-core`: `authz` (lõi 3 chặng),
  `identity/spiffe` (workload authentication), `signals/caep` (continuous eval), `policystore`,
  `ports` (SPI), `testsupport`.
- **Tách demo khỏi lõi**: ✓ reference adopter VSP chuyển hẳn sang repo `authorization-zta/examples/vsp`;
  lõi không còn tham chiếu workload nghiệp vụ.
- **Tách policy framework↔domain**: ✓ `policies/` chỉ còn khung; domain do adopter cấp qua
  `PDPConfig.ExtraModules`/`ExtraData` (in-process) hoặc bundle S3.
- **Importability**: ✓ lõi là Go module riêng `github.com/pmsbkhn/zta-core`, mọi package public,
  publish + tag (`v0.1.0`). `go get` rồi nhúng `authz/pep`.
- **PEP cấu-hình-bằng-file**: ✓ `cmd/zta-pep` — sidecar đọc routes/profile/upstream/PDP từ YAML, cho
  service non-Go (xem [spec 14](specs/14-pep-sidecar.md)).
- **API stability/versioning**: ◐ đã có SemVer qua git tag (`v0.1.0`, `v0.2.0`); chưa có cam kết
  ổn định/experimental theo từng package.
- **Adoption guide**: ◐ README `zta-core` mô tả cách import + dùng sidecar; chưa có guide đầy đủ.
- **Phi-VSP hóa tham số** (C15): ◐ sidecar tham số hóa routes/profile/secret/upstream; trust domain qua
  env; *còn lại*: profiles và naming-convention vẫn là hằng số trong code (mở rộng được = backlog).

---

## 5. Quyết định kiến trúc & lý do (Architecture Decisions)

Định dạng ADR rút gọn. Mỗi quyết định ↔ concern và đánh đổi.

| ADR | Quyết định | Lý do / Concern | Đánh đổi (cần tối ưu) |
|---|---|---|---|
| **AD-1** | Facade AuthZEN 1.0 ẩn engine nội bộ | Hợp đồng chuẩn, engine-agnostic (C3,C10) | Thêm 1 lớp dịch; properties tự do → kiểm soát qua naming-convention thay vì schema cứng |
| **AD-2** | Một port `pdp.Engine` cho mọi engine | Ghép OPA + ReBAC không đụng facade/PEP (C10) | `Decision` phải đủ tổng quát cho mọi engine; ReBAC chưa trả obligation |
| **AD-3** | OPA nhúng làm thư viện, compile-once | Độ trễ thấp, eval in-process (C6) | Bundle gắn vòng đời với binary trừ khi pull S3 (đã có) |
| **AD-4** | Rego phân cấp + data-driven `data.json` | Không "vỡ trận" khi nhiều PEP (C8); thêm service không chạm code | Cần kỷ luật cấu trúc; fitness test là chốt chặn |
| **AD-5** | Fail-closed ở mọi tầng | Zero Trust (C2) | Lỗi cấu hình dễ gây deny im lặng → cần observability tốt |
| **AD-6** | Phễu L0/L1/L2 ở PEP | Không đẩy mọi thứ lên PDP; rẻ→đắt (C6) | Logic PEP phức tạp hơn; phải đồng bộ ngữ nghĩa với Rego |
| **AD-7** | mTLS + SPIFFE-SVID cho danh tính workload | Không giả mạo được; `act` từ cert (C4) | Phụ thuộc SPIRE; vận hành PKI/rotation phức tạp (C11) |
| **AD-8** | Decision token HS256 ràng tuple+digest, fast-path | Hiệu năng + sống sót outage (C6,C7) | **Đối xứng** → secret dùng chung PDP/PEP; chưa có asymmetric (backlog) |
| **AD-9** | Bubble-up step-up, edge→401 / east_west→403 | Service sâu không có session user (C5) | Phải truyền header trung thực qua mọi chặng |
| **AD-10** | CAEP push (SET) thay vì poll | Đánh giá liên tục tức thì (C9) | Cache trong RAM → mất khi PEP restart; best-effort fan-out |
| **AD-11** | S3 immutable (WORM) cho bundle | Toàn vẹn lịch sử, rollback (C8) | Cần object-lock + versioning ở hạ tầng |
| **AD-12** | gRPC + `protobuf.Struct` cho luồng nội bộ | Tối ưu serialization/latency (C6); không schema cứng | Mất kiểm tra kiểu tĩnh cho thuộc tính nghiệp vụ |
| **AD-13** | ~~Lõi trong `internal/`, demo+core chung module~~ → **đã gỡ**: tách lõi sang repo/Go module riêng `github.com/pmsbkhn/zta-core` (mọi package public), demo ở `authorization-zta/examples/vsp` | Tái sử dụng được từ hệ thống khác; versioning rõ (C13/C14) | Vận hành 2 repo + publish/tag; consumer Go kéo cả dep graph (sidecar giải cho non-Go) |
| **AD-14** | Khung Rego (core) tách khỏi domain nghiệp vụ; adopter cấp domain qua `PDPConfig.ExtraModules`/`ExtraData` hoặc bundle S3 | Adopter thay domain không sửa core (C8/C13) | Cần một bước hợp nhất modules khi compile in-process |
| **AD-15** | **PEP sidecar `cmd/zta-pep`** cấu hình bằng YAML cho adopter non-Go | "Triển khai vào bất kỳ hệ thống nào" gồm Java/Node/Python (C14) | Một network hop nữa; cấu hình ngoài code phải được quản trị/secure |

---

## 6. Tính nhất quán & quy tắc tương ứng (Correspondences)

ISO 42010: các view phải nhất quán qua **correspondence rules**. Những ràng buộc bắc cầu giữa các view:

| ID | Quy tắc tương ứng | View liên quan | Cơ chế thực thi |
|---|---|---|---|
| CR-1 | Naming-convention được kiểm **hai lần**: ở Go (`authzen.Validate`) và trong Rego (`global/schema.rego`) | Information ↔ Policy-Lifecycle | Defense-in-depth; cả hai dùng cùng regex colon-pair |
| CR-2 | `engine.Decision` (Logic) ↔ `authzen.Response` (Information) | Logic ↔ Information | `pdp.assemble` map 1-1: Allow→token, Obligations, ReasonCode |
| CR-3 | Outcome PEP (Process) ↔ obligation từ PDP (Information) | Process ↔ Information | `pep.classify` đọc obligation `step_up` → `DenyStepUp` |
| CR-4 | Decision token claims (Information) ↔ điều kiện fast-path (Process) | Information ↔ Process | PEP `tryDecisionToken` so khớp sub/act/res/rd/aal y hệt cách PDP mint |
| CR-5 | SPIFFE ID đăng ký (Deployment) ↔ `act` kỳ vọng ở Rego (Policy) | Deployment ↔ Policy-Lifecycle | `profiles.rego` đòi `act` là workload có `spiffe://` id hợp lệ |
| CR-6 | `required_attributes` (data.json) ↔ `ResourceProps` PEP lift từ body | Policy ↔ Process | PEP nâng đúng field (amount, currency) Rego sẽ kiểm |
| CR-7 | Profile (Information) ↔ map outcome→HTTP (Process) | Information ↔ Process | `edge` challenge 401; `east_west`/`partner` bubble 403 |
| CR-8 | Engine interchangeability (Logic) ↔ wiring (Deployment) | Logic ↔ Deployment | `pdp.New(engine, issuer)` nhận OPA hoặc ReBAC như nhau |

**Vùng dễ lệch (cần canh khi tối ưu):** ngữ nghĩa step-up giữa PEP (`pep.classify`) và Rego
(`wallet.rego` phát obligation) — nếu đổi reason_code hay cấu trúc obligation phải sửa cả hai
(CR-3); và cách tính digest token phải đồng nhất tuyệt đối giữa PDP và PEP (CR-4).

---

## 7. Concerns còn mở & hướng tối ưu (Architecture Backlog)

Liên kết tới concern để ưu tiên. Đây là đầu vào trực tiếp cho mục tiêu "có hướng tối ưu".

### 7.0. Packaging & ranh giới áp dụng — *phần lớn ĐÃ XONG*

Đã thực hiện (C13/C14): ✓ tách lõi sang repo/module riêng `zta-core`, mọi package import được, publish +
tag `v0.1.0`; ✓ tách khung policy khỏi domain (adopter cấp domain qua `ExtraModules`/bundle); ✓
**PEP sidecar `cmd/zta-pep`** cho adopter non-Go (`v0.2.0`); ✓ demo VSP tách sang `authorization-zta`.

Còn lại:
1. **Cam kết API stability theo package** — đánh dấu rõ package nào ổn định / experimental; cân nhắc
   tách multi-module (PEP-light vs PDP-heavy) nếu muốn consumer Go consume-only không kéo OPA/minio. *C14.*
2. **Phi-VSP hóa nốt** (C15): profiles và naming-convention hiện là hằng số trong code → cho phép adopter
   thêm profile / nới convention qua cấu hình (xem ma trận [§4.8.3](#483-ma-trận-tham-số-hóa-cần-phi-vsp-hóa--c15)).
3. **Adoption guide đầy đủ** — hướng dẫn tích hợp đầu-cuối (nhúng PEP / dựng PDP / cấp bundle+PIP+trust
   domain / đặt sidecar), kèm ví dụ non-Go.

### 7.1. Bảo mật / Tin cậy
- **Asymmetric decision token** (AD-8): PDP ký private key, PEP verify public key — bỏ secret dùng
  chung. *Concern C4/C7.* (đã nằm trong next-steps README).
- **IdP enrich subject vào hot path** (C12): `pip.IdentityProvider` còn mock; nối thật để Rego dùng
  roles/entitlements/posture thay vì chỉ AAL từ header.
- **Revocation cache bền vững** (AD-10): hiện in-RAM, mất khi PEP restart; cân nhắc store chia sẻ
  (Redis) hoặc rehydrate từ transmitter khi khởi động. *C9.*
- **Posture/continuous signals** ngoài revocation (device posture) qua cùng kênh CAEP.

### 7.2. Hiệu năng / Khả dụng
- **Negative caching / circuit breaker** tại PEP khi PDP outage kéo dài (hiện chỉ fail-closed mỗi
  request). *C7.*
- **Đo độ phủ fast-path** (tỉ lệ request bỏ qua PDP) — thêm metric để tối ưu TTL token. *C6.*
- **Batch/streaming evaluation** AuthZEN (nhiều quyết định/round-trip) cho luồng nội bộ. *C6.*

### 7.3. Vận hành
- **HA cho SPIRE server + secret management** (Vault token hiện dev-mode). *C11.*
- **Observability**: chuẩn hóa structured log + thêm metric/trace (correlation id đã có; cần exporter).
  *C12.*
- **PEP pull bundle trực tiếp** (hiện chỉ PDP pull S3); sidecar PEP tự cập nhật. *C8.*

### 7.4. Mô hình quyền
- **ReBAC trả obligation & compose với OPA** (AD-2): hiện ReBAC chỉ allow/deny thuần quan hệ; chưa phát
  obligation hay kết hợp ABAC + ReBAC trong cùng quyết định. *C10.*
- **Quản lý OpenFGA model như policy GitOps** (song song bundle Rego). *C8/C10.*

### 7.5. Hợp đồng
- **Kiểm kiểu thuộc tính nghiệp vụ** (AD-1/AD-12): `properties`/`Struct` tự do → cân nhắc schema
  registry/validation tuỳ chọn cho các action quan trọng. *C3.*

---

## 8. Tham chiếu

- ISO/IEC/IEEE 42010:2011 — Architecture description.
- NIST SP 800-207 — Zero Trust Architecture; NIST SP 800-63 — Digital Identity (AAL).
- OpenID AuthZEN 1.0; CloudEvents; RFC 8417 (Security Event Token); SPIFFE/SPIRE.
- [`docs/design-v3.md`](../design-v3.md) — thiết kế gốc; [`README.md`](../../README.md) — milestone log.
- Tech spec từng module: [`specs/`](specs/).
