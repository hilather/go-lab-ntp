# Integration lab BOM

Status: Proposed normative behavior
Owners: Lab
Last reviewed: 2026-08-30

This repo publishes an overlay BOM under `examples/`. **No product logic belongs in `mcp-integration-lab`.** Integrator pin is a later follow-up after Helm cuts a tag. Must not block labgraph, fixture packs, mcp-integration-lab #12, or LabMITM UI.

## Locked ports

| Variable | Default | Notes |
|---|---|---|
| `LABNTP_NTP_PORT` | `10123` | FR default; native host 123 remains opt-in |
| `LABNTP_REST_PORT` | `18123` | Management HTTP (REST + MCP) |

## Overlay

- `examples/labntp.yaml` — one `default` follow-real filter; tester filters commented
- `allowClientCidrs` = `10.99.42.0/24` + `127.0.0.0/8` + `::1/128`
- `spec.management.mcp.allowLegacyClients: true`
- Token file `/run/secrets/labntp-token` (0o644 if UID 65532)
- MCPJungle `examples/mcpjungle/servers/labntp.json` interpolates `bearer_token: ${LABNTP_TOKEN}`
- labinfo id `labntp`, connection `ntp-udp`, no secret unless MAC keys

Do not recopy `testdata/config/valid/full.yaml` into the lab overlay.

## Compose exception

Integrator compose (not default `test-container`):

```yaml
user: "65532:65532"
cap_drop: [ALL]
cap_add: [NET_BIND_SERVICE]
security_opt: ["no-new-privileges:true"]
```

Default smoke in this repo uses `--ntp-listen=:1123` and `cap_drop: ALL`.
