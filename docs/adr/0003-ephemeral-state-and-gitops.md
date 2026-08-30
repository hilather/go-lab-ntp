# ADR 0003: Ephemeral runtime state and GitOps bootstrap

Status: Accepted
Date: 2026-08-30

## Context

Family appliances (LabDNS, LabMITM) treat YAML as GitOps desired state. Runtime
rings are not persisted. LabNTP adds materialized view `epoch` (virtual) that
must appear on GET/export but must not be written back to the bootstrap file.

YAML KnownFields cannot alias. The feature request spells `minpoll`,
`maxpoll`, and `refid` (not camelCase `minPoll` / `refID`). LabMITM muscle
memory would otherwise silently fail.

Schema paths that cannot be aliased:

- `spec.auth` (not `spec.management.auth`)
- `spec.ui.enabled` (not `spec.management.ui`)
- `spec.management.allowedOrigins` (LabDNS spelling; reject `originAllowlist`)

## Decision

Views are desired-state, not sticky RAM. Reset rereads bootstrap YAML and
never writes it. Materialized `epoch` is compile-time only.

Keep the FR wire names `minpoll`, `maxpoll`, `refid`. KnownFields rejects
the camelCase forms. This is an explicit exception to family camelCase.

## Consequences

- GitOps overlays stay the source of truth.
- Operators who write `minPoll:` get `unknown_field`, not a silent default.

## Alternatives considered

- Longest-prefix client groups (LabDNS): rejected (ADR 0009).
- Dual spellings: KnownFields cannot alias.

## Review triggers

A later additive v1alpha1 field (LabMITM ADR 0008 pattern).
