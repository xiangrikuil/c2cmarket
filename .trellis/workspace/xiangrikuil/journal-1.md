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

- Removed demand pages, routes, frontend adapters, mock state, query hooks, navigation, search, notifications, and personal/admin aggregation.
- Removed the Go demand domain, HTTP handlers, PostgreSQL repository, core facade wiring, and search/notification branches.
- Removed Demand from OpenAPI and regenerated frontend types.
- Added migration 65 to remove demand idempotency rows and the `demands` table; rollback recreates only an empty schema.
- Updated current product, API, database, frontend, deployment, and architecture documentation.

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

- Merged the demand-removal branch into the current `origin/staging` baseline while preserving the Nuxt 4 and Cloudflare Worker architecture.
- Reconciled API service multiplier history through migration 66 without rewriting immutable migration 54 or valid business values.
- Removed remaining demand-module routes, SEO references, smoke coverage, OpenAPI entries, and current documentation references.
- Added the branch-baseline rule requiring new work to start from the latest `origin/staging` or `origin/main`.

### Git Commits

| Hash | Message |
|------|---------|
| `9f4039c` | (see git log) |

### Testing

- [OK] Go formatting, vet, unit tests, race tests, and PostgreSQL migration integration checks.
- [OK] Frontend frozen install, 220 Vitest tests, Nuxt typecheck, real-backend build, and Wrangler dry-runs.
- [OK] OpenAPI, migration documentation, deployment, package, conflict-marker, and removed-module residual guards.
- [WARN] Optional local binaries `actionlint`, `govulncheck`, `gitleaks`, `trivy`, and `syft` were unavailable and were not reported as executed.

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

- 通过现有脚本创建 production PostgreSQL custom-format dump、校验和并上传 R2。
- 识别共享 env、容器配置和数据库 SCRAM verifier 漂移；从 production Docker 网络验证真实认证边界。
- 原子同步 `.env.production`，重置数据库角色密码，恢复既有 production backend。
- 未执行 migration、镜像升级、分支合并、PostgreSQL 重启或 current symlink 切换。

### Git Commits

| Hash | Message |
|------|---------|
| `39672e0` | (see git log) |
| `82fc0e7` | (see git log) |

### Testing

- [OK] 本地 dump checksum 与 R2 两个对象验证通过。
- [OK] Docker 网络密码和 backend 完整连接串认证通过。
- [OK] production/staging loopback 与公网 `/health`、`/readyz` 均返回 200。
- [OK] production schema 52、staging schema 67，均 dirty=false。
- [OK] backend 成功启动后 restart count 为 0，未产生新的认证或退出错误。

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

## Session 12: 修订信誉系统实现任务书

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


## Session 13: OAuth identity and administrator bootstrap hardening

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


## Session 14: Auth hardening release contract alignment

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


## Session 15: Harden model audit outbound HTTP

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


## Session 16: Production ingress and trusted client IP hardening

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


## Session 17: Reproducible release and build metadata

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


## Session 18: Verification and data lifecycle hardening

**Date**: 2026-07-26
**Task**: Verification and data lifecycle hardening
**Package**: frontend
**Branch**: `codex/prelaunch-identity-hardening`

### Summary

Added HMAC email challenges, finite idempotency generations, migration 63, and bounded PostgreSQL lifecycle maintenance with integration coverage.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `4839f7a` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 19: 完成上线前安全加固与发布门禁

**Date**: 2026-07-27
**Task**: 完成上线前安全加固与发布门禁
**Package**: frontend
**Branch**: `codex/prelaunch-identity-hardening`

### Summary

完成运行时加固、受保护指标、生产响应头、精确 SHA CI 与安全扫描、运维文档、规范同步、DOMPurify 补丁升级、P2 可行性结论和本地全量发布验证。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `68b6344` | (see git log) |
| `8038a23` | (see git log) |
| `74c3f41` | (see git log) |
| `32e91d4` | (see git log) |
| `1d60853` | (see git log) |
| `36e37a8` | (see git log) |
| `e9f83a5` | (see git log) |
| `c942c6a` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 20: Remove prelaunch demand module

**Date**: 2026-07-27
**Task**: Remove prelaunch demand module
**Package**: frontend
**Branch**: `codex/remove-demand-module`

### Summary

Removed the unlaunched demand module across frontend, backend, OpenAPI, search, notifications, database migration 65, smoke tests, and current product documentation; verified PostgreSQL, full tests, smokes, and desktop/mobile browser behavior.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `399bf78` | feat: remove prelaunch demand module |

### Testing

- [OK] Full Go suite; 47 frontend files / 187 tests; Vue typecheck; real-mode production build.
- [OK] OpenAPI generation/drift and migration documentation checks.
- [OK] PostgreSQL 18 migration/integration gate at version 65 with `demands` absent and `dirty=false`.
- [OK] Eleven real-backend smoke groups and explicit old-demand API 404 checks.
- [OK] Desktop 1440x900 and mobile 390x844 browser QA with no demand entry, overflow, overlap, or console error.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 21: Merge demand removal into staging

**Date**: 2026-07-27
**Task**: Merge demand removal into staging
**Package**: frontend
**Branch**: `codex/merge-demand-removal-staging`

### Summary

Merged demand removal and prelaunch hardening into current staging, reconciled migration 66, and passed local full-scope verification.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `db9742f196bf762d5ffa0e2f3a26810fe9cc9ca7` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 22: Fix PR 15 CI gates

**Date**: 2026-07-27
**Task**: Fix PR 15 CI gates
**Package**: frontend
**Branch**: `codex/remove-demand-module`

### Summary

Repaired clean Nuxt preparation, vulnerable frontend dependencies, PostgreSQL TCP readiness, and demand SEO merge leftovers; completed full local release validation.

### Main Changes

- Added Nuxt preparation to clean frozen installs and upgraded the patched
  frontend dependency graph.
- Changed PostgreSQL 18 readiness to probe the final loopback TCP server.
- Removed merged demand SEO/runtime residuals and added a regression guard.
- Captured both contracts in frontend and backend Trellis specifications.

### Git Commits

| Hash | Message |
|------|---------|
| `a4ab77063fb01338f4be155fa73a9d8b618aaff5` | `fix: restore PR 15 CI gates` |

### Testing

- [OK] Frontend tests, typecheck, real build, audit, and both Wrangler dry-runs.
- [OK] Go format, vet, full tests, race tests, and Govulncheck.
- [OK] PostgreSQL 18 migration/integration, OpenAPI, migration, security,
  Compose, package, and VPS release contracts.
- [OK] Actionlint, Gitleaks, Trivy filesystem/image, and Syft SPDX SBOM.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 23: Inline API payment settings in publish flows

**Date**: 2026-07-29
**Task**: Inline API payment settings in publish flows
**Package**: frontend
**Branch**: `codex/api-publish-three-modes`

### Summary

Extracted one shared account payment editor, opened it inline from free, fixed-package, and limited API publishing, preserved isolated drafts and publish snapshots, updated the query cache contract, and completed full responsive local verification.

### Main Changes

- Extracted account payment draft, validation, QR, removal confirmation, and
  mutation lifecycle into a shared editor used by My Center and publish flows.
- Replaced the publish summary route link with an inline dialog in free,
  fixed-package, and limited API publication.
- Preserved isolated drafts, dirty-close confirmation, query cache updates,
  and publish-time payment snapshots; synchronized the frontend code spec.

### Git Commits

| Hash | Message |
|------|---------|
| `9c339a2` | `feat: edit API payment settings in publish flow` |

### Testing

- [OK] Focused Vitest: 4 files / 31 tests.
- [OK] Full Vitest: 53 files / 226 tests; Nuxt typecheck.
- [OK] Real-backend Nuxt production build and `git diff --check`.
- [OK] Desktop `1440x900` and mobile `390x844` browser QA across all three
  publish modes.

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 24: Persist development account recovery through real backend

**Date**: 2026-07-29
**Task**: Persist development account recovery through real backend
**Package**: frontend
**Branch**: `codex/api-publish-three-modes`

### Summary

Made real backend mode explicit for default Nuxt development on port 5173, added explicit Mock mode and fail-fast API mode validation, aligned local origin/proxy docs, added recovery refetch regressions, and verified password/email persistence through PostgreSQL across a frontend restart.

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `44895f2` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 25: 统一登录访问控制与会话回跳

**Date**: 2026-07-29
**Task**: 统一登录访问控制与会话回跳
**Package**: frontend
**Branch**: `codex/unified-auth-route-guard`

### Summary

为发布、订单、个人中心、商户和管理页面建立统一登录守卫与安全回跳；修复会话失效、公开页匿名探测及限时额度发布错误状态，并完成自动化与浏览器验证。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `9a15a8d` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 26: 未登录隐藏私有导航

**Date**: 2026-07-29
**Task**: 未登录隐藏私有导航
**Package**: frontend
**Branch**: `codex/unified-auth-route-guard`

### Summary

匿名与认证未解析状态只展示公共导航；隐藏私有分组、通知和公告中心，并保留带安全回跳的登录后发布入口。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `9fb4457` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 27: 恢复生产数据库认证与后端

**Date**: 2026-07-31
**Task**: 恢复生产数据库认证与后端
**Package**: backend
**Branch**: `codex/fix-staging-release-traceability`

### Summary

备份生产 PostgreSQL，校准共享 env 与 SCRAM 角色密码，恢复现有生产后端；生产与 staging 内外网健康检查均通过，未迁移 schema 或更换镜像。

### Main Changes

(Add details)

### Git Commits

(No commits - planning session)

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 28: SSH 与 Tailscale 私网部署加固

**Date**: 2026-08-01
**Task**: SSH 与 Tailscale 私网部署加固
**Package**: backend
**Branch**: `codex/tailscale-private-deploy`

### Summary

禁用 root 与密码 SSH，建立 admin/deploy 密钥边界和 Tailscale 私网部署，真实 staging 自动部署通过后删除公网 IPv4/IPv6 OpenSSH Anywhere 规则，并完成回归验证。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `fdf62ee` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 29: API 订单角色视图与 24 小时核验期

**Date**: 2026-08-02
**Task**: API 订单角色视图与 24 小时核验期
**Package**: frontend
**Branch**: `codex/api-order-role-aware-detail`

### Summary

实现卖家交付即完成履约、买家 24 小时核验与自动完成，新增管理员只读订单详情并同步迁移、OpenAPI、角色化前端、测试和 Trellis 规范。

### Main Changes

(Add details)

### Git Commits

| Hash | Message |
|------|---------|
| `a527105` | (see git log) |

### Testing

- [OK] (Add test results)

### Status

[OK] **Completed**

### Next Steps

- None - task complete


## Session 30: 整合 API 订单与推广分支

**Date**: 2026-08-02
**Task**: 整合 API 订单与推广分支
**Package**: frontend
**Branch**: `codex/orders-promotions-integrated`

### Summary

整合订单核验期、API 推广、增长分析、邀请奖励和公开订单号，统一 migration 68-75，通过全量质量门禁并清理本地旧分支与推广 worktree。

### Git Commits

| Hash | Message |
|------|---------|
| `5586e81` | (see git log) |
| `29e1d4d` | (see git log) |
| `07af546` | (see git log) |

### Status

[OK] **Completed**


## Session 31: Fix Nuxt development first-page latency

**Date**: 2026-08-03
**Task**: Fix Nuxt development first-page latency
**Package**: frontend
**Branch**: `codex/orders-promotions-integrated`

### Summary

Reduced clean development homepage TTFB from 9.83s to 3.404s, fixed query-bearing root 500s, preserved production SWR, and pushed the branch.

### Git Commits

| Hash | Message |
|------|---------|
| `37a7d25` | (see git log) |

### Status

[OK] **Completed**


## Session 32: Fix PR 20 CI failures

**Date**: 2026-08-03
**Task**: Fix PR 20 CI failures
**Package**: frontend
**Branch**: `codex/orders-promotions-integrated`

### Summary

Restored PR #20 CI without changing product behavior or Umami configuration, tightened Gitleaks allowlists, and verified the pushed PR head is merge-ready.

### Main Changes

- Fixed the Linux-sensitive App.vue test path and migration 75 PostgreSQL rollback gate.
- Aligned completed-order review fixtures with migration 68 state constraints.
- Scoped public order-number Gitleaks allowlists to exact detected secret values and synchronized release/migration specs.

### Git Commits

| Hash | Message |
|------|---------|
| `9aaaba1` | (see git log) |
| `0f3aaaf` | (see git log) |

### Testing

- [OK] Frontend: 74 files and 369 tests; Node 24 typecheck and real-mode Nuxt build passed.
- [OK] Full go test ./..., go vet ./..., PostgreSQL integration 75->73->75, Gitleaks 8.30.1, migration docs, bash -n, and git diff --check passed.
- [OK] GitHub release-gate and Cloudflare Workers preview build passed for pushed SHA 0f3aaaf.

### Status

[OK] **Completed**

### Next Steps

- Merge PR #20 into staging.


## Session 33: Close launch P0 and active-user trust flow

**Date**: 2026-08-03
**Task**: Close launch P0 and active-user trust flow
**Package**: frontend
**Branch**: `codex/p0-p1-launch-closure`

### Summary

Disabled unsupported launch paths, repaired operations drift, and completed the active-user report, dispute, and appeal flow with authorization, privacy, pagination, and reputation transaction safeguards.

### Git Commits

| Hash | Message |
|------|---------|
| `0bd2874` | (see git log) |
| `3522839` | (see git log) |

### Status

[OK] **Completed**


## Session 34: Close remaining P0 data and moderation gaps

**Date**: 2026-08-03
**Task**: Close remaining P0 data and moderation gaps
**Package**: frontend
**Branch**: `codex/p0-p1-launch-closure`

### Summary

Implemented irreversible API credential retention destruction, lifecycle locking, immutable participant moderation supplements, self-safe contracts, and full PostgreSQL/frontend verification.

### Git Commits

| Hash | Message |
|------|---------|
| `be7d6eb` | (see git log) |

### Status

[OK] **Completed**


## Session 35: Close restricted-account appeal flow

**Date**: 2026-08-04
**Task**: Close restricted-account appeal flow
**Package**: frontend
**Branch**: `codex/p0-p1-launch-closure`

### Summary

Added dedicated linux.do-proven restricted-account appeal sessions, transactional account-governance appeals, standalone frontend flow, contact-value validation, CORS support, Migration 78, and full cross-layer tests.

### Git Commits

| Hash | Message |
|------|---------|
| `8774278` | (see git log) |

### Status

[OK] **Completed**


## Session 36: P1 first transaction guidance

**Date**: 2026-08-04
**Task**: P1 first transaction guidance
**Package**: frontend
**Branch**: `codex/p0-p1-launch-closure`

### Summary

Added fresh-query-gated first transaction guidance to the personal center, covered all six history sources, and verified desktop/mobile layouts.

### Git Commits

| Hash | Message |
|------|---------|
| `04e6bae` | (see git log) |

### Status

[OK] **Completed**


## Session 37: README official rewrite

**Date**: 2026-08-06
**Task**: README official rewrite
**Package**: frontend
**Branch**: `codex/readme-official-rewrite`

### Summary

重写中英文 README，加入匿名产品截图、linux.do 社区徽章与 Sentry 计划接入说明，并同步贡献指南中的本地验证命令。

### Git Commits

| Hash | Message |
|------|---------|
| `4174eb8` | (see git log) |

### Status

[OK] **Completed**


## Session 38: README product positioning

**Date**: 2026-08-06
**Task**: README product positioning
**Package**: frontend
**Branch**: `codex/readme-official-rewrite`

### Summary

补充中英文 README 的项目动机，说明社区信誉记录缺口与 ChatGPT、Claude 相关订阅和 API 服务额度供需，同时保留站外交易和非担保边界。

### Git Commits

| Hash | Message |
|------|---------|
| `3155d11` | (see git log) |

### Status

[OK] **Completed**


## Session 39: 修复健康探针重复授权与 HTTP 确认

**Date**: 2026-08-07
**Task**: 修复健康探针重复授权与 HTTP 确认
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

拆分测量身份与 Origin 授权身份，复用已保存 HTTP 地址的确认状态，并完成后端、PostgreSQL、前端与 Mock 全链路验证。

### Git Commits

| Hash | Message |
|------|---------|
| `ba6c07c` | (see git log) |

### Status

[OK] **Completed**


## Session 40: API probe sharing and model tester

**Date**: 2026-08-08
**Task**: API probe sharing and model tester
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

Replaced service-scoped challenge probes with reusable seller connections, froze and verified delivery targets, added the temporary buyer model tester, unified canonical model keys, and passed full backend/frontend/PostgreSQL gates.

### Git Commits

| Hash | Message |
|------|---------|
| `e9bc73d` | (see git log) |

### Status

[OK] **Completed**


## Session 41: API model catalog models.dev pricing sync

**Date**: 2026-08-08
**Task**: API model catalog models.dev pricing sync
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

Added administrator-reviewed models.dev preview/apply, atomic price version updates, explicit model activation, bulk status controls, OpenAPI contracts, responsive frontend workflow, and full local verification.

### Git Commits

| Hash | Message |
|------|---------|
| `7d79e8d` | (see git log) |

### Status

[OK] **Completed**


## Session 42: API 模型目录同步修复与紧凑界面改版

**Date**: 2026-08-08
**Task**: API 模型目录同步修复与紧凑界面改版
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

修复 models.dev 同步与模型启停稳定性，并将管理员 API 模型目录改为紧凑页签、组合筛选、明确开关状态和批量操作界面；完成全量测试、类型检查、real-mode 构建及桌面/移动端浏览器验收。

### Git Commits

| Hash | Message |
|------|---------|
| `3beccd9` | (see git log) |
| `110d11e` | (see git log) |

### Status

[OK] **Completed**


## Session 43: 真实模型探针与 24 小时健康度

**Date**: 2026-08-08
**Task**: 真实模型探针与 24 小时健康度
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

完成真实流式模型探针、一次性预检凭证、24 小时健康摘要、Runner 告警、成本统计和美西延迟校准，并通过全量与 PostgreSQL/浏览器验证。

### Git Commits

| Hash | Message |
|------|---------|
| `fd7e1e1` | (see git log) |

### Status

[OK] **Completed**


## Session 44: 收紧前端 Reka UI 依赖边界

**Date**: 2026-08-09
**Task**: 收紧前端 Reka UI 依赖边界
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

移除 5 个业务组件对 Reka UI 类型的直接引用，新增 shadcn-vue 依赖边界测试，并将规则写入前端组件规范。

### Git Commits

| Hash | Message |
|------|---------|
| `523e44e` | (see git log) |

### Status

[OK] **Completed**


## Session 45: API 订单纠纷处罚治理

**Date**: 2026-08-09
**Task**: API 订单纠纷处罚治理
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

实现管理员确认逾期后的 180 天阶梯处罚、两条 API 新订单事务门禁、公开限制明细与管理端显式确认，并完成全量本地验证。

### Git Commits

| Hash | Message |
|------|---------|
| `68be0c5` | (see git log) |

### Status

[OK] **Completed**


## Session 46: 工作区剩余改动原子提交整理

**Date**: 2026-08-09
**Task**: 工作区剩余改动原子提交整理
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

将 182 条未提交状态按业务意图重写为 10 个可独立审查和回退的原子提交，修正 shadcn-vue、通用分页、卖家订单和公告分页的历史归组边界；最终树与整理前完全一致，并通过全量 Go、Vitest、typecheck、real-mode build、OpenAPI、migration 和 diff 门禁。保留 `frontend/src/lib/api.ts` 的纯缩进改动、既有 stash 和纠纷处罚隔离工作树，未 push、部署或执行生产 migration。

### Git Commits

| Hash | Message |
|------|---------|
| `332bd00` | `refactor(frontend): align shadcn primitives with official registry` |
| `8b843f0` | `fix(catalog): refresh model preview after conflicts` |
| `98ff952` | `fix(announcements): preserve publication lifecycle timestamps` |
| `22640fe` | `feat(carpools): collect daily and weekly quotas` |
| `9c9e660` | `feat(admin): align listing moderation workflows` |
| `39ffba9` | `feat(pagination): paginate market and workspace lists` |
| `d70a1cc` | `fix(contacts): link linuxdo profiles to summary pages` |
| `84c35ac` | `feat(api-orders): add seller payment reconciliation details` |
| `8f5f865` | `feat(announcements): redesign admin and notification workflows` |
| `a7fc49e` | `refactor(frontend): remove legacy array pagination` |

### Status

[OK] **Completed**


## Session 47: 全仓半成品功能审计

**Date**: 2026-08-10
**Task**: 全仓半成品功能审计
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

完成全仓半成品审计；确认无 P0、2 个 P1、4 个 P2、1 个 P3，并记录 smoke 漂移与 36 个 Trellis 归档候选。

### Main Changes

- 生成跨前端、后端、OpenAPI、smoke 和 Trellis 台账的证据化审计报告。

### Git Commits

(No commits - planning session)

### Testing

- [OK] Go test/vet、104 文件 572 项 Vitest、Nuxt typecheck/real build、OpenAPI、migration、Compose 与本地 health/profile 复现通过。

### Status

[OK] **Completed**

### Next Steps

- 优先修复头像真实模式和公开用户资料，再修我的车源、管理员日志与 smoke。


## Session 48: 公告体验与首页公告条收口

**Date**: 2026-08-10
**Task**: 公告体验与首页公告条收口
**Package**: frontend
**Branch**: `codex/api-health-probe-repeat-prompts`

### Summary

完成用户公告详情单栏化、可选跳转按钮、首页公告条与响应式视觉收口，并通过完整前端门禁。

### Git Commits

| Hash | Message |
|------|---------|
| `72c5967` | (see git log) |
| `3b2fbbb` | (see git log) |
| `82f9a9c` | (see git log) |
| `6490c09` | (see git log) |

### Status

[OK] **Completed**
