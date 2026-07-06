# Dependency and toolchain hardening

## Goal

Make dependency installation and local/CI toolchains repeatable by removing frontend `latest` ranges, declaring supported engines, and aligning documented Go/Node/pnpm/PostgreSQL requirements.

## Requirements

- Update `frontend/package.json` so dependencies and devDependencies do not use `latest`.
- Choose dependency ranges from the existing lockfile/current resolved dependency set; do not opportunistically upgrade unrelated packages.
- Add frontend engines:
  - `node: >=24.11 <25`
  - `pnpm: >=10 <11`
- Keep `pnpm-lock.yaml` repeatable after the package manifest change.
- Align Go version expectations across `backend/go.mod`, Dockerfile(s), CI, and README.
- If the backend declares Go 1.26, CI and Docker runtime/build references must use the matching Go 1.26 line.
- README must clearly list local development prerequisites: Go, Node, pnpm, and PostgreSQL.
- Keep this task limited to dependency/toolchain metadata and verification; do not start frontend API/mock isolation or backend service boundary refactors here.

## Acceptance Criteria

- [x] No `latest` version ranges remain in `frontend/package.json`.
- [x] Frontend `engines` declares Node `>=24.11 <25` and pnpm `>=10 <11`.
- [x] Go version references in `backend/go.mod`, Dockerfile(s), CI, and README are consistent.
- [x] README documents Go, Node, pnpm, and PostgreSQL local prerequisites.
- [x] `cd frontend && pnpm install --frozen-lockfile` passes with supported Node/pnpm engines.
- [x] `cd frontend && pnpm typecheck && VITE_API_MODE=real pnpm build && pnpm test` passes.
- [x] `cd backend && go test ./...` passes.

## Notes

- Source requirement: `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, Phase 2.1-2.2.
- Local machine currently lacks a native `go`; use Docker Go for backend verification unless Go becomes available.
