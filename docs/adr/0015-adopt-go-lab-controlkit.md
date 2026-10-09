# ADR 0015: Adopt go-lab-controlkit

- Status: Accepted
- Date: 2026-10-09

## Context

Management-plane auth, session, origin, rate-limit, and audit primitives were copied in this repo. PR-1 moved them behind one module, and the facades call that module. Nothing observable changes: REST and MCP status and problem codes, problem text, JSON shapes, cookie and header names, published MCP input schemas, audit row JSON, metrics, and operator docs.

Commit 2 recorded the dependency, the facade rules, and the fence. Commit 3 put the module in `go.mod` and swapped the facades.

## Decision

### Dependency

The module is `github.com/hilather/go-lab-controlkit`, Apache-2.0.

PR-1 is developed on pseudo-version `v0.0.0-20261009033931-7fbaa6fee48a`. Commit 3 pins that pseudo-version in `go.mod`. The GitHub PR must repin `go.mod` to `v0.1.0` before it opens. There is no `replace` directive and no committed `go.work` or `go.work.sum`.

### Facade rule (R4)

`cmd`, REST, and MCP keep importing `github.com/hilather/go-lab-ntp/internal/auth` and `internal/audit`. `internal/control/mcp` may import `github.com/hilather/go-lab-controlkit/ratelimit` for the capped limiter only. No other production file imports the module. `cmd` has no controlkit import. REST keeps its limiter.

Facade files:

- `internal/auth/controlkit.go`
- `internal/auth/errors.go`
- `internal/audit/controlkit.go`
- `internal/app/idempotency.go` (storage becomes `idem.Cache`; the `{reason, operations}` marshal stays in this file)
- `internal/control/mcp/auth.go` (capped limiter only; authenticate and authorize stay in this file)

Commit 3 names the session facade `auth.Sessions`. `internal/auth/session.go` defines `type Sessions struct` holding the kit `*session.Store`. `NewStore` returns `*Sessions`. `internal/control/rest` stores `*auth.Sessions`. The method set and signatures stay `SetClock` (nil is a no-op), `Create` (forces class `token`), `Lookup`, `Delete`, `Clear`, `ValidCSRF`, `MaxAge`, and `ExpiresAt`. The sessions rule forbids `type Store struct`, so the wrapper is named `Sessions` and the file is not an allowlist entry.

### Test seam `wrapTokenSource`

`internal/auth/controlkit.go` exports `type TokenSource = authn.TokenSource` and `FromSpecWith(spec, wrap func(TokenSource) TokenSource) (*Verifier, error)`. `FromSpec(spec)` is `FromSpecWith(spec, nil)`.

`FromSpecWith` keeps the pre-facade check order and stops at the first failure, before `authn.Load` sees that failure. Unknown mode returns at `internal/auth/controlkit.go:69`. For each token the pre-pass then checks an empty id (`:84`) and a duplicate id (`:87`). One `Read` follows (`:89`). A read error, or a read that is not exactly one token, returns the unresolved sentence at `:92`. The length floor returns at `:100`. A duplicate value, compared as equal bytes rather than a second digest, returns at `:105`. The duplicate-secret pre-pass compares raw secret bytes where main compared SHA-256 digests; the result is identical and it runs only at load. A role that differs from its trimmed form returns at `:112`, after those secret checks and before `Load`, which trims the role and would accept it. A trimmed unknown role is still the `authn.Load` of the tokens read so far (`:115`), compiled from memory, so a later file is not opened and a valid boot opens each secret once. `wrap` is applied to that per-token file source (`tokenFiles`, `:72`) before the first `Read`. Production passes nil.

`cmd/labntp/serve.go` has an unexported `wrapTokenSource func(auth.TokenSource) auth.TokenSource` at `cmd/labntp/serve.go:60`. Production leaves it nil and calls `auth.FromSpecWith(spec, wrapTokenSource)` at `cmd/labntp/serve.go:137`. The boot driver sets it to count secret-file reads. The variable is a dependency seam with a nil default. It changes no option value.

### Primitives-only rule (R5)

Request order, MCP Basic rejection, REST `setRate`, reset orchestration, and rebind stay in this repo. controlkit supplies the pieces those functions call.

`ManagementRebindOverAPI` variants this repo runs: `move`, `off`, `taken`, `same`.

### Fence (R7)

`scripts/check-auth-fence.sh` runs from `make lint`, and therefore from CI. It uses `git grep -nE` on tracked non-`_test.go` Go files (POSIX ERE) and exits non-zero on any hit outside the allowlist. Each hit is printed as `file:line:rule`.

- `subtle`, scope `internal/auth/` and `internal/control/`: `"crypto/subtle"`.
- `digest`, same scope: `"crypto/sha256"|sha256\.(Sum256|New)\(`.
- `sessions`, scope `internal/auth/` only: `map\[string\]\*?(record|[A-Za-z_]*[Ss]ession[A-Za-z_]*)` and `type[[:space:]]+Store[[:space:]]+struct`.
- `tokenfile`, scope `internal/auth/` only: `os\.(ReadFile|Open|OpenFile)\(|bufio\.NewScanner\(`.
- `byid`, repo-wide non-test Go: `\bPrincipalByID\b`.
- `allowall`, repo-wide non-test Go: `authn\.AllowAll\(`.
- `replace`, scope `go.mod`: `^replace .*go-lab-controlkit`.
- `gowork`: `git ls-files -- go.work go.work.sum` must print nothing. A tracked path is printed as `<path>:1:gowork`.

The allowlist is per rule and per exact path. One entry names one rule and one file. No file is exempt from every rule. `internal/auth/controlkit.go` and `internal/auth/errors.go` are exempt from nothing.

ntp has no cursor signers. There is no `internal/control/rest/cursor.go` and no `internal/control/mcp/cursor.go`, so the `digest` allowlist has no entry. Every rule's allowlist is empty.

SHA-256 outside `internal/auth/` and `internal/control/` is outside `digest`. That includes the idempotency fingerprint, the config revision, and the NTP MAC. `cmd/labntp/mcpstdio.go` `os.ReadFile` is outside `tokenfile`. Before commit 3, a comment in `internal/auth/digest.go` that mentioned crypto/subtle without quotes was not a `subtle` hit. Commit 3 deleted that file.

## Hits on main before the facade swap

These were the hits on `main` before the facade swap. Commit 3 removed them. The list is the historical stdout of `scripts/check-auth-fence.sh`:

```text
internal/auth/digest.go:5:subtle
internal/auth/digest.go:4:digest
internal/auth/digest.go:13:digest
internal/auth/verifier.go:4:digest
internal/auth/session.go:49:sessions
internal/auth/session.go:51:sessions
internal/auth/session.go:76:sessions
internal/auth/session.go:167:sessions
internal/auth/verifier.go:307:tokenfile
```

`byid`, `allowall`, `replace`, and `gowork` printed nothing. Every hit was in a file commit 3 deleted or rewrote as a facade. None of these paths was allowlisted. `make lint` was red on these lines from commit 2 until commit 3 removed them.

| Hit | Commit 3 |
|---|---|
| `internal/auth/digest.go:4` `digest`, `:5` `subtle`, `:13` `digest` | Deletes `internal/auth/digest.go` (the SHA-256 and subtle body). `MinTokenBytes` moves to `internal/auth/controlkit.go`, which keeps neither the digest nor the subtle compare. |
| `internal/auth/verifier.go:4` `digest`, `:307` `tokenfile` | Rewrites token loading into the `internal/auth/controlkit.go` facade. Deletes the `crypto/sha256` import and `readSecretFile`'s `os.ReadFile`. |
| `internal/auth/session.go:49` `sessions`, `:51` `sessions`, `:76` `sessions`, `:167` `sessions` | Rewrites `internal/auth/session.go` as the `auth.Sessions` wrapper. Removes `type Store struct` and `map[string]*record`. `type Sessions struct` does not match the sessions rule. |

## Option-to-test map

Filled from the option-sensitivity run on `cea3796` with this commit's `TestOptionPin*` files copied into each scratch worktree. Flips in `internal/auth/controlkit.go` and `internal/auth/errors.go` were re-run on this tree. The other flips are the `2213055` results; those files did not change. Each row is one facade option. The run flipped that option in a detached worktree and ran `go test ./...`. Class is the plan section 8 class when that section assigns one. A dash means section 8 does not assign one. "No test" means the flip is unobservable for the reason in the row, or the option is unset. 85 flips and 3 unset rows: 59 flips failed a test, 26 passed. Nine of those 26 are masked by the pre-pass above: `SkipMissing`, `DNSDefaults`, `MinSecretBytes`, and the `mapLoadErr` sentences for unknown mode, empty id, duplicate id, a short secret, a duplicate value, and an unresolved file. Six of those nine were already masked at `2213055`. The length floor and the `mapLoadErr` copies for a short secret and a duplicate value became unobservable when this tree's pre-pass started returning them before `Load`. `Duplicates` is not among those passes: the pre-pass masks the load-time duplicate sentence, and bearer lookup still changes when the field flips (`TestOptionPinSpacedSecretRejected`). The other 17 passes are unobservable for the reason in the row. `TestOptionPin*` tests cover flips the `b95ad2d` run did not catch and that stay observable. Reapplying each of those patches in a scratch worktree made the named test fail.

| Option | Value ntp sets | Class | Fails when flipped |
|---|---|---|---|
| `FileOpts.Line` | `FirstNonCommentLine` (`internal/auth/controlkit.go:490`) | product | `TestCharacterizeLoadText`, `TestCharacterizeSecretLinePadding` |
| `FileOpts.Resolve` | `AsGiven` (`internal/auth/controlkit.go:491`) | product | No test. With `BaseDir` empty, `ConfigDirIfRelative` still opens the ref as given. |
| `FileOpts.BaseDir` | `""` (`internal/auth/controlkit.go:492`) | — | No test. `AsGiven` does not consult `BaseDir`. |
| `FileOpts.SkipMissing` | `false` (`internal/auth/controlkit.go:493`) | product | No test. Enforced first by the ntp pre-pass at `internal/auth/controlkit.go:90`: a `Read` that errors, or that does not return one token, is the unresolved sentence, so `SkipMissing` true never changes the load. Kit option is defence in depth; flip unobservable. |
| `FileOpts.Harden` | `false` (`internal/auth/controlkit.go:494`) | starred | `TestOptionPinHardenAllowsOversizeSecret` |
| `FileOpts.TrimRef` | `false` (`internal/auth/controlkit.go:495`) | — | `TestOptionPinTrimRefLeavesPadding` |
| empty `spec.auth.mode` | `ModeBearer`, text `bearer` (`internal/auth/controlkit.go:442`, `authn.ModeBearer` at `:446`) | — | `TestCharacterizeLoadText` |
| empty mode text rewrite | same site, text left `""` while mode stays bearer | — | No test. `ModeText` is not on the wire when the mode is bearer. |
| `Source` | `PerTokenFiles`, one token per `Read` (`internal/auth/controlkit.go:526`) | — | `TestCharacterizeBootBoundSecret`, `TestBootManagementOffNoSecretRead`, `TestServeUIEnabledIsHTML`, `TestServeUIDisabledIs404`, `TestIdentityChangeClearsSessions`, `TestManagementRebindOverAPIMove`, `TestManagementRebindOverAPIOff`, `TestManagementRebindOverAPITaken`, `TestManagementRebindOverAPISame`, `TestRESTResetRebindsManagementWithoutSelfDrain`, `TestCharacterizeLoadText`, `TestCharacterizeSecretLinePadding`, `TestCharacterizePaddedRole`, `TestCharacterizeScopeMatrix`, `TestLoadOrderDuplicateIDMissingFile`, `TestLoadOrderShortSecretThenMissingFile`, `TestLoadOrderPaddedRoleShortSecret`, `TestLoadOrderPaddedRoleMissingFile`, `TestLoadOrderPaddedRoleDuplicateSecret`, `TestOptionPinHardenAllowsOversizeSecret`, `TestFromSpecFileRefAndMinBytes`, `TestResetUnreadableSecretKeepsSnapshotAndVerifier`, `TestResetReadableDemotionRevokesOldBearer`, `TestCharacterizeResetFailureText`, `TestKittestResetZeroTokens`, `TestKittestResetUnreadableSecret` |
| `Duplicates` | `RejectDuplicateValue` (`internal/auth/controlkit.go:459`) | — | `TestOptionPinSpacedSecretRejected`. The pre-pass at `internal/auth/controlkit.go:102-106` compares secret bytes and returns the duplicate-value sentence before `Load`, so that load-time sentence stays masked. The field is still copied onto the kit material and changes bearer lookup: a presented secret that contains a space or a tab is rejected unless `Duplicates` is `FirstMatchWins`. A secret of at least 32 bytes with an internal space or tab loads through `FromSpec` and does not authenticate, which is main's behaviour. This is not dns's starred `FirstMatchWins`. `TestCharacterizeLoadText` locks the pre-pass sentence. |
| `MinSecretBytes` | `MinTokenBytes` (32) (`internal/auth/controlkit.go:460`) | — | No test. The pre-pass at `internal/auth/controlkit.go:100` already rejects a secret shorter than 32, so the flip to 0 is a no-op, and 32 matches the pre-pass. A floor above 32 rejects a 32-byte secret that `FromSpec` accepts, including `TestCharacterizeLoadText`. This is not the starred dns value 0. `TestCharacterizeLoadText` and `TestLoadOrderShortSecretThenMissingFile` lock the pre-pass sentence. |
| `WarnBelowBytes` | `0` (`internal/auth/controlkit.go:461`) | — | No test. Warnings are not exposed by the facade. |
| `Accept` | `nil` on `FromSpec` (`internal/auth/controlkit.go:462`) | product | `TestCharacterizeLoadText`, `TestZeroTokensRefuseListen`, `TestMCPSchemaGapProbe`, `TestStdioSecretRemovalDropsActor`, `TestCharacterizeResetFailureText` |
| `RequireListen` | `BearerNeedsToken(true)` (`internal/auth/controlkit.go:231`) | product | `TestCharacterizeLoadText`, `TestKittestResetZeroTokens` |
| `PathPrefix` | `"spec.auth"` (`internal/auth/controlkit.go:463`) | — | `TestCharacterizeLoadText`, `TestCharacterizePaddedRole`, `TestCharacterizeScopeMatrix`. The pre-pass paths are hardcoded `spec.auth`, so the length floor no longer fails this flip. `Load` errors still carry this prefix. A trimmed unknown role, including `Administrator` in `TestCharacterizePaddedRole`, is what fails. |
| `RejectEmptyRole` | `false` (`internal/auth/controlkit.go:465`) | — | `TestCharacterizeScopeMatrix` |
| `RejectBlankTokens` | `false` (`internal/auth/controlkit.go:466`) | — | No test. `FirstNonCommentLine` already drops a blank line, so the flag never sees one. |
| `LocalhostIsLoopback` | `true` (`internal/auth/controlkit.go:467`) | accidental-owned | `TestCharacterizeDevLoopbackRemoteAddr` |
| `ManagementBound` | `false` (`internal/auth/controlkit.go:468`) | — | No test. It only affects `SpecHash`, which ntp does not call. |
| `Basic` | `nil` (`internal/auth/controlkit.go:469`) | — | No test. Bearer mode does not activate a username-only `BasicSpec`. |
| `DNSDefaults` | `nil` (`internal/auth/controlkit.go:470`) | — | No test. An empty id is rejected by the pre-pass at `internal/auth/controlkit.go:84` before `Load`, so `EmptyID` never applies. `EmptyRoleAndScopes` `administrator` matches scope `EmptyRole`, so this flip does not change `Expand`. Kit option is defence in depth; flip unobservable. |
| `scope.Table` roles | administrator is `allScopes()` (`internal/auth/controlkit.go:479`) | product | `TestManagementRebindOverAPIMove`, `TestManagementRebindOverAPIOff`, `TestManagementRebindOverAPITaken`, `TestManagementRebindOverAPISame`, `TestRESTResetRebindsManagementWithoutSelfDrain`, `TestCharacterizeDevLoopbackRemoteAddr`, `TestCharacterizeScopeMatrix`, `TestStaticBearer`, `TestChangeApplyOmitsZeroDefaultViewFields`, `TestMCPSchemaGapProbe`, `TestStdioFixedActorFollowsTokenDowngrade`, `TestApplyManagementHTTPTightensLimiter`, `TestApplyLowerBodyLimitRejectsOversizedMCPPost`, `TestApplyLowerBodyLimitRejectsOversizedRealMCPPost`, `TestApplyZeroBodyLimitRestoresDefault`, `TestCharacterizeRESTAuthText`, `TestCharacterizeResetFailureText`, `TestCharacterizeLoopbackSessionActorClass`, `TestFilterPutRoundTripDurationStrings`, `TestRejectUnknownMutationFields` |
| `EmptyRole` | `administrator` (`internal/auth/controlkit.go:481`) | product | `TestCharacterizeScopeMatrix` |
| `ExplicitReplacesRole` | `true` (`internal/auth/controlkit.go:482`) | product | `TestCharacterizeScopeMatrix` |
| `AllowUnknownRoleExplicit` | `false` (`internal/auth/controlkit.go:483`) | product | `TestCharacterizeScopeMatrix` |
| `WildcardScope` | `ntp.admin` (`internal/auth/controlkit.go:484`) | product | No test. `HasScope` hardcodes `model.ScopeNTPAdmin` (`internal/auth/principal.go:33`) and does not read this field. |
| `Unmapped` | unset. Authorize stays on `AuthorizeScopes` (`internal/auth/principal.go:46`). No `scope.Gate`. | starred | No test. Same shape as `FirstCapOnly`: the option is not a value this PR sets. |
| `FirstCapOnly` | unset. Authorize stays on `AuthorizeScopes`. One capability per tool, so the difference is unobservable. | starred | No test. |
| `UnknownResource` | unset. No `scope.Gate`. | — | No test. |
| `CookieName` | `labntp_session` (`internal/auth/session.go:10`) | product | `TestCharacterizeSessionCookieFlags`, `TestSessionCookieAndCSRF` |
| `CSRFHeader` | `X-LabNTP-CSRF` (`internal/auth/session.go:12`) | product | `TestCharacterizeSessionCookieFlags`, `TestSessionCookieAndCSRF` |
| session idle | `4h` (`internal/auth/session.go:25`) | product | `TestCharacterizeSessionCookieFlags`, `TestCharacterizeSessionTTL` |
| session absolute | `12h` (`internal/auth/session.go:26`) | product | `TestCharacterizeSessionCookieFlags`, `TestCharacterizeSessionTTL` |
| session max | `64` (`internal/auth/session.go:27`) | product | `TestCharacterizeSessionCookieFlags`, `TestCharacterizeSessionAtCap` |
| `AtCap` | `EvictOldest` (`internal/auth/controlkit.go:312`) | product | `TestCharacterizeSessionAtCap` |
| `IDShape` | `SeparateCookieSecret` (`internal/auth/controlkit.go:313`) | product | `TestOptionPinSeparateCookieSecret` |
| `CSRFCompare` | `DigestConstantTime` (`internal/auth/controlkit.go:314`) | product | `TestOptionPinCSRFCompareCase` |
| `origin.Match` | `FoldTrimSlash` (`internal/auth/controlkit.go:275`) | accidental-owned | `TestCharacterizeOriginText` |
| `HostParse` | `URLParse` (`internal/auth/controlkit.go:276`) | — | `TestOptionPinOriginURLParseUserinfo` |
| `LocalhostFold` | `false` (`internal/auth/controlkit.go:277`) | accidental-owned | `TestCharacterizeOriginText` |
| `ListUnionsLoopback` | `true` (`internal/auth/controlkit.go:278`) | accidental-owned | `TestCharacterizeOriginText`, `TestOptionPinOriginURLParseUserinfo` |
| `ZonedLoopback` | `false` (`internal/auth/controlkit.go:279`) | — | `TestOptionPinOriginZonedLoopbackDenied` |
| `Sentinels` | `nil` (`internal/auth/controlkit.go:280`) | — | `TestOptionPinOriginStarNotSentinel` |
| MCP `DefaultRate` | `config.DefaultRequestsPerSecond` (32) (`internal/control/mcp/auth.go:77`) | accidental-owned | `TestOptionPinMCPDefaultRate` |
| MCP `DefaultBurst` | `config.DefaultBurst` (64) (`internal/control/mcp/auth.go:78`) | accidental-owned | `TestCharacterizeMCPLimiterCapAndDenyOrder`, `TestOptionPinMCPDefaultRate` |
| MCP `NegativeRate` | `NegativeRateDisabled` (`internal/control/mcp/auth.go:79`) | accidental-owned | The named constant is the only legal value. `0` panics in `newLimiter` (`ratelimit.NewKeyed: ratelimit: negative rate meaning is required`). A panic aborts that package's test binary, so `go test ./...` names the first hit in each package: `TestIdentityChangeClearsSessions`, `TestChangeApplyOmitsZeroDefaultViewFields`, `TestApplyLowerBodyLimitRejectsOversizedRealMCPPost`, and the `cmd/labntp` package. `TestCharacterizeMCPLimiterCapAndDenyOrder` panics the same way when run directly. |
| MCP `ZeroRate` | `ZeroRateUseDefault` (`internal/control/mcp/auth.go:80`) | accidental-owned | `TestCharacterizeMCPLimiterCapAndDenyOrder`, `TestOptionPinMCPDefaultRate` |
| MCP `Burst` | `BurstDefaultOnZero` (`internal/control/mcp/auth.go:81`) | accidental-owned | `TestCharacterizeMCPLimiterCapAndDenyOrder` |
| MCP `IdleFloor` | `30s` (`internal/control/mcp/auth.go:83`) | — | No test. With burst at least 1, `Allow` refills a token before either cutoff can change the result. |
| MCP `IdleRefillFactor` | `4` (`internal/control/mcp/auth.go:84`) | — | No test. Same refill-before-cutoff limit as `IdleFloor`. |
| MCP `MaxKeys` | `1024` (`internal/control/mcp/auth.go:85`) | — | `TestCharacterizeMCPLimiterCapAndDenyOrder`, `TestEvictOldestEmptyKey`. The starred `MaxKeys` 0 is not a value ntp sets. The kit rejects `MaxKeys <= 0`. REST is local and uncapped. |
| MCP `Now` | `nil` (`internal/control/mcp/auth.go:86`) | — | `TestOptionPinMCPClockRefills`, `TestOptionPinMCPDefaultRate` |
| REST `setRate` while disabled | return without writing (`internal/control/rest/auth.go:141`) | product | No test. `allow` returns before it reads the stored rate, and `disabled` never clears. |
| REST `setRate` rate `<= 0` | substitute 32 (`internal/control/rest/auth.go:144`) | product | `TestCharacterizeRESTLimiterCtorAndSetRate` |
| REST `setRate` burst `<= 0` | substitute 64 (`internal/control/rest/auth.go:147`) | product | `TestCharacterizeRESTLimiterCtorAndSetRate` |
| REST idle floor | `30s` (`internal/control/rest/auth.go:189`) | — | `TestOptionPinRESTIdleFloor` |
| REST idle factor | `4` (`internal/control/rest/auth.go:191`) | — | `TestCharacterizeRESTLimiterCtorAndSetRate` |
| idempotency max | `256` (`internal/app/svc.go:23`) | — | `TestCharacterizeIdempotency` |
| idempotency eviction | `LRU` (`internal/app/idempotency.go:32`) | product | `TestCharacterizeIdempotency` |
| idempotency fingerprint | object marshal of `{reason, operations}` (`internal/app/idempotency.go:118`) | — | No test. An `idem.Fingerprint` array hash still replays and still conflicts. The hex is not pinned. |
| ring max | `DefaultMax` 128 (`internal/audit/controlkit.go:11`) | — | `TestCharacterizeAuditRow` |
| `SetID` | writes `e.ID` (`internal/audit/controlkit.go:31`) | — | `TestCharacterizeAuditRecord`, `TestCharacterizeAuditRow`, `TestRingAppendListGet` |
| `IsDenied` | `nil` (`internal/audit/controlkit.go:32`) | — | No test. `DeniedShare` is 0, and the flood guard runs only when the share is positive and `IsDenied` is set. |
| `DeniedShare` | `0` (`internal/audit/controlkit.go:33`) | 0 until C2 | No test. `IsDenied` is nil, so the share is not consulted. |
| `DefaultList` | `100` (`internal/audit/controlkit.go:34`) | — | No test. `0` means every row, and `MaxList` 100 then clamps `List` to the same page. |
| `MaxList` | `100` (`internal/audit/controlkit.go:35`) | — | `TestCharacterizeAuditRow` |
| `NewID` | `"aud-" + seq` (`internal/audit/controlkit.go:37`) | — | `TestCharacterizeAuditRecord`, `TestCharacterizeAuditRow` |
| `GetID` | `nil` (`internal/audit/controlkit.go:39`) | — | No test. Nil matches reading `e.ID` after `SetID`. |
| redactor keys | `secret`, `secretref`, `secretfile`, `token`, `password`, `authorization`, `bearer`, `credential`, `credentials`, `apikey`, `api_key`, `privatekey`, `private_key`, `cookie` (`internal/audit/controlkit.go:154`) | product | `TestCharacterizeAuditRow` (dropping `password`) |
| `PEM` | `true` (`internal/audit/controlkit.go:170`) | — | `TestCharacterizeAuditRow` |
| `BearerPrefix` | `false` (`internal/audit/controlkit.go:171`) | — | `TestOptionPinBearerPrefixOff` |
| `ColonLines` | `false` (`internal/audit/controlkit.go:172`) | — | `TestOptionPinColonLinesOff` |
| kerr unknown mode | `"unknown auth mode"` (`internal/auth/errors.go:33`) | product | No test. Enforced first by the ntp pre-pass at `internal/auth/controlkit.go:69`; this `mapLoadErr` sentence is defence in depth; flip unobservable. `TestCharacterizeLoadText` locks the pre-pass sentence. |
| kerr empty id | `"token id is required"` (`internal/auth/errors.go:36`) | product | No test. Enforced first by the ntp pre-pass at `internal/auth/controlkit.go:84`; this `mapLoadErr` sentence is defence in depth; flip unobservable. `TestCharacterizeLoadText` locks the pre-pass sentence. |
| kerr duplicate id | `"duplicate token id"` (`internal/auth/errors.go:39`) | product | No test. Enforced first by the ntp pre-pass at `internal/auth/controlkit.go:87`; this `mapLoadErr` sentence is defence in depth; flip unobservable. `TestCharacterizeLoadText` locks the pre-pass sentence. |
| kerr short secret | `"token entropy is below 256 bits"` (`internal/auth/errors.go:42`) / `"token secret must be at least 32 bytes"` (`:43`) | product | No test. Enforced first by the ntp pre-pass at `internal/auth/controlkit.go:100`; this `mapLoadErr` sentence is defence in depth; flip unobservable. `TestCharacterizeLoadText`, `TestLoadOrderShortSecretThenMissingFile`, `TestLoadOrderPaddedRoleShortSecret`, `TestCharacterizeBootBoundSecret`, and `TestBootManagementOffNoSecretRead` lock the pre-pass sentence. |
| kerr duplicate value | keeps a `"token value matches "` prefix (`internal/auth/errors.go:46`) | product | No test. Enforced first by the ntp pre-pass at `internal/auth/controlkit.go:105`; this `mapLoadErr` copy is defence in depth; flip unobservable. `TestCharacterizeLoadText` and `TestLoadOrderPaddedRoleDuplicateSecret` lock the pre-pass sentence. |
| kerr unknown role | `"unknown role"` (`internal/auth/errors.go:52`) | product | `TestCharacterizeLoadText`, `TestCharacterizePaddedRole`, `TestCharacterizeScopeMatrix`. An untrimmed role is rejected by the pre-pass at `internal/auth/controlkit.go:112` with the same sentence, after that token's read, length, and duplicate-value checks, and before `Load`. A trimmed unknown role, including `Administrator`, still reaches this `mapLoadErr` case. |
| kerr unresolved | `"token secret is unavailable"` (`internal/auth/errors.go:55`) / `"token secret file does not resolve"` (`:56`) | product | No test. Enforced first by the ntp pre-pass at `internal/auth/controlkit.go:92`; this `mapLoadErr` sentence is defence in depth; flip unobservable. `TestCharacterizeLoadText` locks the pre-pass sentence. |
| kerr unauthenticated | `"authentication required"` (`internal/auth/errors.go:75`) | product | `TestCharacterizeDevLoopbackRemoteAddr`, `TestCharacterizeMCPAuthText`, `TestCharacterizeRESTAuthText`. This row is `mapUnauth` only. |
| kerr session mint | `"session material unavailable"` (`internal/auth/errors.go:88`) | product | No test. The sentence is unreachable without a rand failure, and this tree has no mutation hook. |
| kerr origin | `"origin is not allowed"` (`internal/auth/controlkit.go:283`) | product | `TestCharacterizeOriginText`, `TestCharacterizeMCPAuthText`, `TestCharacterizeRESTAuthText` |
| kerr rate limit | `"too many management requests"` (`internal/control/mcp/auth.go:105`, `internal/control/rest/auth.go:179`) | product | `TestCharacterizeMCPLimiterCapAndDenyOrder`, `TestOptionPinMCPDefaultRate`, `TestOptionPinMCPClockRefills`, `TestCharacterizeRESTLimiterCtorAndSetRate`, `TestOptionPinRESTIdleFloor` |
| kerr idempotency | `"idempotency key reused with a different request"` (`internal/app/idempotency.go:48`) | product | `TestCharacterizeIdempotency` |
| kerr missing scope | `"missing scope "` (`internal/auth/principal.go:49`) | product | `TestCharacterizeScopeMatrix`, `TestCharacterizeUnmappedToolAllowed`, `TestCharacterizeFirstCapOnly`, `TestCharacterizeRESTAuthText` |
| kerr CSRF | `"CSRF token is missing or invalid"` (`internal/control/rest/auth.go:101`) | product | `TestCharacterizeRESTAuthText` |
| `Challenge` realm | `"labntp"` (`internal/auth/controlkit.go:239`) | — | `TestCharacterizeRESTAuthText` |
| `Challenge` basic | `false` (`internal/auth/controlkit.go:239`) | — | `TestOptionPinChallengeNoBasic` |

## Consequences

- The AGENTS.md dependency section names the module, and the fence paragraph names the facade files. `scripts/check-auth-fence.sh` enforces the fence in `make lint`.
- Import tests allow `github.com/hilather/go-lab-controlkit/...` only in the facade files above. REST allows no controlkit import.
- Commit 3 removed the fence hits above. `make lint` is no longer red on those lines.
