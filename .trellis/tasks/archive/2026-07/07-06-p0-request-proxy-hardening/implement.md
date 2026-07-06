# P0 request and proxy hardening implementation plan

## Checklist

- [x] Update `backend/internal/validator/request.go` to enforce 1 MiB max body, strict trailing-token rejection, and shared behavior for `DecodeJSON` / `DecodeStrictJSON`.
- [x] Add server tests for empty body, malformed JSON, oversized body, and multiple JSON values.
- [x] Add config fields and parsing/validation for `TRUST_X_FORWARDED_FOR` and `TRUSTED_PROXIES`.
- [x] Add server option fields and trusted proxy IP/CIDR matching.
- [x] Replace unconditional `clientIP` forwarding-header trust with gated trusted-proxy behavior.
- [x] Add rate-limit tests for forged XFF default behavior, trusted proxy behavior, and untrusted peer fallback.
- [x] Update `.env.example`, `.env.production.example`, `compose.yaml`, `compose.prod.yaml`, and backend spec docs.
- [x] Run gofmt on touched Go files.
- [x] Run `cd backend && go test ./...` through Docker Go image if local Go remains unavailable.
- [x] Run `git diff --check` and a secret/plaintext scan over touched files.

## Validation Commands

```bash
docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine gofmt -w internal/validator/request.go internal/config/config.go internal/config/config_test.go internal/server/server.go internal/server/middleware.go internal/server/hardening_test.go internal/app/app.go
docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...
git diff --check
rg -n "PASSWORD|SECRET|TOKEN|COOKIE|CONTACT|-----BEGIN|AKIA|sk-" <touched-files>
```

## Validation Results

- RED check: `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./internal/config ./internal/server` failed before implementation because trusted proxy config/server fields did not exist.
- Targeted GREEN: `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./internal/config ./internal/server` passed.
- Full backend: `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...` passed.
- Whitespace: `git diff --check` passed.
- Secret/plaintext scan over touched files found only env placeholders, existing test fake secrets, and safety-spec terminology; no real credential values were added.

## Risk Points

- `DecodeStrictJSON` raw body bytes are used for idempotency request hashing; do not re-marshal or normalize request bodies.
- `io.LimitReader` size-limit behavior must be tested through server routes, not only unit helpers.
- IPv6 `RemoteAddr` parsing must not break local requests.
- Proxy trust must be opt-in; empty `TRUSTED_PROXIES` must never mean trust all.

## Rollback Points

- Revert validator helper changes if JSON compatibility breaks.
- Revert server proxy option changes independently if proxy matching behavior causes deployment issues.
