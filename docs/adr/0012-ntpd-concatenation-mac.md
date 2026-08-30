# ADR 0012: ntpd concatenation MAC, not HMAC

Status: Accepted
Date: 2026-08-30

## Context

RFC 2104 HMAC-SHA256 is not what ntpd, chrony, and Windows W32Time speak for
symmetric keys. Those stacks use `ALG(key || 48-byte header)`. Shipping HMAC
would fail-closed on a bad digest and break the lab.

MD5 is still present in lab `ntp.keys` files.

## Decision

Symmetric MAC is ntpd/chrony concatenation (D21):

```
digest = ALG(key || header[0:48])
trailer = keyid_be32 || digest
```

Algorithms v1: MD5 (16), SHA1 (20), SHA256 (32). Not HMAC.

Keys are a file ref (`spec.ntp.symmetricKeys.file`). Inline keys are rejected.
Missing keys file fails compile/apply/serve; `labntp validate` does not require
the file to exist.

Vectors under `testdata/packets/mac-{md5,sha1,sha256}.raw` are generated with
this formula and documented to match ntpd concatenation.

## Consequences

- SUTs that speak classic symmetric keys interoperate.
- HMAC is a later ADR if a SUT requires it.

## Alternatives considered

- RFC 2104 HMAC: disagrees with ntpd/chrony/W32Time. Rejected.

## Review triggers

NTS, Autokey, or a SUT that requires HMAC-SHA256.
