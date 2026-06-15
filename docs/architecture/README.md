# Tài liệu Kiến trúc — Nền tảng ZTA (`zta-core`)

Bộ tài liệu này mô tả kiến trúc của **nền tảng (platform) Zero Trust Authorization** `zta-core`, soạn
theo cấu trúc của **ISO/IEC/IEEE 42010:2011 — Systems and software engineering — Architecture
description**.

> **Định vị:** Hệ thống mô tả ở đây **là nền tảng ZTA** (repo/Go module `github.com/pmsbkhn/zta-core`),
> dùng để triển khai vào bất kỳ hệ thống nào. Các service nghiệp vụ **VSP (Gateway / Multi-Bill / VSP
> Wallet)** sống ở repo **`authorization-zta/examples/vsp`** như một **reference adopter** — không thuộc
> lõi. Xem ranh giới core↔adopter và view Adoption/Packaging trong AD.

Mục tiêu: bức tranh đầy đủ, chính xác ở mức mã nguồn về cấu trúc để làm nền cho việc **áp dụng** (import
thư viện / đặt sidecar) và tối ưu kỹ thuật (hiệu năng, vận hành, bảo mật).

> **Trạng thái packaging (đã đạt):** lõi là Go module riêng, mọi package public (không còn `internal/`),
> publish + tag — `v0.1.0` (lõi), `v0.2.0` (thêm PEP sidecar). Hệ thống Go:
> `go get github.com/pmsbkhn/zta-core` rồi nhúng `authz/pep`. Hệ thống non-Go: đặt `cmd/zta-pep` sidecar
> ([spec 14](specs/14-pep-sidecar.md)). Demo VSP đã tách sang repo `authorization-zta`.

## Cách tổ chức

| Tài liệu | Vai trò |
|---|---|
| [`architecture-description.md`](architecture-description.md) | **Tài liệu Mô tả Kiến trúc (AD)** theo ISO 42010: stakeholders, concerns, viewpoints, views, decisions, correspondences. Đọc trước. |
| [`specs/`](specs/) | **Tech spec từng module** — chi tiết hợp đồng, kiểu dữ liệu, luồng điều khiển, cấu hình, lỗi, và điểm tối ưu của mỗi thành phần. |

### Index tech spec theo module

| # | Spec | Gói mã nguồn | Mặt phẳng |
|---|---|---|---|
| 01 | [AuthZEN Data Contract](specs/01-authzen-contract.md) | `authz/authzen`, `proto/authzen/v1` | Control Plane (hợp đồng) |
| 02 | [AuthZEN HTTP Facade](specs/02-api-facade.md) | `authz/api` | Control Plane |
| 03 | [PDP / Unified Router](specs/03-pdp-router.md) | `authz/pdp` | Control Plane |
| 04 | [OPA Engine + Hierarchical Rego](specs/04-engine-opa-policies.md) | `authz/engine`, `policies/` | Control Plane |
| 05 | [ReBAC Engine (OpenFGA)](specs/05-engine-rebac.md) | `authz/rebac` | Control Plane |
| 06 | [Decision Token](specs/06-decision-token.md) | `authz/token` | Control Plane ↔ PEP |
| 07 | [PEP Library (L0/L1/L2)](specs/07-pep-library.md) | `authz/pep`, `authz/pdpclient` | PEP Layer |
| 08 | [gRPC Transport](specs/08-grpc-transport.md) | `authz/grpcpdp`, `proto/authzen/v1` | Control Plane ↔ PEP |
| 09 | [Workload Identity / SPIFFE-SVID](specs/09-spiffe-identity.md) | `identity/spiffe` | Trust fabric |
| 10 | [CAEP / Continuous Evaluation](specs/10-caep-revocation.md) | `signals/caep` | Control Plane ↔ PEP |
| 11 | [Policy Bundle Store + PIP seams](specs/11-policy-bundle-store.md) | `policystore/bundlestore`, `ports/pip`, `testsupport/mock` | Control Plane |
| 12 | [Service Wiring & Entrypoints](specs/12-services-and-cmd.md) | `services`, `cmd/` (+ adopter `examples/vsp` ở repo `authorization-zta`) | Tất cả |
| 13 | [Deployment Topology](specs/13-deployment-topology.md) | `authorization-zta/examples/vsp/{deploy,scripts}` | Vận hành |
| 14 | [PEP Sidecar (zta-pep)](specs/14-pep-sidecar.md) | `cmd/zta-pep` | Adoption (non-Go) |

## Tài liệu nguồn liên quan

- [`../design-v3.md`](../design-v3.md) — thiết kế gốc (v3), Implementation Ready. AD này **mở rộng và
  hiện thực hóa** thiết kế đó với những gì đã được code thật qua M1→M14.
- [`../../README.md`](../../README.md) — README `zta-core`: cách import + dùng sidecar.
- Reference adopter (demo VSP đầu-cuối): repo
  [`authorization-zta`](https://github.com/pmsbkhn/authorization-zta) — `examples/vsp`.

## Quy ước

- Trích dẫn mã nguồn theo dạng `gói/tệp.go:dòng`.
- Tiếng Việt là ngôn ngữ chính; thuật ngữ kỹ thuật chuẩn (PEP, PDP, mTLS, SVID, obligation…) giữ
  nguyên tiếng Anh.
- "Đã verify" = có test tự động và/hoặc đã chạy live theo README; "seam" = ranh giới interface để
  thay thế triển khai.
