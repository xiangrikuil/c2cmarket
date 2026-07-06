# Final maintenance cleanup design

## Scope

This task is a final hardening pass, not a feature rewrite. It adds automated
drift checks, packaging, documentation updates, frontend client tests, and a
maintenance report summarizing the roadmap.

## Migration Drift Check

Add `scripts/check-migrations-doc.mjs`:

- Read `backend/migrations/*.up.sql`.
- Require each migration basename to appear in `backend/migrations/README.md`
  as a backticked migration name.
- Parse `backend/internal/database/postgres.go` for `ExpectedMigrationVersion`.
- Fail if the expected version differs from the highest migration number.

CI should run this script in the backend job after route/OpenAPI parity.

## Source Packaging

Add `scripts/package-source.sh`:

- Build a compressed archive under `output/`.
- Package source from the working tree with explicit exclude patterns for the
  required transient/build/source-control paths.
- List the archive and fail if forbidden paths are present.
- Do not delete user files or clean the workspace.

## Documentation

Update existing docs surgically:

- `docs/project-architecture-api-db-overview-2026-06-23.md`: current migration
  version 36, real/mock frontend boundary, backend module/domain-service
  transition status, and production requirements.
- `docs/ops/deployment-runbook.md`: deployment checklist can point to final
  source packaging and maintenance validation where useful.
- New `docs/maintenance-hardening-report.md`: concise final report with stage
  map, verification evidence, residual risks, next steps, and deployment
  checklist.

## Frontend Tests

Add a Vitest node-mode test for `frontend/src/lib/backendClient.ts`:

- Real mode does not silently create a dev session when `/auth/session` returns
  `SESSION_EXPIRED`.
- Problem Details responses become `BackendProblemError` with stable code,
  detail, and field errors.
- A mutation with stale CSRF token refreshes `/auth/session` and retries with
  the fresh token.

This keeps the test high-value without adding jsdom, browser automation, or
component-mount dependencies.

## Compatibility And Risk

No public API shape changes are intended. Documentation updates must not claim
production readiness beyond the actual checklist. Packaging must be
non-destructive and must not include ignored local secrets or build products.
