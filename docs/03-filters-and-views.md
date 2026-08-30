# Filters and views

Status: Proposed normative behavior
Owners: Data Plane, Compiler
Last reviewed: 2026-08-30
Related ADRs: 0006, 0008, 0009, 0011

## First-match

Walk `spec.filters` in list order. First **enabled** filter whose
`match.cidrs` contains the unmapped client IP wins. Longest-prefix does
**not** override order (ADR 0009). A `/32` must be listed **above** a
covering `/0`.

Duplicate `name` fails validate. Empty `cidrs` on an enabled filter fails
validate. Disabled filters are skipped at match time but still occupy names.

## Catch-all

Among enabled filters, the union of CIDRs must contain `0.0.0.0/0` **and**
`::/0`. Missing either family fails validate/apply with `code: required` on
`spec.filters`. Unmatched packets are dropped.

Client address is `netip.Addr.Unmap()` before match. IPv4-mapped IPv6
(`::ffff:10.99.42.20`) matches `10.99.42.20/32`.

## Overlaps

Overlapping CIDRs are legal. Compile/Plan emit a warning for each pair of
enabled filters whose prefixes intersect. Apply still succeeds.

## Packet decision tree

1. `len > 576` → drop oversize
2. `len < 48` / bad VN / mode ≠ 3 / zero xmit → drop
3. client IP not in `allowClientCidrs` → ignore (no virtual-time leak)
4. global / per-IP admission exceeded → drop admission
5. `restrict.default: ignore` → ignore
6. `restrict.default: limited` over cap → KoD RATE or drop
7. first enabled filter containing IP → serve; none → drop

`allowClientCidrs`:

| YAML | Runtime |
|---|---|
| omitted or `null` | materialize `0.0.0.0/0` + `::/0`; warn `universal_allowlist` |
| `[]` | deny-all |
| explicit CIDRs | used as-is |

## Preview

Management `GET /v1/views/preview?ip=` / `ntp_views_preview` uses the same view math
and does not send NTP. Unmatched / allowlist misses are 200 with a reason,
not 404.
