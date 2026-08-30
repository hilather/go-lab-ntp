# System Architecture

Status: Proposed normative behavior
Owners: Architecture, Data Plane
Last reviewed: 2026-08-30
Related ADRs: 0001, 0002, 0003, 0004, 0005, 0006, 0007, 0008, 0009, 0010, 0011, 0012, 0013

## Problem statement

Laboratory compose graphs share one VM clock. Scratch appliances do not run
timesyncd. QA still needs **controllable virtual time per client** so one
tester can skew Kerberos, jump a cert, or drift TOTP **without** moving the
lab host clock or colliding with another tester.

**LabNTP** is a single-process Go lab appliance. SUTs speak NTPv3/v4 unicast.
LabNTP answers from a per-IP view compiled from fail-closed YAML. It never
sets the host clock and never queries an NTP pool.

## Naming and artifacts

| Kind | Value |
|---|---|
| Product | LabNTP |
| Repository | `github.com/hilather/go-lab-ntp` |
| Go module | `github.com/hilather/go-lab-ntp` |
| Binary / CLI | `labntp` |
| Image | `ghcr.io/hilather/labntp` (`:local` for compose builds) |
| Container user | `65532:65532` |
| Config schema | `labntp.dev/v1alpha1` |
| Kind | `LabNTP` |
| Native REST | `/v1` |
| MCP | `POST /mcp` |
| UI | `/` (embedded operator SPA when `spec.ui.enabled`) |
| Default NTP bind | container `:123` (local escape `--ntp-listen=:1123`) |
| Default management bind | flag default **off**; image CMD `:8088` |
| Host publish default | `10123/udp` |
| Session cookie | `labntp_session` |
| CSRF header | `X-LabNTP-CSRF` |

## Invariants

1. Two planes, one process. The UDP NTP goroutine never imports
   `internal/control` or `internal/web`. `--management-listen` defaults **off**.
2. The host clock never moves. Production code must not call `settimeofday`,
   `clock_settime`, `adjtimex`, or exec `date`/`hwclock`/`chronyc`/`ntpd`/`timedatectl`.
3. Filter match is list order, first enabled wins. Unmatched packets are dropped.
4. Enabled filters must cover `0.0.0.0/0` **and** `::/0`.
5. YAML is fail-closed (`KnownFields(true)`). Unknown fields reject, including
   omit-style aliases (`minPoll`, `refID`, `originAllowlist`, `min-poll`).
6. Secrets are file refs, never inline.
7. Data plane keeps answering if management is unbound or (later) slow.
8. Ready = NTP UDP bound + snapshot installed + (management bound **or**
   `--management-listen=off`). Ready is not “an NTP client could sync.”

## Process model

One process, one container, no persistent volume, read-only bootstrap YAML
plus optional token file and `ntp.keys`.

```text
SUT UDP/123  -->  ntpserver  -->  ntpwire  -->  first-match filter  -->  ntpview
                     |                                      ^
                     | atomic.Pointer[Snapshot]             |
                     +-------- compiler.Compile <---- YAML -+
```

Management REST/MCP compile a candidate spec and Swap the snapshot.
In-flight packets keep the Snapshot they loaded. The operator SPA is
served from the management HTTP server when `spec.ui.enabled` is true
(`cmd/labntp` wires `rest.Config.UI`; production `internal/control/rest`
must not import `internal/web`).

## Package layout

| Package | Role |
|---|---|
| `cmd/labntp` | CLI |
| `internal/ntpwire` | 48-byte codec, timestamps, MAC, KoD |
| `internal/ntpview` | virtual clock math |
| `internal/ntpkeys` | symmetric key file |
| `internal/ntpserver` | UDP listen, admission, dual-stack |
| `internal/querylog` | last-N ring |
| `internal/compiler` | Normalize+Validate+compile Snapshot |
| `internal/snapshot` | immutable Snapshot + atomic Store |
| `internal/config` | KnownFields decode, duration, bytesize |
| `internal/model` | State/Spec/Filter/View — no wire types |
| `internal/domainerr` | catalog codes |
| `internal/app` | plan/apply/reset/preview |
| `internal/audit` | mutation ring |
| `internal/auth` | bearer + cookie CSRF |
| `internal/capabilities` | frozen REST↔MCP table |
| `internal/control/rest` | `/v1` adapter (must not import `internal/web`) |
| `internal/control/mcp` | `/mcp` adapter |
| `internal/web` | `go:embed` operator SPA |
| `internal/observability` | slog JSON, OpenMetrics |
| `internal/testutil` | fake clock |

## Listen / dual-stack

`net.ListenPacket("udp", addr)`. Client IP is `netip.Addr.Unmap()` before
CIDR match. IPv4-mapped IPv6 must match IPv4 prefixes. v1 does not inspect
packet destination (`IP_PKTINFO` is out). Unicast bind plus no multicast
membership is the broadcast/multicast guard.

UID 65532 vs port 123: bind `:123` returns `EACCES` without
`CAP_NET_BIND_SERVICE`. Image stays non-root. Compose restores only that cap.
Local `go run` uses `--ntp-listen=:1123`. Unit tests bind ephemeral ports.

## Import fence

Production `internal/control/rest` must not import `internal/web`.
`cmd/labntp/serve.go` sets `rest.Config.UI = web.NewHandler(nil)` and
`UIEnabled` from the live snapshot. REST tests may import `web`.

## What is not in this slice

The integrator compose pin stays out of repo (PR 14 / later). Documentation
must not claim GHCR publish or Helm vendor pin until they exist.
