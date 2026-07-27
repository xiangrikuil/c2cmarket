# Security Operations

Date: 2026-07-26
Author: Codex

## Production Trust Boundaries

The supported production path is:

```text
browser -> Cloudflare Pages/Tunnel -> loopback-only backend -> private PostgreSQL
```

- PostgreSQL has no production host port.
- The backend host port binds to `127.0.0.1`; do not add another public or
  health-check port.
- Trust forwarded client IP headers only when the direct peer is inside the
  smallest possible `TRUSTED_PROXIES` CIDR. Public Cloudflare edge ranges are
  not a substitute for the backend's immediate Tunnel peer.
- `/health` is public liveness. Keep `/readyz`, `/version`, and `/metrics`
  outside public ingress unless an operational consumer requires them.
- `/metrics` requires a distinct `METRICS_BEARER_TOKEN` of at least 32 bytes in
  production. Restrict it by network policy or an authenticated monitoring
  proxy as well; the bearer token is not the only desired boundary.

## Identity And Bootstrap

OAuth ownership is the immutable `(provider, provider_subject)` pair. Provider
username, display name, avatar, and email are profile data and cannot rebind an
identity. First login creates a new local user inside the identity transaction;
it never merges with a password user or administrator by username or email.

Administrator Bootstrap is create-only and provenance-backed. Use it only for
an empty database, clear both Bootstrap variables immediately after success,
and never use it to promote an existing account. A collision without the
stored Bootstrap marker must fail closed.

## Outbound HTTP

Model-audit targets accept public HTTPS URLs only. The shared outbound policy:

- rejects URL credentials, fragments, invalid ports, private/loopback/link-local
  addresses, cloud metadata, multicast, unspecified ranges, IPv6 ULA, and
  IPv4-mapped private addresses;
- validates every DNS result and binds each connection to a validated address;
- rejects redirects, oversized responses, and requests outside configured
  connect/TLS/header/total timeouts;
- supports an exact-host `MODEL_AUDIT_ALLOWED_HOSTS` production allowlist.

Do not replace this client with `http.DefaultClient`, log target credentials, or
log complete third-party responses.

## Contact And Credential Encryption

Production should use explicit JSON keyrings:

```text
CONTACT_KEY_VERSION=prod-v2
CONTACT_ENCRYPTION_KEYRING={"prod-v1":"<OLD_ENCRYPTION_KEY>","prod-v2":"<CURRENT_ENCRYPTION_KEY>"}
CONTACT_FINGERPRINT_KEYRING={"prod-v1":"<OLD_FINGERPRINT_KEY>","prod-v2":"<CURRENT_FINGERPRINT_KEY>"}
```

New writes use `CONTACT_KEY_VERSION`. Reads select the recorded key version.
`aad_v1` ciphertext binds record ID, field kind, and key version as AES-GCM
additional authenticated data. Keep every referenced old key until all rows
using it have been re-encrypted and a restore drill has verified the result.

Re-encryption defaults to dry-run and processes one bounded batch:

```bash
cd backend
go run ./cmd/contact-reencrypt -kind contact_methods -batch-size 100
```

Supported kinds are `contact_methods`, `model_audit`, `api_quota`, and
`api_order`. Review `eligible`, `reencrypted`, `failed`, and `nextCursor`
without printing plaintext. Use `-apply` only after a backup and dry-run, and
continue with the returned cursor. Never run this automatically against
production during deployment.

`EMAIL_VERIFICATION_PEPPER`, metrics token, contact encryption keys, and
fingerprint keys must be distinct secrets. Do not put keyrings or tokens in
Git, image layers, OpenAPI examples, frontend variables, or logs.

## HTTP And Browser Policy

The Go API sends a deny-by-default CSP, `X-Frame-Options: DENY`,
`Permissions-Policy`, `nosniff`, and a strict referrer policy. Production also
sends HSTS. Cloudflare Pages uses [`frontend/public/_headers`](../frontend/public/_headers)
for the SPA policy and limits network connections to the production and staging
API origins.

`style-src 'unsafe-inline'` remains limited to styles because Vue components
currently use runtime style bindings. There is no `unsafe-eval`, wildcard
script source, or HTTP source. Any new external script, frame, font, image, or
API origin requires an explicit policy review and `check-security-headers.mjs`
update.

## Logging And Incident Handling

Request logs are one-line JSON with method, normalized route path, status,
duration, request ID, and normalized client IP. Do not add cookies,
Authorization, CSRF tokens, OAuth payloads, contact values, payment
instructions, verification codes, API credentials, or raw request bodies.

On suspected disclosure:

1. Restrict the affected endpoint and preserve redacted logs.
2. Identify the secret class and affected key version without printing values.
3. Rotate through the external secret manager, not through a Git commit.
4. For contact keys, retain old versions for reads, dry-run re-encryption, back
   up, apply bounded batches, and verify restore before retirement.
5. Record the incident timeline, affected data, and verification evidence.
