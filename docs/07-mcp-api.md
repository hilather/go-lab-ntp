# MCP API

Status: Proposed normative behavior
Owners: Control Plane
Last reviewed: 2026-08-30
Related ADRs: 0004, 0006

Official SDK `github.com/modelcontextprotocol/go-sdk` **v1.7.0**, protocol
**`2026-07-28`**, Streamable HTTP `POST /mcp` with `Stateless: true`.
`allowLegacyClients` defaults false; lab overlays may set true.

Tools are `ntp_*`. Resources are `labntp://…`. Bearer-only. MCP must not
HTTP-call REST. `labntp mcp-stdio` requires `--token-file`.

Generated tool input schemas must not mark ViewSpec zero-default fields
required (`precision`, `rootDelay`, `rootDispersion`, `jitter`, omitted
`offset`, `leap`, `refid`). Those omit to Go zero on REST/MCP JSON apply
(YAML document decode still materializes `precision: -20`). `mode` and
`stratum` stay required at the schema gate (`stratum` `0` fails
`config.validateView`). Real validation remains `config.validateView`;
the adapter does not implement a second domain gate.
