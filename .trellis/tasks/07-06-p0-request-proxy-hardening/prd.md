# P0 request and proxy hardening

## Goal

Harden backend request ingestion so JSON bodies are parsed consistently and client IP based rate limiting cannot be bypassed by forged forwarding headers.

## Requirements

- Preserve existing public API paths and Problem Details response shape.
- Enforce a 1 MiB maximum JSON body size on all backend JSON request helpers, including legacy `decodeJSON` call sites.
- Reject empty bodies, malformed JSON, unknown fields, oversized bodies, and multiple JSON values with stable Problem Details errors.
- Keep idempotency request hashing based on the accepted raw body bytes for handlers that currently call `decodeStrictJSON`.
- Stop trusting `X-Forwarded-For` and `X-Real-IP` by default.
- Add backend config for `TRUST_X_FORWARDED_FOR` and `TRUSTED_PROXIES`.
- Read forwarding headers only when forwarding trust is enabled and `RemoteAddr` belongs to a configured trusted proxy IP/CIDR.
- Ensure a client cannot rotate forged `X-Forwarded-For` values to bypass per-IP rate limits.
- Update env/compose examples and backend API/security spec notes for the new proxy settings.

## Acceptance Criteria

- [x] Server tests cover empty JSON body, malformed JSON, oversized JSON body, and multiple JSON object rejection.
- [x] Server tests cover forged `X-Forwarded-For` not bypassing rate limits by default.
- [x] Server tests cover trusted proxy mode using the first valid `X-Forwarded-For` client IP only when the immediate peer is trusted.
- [x] Config tests cover proxy trust defaults and invalid/missing trusted proxy configuration.
- [x] `cd backend && go test ./...` passes.
- [x] Source scan confirms no new secret, password, token, or contact plaintext is introduced.

## Notes

- Source requirement: `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, Phase 1.2 and 1.3.
- Parent task: `.trellis/tasks/07-06-maintenance-hardening-roadmap`.
