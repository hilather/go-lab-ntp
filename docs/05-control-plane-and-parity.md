# Control plane and parity

Status: Proposed normative behavior
Owners: Control Plane
Last reviewed: 2026-08-30
Related ADRs: 0004, 0005, 0006

## One service

REST `/v1` and MCP `/mcp` are adapters. They call `app.Service` only. They
must not import each other and must not implement domain logic. The frozen
capability registry is `internal/capabilities`. MCP tools use the family
prefix **`ntp_*`** (not `labntp_*`). Resources stay `labntp://…`.

`features.list` ids are frozen in `api/mcp/v1.json` and
`testdata/mcp/goldens/features.txt`. The operator SPA (PR 13) must not add
feature ids; `spec.ui.enabled` is not a catalog row.

## Live vs reset-only

`GET /v1/features` / `ntp_features_list` returns `apply: live` or
`apply: reset-only`. Apply cannot change listen addresses, `ntp.nts`,
`ntp.symmetricKeys`, or `spec.auth`. Reset rereads bootstrap, wipes the
query log, never writes the file, and rebinds NTP/HTTP per D8
(bind-new-first). Flags still win after Reset.

## Parity

`make test-parity` checks that every non-REST-only catalog row has a live
`ntp_*` tool and that goldens match the registry.

MCP `tools/call` argument schemas are inferred from Go types. Fields
without `omitempty` become JSON Schema `required`. ViewSpec zero-defaults
(`precision`, `rootDelay`, `rootDispersion`, `jitter`, `offset`, `leap`,
`refid`) are optional on MCP input so they match YAML/REST omit-or-zero
defaults. The adapter relaxes the generated `required` list only; it does
not implement domain logic.
