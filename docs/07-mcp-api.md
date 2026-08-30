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
