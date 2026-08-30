# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- None.

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.

## 1.0.0-rc.2 - 2026-08-30

Operator SPA and tag-triggered GHCR. Notes: [docs/releases/v1.0.0-rc.2.md](docs/releases/v1.0.0-rc.2.md).

### Added

- Operator SPA (Vite + React) at `/` when `spec.ui.enabled` is true: filter
  table enable/disable, preview-an-IP, features live vs reset-only, query
  ring, status, gated Reset. Cookie `labntp_session` + CSRF `X-LabNTP-CSRF`;
  no localStorage tokens. `spec.ui.enabled: false` keeps `GET /` as 404
  problem+json. `make web-install web-test web-build web-embed` and CI job
  `web` (Node 22.14.0). Committed `internal/web/dist` is the embed
  (`docs/12-web-ui.md`).
- Tag-triggered GHCR publish (`ghcr.io/hilather/labntp:<tag>` and `sha-<7 hex>`,
  no `:latest` on rc). `.github/workflows/release.yml` tag-gate then
  `publish-image` on `v*` tag push only.

### Changed

- None.

### Fixed

- None.

### Removed or deprecated

- None.

## 1.0.0-rc.1 - 2026-08-30

First public candidate. Notes: [docs/releases/v1.0.0-rc.1.md](docs/releases/v1.0.0-rc.1.md). Operator SPA is not in this tag.

### Added

- Repository foundation: Apache-2.0, Go 1.26 module `github.com/hilather/go-lab-ntp`, family Makefile/CI, scratch Dockerfile (UID 65532, EXPOSE 123/udp 8088/tcp).
- `labntp` CLI: `version`, `help`, `validate`, `canonicalize`, `serve`, `query` (SNTP smoke client), `healthcheck`, `mcp-stdio`.
- Fail-closed `labntp.dev/v1alpha1` YAML (`KnownFields(true)`), duration fields, IEC `bodyLimit`, presence types for `rate`/`minpoll`/`maxpoll`.
- First-party NTPv3/v4 codec (`internal/ntpwire`): 48-byte header, era 0/1 timestamps with D25 clamp, KoD RATE, ntpd concatenation MAC (MD5/SHA1/SHA256, not HMAC).
- Per-view virtual clocks (`follow-real`, `offset`, `absolute`, `freeze`, `rate`) with monotonic elapsed for absolute/rate.
- First-match CIDR filters, dual-stack catch-all required, IPv4-mapped Unmap, overlap warnings, `ntp.keys` compile.
- Unicast UDP data plane: admission, restrict/KoD, MaxUDPSize 576, MaxInflight 1024, query log ring, Reset rebind (bind-new-first). `--management-listen=off` still serves NTP.
- Host clock is never set (`TestNoClockSetSyscalls`, `TestHostClockUnchanged`).
- `app.Service` plan/apply/reset/preview/filters CRUD with idempotency and `expectedRevision`. Apply cannot change listen/NTS/keys/auth.
- REST `/v1` (`application/problem+json`) and MCP `/mcp` (`ntp_*` tools, protocol 2026-07-28, `Stateless: true`). Both call `app.Service` only.
- Lab static bearer (SHA-256, tokens ≥32 bytes, file refs, no Basic). Cookie `labntp_session` + CSRF `X-LabNTP-CSRF`. Management bind fails closed with zero tokens unless listen is off.
- Hand-rolled OpenMetrics (`labntp_packets_total` includes `oversize`), slog JSON, `labntp healthcheck`.
- Scratch image HEALTHCHECK (exec form), `examples/` overlay BOM (`labntp.yaml`, compose smoke, labinfo, MCPJungle `bearer_token`), `make test-container` (`:1123`; gated `:123`+`NET_BIND_SERVICE`).

### Fixed

- Container smoke publishes `127.0.0.1:0:8088/tcp` (explicit random host port) so `docker port` works on GitHub-hosted Docker.
- Container smoke token is ≥32 bytes after newline trim (bearer MinTokenBytes).
- Container smoke NTP query runs in-container (`labntp query` to 127.0.0.1:1123); host-published UDP is the userland-proxy NAT-collision path.
