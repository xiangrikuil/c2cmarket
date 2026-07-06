# Runtime readiness and shutdown hardening

## Goal

Make backend runtime operations safer for production by tightening `/readyz`, graceful process shutdown, and request logging without leaking request bodies or secrets.

## Requirements

- Preserve existing `/health` and `/readyz` paths and response compatibility where possible.
- Add a single authoritative expected migration version constant matching the latest migration number (`000035` at task start).
- `/readyz` must return non-2xx when PostgreSQL is configured and `schema_migrations.version < ExpectedMigrationVersion`.
- `/readyz` must still reject dirty migrations and database failures.
- Keep no-database development mode ready with `database=not_configured`.
- Add tests for healthy latest schema, behind-schema degraded state, dirty schema degraded state, and no database mode.
- Update `cmd/api` to handle `SIGINT` and `SIGTERM`, call `http.Server.Shutdown(ctx)` with a bounded timeout, and close the app store after shutdown.
- Log startup, listen failure, normal shutdown, and forced shutdown clearly without logging request bodies or secrets.
- Add request logging middleware that logs method, path, status, duration, and request ID.
- Ensure `X-Request-Id` is accepted/generated and included in logs without logging cookies, CSRF tokens, contact values, or request bodies.

## Acceptance Criteria

- [x] Readiness tests cover version below expected, dirty schema, configured healthy schema, and no database.
- [x] `cmd/api` uses signal-aware graceful shutdown and treats `http.ErrServerClosed` as normal.
- [x] Request logging test proves method/path/status/request ID are logged and request body is not logged.
- [x] Backend specs document expected migration readiness and request logging baseline.
- [x] `cd backend && go test ./...` passes.

## Notes

- Source requirement: `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, Phase 3.1-3.3.
- Current latest migration at task start: `000035_password_argon2_admin_bootstrap`.
