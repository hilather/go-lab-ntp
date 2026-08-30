# REST `/v1`

Status: Proposed normative behavior
Owners: Control Plane
Last reviewed: 2026-08-30
Related ADRs: 0004, 0005

Management HTTP binds only when `--management-listen` is an address
(default **off**). Errors are `application/problem+json`
(`urn:labntp:error:…`). Native prefix is `/v1`.

## Routes

Health (no bearer): `GET /v1/health/live`, `GET /v1/health/ready`.

Authenticated: version, capabilities, status, schema, features, state
(get/validate/export/reset), changes (plan/apply), filters CRUD, views
preview, queries, audit, session, optional `/v1/metrics`.

Mutations require `expectedRevision` except session. Idempotency-Key is
honored on apply.

Session cookie is `labntp_session`; CSRF header is `X-LabNTP-CSRF`.
Authorization Basic is rejected (401 Bearer).

When `spec.ui.enabled` is true and management HTTP is bound, `GET /` serves
the operator SPA (`docs/12-web-ui.md`). `spec.ui.enabled: false` keeps
`GET /` as 404 `application/problem+json`. Duration fields on filters are
JSON **strings** (`"0s"`); a bare number is 400. `GET /v1/status` nested
`revisions` / `warnings` keys are Go PascalCase (`BootstrapRevision`);
`GET /v1/state` is camelCase (`runtimeRevision`) and is the SPA revision
source.
