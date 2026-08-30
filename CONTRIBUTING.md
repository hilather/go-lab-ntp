# Contributing

## Toolchain

- Go 1.26 (`go1.26.x`). The module path is `github.com/hilather/go-lab-ntp`.
- `make lint` runs `go vet` and golangci-lint v2.12.2.
- `make security-scan` runs `golang.org/x/vuln/cmd/govulncheck@v1.1.4` (optional until a later PR wires it into CI).

See the [Build and test](README.md) section of the README for the required Make targets.

## Development workflow

1. Choose or create a tracked task on [tasks/00-program-board.md](tasks/00-program-board.md).
2. Read the normative design documents and relevant ADRs.
3. Add or update tests that express the intended behavior.
4. Implement the smallest coherent change.
5. Update all affected documentation.
6. Run local CI-equivalent targets.
7. Submit a reviewable pull request with risk, test, compatibility, and release-note information.

## Pull request requirements

Every pull request must state:

- Problem and intended outcome.
- Scope and explicit non-scope.
- Architectural invariants touched (especially the host-clock invariant).
- Security and abuse considerations.
- Test evidence, including regression tests.
- Configuration and compatibility impact.
- Documentation changed.
- Release-note entry or explicit reason that no externally observable behavior changed.

## Change sizing

Prefer small vertical slices that compile and pass tests. Do not merge partial public APIs, undocumented schema fields, or disabled tests as placeholders. Unimplemented Make targets must **exit 1**.

## Commit and review discipline

- Do not mix broad refactors with protocol changes unless necessary.
- Resolve review findings in code, tests, and documentation rather than only in comments.

## Backward compatibility

`labntp.dev/v1alpha1` is the first config API. Additive fields later require an ADR.
