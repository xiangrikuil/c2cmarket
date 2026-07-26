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
**Package**: backend / infrastructure
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

- Added one request-scoped client IP resolver with direct-peer normalization,
  trusted immediate-peer gating, CF header priority, and right-to-left XFF
  stripping.
- Reused the same context value in request logs and rate-limit keys; raw
  forwarding headers are never logged.
- Bound the Compose backend published port to host loopback and added a
  three-environment exposure guard.
- Updated env templates, README, deployment/Tunnel runbooks, and the backend
  production hardening code-spec.

### Git Commits

| Hash | Message |
|------|---------|
| `95dff64` | (see git log) |

### Testing

- [OK] Focused middleware/server/config tests
- [OK] Complete backend Go suite
- [OK] Middleware/server race tests and `go vet`
- [OK] Go formatting, Compose exposure, OpenAPI, migration, and diff guards

### Status

[OK] **Completed**

### Next Steps

- Continue the parent prelaunch task with reproducible release/build-version
  tracking, followed by verification-code lifecycle and runtime hardening.


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


## Session 7: 修订信誉系统实现任务书

**Date**: 2026-07-24
**Task**: 修订信誉系统实现任务书
**Package**: frontend
**Branch**: `docs/open-source-readme`

### Summary

重构信誉系统母 PRD 为六个可验收子任务，修复双盲评价、纠纷责任、限制语义、时间失效、隐私和原帖验证规则，并同步覆盖 Downloads 原文件。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `0f14ad7` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 8: OAuth identity and administrator bootstrap hardening

**Date**: 2026-07-26
**Task**: OAuth identity and administrator bootstrap hardening
**Package**: frontend
**Branch**: `codex/prelaunch-identity-hardening`
**Executor**: Codex

### Summary

Implemented immutable OAuth identity ownership, create-only proven administrator bootstrap, migration 62, regression coverage, and backend identity contract documentation.

### Main Changes

- Made OAuth identity ownership immutable by provider and subject.
- Added collision-safe first-login handling and concurrent identity creation.
- Reworked administrator bootstrap as create-only with a fixed proof marker.
- Added migration 62, regression tests, and the backend identity contract.

### Git Commits

| Hash | Message |
|------|---------|
| `49b99b5` | (see git log) |

### Testing

- [OK] `go test -count=1 ./...`
- [OK] `go vet ./...`
- [OK] `gofmt -l .`
- [OK] `go test -race -count=1 ./internal/module/auth`
- [OK] OpenAPI route guard and migration guard
- [OK] PostgreSQL OAuth concurrency and bootstrap integration scenarios

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 9: Auth hardening release contract alignment

**Date**: 2026-07-26
**Task**: Auth hardening release contract alignment
**Package**: backend
**Branch**: `codex/prelaunch-identity-hardening`
**Executor**: Codex

### Summary

Updated OAuth smoke behavior, synchronized identity and bootstrap contracts, documented migration 62 upgrade handling, and verified the release-facing authentication path.

### Main Changes

- Updated auth smoke so admin-like OAuth usernames remain non-admin.
- Used the development-only session route for admin smoke coverage.
- Synchronized OAuth and Bootstrap contracts across specs and README files.
- Documented migration 62 and existing-administrator upgrade handling.

### Git Commits

| Hash | Message |
|------|---------|
| `6454905` | (see git log) |

### Testing

- [OK] `node --check scripts/auth-smoke.mjs`
- [OK] Local HTTP auth smoke against an isolated in-memory backend
- [OK] `go test -count=1 ./...`
- [OK] `go vet ./...`
- [OK] `gofmt -l .`
- [OK] `go test -race -count=1 ./internal/module/auth`
- [OK] OpenAPI route guard and migration guard

### Status

[OK] **Completed**

### Next Steps

- Continue the parent prelaunch hardening task with outbound SSRF protection.


## Session 10: Harden model audit outbound HTTP

**Date**: 2026-07-26
**Task**: Harden model audit outbound HTTP
**Package**: backend
**Branch**: `codex/prelaunch-identity-hardening`

### Summary

Added public-HTTPS target validation, DNS rebinding-safe IP-bound dialing, redirect/time/body limits, modelaudit wiring, deterministic tests, deployment configuration, and executable backend specs.

### Main Changes

- Added `internal/platform/outboundhttp` with strict public HTTPS URL validation, exact host allowlisting, special-address rejection, connection-time DNS revalidation, and IP-bound dialing.
- Routed model audit target create/update and provider Chat/ListModels through the same policy and shared client.
- Added redirect, timeout, response-size, and error-sanitization boundaries plus deterministic resolver/dialer/TLS tests.
- Wired `MODEL_AUDIT_ALLOWED_HOSTS` through config, app, Compose, environment templates, deployment docs, and an executable backend spec.

### Git Commits

| Hash | Message |
|------|---------|
| `2b8776d` | (see git log) |

### Testing

- [OK] Focused and race tests for `outboundhttp` and `modelaudit`.
- [OK] Full backend `go test -count=1 ./...` and `go vet ./...`.
- [OK] `gofmt`, OpenAPI route, migration docs, production Compose, and diff checks.

### Status

[OK] **Completed**

### Next Steps

- Continue prelaunch hardening with production ingress and trusted client IP handling.


## Session 11: Production ingress and trusted client IP hardening

**Date**: 2026-07-26
**Task**: Production ingress and trusted client IP hardening
**Package**: frontend
**Branch**: `codex/prelaunch-identity-hardening`

### Summary

Bound backend ports to loopback, centralized trusted client IP resolution for logs and rate limits, added Compose exposure guards, and documented Tunnel peer observation.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `b2d8b05` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 12: Reproducible release and build metadata

**Date**: 2026-07-26
**Task**: Reproducible release and build metadata
**Package**: frontend
**Branch**: `codex/prelaunch-identity-hardening`

### Summary

Added fixed-commit source archives and backend image builds, runtime build metadata, image-only production Compose, generated OpenAPI type drift checks, release runbooks, and a macOS-safe concurrent packaging regression.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `e2a251f` | (see git log) |
| `fe66d07` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete
