# Changelog

All notable user-visible and operator-visible changes are recorded here. This file is curated; it is not a raw commit log.

## Unreleased

### Added

- None.

### Changed

- Go toolchain pinned to go1.26.8 (go.mod `toolchain`, CI `GO_VERSION`, Dockerfile); 1.26.0–1.26.7 lack current stdlib security fixes.
- Web development dependency `source-map-js` updates from 1.2.1 to 1.2.2 (GHSA-68fv-2mgg-jv7q, high: event-loop denial of service through indexed source-map section offsets). Lockfile only; the built web assets are byte-identical.
- Web development dependency `undici` updates from 8.10.0 to 8.10.2 (via jsdom; GHSA-rfgv-xxqx-mfg5, GHSA-w293-vg96-wgc3 and GHSA-vp8m-p9jh-q5pm high, plus eight moderate or low undici advisories). Lockfile only; the built web assets are byte-identical.

### Fixed

- A reset that moves management HTTP or turns it off no longer waits on its own request. The old listener closes at once and the old server drains in the background for up to 5 s, then remaining connections are closed. A drain timeout is no longer reported as a failed rebind. Process shutdown waits for that background drain until its timeout. Turning management off leaves the listen address empty.
- Duration formatting no longer crashes on the minimum signed duration. View durations that `time.ParseDuration` cannot represent are rejected.
- Reset does not install a bootstrap whose bearer secret file cannot be read or whose bearer list is empty while management auth is attached. The previous snapshot, listeners, bearer, and cookie sessions stay.
- `labntp mcp-stdio` re-resolves the startup token against the current verifier on each tool call. Demoting or removing that token drops administrator scope.
- A failed reset rolls the NTP listener back if management HTTP rebind fails. The active snapshot is unchanged.
- Management HTTP `bodyLimit`, `requestsPerSecond`, `burst`, and `maxConcurrent` take effect on apply and on reset. A lowered `bodyLimit` applies live to REST and `/mcp`. `/mcp` cannot exceed the body limit it started with until restart. Raising `requestsPerSecond`, `burst`, or `maxConcurrent` applies live to REST; `/mcp`'s own limiter keeps its startup ceiling until restart. An explicit `bodyLimit: 0` means the startup default (1 MiB), and an explicit `maxConcurrent: 0` means the startup default (256).
- REST mutation JSON rejects unknown fields.
- NTP per-IP and limited buckets, and the MCP management per-remote buckets, evict idle keys and stay capped. An empty oldest MCP key is evicted like any other, so the map cannot grow past the cap.
- Tag release CI must be the green push for that tag and SHA. A green main or pull-request run of the same commit does not pass the gate. The tag name is passed into the release script as an environment variable. Only the newest matching tag push is judged; an older queued or in-progress run does not block a newer completed green run.
- A `workflow_dispatch` re-gate of a release tag works when started from a branch. GitHub ignores the release workflow's step-env overrides of `GITHUB_SHA`, `GITHUB_REF` and `GITHUB_REF_NAME`, so the gate saw the branch and failed. The workflow now passes the tag and the checked-out commit to `release-gate -require-ci` as `-tag` and `-sha`; `-tag` without `-sha` uses `git rev-parse HEAD`.
- A manual release re-gate checks the tag against `^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$` before checkout and checks out only `refs/tags/<tag>`. The workflow fails unless `HEAD` is `refs/tags/<tag>^{commit}`, and that commit is the `-sha` passed to `release-gate`. A branch named like the tag can no longer win checkout. `release-gate` exits 75 only when that tag's CI run is missing or not completed, and the workflow retries only status 75. A pre-release tag such as `v1.0.0-pending` does not make any other error retry. `publish-image` checks out the same canonical tag and records the peeled commit.
- Raise `golang.org/x/sys` from v0.41.0 to v0.47.0, past advisory GO-2026-5024 (fixed in v0.44.0). govulncheck found it in a required module only; no LabNTP code path called it.

### Removed or deprecated

- None.

## 1.0.0-rc.3 - 2026-09-04

MCP ViewSpec omit-to-zero, operator chrome, and plain-English docs. Notes: [docs/releases/v1.0.0-rc.3.md](docs/releases/v1.0.0-rc.3.md).

### Added

- Operator [user guide](docs/guide.md): YAML loading, clock modes, REST/MCP state APIs, Docker, and troubleshooting.
- Repository header art and mark under `docs/assets/`.

### Changed

- README and START-HERE rewritten in plain English, with copy-paste quick starts for `validate` / `canonicalize` and the `/v1/state*` APIs.
- Documentation catalog lists the user guide as the operator front door.
- Operator chrome: Lab* family shell (56px masthead, CLOCKS/LAB rail, IBM Plex,
  dark tokens). Filters is list-order inventory + clock inspector + selected-filter
  math. Preview restyles `GET /v1/views/preview?ip=`. Queries / Features / Status /
  Reset inherit the shell only. Same REST (`PUT /v1/filters/{name}`, preview GET);
  no new endpoints.

### Fixed

- MCP `ntp_change_apply` (and other tools that embed `ViewSpec`) no longer
  reject omitted zero-default view fields (`precision`, `rootDelay`,
  `rootDispersion`, `jitter`, `offset`, `leap`, `refid`). Generated JSON
  Schema `required` now matches REST/typed apply omit-to-zero; YAML
  document decode still materializes `precision: -20`. `config.validateView`
  is unchanged. Fixes #2.

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
