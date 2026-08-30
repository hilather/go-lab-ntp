# ADR 0009: First-match filters, not longest-prefix

Status: Accepted
Date: 2026-08-30

## Context

LabDNS client groups classify with longest-prefix. The LabNTP feature request
is explicit: list order wins so a tester `/32` **above** a default `/0` is
the intended pattern. Longest-prefix would surprise operators who reorder
for priority.

## Decision

Walk `filters` in list order. First **enabled** filter whose `match.cidrs`
contains the unmapped client IP wins (D6). Longest-prefix does not override
order. Unmatched packets are dropped (no silent follow-real).

Overlapping CIDRs are legal. Compile/Plan emit a **warning**, not an error.

A default catch-all covering `0.0.0.0/0` **and** `::/0` is required among
enabled filters.

## Consequences

- A `/32` listed **below** a covering `/0` does not win. Tests lock that.
- NAT collision: two testers with the same egress IP share a view.

## Alternatives considered

- Longest-prefix (LabDNS `AccessIndex.Classify`): rejected.

## Review triggers

MAC / key-id match (not v1).
