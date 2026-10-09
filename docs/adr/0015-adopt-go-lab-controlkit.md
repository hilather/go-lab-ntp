# ADR 0015: Adopt go-lab-controlkit

- Status: Accepted
- Date: 2026-10-09

## Context

Management-plane auth, session, origin, rate-limit, and audit primitives are copied in this repo. PR-1 moves them behind one module and changes nothing observable: REST and MCP status and problem codes, problem text, JSON shapes, cookie and header names, published MCP input schemas, audit row JSON, metrics, and operator docs.

This commit records the dependency, the facade rules, and the fence. The module is not in `go.mod` yet. The facade swap is the next commit.

## Decision

### Dependency

The module is `github.com/hilather/go-lab-controlkit`, Apache-2.0.

PR-1 is developed on pseudo-version `v0.0.0-20261009033931-7fbaa6fee48a`. Commit 3 may require that pseudo-version locally. The GitHub PR must repin `go.mod` to `v0.1.0` before it opens. There is no `replace` directive and no committed `go.work` or `go.work.sum`.

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

Commit 3's `internal/auth/controlkit.go` exports `type TokenSource = authn.TokenSource` and `FromSpecWith(spec, wrap func(TokenSource) TokenSource) (*Verifier, error)`. `FromSpec(spec)` is `FromSpecWith(spec, nil)`. `wrap` is applied to the `PerTokenFiles` source before `authn.Load`.

`cmd/labntp/serve.go` gains an unexported `wrapTokenSource func(auth.TokenSource) auth.TokenSource`. Production leaves it nil and calls `auth.FromSpecWith(spec, wrapTokenSource)` at today's `FromSpec` site. The boot driver sets it to count secret-file reads. The variable is a dependency seam with a nil default. It changes no option value.

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

SHA-256 outside `internal/auth/` and `internal/control/` is outside `digest`. That includes the idempotency fingerprint, the config revision, and the NTP MAC. `cmd/labntp/mcpstdio.go` `os.ReadFile` is outside `tokenfile`. A comment in `internal/auth/digest.go` that mentions crypto/subtle without quotes is not a `subtle` hit.

## Hits on main before the facade swap

Stdout of `scripts/check-auth-fence.sh` on this tree. The matched production lines are unchanged from `main`:

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

`byid`, `allowall`, `replace`, and `gowork` printed nothing. Every hit is in a file commit 3 deletes or rewrites as a facade. None of these paths is allowlisted. `make lint` fails on these lines until that commit.

| Hit | Commit 3 |
|---|---|
| `internal/auth/digest.go:4` `digest`, `:5` `subtle`, `:13` `digest` | Deletes `internal/auth/digest.go` (the SHA-256 and subtle body). `MinTokenBytes` moves to `internal/auth/controlkit.go`, which keeps neither the digest nor the subtle compare. |
| `internal/auth/verifier.go:4` `digest`, `:307` `tokenfile` | Rewrites token loading into the `internal/auth/controlkit.go` facade. Deletes the `crypto/sha256` import and `readSecretFile`'s `os.ReadFile`. |
| `internal/auth/session.go:49` `sessions`, `:51` `sessions`, `:76` `sessions`, `:167` `sessions` | Rewrites `internal/auth/session.go` as the `auth.Sessions` wrapper. Removes `type Store struct` and `map[string]*record`. `type Sessions struct` does not match the sessions rule. |

## Option-to-test map

filled in by commit 4

## Consequences

- The AGENTS.md dependency section names the module, and the fence paragraph names the facade files. `scripts/check-auth-fence.sh` enforces the fence in `make lint`.
- Import tests allow `github.com/hilather/go-lab-controlkit/...` only in the facade files above. REST allows no controlkit import.
- `make lint` is red from this commit until commit 3 removes the hits above. The failure is those fence lines.
