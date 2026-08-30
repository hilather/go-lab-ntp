# Security architecture

Status: Proposed normative behavior
Owners: Security
Last reviewed: 2026-08-30
Related ADRs: 0005

## Bearer

`spec.auth.mode: bearer`. Tokens are file refs, ≥32 bytes, SHA-256 digest
compare (`crypto/subtle`). No HTTP Basic. Roles expand to `ntp.read`,
`ntp.write`, `ntp.admin`, `ntp.audit.read`. Management bind fails closed
with zero usable tokens unless `--management-listen=off`.

## SPA session

Cookie `labntp_session` HttpOnly SameSite=Lax Path=/; idle 4h, absolute 12h,
max 64 sessions. CSRF header `X-LabNTP-CSRF` on cookie-authenticated
mutations. CSRF secret is in-memory only (`web/src/api/client.ts`). Do not
store tokens in localStorage. Vitest `assertNoTokenStorage` and
`web/src/api/storage.test.ts` lock this. See [12-web-ui.md](12-web-ui.md).

## Origins

Missing Origin is allowed. Loopback Origins (`http://127.0.0.1:8088`,
`http://localhost:5173`) are allowed. A present non-loopback Origin must be
an exact `spec.management.allowedOrigins` entry (scheme+host+port). `*` is
not allowed. Overlay stays `allowedOrigins: []` (deny-all for non-loopback).
A host-published browser on `:18123` gets 403 on `/v1/*` until the
integrator lists that origin. `originAllowlist` is an unknown YAML field.
