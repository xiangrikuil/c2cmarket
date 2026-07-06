# CI and frontend test safety net implementation plan

## Pre-check

- Inspect `frontend/package.json`, `frontend/vite.config.ts`, existing frontend `__tests__`, backend route registration, OpenAPI file, and current scripts.
- Confirm whether current lockfile already contains Vitest. If not, add it with pnpm so `pnpm-lock.yaml` updates deterministically.

## Steps

1. Add frontend test runner wiring.
   - Add `test` script to `frontend/package.json`.
   - Add Vitest dev dependency if missing.
   - Keep config minimal and aligned with Vite/Vue/TS setup.

2. Add or reuse a minimal frontend smoke/unit test.
   - Prefer existing pure helper tests if they already exist.
   - If adding a test, choose stable pure code and avoid DOM/backend/network requirements.

3. Add `scripts/check-openapi-routes.mjs`.
   - Parse route registrations from `backend/internal/server/routes.go`.
   - Parse OpenAPI path/method pairs from `docs/openapi/c2c-market-api-v1.yaml`.
   - Normalize `/api/v1` group prefixes and compare exact `METHOD path` pairs.
   - Print actionable drift output and exit non-zero on mismatch.

4. Add GitHub Actions CI.
   - Backend job: checkout, setup Go from `backend/go.mod`, run `go test ./...`, run route/OpenAPI guard.
   - Frontend job: checkout, setup pnpm 10 and Node 24, install frozen lockfile, run typecheck, build, and test.

5. Validate.
   - `node scripts/check-openapi-routes.mjs`
   - `cd backend && go test ./...`
   - `cd frontend && pnpm install --frozen-lockfile`
   - `cd frontend && pnpm typecheck`
   - `cd frontend && pnpm build`
   - `cd frontend && pnpm test`

## Review gates

- If route/OpenAPI drift exists before this task, either fix the OpenAPI/route mismatch only when it is clearly a documentation drift, or record the mismatch and stop before making unrelated behavior changes.
- If dependency install needs network and the sandbox blocks it, rerun the same install command with escalation approval.
- Do not continue into P0 security changes until this phase is passing or the exact blocker is documented.

## Validation Results

- `CI=true pnpm install --frozen-lockfile` from `frontend/`: passed after fixing `frontend/pnpm-workspace.yaml` build-script approvals for `maplibre-gl` and `vue-demi`.
- `node scripts/check-openapi-routes.mjs`: passed with `OpenAPI route guard passed (211 method/path pairs).`
- `CI=true pnpm test` from `frontend/`: passed, 5 test files and 5 tests.
- `CI=true pnpm typecheck` from `frontend/`: passed.
- `CI=true VITE_API_MODE=real pnpm build` from `frontend/`: passed. Build logs include existing Rolldown `INVALID_ANNOTATION` warnings from `@vueuse/core`, but Vite completed successfully.
- `go test ./...` from `backend/`: blocked in this local environment because `go` is not installed (`zsh:1: command not found: go`).
- Docker fallback for backend tests: blocked because `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket/backend:/app -w /app golang:1.26-alpine go test ./...` could not pull the Go image. Docker reported `failed to resolve reference "docker.io/library/golang:1.26-alpine": ... context deadline exceeded`.
