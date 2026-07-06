# Runtime readiness and shutdown hardening implementation plan

## Checklist

- [x] Add `ExpectedMigrationVersion = 35` in `backend/internal/database`.
- [x] Extend `health.Status` and `/readyz` response with expected migration version.
- [x] Update `PostgresReadiness` to fail when current schema version is behind expected.
- [x] Add readiness tests for latest schema, behind schema, dirty schema, and no database.
- [x] Add request logging middleware with status capture.
- [x] Wire request logging after request ID middleware in server construction.
- [x] Add middleware test proving request log includes method/path/status/request id and omits body content.
- [x] Update `cmd/api/main.go` for signal-aware graceful shutdown.
- [x] Update backend specs for readiness expected version, graceful shutdown, and request logging baseline.
- [x] Run gofmt and `cd backend && go test ./...`.
- [x] Run `git diff --check` and secret/plaintext scan over touched files.

## Validation Results

- `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./internal/database ./internal/middleware ./internal/server` passed.
- `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...` passed.
- `git diff --check` passed.
- Secret scan over touched files found only existing spec safety terms and test fake-secret strings, no new real secret material.

## Validation Commands

```bash
docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine gofmt -w cmd/api/main.go internal/database/postgres.go internal/health/health.go internal/middleware/request_log.go internal/middleware/request_log_test.go internal/server/health_handler.go internal/server/router_test.go
docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...
git diff --check
rg -n "PASSWORD|SECRET|TOKEN|COOKIE|CONTACT|-----BEGIN|AKIA|sk-" <touched-files>
```

## Risk Points

- Keep `/readyz` no-database behavior friendly for local development.
- Do not log query strings, request bodies, cookies, CSRF tokens, or full Problem Details.
- Avoid making graceful shutdown hard to test; keep shutdown orchestration explicit and small.
- Expected migration version must be updated later when new migrations are added.

## Rollback Points

- Readiness version enforcement is localized to database health and response DTOs.
- Request logging is one middleware wrapper.
- Shutdown handling is localized to `cmd/api/main.go`.
