# ADR 0001: Use Go for the service

Status: Accepted
Date: 2026-08-30

## Context

LabNTP combines a latency-sensitive UDP NTP data plane, per-IP virtual clocks,
fail-closed YAML, later REST/MCP, container deployment, race testing, and
fuzzing. The family (LabDNS, LabMail, LabMITM, TacLab) is already Go. The
repo module is `github.com/hilather/go-lab-ntp`. The toolchain pin is Go 1.26.

## Decision

Implement the service in Go. Prefer the standard library for UDP, time, and
crypto. Do not take a third-party NTP library (ADR 0002). Isolate a later
official MCP SDK behind an internal adapter.

## Consequences

- A single static binary deploys as `ghcr.io/hilather/labntp` (scratch, UID 65532).
- Go concurrency fits a UDP read loop plus bounded inflight handlers.
- Race detection and fuzzing support hardening.
- The family CI/Make/docs shape can be copied rather than invented.

## Alternatives considered

- Wrap chrony/ntpd: one global clock; no per-IP views; Python/host daemon on
  scratch. Rejected.
- Rust: a break from the family for the initial team.
- Shared `labappliance` module: FR forbids it; would block this repo on another wave.

## Review triggers

Review when the toolchain pin changes or a new requirement conflicts with an invariant.
