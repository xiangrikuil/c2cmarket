# Final maintenance cleanup

## Goal

Complete the final maintenance-hardening cleanup: migration documentation drift
checks, source packaging, obvious architecture/deployment documentation updates,
high-value frontend tests, and the final maintenance report/verification record.

## Confirmed Facts

- The parent prompt stages 8-10 require `scripts/check-migrations-doc.mjs`,
  CI coverage for that check, `scripts/package-source.sh`, architecture docs,
  frontend critical-path tests, and `docs/maintenance-hardening-report.md`.
- `backend/migrations/README.md` now lists migrations through
  `000036_search_trigram_alignment`, and backend readiness expects migration
  version 36.
- CI currently runs backend tests, OpenAPI route parity, frontend install,
  typecheck, real-mode build, and frontend tests; it does not yet check migration
  docs.
- Existing frontend tests use Vitest in node mode. A high-value final test can
  cover `frontend/src/lib/backendClient.ts` without adding DOM/browser
  dependencies.
- `docs/project-architecture-api-db-overview-2026-06-23.md` is visibly stale in
  the migration section and should be updated with current migration and
  maintenance-hardening status instead of rewritten from scratch.

## Requirements

- Add a migration documentation drift check script that fails when
  `backend/migrations/README.md` omits any `*.up.sql` migration or when
  `ExpectedMigrationVersion` does not match the latest migration number.
- Wire the migration documentation check into CI.
- Add a source packaging script that creates a source archive while excluding
  `.git/`, `output/`, `tmp/`, `.DS_Store`, `__MACOSX/`, `node_modules/`,
  `dist/`, `build/`, and `coverage/`, and verifies the archive contents.
- Update architecture/deployment documentation for current migration version,
  real frontend API vs mock boundary, backend domain-service migration strategy,
  and production configuration requirements.
- Add focused frontend tests for real-backend/session/CSRF/Problem Details
  behavior without broad UI test infrastructure churn.
- Add `docs/maintenance-hardening-report.md` covering completed stages, tests,
  residual debt, next steps, and deployment checklist.
- Run final verification covering backend tests, frontend typecheck/build/test,
  CI/documentation scripts, and packaging checks where feasible.

## Acceptance Criteria

- [x] `node scripts/check-migrations-doc.mjs` passes and is run by CI.
- [x] `scripts/package-source.sh` creates an archive that excludes the required
      transient/build/source-control paths.
- [x] Architecture/deployment docs mention the current migration version,
      frontend real/mock boundary, backend module migration strategy, and
      production required configuration.
- [x] Frontend tests include high-value coverage for real backend session/error
      behavior and pass under `pnpm --dir frontend test`.
- [x] `docs/maintenance-hardening-report.md` exists with completed changes,
      validation results, remaining risks, next steps, and deployment checklist.
- [x] Parent roadmap final cleanup and final verification acceptance items are
      checked after validation.
- [x] Backend tests, frontend typecheck/build/test, route/migration doc checks,
      package check, and `git diff --check` pass or any inability to run is
      explicitly recorded.

## Notes

- Source requirement:
  `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, phases
  8-10 and final verification.
