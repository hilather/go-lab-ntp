# ADR 0005 — Lab static bearer

- Status: Accepted
- Date: 2026-08-30
- Related: D10

Static bearer, SHA-256 digest compare, tokens ≥32 bytes, file refs only, no
HTTP Basic. SPA cookie `labntp_session` + CSRF `X-LabNTP-CSRF`. Management
bind requires ≥1 usable token unless listen is off.
