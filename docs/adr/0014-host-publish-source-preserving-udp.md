# ADR 0014: Host-publish per-IP views need source-preserving UDP

- Status: Accepted
- Date: 2026-08-30

## Context

LabNTP filters on SUT source IP. Docker `userland-proxy` SNATs host-published UDP.

## Decision

Document **NAT collision** and `userland-proxy`. Compose-network and `views.preview` stay the reliable paths without source preservation. Default host port stays **10123**. Do not add a Go userland-proxy probe.

## Consequences

Integrator AGENTS.md / daemon.json owns `userland-proxy: false` or macvlan.
