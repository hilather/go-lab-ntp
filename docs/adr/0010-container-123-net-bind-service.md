# ADR 0010: Container listen :123 and NET_BIND_SERVICE

Status: Accepted
Date: 2026-08-30

## Context

NTP clients speak UDP/123. LabDNS avoided privileged ports with 5353. LabNTP
cannot: SUT configs and `ntpdate`/`chronyc` examples assume 123. The family
forbids root (UID 65532, `cap_drop: ALL`). Bind `:123` as 65532 returns
`EACCES` without `CAP_NET_BIND_SERVICE`.

Host UDP/123 collides with systemd-timesyncd. The feature request default
host publish is **10123**, not IANA 123.

## Decision

- Container NTP listen default `:123`.
- Image is `USER 65532:65532` + `cap_drop: ALL`.
- Integrator compose restores **only** `cap_add: [NET_BIND_SERVICE]`.
- Default `make test-container` (later) uses `--ntp-listen=:1123` and
  `cap_drop: ALL`. Gated `LABNTP_TEST_NET_BIND=1` proves `:123`+cap.
- Host map default `10123:123/udp`. Host UDP/123 is profile opt-in.
- `--ntp-listen` overrides for cap-less local runs.
- Integrator preflight must not treat `EACCES` as occupied.

## Consequences

- Local `go run` without the cap uses `:1123`.
- Unit tests always bind ephemeral ports.
- The image is not root. Ambient `NET_BIND_SERVICE` is a compose exception.

## Alternatives considered

- Container listen `:1123` (LabDNS analog): surprises operators mapping
  `10123:1123`. Rejected as default.
- Run as root: family forbids it.

## Review triggers

A runtime that cannot grant ambient `NET_BIND_SERVICE` to UID 65532.
