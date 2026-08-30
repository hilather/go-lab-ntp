# Deployment

Status: Proposed normative behavior
Owners: Platform
Last reviewed: 2026-08-30

Scratch image, UID **65532:65532**, `cap_drop: ALL`, read-only root. Container NTP listen is **`:123`**. Integrator compose restores **only** `cap_add: [NET_BIND_SERVICE]` so bind-to-123 works. Default `make test-container` does **not** — it uses `--ntp-listen=:1123`. Gated `LABNTP_TEST_NET_BIND=1` proves `:123`+cap.

Host publish default is **10123/udp** (FR). Native host UDP/123 is profile opt-in. Integrator preflight must not treat `EACCES` as occupied (TacLab 49 pattern). Do not silently remap 10123 to IANA 123.

Docker must pass `NET_BIND_SERVICE` as an **ambient** capability to a non-root process (dockerd ≥20.10 with default seccomp). `no-new-privileges:true` is compatible because the cap is already in the bounding set at exec.

## Image

- `ghcr.io/hilather/labntp` / `labntp:local`
- HEALTHCHECK exec form: `/labntp healthcheck --url=http://127.0.0.1:8088/v1/health/ready`
- CMD binds `--management-listen=:8088` so HEALTHCHECK works (`--management-listen` still defaults **off** in the binary)
- No shell. No Node stage. The operator SPA is `go:embed` of the committed
  `internal/web/dist` Vite tree (`docs/12-web-ui.md`).

## GHCR

Tag-triggered workflow [`.github/workflows/release.yml`](../.github/workflows/release.yml):

- Tags: `ghcr.io/hilather/labntp:<git-tag>` and `ghcr.io/hilather/labntp:sha-<7 hex>`.
- No `:latest` on prerelease (`v*-rc*`).
- Platform `linux/amd64`. Provenance + SBOM on.
- **Digest is the integrator pin**, not the `sha-*` tag.
- `workflow_dispatch` re-gates only; it does not push GHCR.

First push may create a private package. A human may need to mark
`ghcr.io/hilather/labntp` public in the org UI so the integrator can pull
without a token.

## Host-publish source IP

Host-publish per-IP isolation requires **source-preserving UDP**. Docker `userland-proxy` (default true on many daemons) SNATs host-published UDP so every laptop/VM hitting `${LAB_PUBLIC_HOST}:10123` appears as one bridge IP (**NAT collision**). Compose-network sources (`labntp:123`) and `GET /v1/views/preview` remain reliable without that. Integrator should set `userland-proxy: false` or use macvlan. This repo does not probe dockerd.

See [ADR 0014](adr/0014-host-publish-source-preserving-udp.md).
