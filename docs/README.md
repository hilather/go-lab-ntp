# Documentation

Operator front door: [README.md](../README.md). Onboarding: [START-HERE.md](../START-HERE.md). Agent rules: [AGENTS.md](../AGENTS.md).

This page is the catalog. Normative design documents win over task summaries.

## Root

| Path | Role |
|---|---|
| [README.md](../README.md) | Product page, quick starts |
| [START-HERE.md](../START-HERE.md) | Onboarding |
| [AGENTS.md](../AGENTS.md) | Mandatory contributor / agent instructions |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | PR workflow |
| [SECURITY.md](../SECURITY.md) | Vulnerability reporting |
| [CHANGELOG.md](../CHANGELOG.md) | Curated history |
| [LICENSE](../LICENSE) | Apache-2.0 |

## Architecture

| Path | Topic |
|---|---|
| [01-architecture.md](01-architecture.md) | Process, packages, invariants |
| [02-ntp-semantics.md](02-ntp-semantics.md) | Wire, modes, NAT collision, userland-proxy, KoD, MAC |
| [03-filters-and-views.md](03-filters-and-views.md) | First-match, catch-all, preview |
| [04-state-and-configuration.md](04-state-and-configuration.md) | YAML, revision, reset, live vs reset-only |
| [05-control-plane-and-parity.md](05-control-plane-and-parity.md) | Registry, REST↔MCP, live vs reset-only |
| [06-rest-api.md](06-rest-api.md) | `/v1` |
| [07-mcp-api.md](07-mcp-api.md) | protocol pin, `ntp_*` tools |
| [08-security-architecture.md](08-security-architecture.md) | bearer, CSRF, allowlist |
| [09-observability.md](09-observability.md) | metrics, health, query ring |
| [11-deployment.md](11-deployment.md) | scratch image, ports, NET_BIND_SERVICE |
| [13-integration-lab.md](13-integration-lab.md) | overlay BOM for mcp-integration-lab |
| [implementation-design.md](implementation-design.md) | Implementation design (source of truth until ADRs amend it) |
| [releases/v1.0.0-rc.1.md](releases/v1.0.0-rc.1.md) | First public candidate notes |

## ADRs

| Path | Topic |
|---|---|
| [0001-use-go.md](adr/0001-use-go.md) | Go |
| [0002-first-party-ntpwire.md](adr/0002-first-party-ntpwire.md) | First-party codec (D2) |
| [0003-ephemeral-state-and-gitops.md](adr/0003-ephemeral-state-and-gitops.md) | GitOps; `minpoll`/`refid` spelling |
| [0004-shared-capability-registry.md](adr/0004-shared-capability-registry.md) | REST↔MCP table (D12) |
| [0005-lab-static-bearer.md](adr/0005-lab-static-bearer.md) | Bearer + CSRF (D10) |
| [0006-pin-mcp-protocol-versions.md](adr/0006-pin-mcp-protocol-versions.md) | MCP 2026-07-28 (D11) |
| [0007-never-set-host-clock.md](adr/0007-never-set-host-clock.md) | Host clock invariant (D14) |
| [0008-absolute-is-step-then-follow.md](adr/0008-absolute-is-step-then-follow.md) | `absolute` vs `freeze` (D5) |
| [0009-first-match-not-longest-prefix.md](adr/0009-first-match-not-longest-prefix.md) | Filter order (D6) |
| [0010-container-123-net-bind-service.md](adr/0010-container-123-net-bind-service.md) | Container port 123 (D4) |
| [0011-rate-epoch-is-virtual.md](adr/0011-rate-epoch-is-virtual.md) | Rate epoch (D19) |
| [0012-ntpd-concatenation-mac.md](adr/0012-ntpd-concatenation-mac.md) | MAC construction (D21) |
| [0013-monotonic-elapsed.md](adr/0013-monotonic-elapsed.md) | Monotonic elapsed (D22) |
| [0014-host-publish-source-preserving-udp.md](adr/0014-host-publish-source-preserving-udp.md) | userland-proxy / NAT collision (D24) |

## Tasks

| Path | Topic |
|---|---|
| [tasks/00-program-board.md](../tasks/00-program-board.md) | PR board |
| [tasks/README.md](../tasks/README.md) | How to read the board |
