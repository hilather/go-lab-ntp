# ADR 0008: absolute is step-then-follow

Status: Accepted
Date: 2026-08-30

## Context

The feature request calls `absolute` a “step” to an RFC3339 instant and gives
`freeze` its own field. Treating absolute as freeze would duplicate modes.

## Decision

`absolute` is **step-then-follow** at rate 1.0 (D5). At apply, the virtual
clock jumps to the RFC3339 instant and then tracks **monotonic** elapsed time
(ADR 0013):

```
served(t) = absolute + t.Sub(epochMono)
```

`freeze` is the stop-clock mode (`freezeAt`). Explicit `rate: 0` equals
freeze-at-epoch-virtual.

## Consequences

- After apply of `absolute: 2035-01-01T00:00:00Z`, ten real seconds later
  served time is `2035-01-01T00:00:10Z`.
- Modes are exclusive (forbidden-field matrix in docs/02-ntp-semantics.md).

## Alternatives considered

- `absolute` means freeze at that instant: duplicates `freeze`. Rejected.

## Review triggers

A later mode that combines rate with an absolute anchor (today: `mode: rate`
plus YAML `epoch`).
