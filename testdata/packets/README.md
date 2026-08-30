# NTP packet fixtures

`mac-{md5,sha1,sha256}.raw` are NTPv4 client headers plus
`keyid_be32 || ALG(key || header)` trailers. The construction is **ntpd
concatenation, not HMAC**, matching `internal/ntpwire.MAC` and classic
ntpd/chrony symmetric keys. Keys: `testdata/keys/ntp.keys`.
