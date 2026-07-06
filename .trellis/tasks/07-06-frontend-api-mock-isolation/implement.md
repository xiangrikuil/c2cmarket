# Frontend API mock isolation implementation plan

## Checklist

- [x] Add `src/api/client.ts`, `problem.ts`, `csrf.ts`, and `types.ts` as narrow exports over existing backend client primitives.
- [x] Add `src/mocks/storage.ts` and `src/mocks/demand.ts`.
- [x] Add `src/features/demand/types.ts` and `src/features/demand/api.ts`.
- [x] Move demand type definitions out of `src/lib/api.ts`.
- [x] Replace demand CRUD implementations in `src/lib/api.ts` with compatibility imports/re-exports.
- [x] Update home/search/admin/notification compatibility code in `src/lib/api.ts` to read demand rows through the migrated demand feature API.
- [x] Update `src/queries/useMarketQueries.ts` demand imports to use `features/demand`.
- [x] Add a demand feature Vitest test for mock list/create/detail/close.
- [x] Add production `VITE_ENABLE_MOCK=true` build guard in `vite.config.ts`.
- [x] Run line-count and source scans.
- [x] Run frontend typecheck, real-mode build, and tests.
- [x] Run `git diff --check`.

## Validation Commands

```bash
wc -l frontend/src/lib/api.ts
rg -n "type Demand = \\(typeof demands\\)|SubmitDemandPayload" frontend/src/lib/api.ts frontend/src/features/demand
rg -n "VITE_ENABLE_MOCK" frontend/vite.config.ts README.md frontend/README.md .trellis/spec/frontend/quality-guidelines.md
PATH=/Users/lixinjian/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH /Users/lixinjian/.npm/_npx/a1a38f5f0f780954/node_modules/.bin/pnpm --dir frontend typecheck
VITE_API_MODE=real PATH=/Users/lixinjian/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH /Users/lixinjian/.npm/_npx/a1a38f5f0f780954/node_modules/.bin/pnpm --dir frontend build
PATH=/Users/lixinjian/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH /Users/lixinjian/.npm/_npx/a1a38f5f0f780954/node_modules/.bin/pnpm --dir frontend test
git diff --check
```

## Validation Results

- `wc -l frontend/src/lib/api.ts`: 3918 lines, below the 3980-line baseline.
- Source scans confirmed demand hooks import from `features/demand/api.ts`, demand types are not derived from `src/data/mock.ts`, and `VITE_ENABLE_MOCK` guard/docs are present.
- `pnpm --dir frontend typecheck`: passed.
- `VITE_API_MODE=real pnpm --dir frontend build`: passed; Rolldown emitted existing third-party `@vueuse/core` pure annotation warnings.
- `pnpm --dir frontend test`: passed, 6 test files and 6 tests.
- `VITE_ENABLE_MOCK=true VITE_API_MODE=real pnpm --dir frontend build`: failed as expected with `Production frontend builds must not set VITE_ENABLE_MOCK=true.`
- `git diff --check`: passed.

## Risk Points

- `src/lib/api.ts` still owns many unrelated mock stores. Keep changes tightly scoped to demand.
- Avoid importing `src/mocks/demand.ts` into the real code path except through mock-only branches.
- Demand admin/search/home/notification compatibility must still see mock-created demand rows.
