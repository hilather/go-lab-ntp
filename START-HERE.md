# Start here

LabNTP is a laboratory NTPv3/v4 server with per-IP virtual clocks. Systems
under test speak NTP. LabNTP answers from a compiled view so one tester can
skew time without moving the lab host clock.

If you want to **run** what exists today, stay on this page, then follow the
[README](README.md). If you want to **change** it, read [AGENTS.md](AGENTS.md)
before touching code.

## Five-minute path

1. Install **Go 1.26** and clone this repository.
2. `go build -o bin/labntp ./cmd/labntp`
3. `./bin/labntp version`
4. `./bin/labntp validate --config testdata/config/valid/full.yaml`
5. `./bin/labntp canonicalize --config testdata/config/valid/full.yaml --format yaml`
6. Serve NTP only (`--management-listen` defaults **off**; pass an address to bind REST/MCP):

```bash
./bin/labntp serve \
  --config testdata/config/valid/full.yaml \
  --ntp-listen=:1123 \
  --management-listen=off
```

`labntp query --server 127.0.0.1:1123` is a smoke SNTP client. It is never
imported by the server.

To bind REST/MCP and the operator SPA, pass `--management-listen=:8088` and
keep `spec.ui.enabled: true` (overlay). `spec.ui.enabled: false` leaves
`GET /` as 404. Local Vite: Node 22.14.0, `make web-install`,
`npm --prefix web run dev`. See [docs/12-web-ui.md](docs/12-web-ui.md).

YAML field rules and revisions live in
[docs/04-state-and-configuration.md](docs/04-state-and-configuration.md).
Wire, modes, NAT collision, and `userland-proxy`:
[docs/02-ntp-semantics.md](docs/02-ntp-semantics.md). Filters:
[docs/03-filters-and-views.md](docs/03-filters-and-views.md).

The host clock never moves. That is an invariant, not a default.

## What to read next

| If you are… | Read |
|---|---|
| Running a lab | [README.md](README.md), [docs/01-architecture.md](docs/01-architecture.md) |
| Writing YAML | [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md) |
| Implementing the data plane | [docs/02-ntp-semantics.md](docs/02-ntp-semantics.md), [docs/adr/0002-first-party-ntpwire.md](docs/adr/0002-first-party-ntpwire.md), [docs/adr/0007-never-set-host-clock.md](docs/adr/0007-never-set-host-clock.md) |
| Changing behavior | [AGENTS.md](AGENTS.md), then the normative doc for that area |

The full catalog is in [docs/README.md](docs/README.md).

## For contributors and agents

Before changing code:

1. Read [AGENTS.md](AGENTS.md) completely.
2. Read architecture, NTP semantics, filters, state, and the ADRs for the area:
   [docs/01-architecture.md](docs/01-architecture.md),
   [docs/02-ntp-semantics.md](docs/02-ntp-semantics.md),
   [docs/03-filters-and-views.md](docs/03-filters-and-views.md),
   [docs/04-state-and-configuration.md](docs/04-state-and-configuration.md).
3. Do not invent paths, types, or capability IDs. If an invariant must change, write an ADR first.
