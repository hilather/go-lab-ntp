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
mutations. CSRF secret is in-memory only. Do not store tokens in
localStorage.

## Origins

Missing Origin is allowed. A present non-loopback Origin must be in
`spec.management.allowedOrigins`. `*` is not allowed.
`originAllowlist` is an unknown YAML field.
