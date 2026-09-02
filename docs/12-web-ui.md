# Operator SPA

Status: Proposed normative behavior
Owners: Operator UI
Last reviewed: 2026-09-02
Related ADRs: 0004, 0005, 0009

The operator SPA is a same-origin Vite + React app embedded with `go:embed`
of `internal/web/dist`. It talks REST `/v1` only. REST and MCP remain the
control plane; the SPA is an adapter.

## Screens

Exactly these routes. There is no audit page and no YAML editor. The
table-only first cut (enable/disable + chips) is superseded by the
Matt-approved operator chrome: Filters is a list-order inventory plus
clock inspector. Inspector Save binds Filter fields `PUT` already
accepts. Do not invent precision / rootDelay / jitter / minpoll.

| Path | Page |
|---|---|
| `/login` | Exchange a bearer for cookie `labntp_session` |
| `/` | Filters (home): list-order inventory, inspector, selected-filter math |
| `/preview` | Preview-an-IP (`GET /v1/views/preview?ip=`) |
| `/queries` | Query ring (polled 5s, `limit=256`) — leftover body, family shell |
| `/features` | Frozen `features.list` live vs reset-only — leftover body, family shell |
| `/status` | Ready, NTP bound, drifted, hostTime, revisions — leftover body, family shell |
| `/reset` | Gated Reset (`ntp.admin`, phrase `RESET`) — leftover body, family shell |

## Chrome

Family shell: 56px masthead (LabNTP, live/ready chips, actual session
scopes, Sign out) and a ~196px rail grouped **CLOCKS** (Filters, Preview,
Queries) / **LAB** (Features, Status, Reset). Reset stays `ntp.admin`.
No navy bar. Tokens: `--bg #0b0c0e`, `--elev #121317`, `--panel #181a1f`,
`--fg #ecece8`, `--muted #9a9b97`, `--subtle #6d6e6a`, `--accent #47c4dc`,
`--danger #c45c5c`. Type is IBM Plex Sans + Mono via Google Fonts CDN
(`fonts.googleapis.com`, `fonts.gstatic.com`) with `system-ui` fallback.
Filters workspace is ~340px inventory + inspector (Filters-only; not a
shell column).

## Filters

Filter match is list order. Show ordinals. Caption: first **enabled** CIDR
hit wins. Longest-prefix does not. Catch-all stays last. No drag-reorder.

Enable/disable is `PUT /v1/filters/{name}` with the GET object and only
`enabled` flipped. Reasons `ui: enable filter` / `ui: disable filter`.
Inspector Save is a second PUT of fields the API already accepts (mode,
offset / absolute / freezeAt / rate, leap, stratum, refid) after the
validate forbidden-field matrix. Reason `ui: save view`. Duration fields
are **strings** (`"0s"`, `"-6m"`). Omit `rate` unless the current mode is
`rate`. Keep YAML `epoch` on rate; drop it on every other mode.
`expectedRevision` comes from `GET /v1/state` `runtimeRevision` (camelCase).
Do not read `status.revisions.runtimeRevision` — nested Status keys are
PascalCase (`BootstrapRevision`).

In-pane “what this client would see” is selected-filter math only (sample
IP default = first CIDR host). It does **not** call
`GET /v1/views/preview`. Host time is `GET /v1/status` `hostTime`.
follow-real / offset / freeze match the product formulas. absolute and
rate show elapsed 0 because compiled `epochMono` is not on the Filter DTO
— the Preview route is the compiled match walk. Do not subtract
`view.epoch` from host (epoch is virtual and rate-only).

## Preview

Preview is `GET /v1/views/preview?ip=` and does not send NTP. Host-publish
**NAT collision** / Docker `userland-proxy` does not affect preview.
Allowlist / unmatched is 200 with copy
`IP {ip} is {reason}. Served time is not available.` Empty submit:
`Enter an IP address.`

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

Mira reviewed the operator chrome afters; Matt approved this pass. This
document describes that chrome. Mira is not a merge gate, tag gate, or
GHCR gate. Security defects (localStorage tokens, CSRF missing, Basic
auth) are a tag blocker because CI must be green.
