# ADR 0004 — Shared capability registry

- Status: Accepted
- Date: 2026-08-30
- Related: D12

REST and MCP share one frozen table in `internal/capabilities`. Adapters
bind `ServiceMethods` by name. MCP tools use `ntp_*`. Resources use
`labntp://`.
