# ADR 0002: First-party NTPv3/v4 codec

Status: Accepted
Date: 2026-08-30

## Context

NTP unicast server/client packets are a 48-byte header plus an optional MAC
trailer. Third-party libraries (`beevik/ntp`, `facebook/time`) are
client-oriented, leak types, and invite client-mode discipline LabNTP must
not do (never query a pool, never set the host clock). LabDNS already hid
wire types in `internal/dnswire`.

## Decision

Implement `internal/ntpwire` as the only packet codec. No NTP module
dependency. Direct production deps for the data-plane slice: `gopkg.in/yaml.v3`
only. Parse/encode the 48-byte header, LI/VN/Mode packing, 32.32 timestamps
with era 0/1 + D25 clamp, KoD RATE, and ntpd concatenation MAC (ADR 0012).

`internal/ntpwire` must not import `internal/control` or `internal/web`.

## Consequences

- Wire types never escape into REST/MCP/YAML.
- Tests lock unix 0 ↔ NTP seconds `2208988800` and per-algorithm MAC trailers.
- SNTP clients are first-class; we are an SNTP **server** (no clock filter).

## Alternatives considered

- Import `beevik/ntp` or `facebook/time`: extra dep, type leak, client APIs. Rejected (D2).
- Exec ntpd/chrony: rejected (ADR 0001).

## Review triggers

NTS, extension fields, or a SUT that requires HMAC (see ADR 0012).
