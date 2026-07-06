# Frontend API mock isolation

## Goal

Start the long-running frontend API architecture cleanup with one independently verifiable domain slice: demand posts. The change must reduce `src/lib/api.ts`, route demand query usage through `src/features/demand/api.ts`, move demand mock persistence under `src/mocks`, and harden production builds against accidental mock mode.

## Requirements

- Create the new frontend API/mocking directory surface expected by the roadmap:
  - `frontend/src/api/client.ts`
  - `frontend/src/api/problem.ts`
  - `frontend/src/api/csrf.ts`
  - `frontend/src/api/types.ts`
  - `frontend/src/mocks/storage.ts`
- Migrate the demand domain API to `frontend/src/features/demand/api.ts`.
- Move demand-specific mock persistence and seed normalization to `frontend/src/mocks/demand.ts`.
- Demand types must live outside `src/data/mock.ts`; frontend demand API types must not be derived from mock seed arrays.
- `frontend/src/queries/useMarketQueries.ts` demand hooks must import demand functions/types from `features/demand`, not from `src/lib/api.ts`.
- Keep `src/lib/api.ts` as a legacy facade, but reduce its line count and avoid keeping demand business API implementations there.
- Production build must reject explicit `VITE_ENABLE_MOCK=true`; demo/dev mock mode remains allowed only outside production.
- Add a minimal demand-domain test proving mock create/list/detail/close behavior works through the new feature API.
- Do not change backend routes, OpenAPI, database pagination, or backend service interfaces in this child task.

## Acceptance Criteria

- [x] `frontend/src/features/demand/api.ts` exists and exposes demand API functions.
- [x] `frontend/src/mocks/demand.ts` owns demand mock sessionStorage behavior.
- [x] Demand hooks in `useMarketQueries.ts` no longer import demand API functions from `src/lib/api.ts`.
- [x] `frontend/src/lib/api.ts` line count is lower than the task-start baseline of 3980 lines.
- [x] `frontend/src/data/mock.ts` is not used as the source for demand TypeScript types.
- [x] Production Vite config fails when `VITE_ENABLE_MOCK=true`.
- [x] A demand-domain Vitest test covers mock list/create/detail/close through `features/demand/api.ts`.
- [x] `pnpm --dir frontend typecheck`, `VITE_API_MODE=real pnpm --dir frontend build`, and `pnpm --dir frontend test` pass.

## Notes

- Source requirement: `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, Phase 4.
- The selected first migrated domain is `demand` because it is explicitly listed in the roadmap, already has a real backend adapter, and is smaller than carpool/API-market.
- Validation result: `frontend/src/lib/api.ts` is 3918 lines, below the 3980-line baseline. Frontend typecheck, real-mode build, Vitest, production mock guard failure check, source scans, and `git diff --check` passed on 2026-07-06.
