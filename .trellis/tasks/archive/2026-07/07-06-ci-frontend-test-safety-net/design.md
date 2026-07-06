# CI and frontend test safety net design

## Scope

This child task implements Phase 0 only. It creates a verification baseline before any security or architecture refactor. The task does not change business handlers, frontend UI behavior, persistence, route registration, or OpenAPI endpoint semantics.

## Frontend test baseline

The frontend currently has typecheck/build scripts but no test script. Add Vitest as a dev dependency and a minimal test file that imports stable pure TypeScript code from the existing app. The first test should avoid Vue DOM rendering and backend calls so the baseline is reliable in CI. Good candidates are pure helpers such as pricing/quota/form-validation utilities or publish assistant helpers that already have local tests nearby. If existing test files are already present, wire them through the `test` script instead of inventing a second test convention.

Vitest config can live in `vite.config.ts` if minimal, or in a small `vitest.config.ts` only if needed. Prefer the least intrusive setup that runs current `*.test.ts` files.

## Route/OpenAPI guard

Add a Node script under `scripts/` that parses `backend/internal/server/routes.go` for chi registrations and parses `docs/openapi/c2c-market-api-v1.yaml` for OpenAPI paths/methods. The guard compares exact normalized `METHOD path` pairs.

Normalization rules:

- Backend routes are registered inside `/api/v1` groups for business APIs and at root for `/health` and `/readyz`.
- Chi `{id}` path parameters already match OpenAPI syntax and should be preserved.
- Only standard HTTP methods are compared: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`.
- The script should report missing-from-OpenAPI and missing-from-backend lists clearly and exit non-zero on drift.

This is deliberately a route-pair guard. It does not validate schemas, tags, examples, or response bodies.

## CI workflow

Create `.github/workflows/ci.yml` with separate backend and frontend jobs. Backend job should use `actions/setup-go@v5` with `go-version-file: backend/go.mod`, run `go test ./...`, and run the route/OpenAPI guard. Frontend job should use pnpm 10 and Node 24, install with `pnpm install --frozen-lockfile`, then run `pnpm typecheck`, `pnpm build`, and `pnpm test`.

## Compatibility and rollback

All changes are additive to tooling. Rollback is direct: remove the workflow, route guard, Vitest wiring, and tests. Since no runtime behavior changes are made, rollback does not require migration or data handling.
