# Final maintenance cleanup implementation plan

## Checklist

- [x] Add `scripts/check-migrations-doc.mjs`.
- [x] Add the migration doc check to `.github/workflows/ci.yml`.
- [x] Add `scripts/package-source.sh` with archive-content verification.
- [x] Update architecture/deployment docs and add
      `docs/maintenance-hardening-report.md`.
- [x] Add `frontend/src/lib/__tests__/backendClient.test.ts`.
- [x] Run documentation/script checks:
      `node scripts/check-migrations-doc.mjs`,
      `scripts/package-source.sh`, `git diff --check`.
- [x] Run backend verification: `cd backend && go test ./...`.
- [x] Run frontend verification:
      `pnpm --dir frontend typecheck`,
      `VITE_API_MODE=real pnpm --dir frontend build`,
      `pnpm --dir frontend test`.
- [x] Update PRD checkboxes and parent roadmap final acceptance items.
- [x] Commit and archive this child task.

## Validation Commands

```bash
node scripts/check-migrations-doc.mjs
scripts/package-source.sh
cd backend && go test ./...
pnpm --dir frontend typecheck
VITE_API_MODE=real pnpm --dir frontend build
pnpm --dir frontend test
git diff --check
```

## Rollback Points

- CI/script changes are isolated to `.github/workflows/ci.yml` and `scripts/`.
- Frontend tests are additive and can be removed without runtime behavior change.
- Documentation/report changes are additive or narrow updates only.
