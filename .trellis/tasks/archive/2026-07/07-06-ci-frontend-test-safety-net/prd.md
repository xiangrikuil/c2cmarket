# CI and frontend test safety net

## Goal

Phase 0: add automated verification baseline, frontend Vitest smoke tests, CI, and OpenAPI/routes alignment guard before larger maintenance changes.

## Requirements

- Establish a reliable automated baseline before security and architecture changes.
- Add a frontend `test` script using Vitest with a minimal, low-risk unit/smoke test that exercises existing project code without requiring a browser or backend service.
- Add CI workflow coverage for backend tests, frontend install/typecheck/build/test, and the OpenAPI/routes alignment guard.
- Add a lightweight route/OpenAPI alignment script that checks exact `METHOD path` route pairs rather than only comparing counts.
- Keep all verification scripts local and deterministic. The route/OpenAPI guard must not require a running backend or network.
- Avoid changing business behavior, route implementations, OpenAPI semantics, or frontend UI in this phase.
- Prefer existing repo patterns and package manager files. Do not perform broad dependency upgrades beyond adding the test tooling required for this phase.

## Acceptance Criteria

- [x] `frontend/package.json` includes `test` and necessary Vitest dev dependency entries.
- [x] A minimal frontend test exists and runs with `pnpm test` from `frontend/`.
- [x] A route/OpenAPI alignment script exists under `scripts/` and fails when a backend route method/path is missing from OpenAPI or vice versa.
- [x] CI workflow exists under `.github/workflows/` and runs backend `go test ./...`, the route/OpenAPI guard, frontend `pnpm install --frozen-lockfile`, `pnpm typecheck`, `pnpm build`, and `pnpm test`.
- [x] Phase validation commands pass locally or any environment-specific blocker is recorded with exact output.
- [x] No product behavior, runtime route behavior, or OpenAPI contract behavior changes are introduced by this child task.

## Notes

- This is the first child task of `.trellis/tasks/07-06-maintenance-hardening-roadmap`.
- The route/OpenAPI check is intentionally a guardrail, not a full schema validator.
