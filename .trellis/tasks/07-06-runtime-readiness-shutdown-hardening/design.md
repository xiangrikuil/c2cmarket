# Runtime readiness and shutdown hardening design

## Boundaries

- Backend runtime-only change touching `cmd/api`, readiness health contracts, middleware, server tests, and backend specs.
- No public business API route changes.
- No expansion of `backend/internal/server.Service` or `backend/internal/module/core.Service`.
- No request body, cookie, CSRF, contact, password, or token logging.

## Readiness

- Add `ExpectedMigrationVersion = 35` in `backend/internal/database`.
- Extend `database.PostgresReadiness` to compare the current `schema_migrations.version` against `ExpectedMigrationVersion`.
- Extend `health.Status` with `ExpectedSchemaVersion int64` so `/readyz` can expose the expected version when PostgreSQL is configured.
- Preserve no-database development readiness as `status=ok`, `database=not_configured`.
- When schema is dirty or behind, return `503` with `status=degraded`, `database=error`, `schemaVersion`, `schemaDirty`, `expectedSchemaVersion`, and a stable reason string.

## Shutdown

- Keep explicit `http.Server` timeouts already present in `cmd/api`.
- Move listen/shutdown orchestration into a small testable helper if needed, but keep process entrypoint simple.
- Use `signal.NotifyContext` for `SIGINT` and `SIGTERM`.
- Run `ListenAndServe` in a goroutine.
- On signal, call `Shutdown` with a 15 second timeout.
- Treat `http.ErrServerClosed` as normal; log other listen errors as failures.
- Keep `application.Close()` deferred so PostgreSQL pool closes after server shutdown.

## Request Logging

- Add middleware in `backend/internal/middleware` that wraps `ResponseWriter` to capture status code.
- Log one line per request after handler returns with method, path, status, duration, and request ID.
- Compose logging after request ID assignment so generated IDs appear in logs.
- Do not log query strings or request bodies to reduce sensitive-data risk.

## Rollback

- Readiness comparison can be reverted in `database.PostgresReadiness` without touching business code.
- Request logging middleware is a wrapper in server construction and can be removed independently.
- Shutdown helper is localized to `cmd/api`.
