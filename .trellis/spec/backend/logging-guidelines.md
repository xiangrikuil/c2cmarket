# Logging Guidelines

> How logging is done in this project.

---

## Overview

The backend currently uses Go standard-library logging in `cmd/api` and `internal/app`. Operational logging is intentionally minimal until structured logging is introduced. Any future logging must preserve the product boundary around contacts and credentials.

---

## Log Levels

- `info`: process startup and shutdown context.
- `error`: process-level fatal failures or operational failures at the layer with enough context.
- Avoid debug logging request bodies in product endpoints.

---

## Structured Logging

Structured logging is not wired yet. When added, include:

- `request_id`
- route key or endpoint name
- actor user ID when authenticated
- target resource type and ID when relevant

Do not include raw request bodies by default.

---

## What to Log

- Server startup and listener address.
- Future production failures for persistence, idempotency conflicts, and admin actions.
- Contact access is modeled as `contact_access_logs` in the database contract; do not rely only on process logs for audit.

---

## What NOT to Log

Never log:

- Contact method full values.
- Passwords, API keys, Sub2API keys, access tokens, refresh tokens, sessions, cookies, MFA codes, recovery codes, or panel owner credentials.
- Evidence URLs after they are rejected as credential-bearing.
- Full Problem Details bodies if they could include user-submitted sensitive text.
