# Journal - xiangrikuil (Part 1)

> AI development session journal
> Started: 2026-07-06

---



## Session 1: Auth Hardening Bootstrap

**Date**: 2026-07-06
**Task**: Auth Hardening Bootstrap
**Package**: backend
**Branch**: `main`

### Summary

Committed Argon2id password hashing, legacy rehash, env-driven first-admin bootstrap, migration cleanup, and backend spec updates.

### Main Changes

- Added `argon2id_v1` for new password credentials and kept `sha256_salted_v1` as legacy verification-only.
- Rehashed successful legacy password logins to Argon2id before session creation completes.
- Replaced the fixed admin password seed with explicit `C2C_BOOTSTRAP_ADMIN_USERNAME` / `C2C_BOOTSTRAP_ADMIN_PASSWORD` startup bootstrap.
- Added migration cleanup, environment examples, compose wiring, backend tests, and backend spec updates.

### Git Commits

| Hash | Message |
|------|---------|
| `af95f14` | (see git log) |

### Testing

- [OK] `docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/src -w /src/backend golang:1.26-alpine go test ./...`
- [OK] `git diff --check`
- [OK] fixed admin hash/salt literal scan returned no matches

### Status

[OK] **Completed**

### Next Steps

- Continue the parent maintenance roadmap with P0 request/proxy hardening.


## Session 2: Backend service boundary cleanup

**Date**: 2026-07-06
**Task**: Backend service boundary cleanup
**Package**: backend
**Branch**: `main`

### Summary

Split carpool handlers from the legacy server.Service facade, documented core.Service as a compatibility facade, recorded the backend service-boundary pattern, verified backend tests, and archived the child task.

### Main Changes

- Added `server.CarpoolService` and `server.ApplicationService` so carpool handlers depend on a focused domain transport boundary.
- Moved carpool handler service calls from `s.app` to `s.carpools`.
- Documented `core.Service` as a legacy compatibility facade and recorded the focused server-side service interface pattern in backend specs.
- Updated the parent maintenance roadmap and archived the backend service boundary cleanup child task.

### Git Commits

| Hash | Message |
|------|---------|
| `635caf1272072deda8b5f027de94133bff85386e` | `chore: split carpool server service boundary` |

### Testing

- [OK] Docker `go test ./...` in `backend`
- [OK] `git diff --check`
- [OK] Source scans for carpool handler `s.app` usage and migrated methods in legacy `server.Service`

### Status

[OK] **Completed**

### Next Steps

- Continue the parent maintenance roadmap with database-level pagination, search index/query alignment, and final docs/source/test hardening tasks.


## Session 3: Complete maintenance hardening roadmap

**Date**: 2026-07-06
**Task**: Complete maintenance hardening roadmap
**Package**: frontend
**Branch**: `main`

### Summary

Completed final maintenance cleanup checks, archived the final child task and parent roadmap, and ignored generated source package output.

### Main Changes

- Added `scripts/check-migrations-doc.mjs` and wired it into CI.
- Added `scripts/package-source.sh`, then ignored generated `output/` archives.
- Added focused `backendClient` Vitest coverage for real session failures,
  Problem Details decoding, and stale CSRF retry.
- Updated architecture/deployment docs and added
  `docs/maintenance-hardening-report.md`.
- Archived the final cleanup child task and the parent maintenance roadmap.

### Git Commits

| Hash | Message |
|------|---------|
| `311fb1a` | (see git log) |
| `1192f4e` | (see git log) |

### Testing

- [OK] Migration docs check: 36 migrations documented, latest version 36.
- [OK] Source package self-check excluded forbidden generated/control paths.
- [OK] OpenAPI route guard: 211 method/path pairs.
- [OK] Docker backend `go test ./...` with `GOPROXY=https://goproxy.cn,direct`.
- [OK] Frontend `vue-tsc`, real-mode Vite build, and Vitest suite using Node 24.
- [OK] `git diff --check`.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 4: Report dispute moderation v0.4.1

**Date**: 2026-07-06
**Task**: Report dispute moderation v0.4.1
**Package**: frontend
**Branch**: `main`

### Summary

Aligned the report/dispute moderation model with v0.4.1: clean pre-launch schema, public result codes, moderation audit logs, canonical target snapshots, OpenAPI/frontend sync, and verification.

### Main Changes

- Reworked the report/dispute baseline for the pre-launch clean schema decision.
- Added `public_result_code`, `moderation_audit_logs`, canonical target snapshots, duplicate active report protection, and report target resolver coverage.
- Synced backend DTOs, OpenAPI, frontend adapters, entry points, and Trellis/backend specs with the v0.4.1 contract.

### Git Commits

| Hash | Message |
|------|---------|
| `a27d8c7` | (see git log) |

### Testing

- [OK] `docker run --rm -e GOPROXY=https://goproxy.cn,direct ... golang:1.26-alpine go test ./...`
- [OK] `./node_modules/.bin/vue-tsc -b`
- [OK] `VITE_API_MODE=real ./node_modules/.bin/vite build`
- [OK] `git diff --check`

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 5: OAuth account recovery setup

**Date**: 2026-07-06
**Task**: OAuth account recovery setup
**Package**: frontend
**Branch**: `main`

### Summary

Forced frontend account recovery setup after linux.do OAuth: incomplete accounts redirect to account settings, complete verified email plus backup password, and can return to the original page.

### Main Changes

- Added staging/main CI release gates and a reusable GHCR build/deploy workflow.
- Added immutable SHA image deployment, fixed 8080/8081 environment mapping,
  production backup-before-migration, health checks, and versioned current links.
- Added release regression tests, VPS/GitHub setup documentation, and the
  backend deployment contract.

### Git Commits

| Hash | Message |
|------|---------|
| `95dff64` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 6: API order delivery credential flow

**Date**: 2026-07-09
**Task**: API order delivery credential flow
**Package**: frontend
**Branch**: `main`

### Summary

Committed and pushed marketplace updates, including API order payment QR snapshots and one-time station delivery credentials.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `672554c` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 7: Cloudflare Pages pnpm workspace fix

**Date**: 2026-07-15
**Task**: Cloudflare Pages pnpm workspace fix
**Package**: frontend
**Branch**: `codex/complete-ui-business-consistency`

### Summary

Added an explicit pnpm root package and pinned Node 24.13.0; verified Cloudflare's pnpm 10.11.1 install, production build for https://c2cmarket.shop, and all 118 frontend tests.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `9f4039c` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 8: Cloudflare deployment and marketplace consistency delivery

**Date**: 2026-07-15
**Task**: Cloudflare deployment and marketplace consistency delivery
**Package**: frontend
**Branch**: `codex/complete-ui-business-consistency`

### Summary

Fixed Cloudflare frontend build compatibility, committed the complete marketplace business consistency update, passed backend and frontend quality gates, rebased onto origin/main, and pushed the feature branch.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `39672e0` | (see git log) |
| `82fc0e7` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 9: Staging marketplace consistency PR

**Date**: 2026-07-17
**Task**: Staging marketplace consistency PR
**Package**: frontend
**Branch**: `codex/staging-marketplace-consistency`

### Summary

Rebased the workspace onto current staging, preserved three pending fixes, verified backend/frontend and migrations, and committed all Git-visible changes for PR review.

### Main Changes

- Created `codex/staging-marketplace-consistency` from current `origin/staging` and preserved the three pending operational/OAuth fixes.
- Committed the full marketplace identity, order, account-navigation, email, API/OpenAPI, migration, test, and Trellis spec changes as `ff8dba1`.
- Documented migration 52 after verifying both an applied development database upgrade and an isolated empty-database migration chain.

### Git Commits

| Hash | Message |
|------|---------|
| `ff8dba1` | (see git log) |

### Testing

- [OK] `go test ./...`
- [OK] Frontend Vitest: 40 files / 134 tests
- [OK] Vue type-check and real-API production build
- [OK] Applied database migration through Version 52 and isolated migration 1 through 52
- [OK] `git diff --check`

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 10: VPS 直连生产与 Staging 迁移

**Date**: 2026-07-17
**Task**: VPS 直连生产与 Staging 迁移
**Package**: frontend
**Branch**: `codex/staging-marketplace-consistency`

### Summary

将 production/staging 后端与 PostgreSQL 迁移到 RackNerd VPS；启用 Caddy Cloudflare Full strict 直连、loopback 容器端口、R2 systemd 每日备份，并停用 Mac mini 后端、Tunnel 与旧备份任务。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `c95e91b` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 11: GHCR backend CI/CD

**Date**: 2026-07-18
**Task**: GHCR backend CI/CD
**Package**: frontend
**Branch**: `codex/staging-marketplace-consistency`

### Summary

Added tested GHCR image publishing and environment-gated staging/production VPS deployment with immutable SHA tags, backup-before-migration, health checks, versioned releases, regression tests, and operations documentation.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `75e0339` | `ci: deploy backend from ghcr` |

### Testing

- [OK] Release shell syntax and smoke tests passed.
- [OK] Both workflow files parsed as YAML and production/staging Compose
  configurations expanded successfully.
- [OK] Local backend Docker build, complete Go tests, OpenAPI/migration checks,
  frontend typecheck/build, and 137 frontend tests passed.

### Status

[OK] **Completed**

### Next Steps

- None - task complete
