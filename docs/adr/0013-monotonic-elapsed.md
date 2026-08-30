# ADR 0013: Monotonic elapsed for absolute/rate

Status: Accepted
Date: 2026-08-30

## Context

`time.Now().UTC()` strips the monotonic reading. Subtracting two wall times
jumps every `absolute` / `rate` view when the VM NTP steps — the opposite of
“virtual time vs the VM clock” for those modes.

`follow-real` and `offset` *should* track wall UTC so a host timesyncd step
moves those views with the VM.

## Decision

`Clock.Now()` returns `time.Now()` **without** `.UTC()` (D22).

- `follow-real` / `offset` use `t.UTC()` (wall).
- `absolute` / `rate` elapsed uses `t.Sub(epochMono)` (monotonic if both sides
  have it).
- Convert to UTC only at NTP encode (`ntpwire.FromTime(t.UTC())`).

Fake test clocks implement `Now()` so consecutive calls are comparable with
`Sub` (advance a single `time.Time` by `Add`).

## Consequences

- A host timesyncd step does not jump rate/absolute views.
- It does move follow-real/offset, matching “virtual time vs the VM clock.”

## Alternatives considered

- Always `.UTC()`: rate/absolute jump on VM NTP. Rejected.

## Review triggers

A later mode that must ignore VM steps while still advertising wall time.
