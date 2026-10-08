# State and configuration

Status: Proposed normative behavior
Owners: Config, Compiler
Last reviewed: 2026-08-30
Related ADRs: 0003, 0008, 0009, 0011

## Document

```yaml
apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: lab-time
spec:
  listeners:
    ntp:
      address: ":123"
    management:
      address: ":8088"
      restPath: /v1
      mcpPath: /mcp
  auth:
    mode: bearer
    tokens: []
  ui:
    enabled: true
  management:
    allowedOrigins: []
    mcp:
      allowLegacyClients: false
    bodyLimit: 1MiB
    requestsPerSecond: 32
    burst: 64
    maxConcurrent: 256
  ntp: { ... }
  filters: [ ... ]
```

One path each (KnownFields cannot alias): `spec.auth`, `spec.ui.enabled`,
`spec.management.allowedOrigins`. `originAllowlist`, `minPoll`, `refID`,
and kebab-case `min-poll` are unknown fields.

YAML view wire names keep `minpoll`, `maxpoll`, `refid` (ADR 0003).

## Pipeline

1. `config.Decode` — YAML `KnownFields(true)` and JSON `DisallowUnknownFields`.
   Reject multi-doc, empty, non-UTF-8, oversize (1 MiB). `bodyLimit: 1MiB`
   rewrites to 1048576; bare `1048576` is also accepted.
2. `config.Normalize` — materialize defaults. Duration strings → `time.Duration`
   for `offset`, `rootDelay`, `rootDispersion`, `jitter`. Bare `offset: 5`
   is rejected. Values outside `time.ParseDuration`, including the minimum
   signed duration, are invalid.
3. `config.Validate` — catch-all, CIDRs, forbidden-field matrix, stratum 1–16,
   leap enum, finite `|rate| ≤ 100`, `minpoll <= maxpoll` in `[-6, 17]`,
   `nts.enabled` must be false, reserved keys (`chrony`, `ntpd`, `timesyncd`,
   `ptp`, `broadcast`, `multicast`, `pool`). Secret **paths** are required
   when tokens/keys are declared; file existence is not required at validate.
4. `compiler.Compile` — prefixes, first-match slice, view epochs, key file
   if `file:` is set (missing file fails compile), revision hash.

Presence types: `ViewSpec.Rate *float64`, `MinPoll`/`MaxPoll *int`. Omitted
`rate` on `mode: rate` fails; explicit `rate: 0` is legal.

## Revision

`sha256:` plus lowercase hex of SHA-256 of canonical JSON. Reset rereads
bootstrap and never writes it. Materialized `epoch` is not persisted back
to the file.

## Live vs reset-only

Live (`app.Service`): filters, view fields, restrict, admission,
allowClientCidrs, query-log size, management HTTP limits.

Reset-only: listen addresses, `ntp.nts.enabled`, `ntp.symmetricKeys.file`,
`spec.auth`. Reset rebinds NTP iff the effective listen address (after
`--ntp-listen`) changed; bind **new first**, then drain/close old. Management
HTTP serves on the new listener first, closes the old listener immediately,
and drains the old server in the background for up to 5s. Turning management
off closes the listener immediately and does not wait on in-flight requests.
Flags always win over YAML on serve and Reset.

`app.Service` Plan/Apply/Reset implements this split. Reset rebinds NTP and
management HTTP when the effective listen address changed (bind-new-first).
A token reread failure returns `validation_failed` before rebind or swap.
A successful reread still replaces the verifier and clears sessions when
the identity changes. A failed management HTTP rebind leaves the previous
listener and snapshot, and restores the previous NTP address. Management HTTP
`bodyLimit`, `requestsPerSecond`, `burst`, and `maxConcurrent` are applied
to the running server on apply and on reset, not only at process start.
An explicit `bodyLimit: 0`, `requestsPerSecond: 0`, `burst: 0`, or
`maxConcurrent: 0` means the startup default (1 MiB, 32/s, burst 64, 256).
A lowered `bodyLimit` applies live to REST and `/mcp`, while `/mcp` cannot
exceed the limit it started with until restart; raising
`requestsPerSecond`, `burst`, or `maxConcurrent` applies live to REST, but
`/mcp`'s own limiter keeps its startup ceiling until restart.
