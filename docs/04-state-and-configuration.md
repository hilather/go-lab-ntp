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
   is rejected.
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
`--ntp-listen`) changed; bind **new first**, then drain/close old. Flags
always win over YAML on serve and Reset.

`app.Service` Plan/Apply/Reset implements this split. Reset rebinds NTP and
management HTTP when the effective listen address changed (bind-new-first).
