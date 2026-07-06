# Frontend API mock isolation design

## Boundaries

- Frontend-only architecture cleanup.
- First migrated domain slice: demand posts.
- No backend route, OpenAPI, database, or service-interface changes.
- `src/lib/api.ts` remains the compatibility facade for unrelated domains during the incremental migration.

## New API Surface

- Add `src/api/*` as a narrow compatibility layer over the existing backend client primitives:
  - `client.ts`: request/mutation/base URL exports only.
  - `problem.ts`: Problem Details error export.
  - `csrf.ts`: CSRF token exports.
  - `types.ts`: backend session/auth DTO exports.
- This establishes the target directory without forcing a broad client migration in the same child task.

## Demand Domain Flow

- `src/features/demand/types.ts` owns `DemandRecord`, `DemandStatus`, and `SubmitDemandPayload`.
- `src/features/demand/api.ts` owns demand API functions:
  - `getDemands`
  - `getDemandById`
  - `submitDemand`
  - `closeDemand`
- Real mode delegates to the existing `src/lib/demandBackend.ts`.
- Mock mode dynamically loads `src/mocks/demand.ts`, so mock persistence is not part of the real path.
- `src/lib/api.ts` imports/re-exports the demand functions for compatibility but no longer implements demand CRUD itself.

## Mock Boundary

- `src/mocks/storage.ts` owns small sessionStorage helpers for mock modules.
- `src/mocks/demand.ts` owns demand seed normalization, mock store reads/writes, and mock actions.
- Production Vite config rejects `VITE_ENABLE_MOCK=true`; production builds still require `VITE_API_MODE=real` or `VITE_API_BASE_URL`.

## Compatibility

- Existing pages continue to use query hooks.
- Demand hooks switch to `features/demand/api.ts`; other hooks remain on the legacy facade.
- `lib/api.ts` can use demand feature functions for home/search/admin/notification compatibility while other domains are migrated later.

## Rollback

- Revert the new `features/demand`, `src/api`, and `src/mocks/demand` files plus the focused `lib/api.ts` edits.
- No backend or persisted database rollback is needed.
