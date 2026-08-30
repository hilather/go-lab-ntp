# LabNTP

LabNTP is a laboratory NTPv3/v4 server with **per-IP virtual clocks**. One
tester can skew Kerberos (~5 min), jump a cert into not-yet-valid / expired, or
drift a TOTP step **without** moving the lab host clock or colliding with
another tester on the same compose graph.

It is a first-party Go scratch appliance in the LabDNS / LabMail / LabMITM
family. It is **not** a production time source, **not** a chrony/ntpd wrapper,
and it never calls `settimeofday` / `clock_settime` / `adjtimex`.

| Kind | Value |
|---|---|
| Repository | [`github.com/hilather/go-lab-ntp`](https://github.com/hilather/go-lab-ntp) |
| Module | `github.com/hilather/go-lab-ntp` |
| Binary | `labntp` |
| Image | `ghcr.io/hilather/labntp` (`:local` for compose builds) |
| Config | `labntp.dev/v1alpha1` / kind `LabNTP` |
| Data plane | UDP NTPv3/v4 unicast (container `:123`, host publish default **10123**) |
| Management | REST `/v1` + MCP `/mcp` + SPA `/` (when `spec.ui.enabled`) |
| License | Apache-2.0 |

## What exists today

- Fail-closed YAML (`KnownFields(true)`). `labntp validate` / `canonicalize`.
- First-party NTPv3/v4 codec (`internal/ntpwire`). No NTP library.
- Per-view virtual clocks: `follow-real`, `offset`, `absolute`, `freeze`, `rate`.
- First-match CIDR filters, dual-stack UDP, KoD RATE, ntpd concatenation MAC.
- `labntp serve --config … --ntp-listen=:1123 --management-listen=off`.
- `labntp query --server 127.0.0.1:1123` SNTP smoke client (never used by the server).
- REST `/v1` + MCP `/mcp` (`ntp_*` tools) when `--management-listen` is an address. Bearer required. `labntp healthcheck`.
- Operator SPA at `/` when `spec.ui.enabled: true` (cookie `labntp_session` + `X-LabNTP-CSRF`; no localStorage tokens). `spec.ui.enabled: false` → `GET /` is 404 problem+json.

`--management-listen` defaults **off**. The image CMD passes `:8088` so
HEALTHCHECK against `/v1/health/ready` works. Overlay BOM lives under
`examples/` (`docs/13-integration-lab.md`). SPA details: [docs/12-web-ui.md](docs/12-web-ui.md).

## Quick start

```bash
go build -o bin/labntp ./cmd/labntp
./bin/labntp version
./bin/labntp validate --config testdata/config/valid/full.yaml
./bin/labntp canonicalize --config testdata/config/valid/full.yaml --format yaml
./bin/labntp serve \
  --config testdata/config/valid/full.yaml \
  --ntp-listen=:1123 \
  --management-listen=off
# in another shell
./bin/labntp query --server 127.0.0.1:1123
```

UID 65532 cannot bind `:123` without `CAP_NET_BIND_SERVICE`. Local runs use
`--ntp-listen=:1123`. Compose later restores only that cap for container `:123`.

Host-publish per-IP isolation requires source-preserving UDP. Docker
`userland-proxy: true` SNATs host-published UDP so every client of
`${LAB_PUBLIC_HOST}:10123` appears as one bridge IP (**NAT collision**).
Compose-network sources (`labntp:123`) and `GET /v1/views/preview` stay
reliable without that. See [docs/02-ntp-semantics.md](docs/02-ntp-semantics.md).

## Documentation

Start at [START-HERE.md](START-HERE.md). Agent rules: [AGENTS.md](AGENTS.md).
Catalog: [docs/README.md](docs/README.md). Implementation design:
[docs/implementation-design.md](docs/implementation-design.md).

## Build and test

Go **1.26**. Node **22.14.0** for `web/`.
`make format lint test test-docs test-config-compat test-parity`.
`make test-container` requires Docker (default `:1123` + `cap_drop ALL`; gated
`LABNTP_TEST_NET_BIND=1` for `:123`). `make web-install web-test web-build`
builds the operator SPA. Local Vite: `npm --prefix web run dev` (proxies
`/v1` to `127.0.0.1:8088`).
