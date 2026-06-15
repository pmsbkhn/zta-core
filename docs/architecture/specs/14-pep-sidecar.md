# Spec 14 — PEP Sidecar (`zta-pep`)

**Gói:** `cmd/zta-pep` (zta-core)
**Vai trò:** Đưa enforcement quyền tới **service non-Go** (Java/Node/Python…): một reverse proxy cấu hình
bằng YAML đặt trước service, chạy đúng phễu PEP của nền tảng mà không cần nhúng code Go. Khung hóa **C14**
(dễ áp dụng, mọi hệ thống) và **C13** (tái sử dụng).
**View liên quan:** Adoption/Packaging (VP-8), Process (VP-4), Deployment (VP-7).

---

## 1. Mô hình

```
client ──▶ zta-pep (sidecar)  ──proxy──▶  upstream service (bất kỳ ngôn ngữ)
                │ L0/L1/L2 + fast-path + revocation
                ▼
              PDP (gRPC/mTLS hoặc HTTP)
```

Adopter **không sửa code service**: đặt sidecar trong request path (cùng pod / cạnh service), khai báo
routes + PDP + upstream trong file. Sidecar là `pep` library + reverse proxy đóng gói thành binary —
generalize `examples/vsp` gateway/wallet thành cấu-hình-thay-vì-code.

## 2. Cấu hình (YAML)

```yaml
listen: ":8080"
profile: east_west            # edge | east_west | partner
pep_id: "orders-sidecar"
upstream: "http://127.0.0.1:9000"
require_peer_svid: true       # true → serve mTLS (SVID từ SPIRE/SVID_*); false → HTTP
pdp:
  grpc_addr: "pdp:9090"       # ưu tiên; hoặc http_url
  # http_url: "http://pdp:8080"
token_secret: ""              # optional: bật decision-token fast-path (khớp HS256 với PDP)
caep_secret:  ""              # optional: bật CAEP receiver tại POST /events
routes:
  - { method: POST, path: /orders, action: "orders:create", resource_type: "orders:order", resource_props: [amount, currency] }
```

Đọc qua `sigs.k8s.io/yaml` (YAML→JSON→struct). `parseConfig` validate: profile hợp lệ; `listen`,
`upstream`, `pep_id` bắt buộc; ≥1 route. Đường dẫn config: cờ `-config` hoặc env `ZTA_PEP_CONFIG`
(mặc định `/etc/zta/pep.yaml`).

## 3. Hành vi (cmd/zta-pep/main.go)

- `buildPDP`: `pdp.grpc_addr` → `services.PDPGRPCClient` (mTLS); else `pdp.http_url` → `pdpclient.New`.
  Cả hai impl `pep.PDP`.
- `handler`: dựng `pep.New` với `Routes` từ config + `Attestor` mặc định `attestedInTrustDomain` (chấp nhận
  mọi `spiffe://` — bảo đảm mật mã là **mТLS handshake**; đây là seam, thay bằng attestor chặt hơn nếu cần);
  `TokenVerifier` nếu có `token_secret`; CAEP receiver + `RevocationChecker` nếu có `caep_secret`.
  Mux: `GET /healthz`, `POST /events` (nếu CAEP), còn lại `/` → `guard.Middleware(proxy)` (L1 chặn route
  ngoài danh sách).
- **Edge translation**: `profile: edge` → `proxy.ModifyResponse` dịch `X-Step-Up-Required` dội lên từ
  upstream thành **401** challenge (giống gateway demo). `east_west`/`partner` → giữ 403 bubble-up.
- **Serve**: `require_peer_svid: true` → `services.LoadServerTLS()` (fatal nếu không có SVID) → mTLS;
  ngược lại HTTP. Fail-closed: lỗi/non-200 từ PDP → deny.

## 4. Kiểm thử

- `cmd/zta-pep/main_test.go`: `parseConfig` (hợp lệ + profile sai + thiếu field); allow → proxy tới
  upstream (200); east_west step-up → 403 + `X-Step-Up-Required`; edge dịch step-up upstream → 401.
- Tách `handler()` khỏi `run()` (serve) để test bằng `httptest` với fake PDP.

## 5. Triển khai

```
go install github.com/pmsbkhn/zta-core/cmd/zta-pep@v0.2.0
zta-pep -config /etc/zta/pep.yaml
```
mTLS: đặt `SPIFFE_ENDPOINT_SOCKET` (SPIRE agent) hoặc `SVID_CERT/KEY/BUNDLE`; gRPC tới PDP cũng dùng SVID
đó. Mẫu cấu hình: `cmd/zta-pep/config.example.yaml`.

## 6. Hướng tối ưu / giới hạn

- **Attestor mặc định chấp nhận mọi `spiffe://`** (tin vào mТLS handshake). Cần allow-list / tra cứu
  registration SPIRE thì thay attestor (hiện phải fork binary; có thể mở thành plugin/cấu hình).
- **Profiles/naming-convention** vẫn theo nền tảng (edge/east_west/partner) — chưa cho adopter tự định
  nghĩa profile qua config. *C15.*
- **Upstream luôn HTTP plain** (giả định cùng pod/localhost); chưa hỗ trợ mTLS tới upstream.
- Chưa **hot-reload** config (đổi routes phải restart sidecar).
- Một network hop thêm so với nhúng library — đánh đổi của AD-15.
