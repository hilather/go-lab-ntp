# Observability

Status: Proposed normative behavior
Owners: Observability
Last reviewed: 2026-08-30

Hand-rolled OpenMetrics (no Prometheus client). slog JSON to stderr.

## Metrics

| Metric | Kind | Labels |
|---|---|---|
| `labntp_packets_total` | counter | `version`, `decision` (`serve`,`drop`,`kod`,`ignore`,`allowlist`,`admission`,`unmatched`,`short`,`oversize`,`version`,`mode`,`zero_xmit`) |
| `labntp_filter_hits_total` | counter | `filter` (cap 128 names then `other`) |
| `labntp_querylog_dropped_total` | counter | — |
| `labntp_apply_total` | counter | `result` |
| `labntp_http_requests_total` | counter | `code_class`, `capability` |
| `labntp_mcp_calls_total` | counter | `capability` |
| `labntp_auth_failures_total` | counter | — |
| `labntp_udp_inflight` | gauge | — |

Never label with client IP.

## Health

`GET /v1/health/live` — process. `GET /v1/health/ready` — NTP bound +
snapshot + (management bound or `--management-listen=off`). Ready stays
true on the old NTP socket until a new bind succeeds (D8).

`labntp healthcheck --url=http://127.0.0.1:8088/v1/health/ready`.

Events: `ntp.query`, `state.apply`, `state.reset`, `auth.failure`.
