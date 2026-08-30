# Program Board

Status: in progress (PRs 1–12 in this working tree; SPA remaining)
Last reviewed: 2026-08-30

## Work packages

| Order | Task | ID | Depends on | Primary output | Status |
|---:|---|---|---|---|---|
| 1 | Repository foundation | FND-001 | None | Go module, CI, Make, docs | done (this slice) |
| 2 | Domain and KnownFields loader | CFG-001 | FND-001 | `labntp.dev/v1alpha1` model | done (this slice) |
| 3 | NTP wire codec | WIRE-001 | FND-001 | `internal/ntpwire` | done (this slice) |
| 4 | Virtual clock math | VIEW-001 | CFG-001 | `internal/ntpview` | done (this slice) |
| 5 | Filters, compiler, snapshot | SNAP-001 | CFG-001, VIEW-001 | first-match + keys | done (this slice) |
| 6 | UDP data plane | NTP-001 | WIRE-001, SNAP-001 | `labntp serve` NTP-only | done (this slice) |
| 7 | Application service | APP-001 | SNAP-001, NTP-001 | plan/apply/reset/preview | done (this slice) |
| 8 | REST `/v1` | API-001 | APP-001 | control-plane adapters | done (this slice) |
| 10 | Auth, CSRF, audit | SEC-001 | API-001 | bearer + session | done (this slice) |
| 9 | MCP `/mcp` | MCP-001 | API-001, SEC-001 | `ntp_*` tools | done (this slice) |
| 11 | Observability | OBS-001 | NTP-001, API-001 | metrics, logs, health | done (this slice) |
| 12 | CLI, container, examples BOM | DEP-001 | NTP-001, SEC-001, OBS-001 | image + overlay | done (this slice) |
| 13 | Operator SPA | UI-001 | API-001, SEC-001, DEP-001 | Mira reviews | pending |

Control-plane order is **8 → 10 → 9** so `POST /mcp` never lands without a bearer verifier.

## This slice

`make test lint test-docs test-parity test-config-compat` must pass.
`make test-container` requires Docker. `web-*` remains fail-closed until PR 13.
