# Repository Instructions for Agents

These instructions apply to every human or AI agent working in this repository. More specific `AGENTS.md` files may add stricter rules but may not weaken this file.

## Required reading

Before modifying code, read:

1. `docs/01-architecture.md`
2. `docs/02-ntp-semantics.md`
3. `docs/03-filters-and-views.md`
4. `docs/04-state-and-configuration.md`
5. Every ADR relevant to the area being changed
6. `docs/implementation-design.md` until a later ADR supersedes a decision

The numbered pack is the source of truth after foundation. Do not invent paths, types, validate rules, or capability IDs. If an invariant must change, write an ADR first.

## Architectural rules

- REST and MCP (later) are adapters. Domain behavior belongs in `internal/app`, `internal/ntpserver`, `internal/ntpview`, `internal/ntpwire`, `internal/compiler`, `internal/config`, or `internal/model`.
- REST handlers and MCP handlers must never implement independent business logic and must never call each other.
- Production files in `internal/control/rest` **must not** import `internal/web`. `cmd/labntp/serve.go` wires `rest.Config.UI` and `UIEnabled` from the live snapshot. Tests in `rest` may import `web`.
- `internal/ntpserver`, `internal/ntpwire`, `internal/ntpview`, and `internal/ntpkeys` **must not** import `internal/control` or `internal/web`.
- `cmd/labntp/query.go` is a CLI-only SNTP client. `internal/ntpserver` and the serve path must not import it.
- The NTP data plane must keep answering if REST/MCP/UI is slow or unbound (`--management-listen=off`).
- Desired state is YAML. Query log and materialized `epoch` are not persisted back to the bootstrap file. Reset rereads bootstrap and never writes it.
- Do not import an NTP library (`beevik/ntp`, `facebook/time`, chrony, ntpd). First-party `internal/ntpwire` only (ADR 0002).
- Direct production deps: `gopkg.in/yaml.v3` and the official MCP SDK. The MCP adapter may import the SDK’s already-pinned `github.com/google/jsonschema-go` only to relax generated tool-input `required` for ViewSpec zero-defaults. No Prometheus client.
- **Never set the LabNTP process / lab host clock** (D14 / ADR 0007). Forbidden selectors: `Settimeofday`, `ClockSettime`, `Adjtimex`, `ClockAdjtime`, `Adjtime`. Forbidden `exec.Command` / `CommandContext` string-literal basenames: `date`, `hwclock`, `chronyc`, `ntpd`, `timedatectl`. Do **not** match identifier `date` / `Date` (`time.Date` is required for era constants). `unix.ClockGettime` is allowed **only** in `_test.go` to *read* clocks.
- Filter match is **list order, first enabled wins**. Longest-prefix does not win (ADR 0009).
- `absolute` is step-then-follow at rate 1.0. `freeze` is the stop-clock mode (ADR 0008).
- `Clock.Now()` must not call `.UTC()` (ADR 0013). Convert to UTC only at NTP encode.
- Symmetric MAC is ntpd concatenation `ALG(key \|\| header)`, not HMAC (ADR 0012).
- YAML wire names `minpoll`, `maxpoll`, `refid` (not `minPoll` / `refID`). KnownFields rejects the camelCase forms (ADR 0003).
- Schema paths: `spec.auth`, `spec.ui.enabled`, `spec.management.allowedOrigins`. `originAllowlist` is unknown.
- Hide third-party YAML library types behind internal adapters.

## Tests and regressions

- Every area must have regression tests.
- A bug fix must begin with or include a test that fails before the fix and passes after it.
- Configuration changes require valid, invalid, reserved-key, normalization, and revision tests.
- Required locks: KnownFields unknown fields, kebab-case, `minPoll`/`refID` reject, omitted vs `rate: 0`, catch-all required, first-match, host clock unchanged, unix 0 NTP timestamp, MAC trailer, oversize drop.
- Never delete or weaken a test merely to make CI pass unless the test is provably incorrect; document the reason in the change.

## CI is mandatory

- All required CI checks must pass before merge and before a release tag is created.
- Do not bypass, skip, mark optional, or administratively override a failing check to ship a change.
- Placeholder Make targets for unimplemented later PRs **exit 1**, not 0.

## Documentation is mandatory

- Update affected architecture, configuration, security, testing, and ADR documents in the same change as the implementation.
- Stale documentation is a defect. Do not claim REST/MCP/UI exist until they do.
- Cross-file links in README and `docs/` may be relative in this repo until a later release pack freezes HTTPS URLs.
- `docs/02-ntp-semantics.md` must contain the phrases `NAT collision` and `userland-proxy`. The host-clock invariant must remain documented.

## Dependencies

- Prefer the Go standard library.
- Pin direct dependencies and review transitive changes.
- Allowed data-plane direct dep: `gopkg.in/yaml.v3`. MCP SDK plus its pinned `jsonschema-go` for adapter input-schema `required` only.
- No Prometheus client. No NTP library.

## Required completion commands

```text
make format
make lint
make test
make test-race
make test-fuzz-smoke
make test-config-compat
make test-docs
make test-changelog
```

If a target does not yet exist, the task that first needs it must add it rather than silently omitting the check. Placeholders must fail closed, not succeed as no-ops.
