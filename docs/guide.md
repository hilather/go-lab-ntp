# User guide

LabNTP is an NTP server for labs. Clients ask “what time is it?” LabNTP answers with a time that depends on **who asked**. The host clock stays where it is.

This page is the operator manual. The [README](../README.md) is the front door. The numbered files in `docs/` are the precise rules.

## What you can do with it

- Give one tester a clock five minutes behind so Kerberos tickets look expired or not-yet-valid.
- Give another tester a clock in 2035 so a certificate looks expired.
- Freeze a clock on a known instant so a replay is the same every run.
- Run time at 2×, 0.5×, or backwards for TOTP and cache-expiry tests.
- Do all of that from a YAML file, a REST call, or an MCP tool, on the same process that answers NTP.

You cannot use it as a production time source. You cannot point it at an NTP pool. You cannot make it set the machine clock.

## Install

You need Go 1.26.

```bash
git clone https://github.com/hilather/go-lab-ntp.git
cd go-lab-ntp
go build -o bin/labntp ./cmd/labntp
./bin/labntp version
./bin/labntp help
```

The container image is `ghcr.io/hilather/labntp`. See [Deployment](11-deployment.md) for ports and capabilities.

## Five minutes to a working server

On a laptop, bind NTP on `:1123`. Port 123 needs `CAP_NET_BIND_SERVICE`.

```bash
./bin/labntp validate --config testdata/config/valid/full.yaml
./bin/labntp serve \
  --config testdata/config/valid/full.yaml \
  --ntp-listen=:1123 \
  --management-listen=off
```

In another terminal:

```bash
./bin/labntp query --server 127.0.0.1:1123
```

That is the whole data plane: load YAML, serve NTP, query it. Management REST, MCP, and the UI stay off until you pass `--management-listen=:8088`.

## How YAML becomes a clock

The bootstrap file is the desired state. LabNTP does not watch the file after start. It loads it once, then serves from an in-memory snapshot.

1. **Decode.** One UTF-8 document, YAML or JSON, at most 1 MiB. Unknown fields fail. Multi-doc files fail.
2. **Normalize.** Defaults fill in. Duration strings become durations. `bodyLimit: 1MiB` becomes `1048576`.
3. **Validate.** Catch-all CIDRs, forbidden fields, stratum, leap, rate bounds, reserved names (`chrony`, `ntpd`, …). Secret **paths** must be present if tokens or keys are declared; the files themselves are checked at compile, not at validate.
4. **Compile.** First-match filter slice, view epochs, optional `ntp.keys`, revision hash. A missing key file fails here.

`labntp validate --config path` runs 1–3 (and reports the revision). `labntp canonicalize --config path --format yaml|json` prints the document after 1–3. `labntp serve --config path` runs all four and binds UDP.

CLI flags always win over YAML listen addresses.

```bash
./bin/labntp validate --config testdata/config/valid/full.yaml
# ok revision=sha256:<hex>

./bin/labntp canonicalize --config testdata/config/valid/full.yaml --format yaml
```

Bare `offset: 5` is rejected. Write `offset: 5s`. Omitted `rate` on `mode: rate` fails; explicit `rate: 0` is a freeze at the epoch.

The precise field list lives in [State and configuration](04-state-and-configuration.md).

## Write a view

A filter is a name, a CIDR list, and a view. Walk the list top to bottom. The first **enabled** filter that contains the client IP is the one that answers. Disabled filters still occupy names. Duplicate names fail validate.

Always keep a catch-all for IPv4 and IPv6 at the bottom:

```yaml
    - name: default
      enabled: true
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
        stratum: 2
        refid: GPS
```

### Kerberos (offset)

```yaml
    - name: tester-a-kerberos
      enabled: true
      match:
        cidrs: ["10.99.42.20/32"]
      view:
        mode: offset
        offset: -6m
        leap: none
        stratum: 1
        refid: LOCL
```

That client is six minutes in the past. Everyone else still hits `default`.

### Expired or not-yet-valid certificates (absolute)

`absolute` steps to a timestamp, then follows real elapsed time at rate 1.0. It is not a stopped clock.

```yaml
    - name: tester-b-expired-cert
      enabled: true
      match:
        cidrs: ["10.99.42.30/32"]
      view:
        mode: absolute
        absolute: "2035-01-01T00:00:00Z"
        stratum: 1
```

### Deterministic replay (freeze)

```yaml
    - name: stopped
      enabled: true
      match:
        cidrs: ["10.0.0.2/32"]
      view:
        mode: freeze
        freezeAt: "2020-01-01T00:00:00Z"
        stratum: 1
        refid: LOCL
```

### TOTP / cache expiry (rate)

`rate` is a multiplier on elapsed time. `|rate| ≤ 100`. `rate: 0` holds the epoch.

```yaml
    - name: fast-totp
      enabled: true
      match:
        cidrs: ["10.99.42.40/32"]
      view:
        mode: rate
        rate: 2
        stratum: 1
```

Put specific prefixes **above** the catch-all. Overlaps are legal and produce a warning; first match still wins.

## Turn on the control plane

```bash
./bin/labntp serve \
  --config testdata/config/valid/full.yaml \
  --ntp-listen=:1123 \
  --management-listen=:8088
```

Health needs no token:

```bash
curl -sS http://127.0.0.1:8088/v1/health/live
curl -sS http://127.0.0.1:8088/v1/health/ready
```

Everything else needs `Authorization: Bearer …`. Basic auth is rejected. The testdata admin token is in `testdata/tokens/admin` (32+ bytes, newline-trimmed).

```bash
export TOKEN=$(tr -d '\n' < testdata/tokens/admin)
export MGT=http://127.0.0.1:8088
```

If `spec.ui.enabled` is true, open `http://127.0.0.1:8088/`. The UI uses cookie `labntp_session` and header `X-LabNTP-CSRF`. It does not put tokens in `localStorage`.

## State loading APIs

Think of three layers:

| Layer | What it does |
|---|---|
| Bootstrap file | What `serve` and `reset` read. YAML or JSON on disk. Never rewritten by LabNTP. |
| Runtime snapshot | What NTP actually serves. Has a `runtimeRevision`. |
| Control plane | REST `/v1` and MCP `ntp_*`. Read, validate, plan, apply, export, reset. |

`GET /v1/state` is the revision source the operator UI uses. Duration fields on the wire are JSON **strings** (`"0s"`). A bare number is 400.

### Get the live document

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/state"
```

You get `bootstrapRevision`, `runtimeRevision`, `generation`, `drifted`, `loadedAt`, and a redacted `canonical` spec. Secrets are not in the body.

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/state:export?format=yaml"
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/state:export?format=json"
```

Export is the canonical desired state plus drift material. YAML comes back as `application/yaml`. JSON wraps the body.

### Validate YAML you have not applied

REST bodies are JSON. Canonicalize the file, then wrap it as `state`:

```bash
./bin/labntp canonicalize --config my-lab.yaml --format json > /tmp/labntp.json

curl -sS -X POST "$MGT/v1/state:validate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"state\": $(cat /tmp/labntp.json)}"
```

That is a dry run. Nothing is swapped. The response is a plan: previous revision, candidate revision, diff, warnings.

You can also validate a list of operations against the live snapshot (same endpoint, `operations` instead of or in addition to `state`).

### Plan, then apply

Writes need `expectedRevision` (from `GET /v1/state` → `runtimeRevision`, or header `If-Match` / `X-LabNTP-Expected-Revision`). Apply honors `Idempotency-Key`.

Live (apply can change these):

- filters and view fields
- restrict, admission, allowClientCidrs
- query-log size
- management HTTP limits (body, RPS, burst, concurrency)

Reset-only (apply cannot change these):

- listen addresses
- `ntp.nts.enabled`
- `ntp.symmetricKeys.file`
- `spec.auth`

Flags still win over YAML on both serve and reset.

```bash
REV=$(curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/state" | jq -r .runtimeRevision)

curl -sS -X POST "$MGT/v1/changes:plan" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"expectedRevision\": \"$REV\",
    \"reason\": \"preview a freeze\",
    \"operations\": [{
      \"op\": \"upsertFilter\",
      \"filter\": {
        \"name\": \"stopped\",
        \"enabled\": true,
        \"match\": { \"cidrs\": [\"10.0.0.2/32\"] },
        \"view\": {
          \"mode\": \"freeze\",
          \"freezeAt\": \"2020-01-01T00:00:00Z\",
          \"stratum\": 1,
          \"refid\": \"LOCL\"
        }
      }
    }]
  }"
```

Swap `changes:plan` for `changes:apply` when the diff looks right. Send the same body plus `Idempotency-Key` if the client might retry.

Closed verbs:

| `op` | Body field |
|---|---|
| `replaceFilters` | `filters` |
| `upsertFilter` | `filter` |
| `removeFilter` | `name` |
| `replaceRestrict` | `restrict` |
| `replaceAdmission` | `admission` |
| `replaceAllowClientCidrs` | `allowClientCidrs` |
| `replaceQueryLog` | `queryLog` |
| `replaceManagementHTTP` | `managementHTTP` |

Filter CRUD also has resource routes that go through the same compile+swap path:

```bash
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/filters"
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/filters/default"

curl -sS -X PUT "$MGT/v1/filters/stopped" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"expectedRevision\": \"$REV\",
    \"reason\": \"add freeze view\",
    \"filter\": {
      \"name\": \"stopped\",
      \"enabled\": true,
      \"match\": { \"cidrs\": [\"10.0.0.2/32\"] },
      \"view\": { \"mode\": \"freeze\", \"freezeAt\": \"2020-01-01T00:00:00Z\", \"stratum\": 1 }
    }
  }"
```

You cannot delete the last catch-all.

### Preview without sending NTP

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$MGT/v1/views/preview?ip=10.99.42.20"
```

A miss is still 200, with a `reason` instead of a 404. Use this when Docker SNAT would otherwise hide the real client IP.

### Reset to the file

```bash
curl -sS -X POST "$MGT/v1/state:reset" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason":"end of run"}'
```

Reset rereads the bootstrap mount, wipes the query log, and swaps. If the effective listen address changed (after flags), it binds the **new** socket first, then drains the old one. It never writes the YAML file. Materialized `epoch` is not persisted back to disk.

### MCP equivalents

`POST /mcp` with the same bearer. Protocol `2026-07-28`. Stdio: `labntp mcp-stdio --config … --token-file …`.

| Job | Tool |
|---|---|
| Read live state | `ntp_state_get` |
| Dry-run a document or ops | `ntp_state_validate` |
| Dry-run ops on the snapshot | `ntp_change_plan` |
| Apply | `ntp_change_apply` |
| Export YAML/JSON | `ntp_state_export` |
| Reread bootstrap | `ntp_state_reset` |
| Preview an IP | `ntp_views_preview` |
| Filters | `ntp_filters_list`, `ntp_filters_get`, `ntp_filters_put`, `ntp_filters_delete` |

MCP does not HTTP-call REST. Both call `app.Service`. ViewSpec zero-defaults
may be omitted on `ntp_change_apply` (JSON/typed omit → Go zero; YAML
document decode still materializes `precision: -20`). See [MCP API](07-mcp-api.md).

## Who is allowed to ask

`spec.ntp.allowClientCidrs`:

| YAML | Runtime |
|---|---|
| omitted or `null` | Allow the world, plus a `universal_allowlist` warning |
| `[]` | Deny everyone |
| explicit CIDRs | Those prefixes only |

Packets from outside the allow list are ignored before filter match, so a stranger cannot probe virtual time. After that: admission limits, then `restrict` (serve / limited+KoD / ignore), then first-match.

## Docker notes

- Image user is `65532:65532`. Root filesystem is read-only.
- Container NTP listen is `:123`. Host publish default is **10123/udp**.
- Restore only `cap_add: [NET_BIND_SERVICE]` for port 123. Do not treat `EACCES` as “port in use.”
- `--management-listen` defaults **off** in the binary. The image CMD passes `:8088` so healthchecks work.

If two testers share an egress IP, they share a view. That is a **NAT collision**. Docker `userland-proxy` SNATs host-published UDP, which is the usual way it happens. Set `userland-proxy: false`, use macvlan/ipvlan, or talk to `labntp:123` on the compose network. [NTP semantics](02-ntp-semantics.md) covers this.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `validate` rejects a field you just added | Unknown name. Wire names are `minpoll`, `maxpoll`, `refid`, `allowedOrigins`. Not `minPoll` / `min-poll` / `originAllowlist`. |
| `offset: 5` fails | Durations need a unit: `5s`, `-6m`. |
| Serve dies on port 123 | Unprivileged bind. Use `--ntp-listen=:1123` or add `NET_BIND_SERVICE`. |
| Two testers see the same skew | Same source IP after NAT. Preview with `/v1/views/preview?ip=` using the compose-network address. |
| `GET /` is 404 | `spec.ui.enabled: false`, or management is off. |
| Apply 409 / revision error | Snapshot moved. Re-read `GET /v1/state` and send the new `runtimeRevision`. |
| Apply refuses a listen/auth change | Reset-only field. `POST /v1/state:reset` after editing the file (or restart). |
| Health ready is down | NTP not bound, snapshot missing, or management neither bound nor explicitly off. |

## What to read next

- [Architecture](01-architecture.md) — process, packages, invariants
- [Filters and views](03-filters-and-views.md) — first-match and catch-all
- [REST `/v1`](06-rest-api.md) — route list
- [Security](08-security-architecture.md) — bearer, CSRF, origins
- [Implementation design](implementation-design.md) — long form

The host clock never moves. That is an invariant, not a default.
