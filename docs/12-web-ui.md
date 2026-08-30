# Operator SPA

Status: Proposed normative behavior
Owners: Operator UI
Last reviewed: 2026-08-30
Related ADRs: 0004, 0005, 0009

The operator SPA is a same-origin Vite + React app embedded with `go:embed`
of `internal/web/dist`. It talks REST `/v1` only. REST and MCP remain the
control plane; the SPA is an adapter.

## Screens

Exactly these routes. There is no audit page, no YAML editor, and no
view-field form beyond enable/disable + chips.

| Path | Page |
|---|---|
| `/login` | Exchange a bearer for cookie `labntp_session` |
| `/` | Filters (home): table, enable/disable |
| `/preview` | Preview-an-IP |
| `/queries` | Query ring (polled 5s, `limit=256`) |
| `/features` | Frozen `features.list` live vs reset-only |
| `/status` | Ready, NTP bound, drifted, hostTime, revisions |
| `/reset` | Gated Reset (`ntp.admin`, phrase `RESET`) |

Filter match is list order. Caption: first enabled match wins. Longest-prefix
does not. Enable/disable is `PUT /v1/filters/{name}` with the GET object and
only `enabled` flipped. Duration fields are **strings** (`"0s"`).
`expectedRevision` comes from `GET /v1/state` `runtimeRevision` (camelCase).
Do not read `status.revisions.runtimeRevision` — nested Status keys are
PascalCase (`BootstrapRevision`).

Preview does not send NTP. Host-publish **NAT collision** / Docker
`userland-proxy` does not affect preview.

Features page renders the frozen twelve ids. UI enablement is bootstrap YAML;
reread with Reset. Not a `features.list` id. Do not add ids.

Status page: `ready` / `hostTime` / `listeners` from `GET /v1/status`;
`runtimeRevision` / `bootstrapRevision` / `generation` / `drifted` from
`GET /v1/state`. Banner: LabNTP is not a production time source. The host
clock never moves.

Reset rereads bootstrap YAML, wipes the query ring, never writes the file,
rebinds per D8. Phrase `RESET`, checkbox, optional reason. Submit requires
`ntp.admin`.

## Auth

Cookie `labntp_session` HttpOnly SameSite=Lax Path=/. CSRF header
`X-LabNTP-CSRF` is held in process memory (`web/src/api/client.ts`). Never
`localStorage` / `sessionStorage` tokens. Never HTTP Basic. Vitest
`assertNoTokenStorage` locks this.

`GET`/`HEAD` never send CSRF. Mutations attach the in-memory secret.
`credentials: "same-origin"` always.

Login copy: exchange a scoped API bearer for an HttpOnly session cookie.

## `spec.ui.enabled`

`false` (or UI handler unset) → `GET /` is **404 `application/problem+json`**.
REST `/v1` and MCP `/mcp` are unchanged. Omitted `ui.enabled` is decoded as
in rc.1. Overlay BOM (`examples/labntp.yaml`) keeps `true`. Container smoke
(`testdata/container/config.yaml`) keeps `false` and asserts `GET /` 404.

There is no live Apply op for `spec.ui`. Rewrite bootstrap YAML, then Reset
or process restart. `UIEnabled` reads `store.Load().Canonical.Spec.UI.Enabled`.

`--management-listen` still defaults **off**. Image CMD binds `:8088` so
HEALTHCHECK and the SPA work in compose.

## Origins

The SPA is **same-origin**. Overlay `allowedOrigins: []` stays deny-all (D13;
no `*`). Missing Origin is allowed. Loopback (`http://127.0.0.1:8088`,
`http://localhost:5173` Vite) is exempt. A browser on
`http://${LAB_PUBLIC_HOST}:18123` sends a non-loopback Origin and every
`/v1/*` fetch is **403** until the lab-owned overlay lists
`allowedOrigins: ["http://<lab-host>:18123"]` (scheme+host+port exact).

## Embed and CI

`make web-install web-test web-build web-embed` build the Vite app and copy
`web/dist` → `internal/web/dist`. Dockerfile has **no Node stage**. The
committed `internal/web/dist` is what `go:embed`, `go test`, `docker build`,
and GHCR ship.

CI job `web` (Node **22.14.0**) asserts the **checkout** is a real Vite tree
**before** `make web-build`: `internal/web/dist/index.html` has
`<title>LabNTP</title>` or `#root`, the stub sentence is absent, hashed JS
exists. Then `web-install web-test web-build` proves `web/src` still
compiles. There is **no** full-tree `git diff` of `dist` (Vite 8 is not
bit-identical across runners).

Go unit test `TestCommittedDistIsProduction` fails if `Files()` is the stub
page (`UI assets were not copied`). Tag-gate cannot publish a stub even if
the `web` YAML steps are reordered.

`internal/control/rest` production files must not import `internal/web`.
`cmd/labntp/serve.go` sets `rest.Config.UI`. Tests in `rest` may import `web`.

## Local Vite

Node **22.14.0**, npm ≥10.9.0.

```bash
make web-install
npm --prefix web run dev
```

Dev server proxies `/v1` and `/mcp` to `http://127.0.0.1:8088`. Serve LabNTP
with `--management-listen=:8088` and `spec.ui.enabled: true`.

## Mira

Mira reviews the operator UI after this implementation lands on the branch
that will be tagged. Mira is not a merge gate, tag gate, or GHCR gate.
Security defects (localStorage tokens, CSRF missing, Basic auth) are a tag
blocker because CI must be green.
