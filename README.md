# zta-core

**Nền tảng Zero Trust Authorization tái sử dụng** — lõi quyết định (PDP) và thực thi (PEP) theo chuẩn
**OpenID AuthZEN 1.0** + **NIST ZTA**, đóng gói để **import/triển khai vào bất kỳ hệ thống nào**.

Đây là lõi đã tách khỏi repo gốc `authorization-zta` (nay là *reference adopter* VSP). Mọi quyết định
và thực thi quyền tại 3 chặng (`edge` / `east_west` / `partner`) nằm ở đây; nghiệp vụ cụ thể do hệ thống
áp dụng cung cấp (domain policy + trust domain + PIP).

```
go get github.com/pmsbkhn/zta-core
```

## Bố cục (mọi package đều import được — không nằm trong internal/)

```
authz/            LÕI authorization 3 chặng
  authzen/        Hợp đồng AuthZEN (VSP Standard Contract) + validate
  api/            Facade AuthZEN 1.0 (HTTP)
  pdp/            Unified Router + port pdp.Engine
  engine/         Adapter OPA nhúng (impl pdp.Engine)
  rebac/          Adapter ReBAC/OpenFGA (impl pdp.Engine)
  token/          Decision token (HS256, ràng tuple+digest)
  pep/            PEP library: phễu L0/L1/L2, fast-path, bubble-up
  pdpclient/      Client AuthZEN HTTP cho PEP
  grpcpdp/        Server gRPC PDP + client (impl pep.PDP)
identity/spiffe/  Workload authentication: SVID/mTLS/rotation/Source (go-spiffe)
signals/caep/     Continuous evaluation: SET (RFC 8417) + RevocationCache
policystore/      Adapter policy bundle bất biến (S3/MinIO)
ports/pip/        SPI: IdentityProvider / WorkloadAttestor / PolicyStore
services/         Wiring tái dùng: PDPService/Handler, mTLS (mtls.go), gRPC (grpc.go)
testsupport/mock/ Test kit: fake các PIP
policies/         KHUNG policy nền tảng (Rego) — KHÔNG chứa domain nghiệp vụ
proto/authzen/v1/ Hợp đồng Protobuf/gRPC
cmd/              PDP generic + ops tools (caepemit, bundlepush)
```

## Cách một hệ thống áp dụng

1. **Nhúng PEP** (service Go) — đặt `pep` middleware trước handler, trỏ tới PDP qua `pdpclient` (HTTP)
   hoặc `grpcpdp` (gRPC/mTLS). Service non-Go: dùng PEP sidecar (lộ trình).
2. **Dựng PDP** — chạy `cmd/pdp` (generic) và cấp **domain policy riêng** qua compiled bundle (S3) hoặc
   `services.PDPConfig.ExtraModules`/`ExtraData` (in-process).
3. **Cắm triển khai riêng** vào các SPI: `pdp.Engine`, `pep.PDP/TokenVerifier/RevocationChecker`,
   `pip.*`, `caep.Sink`, `spiffe.Source`.

## Tài liệu kiến trúc

Mô tả kiến trúc theo ISO/IEC/IEEE 42010 + tech spec từng module hiện ở
[`authorization-zta/docs/architecture`](https://github.com/pmsbkhn/authorization-zta/tree/main/docs/architecture)
(sẽ chuyển về đây).

## Trạng thái

Tách từ `authorization-zta` (sau M14). Lõi đứng độc lập, `go build ./...` + `go test ./...` xanh. Reference
adopter VSP (demo đầu cuối: gateway → multibill → wallet → pdp, mTLS/SPIRE, CAEP, ReBAC) ở repo
`authorization-zta/examples/vsp`.
