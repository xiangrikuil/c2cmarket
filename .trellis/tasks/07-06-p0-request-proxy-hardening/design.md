# P0 request and proxy hardening design

## Boundaries

- Backend-only change touching request validation, server rate-limit keying, startup config, and documentation/spec examples.
- No public route, request schema, response schema, or frontend behavior changes beyond stricter invalid-body rejection.
- No changes to the `backend/internal/server.Service` or `backend/internal/module/core.Service` interfaces.

## JSON Request Contract

- Centralize JSON body limit and strict decode behavior in `backend/internal/validator/request.go`.
- Use the project-standard `io.LimitReader` before reading/decode so oversized bodies produce a deterministic application error without calling `http.MaxBytesReader(nil)`.
- Decode with `DisallowUnknownFields`, then perform a second decode into an empty value and require `io.EOF` to reject trailing JSON values.
- Return `400 VALIDATION_FAILED` for empty, malformed, unknown-field, and trailing-value bodies.
- Return `413 VALIDATION_FAILED` for bodies larger than 1 MiB.
- Keep `DecodeStrictJSON[T]` returning the accepted raw body bytes for idempotency hashing.
- Route legacy `DecodeJSON` through the same strict helper so older auth/dev handlers inherit the hardened behavior.

## Trusted Proxy Contract

- Config fields:
  - `TRUST_X_FORWARDED_FOR`: boolean, default false.
  - `TRUSTED_PROXIES`: comma-separated IP or CIDR list.
- Config load fails if forwarding trust is enabled without at least one trusted proxy or with invalid proxy entries.
- Server stores proxy trust options independently from CORS/security options.
- Rate-limit IP key uses:
  1. direct `RemoteAddr` host by default;
  2. first valid `X-Forwarded-For` address only when forwarding trust is enabled and the direct peer is a trusted proxy;
  3. direct `RemoteAddr` fallback for missing/invalid forwarding headers or untrusted peers.
- `X-Real-IP` follows the same trusted-proxy gate and is only a fallback after `X-Forwarded-For`.

## Compatibility

- Valid JSON requests keep existing behavior.
- Invalid requests may fail earlier with clearer Problem Details responses.
- Deployments behind reverse proxies must explicitly set both proxy env vars to preserve real-client rate limiting.

## Rollback

- JSON hardening is localized to `backend/internal/validator/request.go` and server tests.
- Proxy trust is localized to config, server options, and `backend/internal/server/middleware.go`.
- Env/docs changes are safe to revert independently if runtime config needs adjustment.
