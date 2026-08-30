# NTP semantics

Status: Proposed normative behavior
Owners: Data Plane
Last reviewed: 2026-08-30
Related ADRs: 0002, 0005, 0007, 0008, 0011, 0012, 0013

## Wire

First-party `internal/ntpwire` only. NTPv3 and NTPv4 share a 48-byte header
(RFC 5905 §7.3). Header octet 0 is `(LI << 6) | (VN << 3) | Mode`.

Admitted: VN in `{3,4}` (echoed), Mode 3 (client). Mode 4 reply. VN 1–2,
mode 7, broadcast, multicast, and symmetric-active/passive are dropped.

UDP read cap `MaxUDPSize = 576`. `len < 48` → `short`. `len > 576` →
`oversize` drop (no reply). With keys off, `48 < n ≤ 576` strips trailing
bytes and still answers SNTP. With keys on, length must be exactly
`48+4+digestLen`.

Zero transmit timestamp (RFC 4330 duplicate/bogus) is dropped (`zero_xmit`).

## Timestamps (32.32)

NTP seconds are seconds since 1900-01-01 UTC, era-truncated. Unix 0 encodes
as NTP seconds `2208988800`.

Served timestamps are clamped to era 0 start … era 1 end (exclusive):
`[1900-01-01T00:00:00Z, 2172-03-15T12:56:32Z)` (D25). Pre-1900 is not
well-defined on the wire. Negative rate rewind that hits the floor stays at
the floor. SNTP clients that assume era 0 will mis-read dates after
2036-02-07T06:28:16Z. The 2035 cert fixture is era 0.

Convert to UTC only at encode. `Clock.Now()` must not call `.UTC()` (ADR 0013).

## Mode formulas

Let `t = Clock.Now()`. Wall UTC: `tWall = t.UTC()`. Elapsed:
`elapsed = t.Sub(epochMono)`.

| Mode | `served(t)` |
|---|---|
| `follow-real` | `tWall` |
| `offset` | `tWall + offset` |
| `absolute` | `absolute + elapsed` (step-then-follow) |
| `freeze` | `freezeAt` |
| `rate` | `epochVirtual + saturatingDuration(elapsed * rate)` then D25 clamp |

`rate: 0` is freeze-at-epoch-virtual. `|rate| ≤ 100`, finite. See ADR 0008
and ADR 0011.

Jitter is a stable wander over the **host** unix second:

```
h = SHA-256(filterName + "\x00" + le64(generation) + le64(hostUnixSecond))
delta = (2u - 1) * jitter
```

The same delta is added to receive, transmit, and reference timestamps of
one packet.

## Kiss-of-death

When `restrict.default: limited` and the per-IP limiter denies and
`restrict.kod: true`, reply KoD RATE: Mode=4, Stratum=0, LI=3,
RefID=`RATE`, originate = client transmit, other timestamps zero. KoD does
not use the client’s virtual clock.

When `kod: false` and limited: silent drop. `restrict.default: ignore`:
silent drop before filter match.

## MAC

ntpd concatenation, not HMAC (ADR 0012): `digest = ALG(key || header[0:48])`
for MD5/SHA1/SHA256. Trailer is `keyid_be32 || digest`. Vectors in
`testdata/packets/mac-*.raw` are generated with that formula so they match
ntpd concatenation.

## NAT collision and userland-proxy

v1 match is IP/CIDR only. Two testers with the same egress IP share a view
(**NAT collision**). Assign distinct source IPs or a `/28` per tester.

Host-publish per-IP isolation requires **source-preserving UDP**. Docker
`userland-proxy` (default true on many daemons) SNATs host-published UDP so
every laptop/VM hitting `${LAB_PUBLIC_HOST}:10123` appears as one bridge IP.
Set `/etc/docker/daemon.json` `{"userland-proxy": false}` then restart
docker, **or** use macvlan/ipvlan. Compose-network clients of `labntp:123`
do not need that.

The host clock never moves. Virtual time is a view formula, not
`settimeofday`.
