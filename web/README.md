# LabNTP operator SPA

React + TypeScript + Vite (Node **22.14.0**). The UI talks REST only (`/v1`).

Browser auth is `POST /v1/session` (bearer only — no HTTP Basic) → HttpOnly
`labntp_session` + CSRF in the JSON body / `GET /v1/session` reload recovery.
Mutations send `X-LabNTP-CSRF`. The token is never written to `localStorage`
or `sessionStorage`.

Pages: sign-in, filters (enable/disable), preview-an-IP, features (live vs
reset-only), query ring, status, gated reset. Query ring is polled (`GET
/v1/queries`, 5s). There is no audit page and no YAML editor.

`web/go.mod` is a nested-module fence so parent `go test ./...` does not walk
`node_modules`. Do not import `github.com/hilather/go-lab-ntp/web` from the
parent module. `//go:embed` cannot leave a module, so `make web-build` copies
`web/dist` into `internal/web/dist`. The committed fallback is
`internal/web/stub`.

```bash
npm --prefix web test
npm --prefix web run typecheck
npm --prefix web run build
```

Dev server proxies `/v1` and `/mcp` to `http://127.0.0.1:8088`.
