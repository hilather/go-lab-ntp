<p align="center">
  <img src="docs/assets/header.jpg" alt="LabNTP — per-IP virtual clocks" width="100%">
</p>

<p align="center">
  <img src="docs/assets/mark.svg" alt="LabNTP mark" width="56" height="56">
</p>

<h1 align="center">LabNTP</h1>

<p align="center">
  <strong>One NTP server. A different clock for every client.</strong>
</p>

<p align="center">
  Skew Kerberos by five minutes. Jump a certificate into not-yet-valid or expired.
  Drift a TOTP step. Do it from YAML, REST, or MCP — without moving the lab host clock,
  and without colliding with the tester sitting next to you.
</p>

<p align="center">
  <a href="https://github.com/hilather/go-lab-ntp/blob/main/docs/guide.md"><strong>User guide</strong></a>
  ·
  <a href="#quick-start">Quick start</a>
  ·
  <a href="#load-and-check-yaml">YAML loading</a>
  ·
  <a href="#state-apis">State APIs</a>
  ·
  <a href="LICENSE">Apache-2.0</a>
</p>

<p align="center">
  <img alt="Go 1.26" src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white">
  <img alt="NTPv3/v4" src="https://img.shields.io/badge/NTP-v3%20%2F%20v4-4fd6c0?style=flat-square">
  <img alt="Apache 2.0" src="https://img.shields.io/badge/license-Apache--2.0-1c242c?style=flat-square">
  <img alt="Image" src="https://img.shields.io/badge/image-ghcr.io%2Fhilather%2Flabntp-11151b?style=flat-square">
</p>

---

LabNTP is a laboratory NTPv3/v4 server. Systems under test talk NTP the way they always do. LabNTP answers from a **virtual clock bound to the client's IP**. Two testers on the same compose graph can live in two different years. The machine they share keeps the real time.

It belongs with LabDNS, LabMail, and LabMITM. It is **not** a production time source, **not** a chrony or ntpd wrapper, and it never calls `settimeofday`, `clock_settime`, or `adjtimex`.

| | |
|---|---|
| Binary | `labntp` |
| Module | [`github.com/hilather/go-lab-ntp`](https://github.com/hilather/go-lab-ntp) |
| Image | `ghcr.io/hilather/labntp` |
| Config | `labntp.dev/v1alpha1` · kind `LabNTP` |
| Data plane | UDP NTPv3/v4 unicast · container `:123` · host publish **10123** |
| Control plane | REST `/v1` · MCP `/mcp` · operator UI `/` |
| License | Apache-2.0 |

## Why this exists

Lab compose graphs share one VM clock. QA still needs time it can lie about:

- Kerberos tickets that should look five minutes off
- A certificate that is not valid yet, or already expired
- TOTP codes that belong to the next time step
- A clock frozen on a known instant so a replay is deterministic

If you moved the host clock to do any of that, everything else on the box would move with it. LabNTP keeps the lie inside the NTP reply.

## Clock modes

Each filter points at a view. The first enabled filter whose CIDR contains the client wins.

| Mode | What the client sees |
|---|---|
| `follow-real` | The host's wall clock |
| `offset` | Wall clock plus a duration (`-6m`, `5s`, …) |
| `absolute` | Step to a timestamp, then run at rate 1.0 |
| `freeze` | A stopped clock at `freezeAt` |
| `rate` | Time that runs fast, slow, backward, or (`rate: 0`) stays put |

A catch-all for both `0.0.0.0/0` and `::/0` is required. Put specific `/32`s **above** the catch-all. Longest prefix does not win — list order does.

## Quick start

You need **Go 1.26**. Bind `:1123` on a laptop; unprivileged users cannot bind `:123`.

```bash
git clone https://github.com/hilather/go-lab-ntp.git
cd go-lab-ntp
go build -o bin/labntp ./cmd/labntp
./bin/labntp version
```

### Load and check YAML

Desired state is a single YAML (or JSON) document. Unknown fields are rejected. `validate` loads, normalizes, and prints the revision hash. `canonicalize` prints the document the compiler actually uses.

```bash
./bin/labntp validate --config testdata/config/valid/full.yaml
# ok revision=sha256:…

./bin/labntp canonicalize --config testdata/config/valid/full.yaml --format yaml
./bin/labntp canonicalize --config testdata/config/valid/full.yaml --format json
```

`--config` is required. The file may be YAML or JSON.

### Serve NTP

```bash
./bin/labntp serve \
  --config testdata/config/valid/full.yaml \
  --ntp-listen=:1123 \
  --management-listen=off
```

In another terminal:

```bash
./bin/labntp query --server 127.0.0.1:1123
```

`query` is a smoke SNTP client. The server never imports it.

### Serve NTP plus REST, MCP, and the UI

```bash
./bin/labntp serve \
  --config testdata/config/valid/full.yaml \
  --ntp-listen=:1123 \
  --management-listen=:8088
```

Management stays **off** unless you pass an address. The container image CMD binds `:8088` so `HEALTHCHECK` against `/v1/health/ready` works. Set `spec.ui.enabled: true` for the operator UI at `/`. Set it `false` and `GET /` is a 404 problem document.

## A working YAML document

This is the shape `labntp serve --config` loads. A fuller copy lives at [`testdata/config/valid/full.yaml`](testdata/config/valid/full.yaml). The lab overlay is [`examples/labntp.yaml`](examples/labntp.yaml).

```yaml
apiVersion: labntp.dev/v1alpha1
kind: LabNTP
metadata:
  name: lab-time
spec:
  listeners:
    ntp:
      address: ":123"
    management:
      address: ":8088"
      restPath: /v1
      mcpPath: /mcp
  auth:
    mode: bearer
    tokens:
      - id: admin
        role: administrator
        secretFile: /run/secrets/labntp-token
  ui:
    enabled: true
  ntp:
    versions: [3, 4]
    serveMode: unicast
    nts:
      enabled: false
    restrict:
      default: serve
      kod: true
    allowClientCidrs:
      - "10.99.42.0/24"
      - "127.0.0.0/8"
      - "::1/128"
  filters:
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
    - name: tester-b-expired-cert
      enabled: true
      match:
        cidrs: ["10.99.42.30/32"]
      view:
        mode: absolute
        absolute: "2035-01-01T00:00:00Z"
        stratum: 1
    - name: default
      enabled: true
      match:
        cidrs: ["0.0.0.0/0", "::/0"]
      view:
        mode: follow-real
        stratum: 2
        refid: GPS
```

Field names are `minpoll`, `maxpoll`, `refid`. CamelCase (`minPoll`, `refID`) and kebab-case (`min-poll`) are unknown fields and fail validate. Durations are strings (`offset: -6m`), never a bare number. Secrets are file paths, never inline.

How the file becomes a live snapshot is documented in [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md). Everyday use is in the [user guide](docs/guide.md).

## State APIs

YAML on disk is the bootstrap. After serve starts, you read and change live state through REST or MCP. Reset **rereads** the bootstrap file. It never writes it.

Bearer is required on every route except health. Tokens are file-backed and at least 32 bytes.

```bash
export TOKEN=$(tr -d '\n' < testdata/tokens/admin)
export MGT=http://127.0.0.1:8088
```

### Read what is running

```bash
curl -sS "$MGT/v1/health/ready"
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/state"
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/state:export?format=yaml"
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/status"
curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/schema/config"
```

`GET /v1/state` returns the redacted spec plus `bootstrapRevision`, `runtimeRevision`, `generation`, and `drifted`. Use `runtimeRevision` as `expectedRevision` on the next write.

### Validate a candidate without applying it

`POST /v1/state:validate` takes JSON. Canonicalize YAML first if that is what you edit.

```bash
./bin/labntp canonicalize --config testdata/config/valid/full.yaml --format json > /tmp/labntp.json

curl -sS -X POST "$MGT/v1/state:validate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"state\": $(cat /tmp/labntp.json)}"
```

### Plan and apply live edits

Mutations need `expectedRevision`. `Idempotency-Key` is honored on apply. Listen addresses, NTS, key files, and auth are **reset-only** — apply cannot change them.

```bash
REV=$(curl -sS -H "Authorization: Bearer $TOKEN" "$MGT/v1/state" | jq -r .runtimeRevision)

curl -sS -X POST "$MGT/v1/changes:plan" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"expectedRevision\": \"$REV\",
    \"reason\": \"skew tester-a for Kerberos\",
    \"operations\": [{
      \"op\": \"upsertFilter\",
      \"filter\": {
        \"name\": \"tester-a-kerberos\",
        \"enabled\": true,
        \"match\": { \"cidrs\": [\"10.99.42.20/32\"] },
        \"view\": { \"mode\": \"offset\", \"offset\": \"-6m\", \"stratum\": 1, \"refid\": \"LOCL\" }
      }
    }]
  }"

curl -sS -X POST "$MGT/v1/changes:apply" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: skew-a-1" \
  -d "{
    \"expectedRevision\": \"$REV\",
    \"reason\": \"skew tester-a for Kerberos\",
    \"operations\": [{
      \"op\": \"upsertFilter\",
      \"filter\": {
        \"name\": \"tester-a-kerberos\",
        \"enabled\": true,
        \"match\": { \"cidrs\": [\"10.99.42.20/32\"] },
        \"view\": { \"mode\": \"offset\", \"offset\": \"-6m\", \"stratum\": 1, \"refid\": \"LOCL\" }
      }
    }]
  }"
```

Closed apply verbs: `replaceFilters`, `upsertFilter`, `removeFilter`, `replaceRestrict`, `replaceAdmission`, `replaceAllowClientCidrs`, `replaceQueryLog`, `replaceManagementHTTP`.

### Preview time for an IP (no NTP packet)

```bash
curl -sS -H "Authorization: Bearer $TOKEN" \
  "$MGT/v1/views/preview?ip=10.99.42.20"
```

### Reset back to the file on disk

```bash
curl -sS -X POST "$MGT/v1/state:reset" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason":"back to bootstrap"}'
```

### Same work over MCP

When management is bound, `POST /mcp` speaks protocol `2026-07-28`. Tools are `ntp_*`. Resources are `labntp://…`.

| REST | MCP |
|---|---|
| `GET /v1/state` | `ntp_state_get` |
| `POST /v1/state:validate` | `ntp_state_validate` |
| `GET /v1/state:export` | `ntp_state_export` |
| `POST /v1/state:reset` | `ntp_state_reset` |
| `POST /v1/changes:plan` | `ntp_change_plan` |
| `POST /v1/changes:apply` | `ntp_change_apply` |
| `GET /v1/views/preview` | `ntp_views_preview` |

`labntp mcp-stdio --config … --token-file …` is the stdio adapter.

The [user guide](docs/guide.md) walks each of these with copy-paste examples. OpenAPI is [`api/openapi/v1.json`](api/openapi/v1.json).

## Docker

```bash
docker pull ghcr.io/hilather/labntp
```

The image runs as UID `65532`, drops all capabilities, and listens on container `:123/udp` plus `:8088/tcp`. Compose must add **only** `CAP_NET_BIND_SERVICE` if you want port 123. Local `make test-container` uses `--ntp-listen=:1123` instead.

Host-publish per-IP isolation needs **source-preserving UDP**. Docker `userland-proxy` (on by default on many daemons) SNATs host-published UDP, so every laptop hitting `${LAB_PUBLIC_HOST}:10123` looks like one bridge IP. That is a **NAT collision**: two testers share a view. Compose-network clients of `labntp:123`, and `GET /v1/views/preview`, do not have that problem. Details: [docs/02-ntp-semantics.md](docs/02-ntp-semantics.md).

The host clock never moves. Virtual time is a view formula, not a syscall.

## Operator UI

With `spec.ui.enabled: true` and management bound, `/` serves the operator SPA. Session cookie `labntp_session`, CSRF header `X-LabNTP-CSRF`. Tokens are not stored in `localStorage`. See [docs/12-web-ui.md](docs/12-web-ui.md).

## Documentation

| Start here | |
|---|---|
| [User guide](docs/guide.md) | How to run it, write YAML, and use the state APIs |
| [START-HERE.md](START-HERE.md) | Five-minute path and what to read next |
| [docs/README.md](docs/README.md) | Full catalog, including ADRs |

| When you need | Read |
|---|---|
| Architecture | [docs/01-architecture.md](docs/01-architecture.md) |
| Wire, modes, NAT | [docs/02-ntp-semantics.md](docs/02-ntp-semantics.md) |
| Filters | [docs/03-filters-and-views.md](docs/03-filters-and-views.md) |
| YAML pipeline, revision, reset | [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md) |
| REST `/v1` | [docs/06-rest-api.md](docs/06-rest-api.md) |
| MCP | [docs/07-mcp-api.md](docs/07-mcp-api.md) |
| Image and ports | [docs/11-deployment.md](docs/11-deployment.md) |

## Build and test

Go **1.26**. Node **22.14.0** for `web/`.

```bash
make format lint test test-docs test-config-compat test-parity
```

`make test-container` needs Docker. `make web-install web-test web-build` builds the operator UI. Local Vite: `npm --prefix web run dev` (proxies `/v1` to `127.0.0.1:8088`).

## What this is not

- A clock you should point production at
- A replacement for chrony, ntpd, or systemd-timesyncd
- NTS, broadcast, multicast, or a pool client
- Something that will ever set the host clock

## License

[Apache License 2.0](LICENSE).
