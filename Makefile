# LabNTP task runner. Tool versions are pinned; do not use @latest.

GO ?= go
export GOTOOLCHAIN ?= local
export GOPROXY ?= https://proxy.golang.org,direct

GOLANGCI_LINT_VERSION ?= v2.12.2
GOVULNCHECK_MOD ?= golang.org/x/vuln/cmd/govulncheck@v1.1.4
GOLANGCI_LINT_MOD ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: help fmt format lint vet build generate verify-generated test test-race \
	test-fuzz-smoke test-parity test-config-compat test-docs test-container \
	security-scan test-changelog web-install web-test web-build web-embed

help:
	@printf '%s\n' \
		'LabNTP Make targets (Go 1.26; module github.com/hilather/go-lab-ntp)' \
		'  format              go fmt ./...' \
		'  fmt                 alias for format' \
		'  vet                 go vet ./...' \
		'  lint                go vet + golangci-lint $(GOLANGCI_LINT_VERSION)' \
		'  build               go build -o bin/labntp ./cmd/labntp' \
		'  generate            write api/capabilities, openapi, mcp, metrics JSON' \
		'  verify-generated    fail if generate would change those files' \
		'  test                go test ./...' \
		'  test-race           go test -race ./...' \
		'  test-fuzz-smoke     buildinfo + config + ntpwire fuzz corpora' \
		'  test-docs           required documents, metadata, links, and required phrases' \
		'  security-scan       govulncheck' \
		'  test-parity         REST/MCP capability parity goldens' \
		'  test-config-compat  positive+negative v1alpha1 config fixtures' \
		'  web-install         npm ci in web/ (not implemented)' \
		'  web-test            Vitest SPA tests (not implemented)' \
		'  web-build           production Vite build (not implemented)' \
		'  web-embed           copy web/dist into internal/web/dist (not implemented)' \
		'  test-container      build image and check non-root/read-only/no-caps (:1123; gated :123)' \
		'  test-changelog      observable paths require a CHANGELOG.md entry'

fmt: format

format:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint: vet
	$(GO) run $(GOLANGCI_LINT_MOD) run ./...

build:
	$(GO) build -o bin/labntp ./cmd/labntp

generate:
	$(GO) run ./scripts/generate

verify-generated:
	$(GO) run ./scripts/generate -check

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-fuzz-smoke:
	$(GO) test ./scripts/checkdocs -run TestFuzzCorporaPresent -count=1
	$(GO) test ./internal/buildinfo -fuzz=FuzzInfoString -fuzztime=5s -count=1
	$(GO) test ./internal/config -fuzz=FuzzDecode -fuzztime=5s -count=1
	$(GO) test ./internal/ntpwire -fuzz=FuzzParse -fuzztime=10s -count=1

test-docs:
	$(GO) run ./scripts/checkdocs

security-scan:
	$(GO) run $(GOVULNCHECK_MOD) ./...

test-parity:
	$(GO) test ./internal/capabilities ./internal/control/rest ./internal/control/mcp -count=1

test-config-compat:
	$(GO) test ./internal/config -run TestConfigCompat -count=1

web-install:
	@echo 'web-install: not implemented (PR 13)' >&2; exit 1

web-test:
	@echo 'web-test: not implemented (PR 13)' >&2; exit 1

web-build:
	@echo 'web-build: not implemented (PR 13)' >&2; exit 1

web-embed:
	@echo 'web-embed: not implemented (PR 13)' >&2; exit 1

test-container:
	bash scripts/test-container.sh

test-changelog:
	$(GO) run ./scripts/checkchangelog
