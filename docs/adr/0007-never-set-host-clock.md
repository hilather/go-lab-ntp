# ADR 0007: Never set the host clock

Status: Accepted
Date: 2026-08-30

## Context

Lab compose graphs share one VM clock. Stepping that clock poisons Kerberos,
TLS, TOTP, and logs for every tester and every sibling appliance. The product
is **virtual time on the NTP wire**, not `settimeofday` in the LabNTP process.

SUTs *will* step *their* clocks after querying LabNTP; that is intended.

## Decision

Production code must not set the LabNTP process / lab host clock (D14).

Gate `TestNoClockSetSyscalls`:

1. Selector names `Settimeofday`, `ClockSettime`, `Adjtimex`, `ClockAdjtime`,
   `Adjtime` (any package).
2. String-literal arguments of `exec.Command` / `exec.CommandContext` equal
   to `date`, `hwclock`, `chronyc`, `ntpd`, `timedatectl` (basename after last `/`).

Do **not** match identifier `date` / `Date` (`time.Date` is required for era
constants) or identifier `Command` alone.

Runtime `TestHostClockUnchanged`: `|ΔCLOCK_REALTIME − ΔCLOCK_MONOTONIC| < 50ms`
across a 1000-packet flood. `unix.ClockGettime` / `syscall.ClockGettime` is
allowed **only** in `_test.go` to *read* clocks.

## Consequences

- Virtual time is a view formula, not a kernel clock.
- Sibling scratch appliances stay on real time in v1.

## Alternatives considered

- Control-plane time bus into LabLDAP/TacLab: rejected for v1.
- Stepping the VM clock for one tester: contaminates everyone.

## Review triggers

A later inverse repro (lab service clock wrong) only if that process queries NTP.
