# ADR 0011: Rate epoch is virtual

Status: Accepted
Date: 2026-08-30

## Context

`mode: rate` needs an anchor. The feature request’s “epoch = when this view
was applied” is the omitted-epoch case. Explicit `epoch` is how “start at
2035 and run 2×” works without a second mode. There is no user `epochReal`.

Go/YAML zero `float64` is `0`, which is a legal rate, so `ViewSpec.Rate` is
`*float64` (D20).

## Decision

YAML `epoch` is optional **virtual** RFC3339, legal only on `mode: rate` (D19).

On compile:

- `epochMono = compileClock.Now()` (keeps monotonic)
- `epochWall = epochMono.UTC()`
- `rate` + YAML `epoch` set → `epochVirtual = parse(epoch)`
- `rate` + YAML `epoch` omitted on a **new** view → `epochVirtual = epochWall`

Live apply of an existing rate view:

- `rate` equal and YAML `epoch` unchanged → keep previous pair (no jump)
- `rate` changed, YAML `epoch` still omitted → re-anchor
  `epochVirtual = oldView.Served(now)`
- YAML `epoch` changed → explicit jump

`served(t) = epochVirtual + saturatingDuration(elapsed * rate)` then D25 clamp.

## Consequences

- Omitted `rate` on `mode: rate` fails validate; explicit `rate: 0` is legal.
- Present `rate` must be finite and `|rate| ≤ 100`.

## Alternatives considered

- A separate `epochReal` field: extra mode surface. Rejected.

## Review triggers

A later sticky-RAM view that is not GitOps desired state.
