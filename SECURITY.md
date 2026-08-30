# Security Policy

## Security posture

LabNTP is a laboratory time source, not a production NTP service. A UDP/123
listener can become an amplification source or leak virtual time to the wrong
client. Secure defaults are mandatory: `allowClientCidrs`, admission caps, and
unmatched-packet drop.

The process never sets the host clock.

## Reporting vulnerabilities

Report security vulnerabilities through [GitHub private vulnerability reporting](https://github.com/hilather/go-lab-ntp/security/advisories/new) on [`hilather/go-lab-ntp`](https://github.com/hilather/go-lab-ntp). Do not file vulnerabilities in the public issue tracker before coordinated disclosure.

Include, when possible: affected version or commit, listen flags, a minimal
reproduction, and impact (virtual-time leak, amplification, secret leak). Do
not attach live tokens or key files.

## Supported versions

| Version | Supported |
|---|---|
| Unreleased `main` | Yes — first public candidate |
| Pre-release development binaries (`dev` ldflags) | Best-effort until the first annotated tag |
| Any unreleased fork or modified image | No |

## Minimum security requirements

- Packets outside `allowClientCidrs` are ignored (no virtual-time leak).
- Unmatched filters drop. Default catch-all `0.0.0.0/0` and `::/0` is required.
- Secrets are file refs only (bearer tokens, NTP symmetric keys).
- Containers run as non-root UID 65532 with a read-only filesystem. Compose may restore only `NET_BIND_SERVICE`.
- Management (later) requires a usable bearer token to bind.
- Client IPs are never metric labels.

See [docs/01-architecture.md](docs/01-architecture.md) and
[docs/02-ntp-semantics.md](docs/02-ntp-semantics.md).
