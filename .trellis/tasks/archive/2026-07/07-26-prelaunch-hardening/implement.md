# Implementation Plan

日期：2026-07-26
执行者：Codex

## Guardrails Before Every Slice

- Re-run `git status --short` for target files and inspect both staged and unstaged diffs.
- Search every value/config/contract before changing it.
- Preserve overlapping user edits and do not use destructive Git cleanup.
- Add forward-only migrations and update migration docs/expected version together.
- Never read or print real environment values; tests use explicit fake values.
- Run focused tests after each small edit, then the slice gate.

## 1. Identity Blockers

- [x] Add failing OAuth tests for first/repeat/rename/normal-user collision/admin collision/two-provider/race/no-rebind/rollback.
- [x] Replace username-first OAuth upsert with identity-first transactional repository behavior.
- [x] Add deterministic unique handle generation and constrain provider-specific binding to the identity-owned user.
- [x] Bring the in-memory auth implementation to the same identity semantics.
- [x] Add bootstrap provenance schema and stable collision errors.
- [x] Make Bootstrap create-only and idempotent only for the proven bootstrap-owned account.
- [x] Prefer an explicit one-time bootstrap command; otherwise require an enable flag and retain fail-closed startup behavior.
- [x] Run auth unit, store integration, migration, full backend and OpenAPI checks.

Rollback point: identity migration and auth store/service files only.

## 2. SSRF, Ingress, And Trusted Client IP

- [x] Add deterministic failing tests for loopback, unspecified, metadata, RFC1918, link-local, multicast, ULA, mapped IPv6, DNS-private results, redirect, slow/large response and normal public HTTPS.
- [x] Add `internal/platform/outboundhttp` URL policy, resolver/dialer-bound transport and bounded response helpers.
- [x] Inject safe client into model audit adapters and validate target URLs before storage/use.
- [x] Add production host allowlist config without logging target secrets.
- [x] Make standardized client IP request-scoped and reuse it in rate limit/log/audit paths.
- [x] Add Cloudflare header support only for trusted proxy sources if required by existing deployment.
- [x] Remove production host-wide backend exposure; keep local health access loopback-only when a host port is necessary.
- [x] Add Compose and handler tests.

Rollback point: outbound client package, model audit adapter, server middleware and Compose override.

## 3. Reproducible Release

- [x] Rewrite source packaging around `git archive <commit-or-tag>` and refuse dirty/unresolved refs.
- [x] Inspect archive contents against explicit forbidden patterns including `.env*` except examples.
- [x] Add buildinfo injection and `/version`; include application version, Git SHA, build time and migration target.
- [x] Pass build args/labels through Docker and pin builds to the requested commit.
- [x] Strengthen OpenAPI check beyond method/path where practical and add generated frontend type drift check.
- [x] Add release scripts/tests and update README/runbook.

Rollback point: package script, buildinfo/version route, Dockerfiles and contract scripts.

## 4. Verification And Data Lifecycle

- [x] Add `EMAIL_VERIFICATION_PEPPER` config and HMAC digest helper.
- [x] Atomically lock latest active challenge, increment attempts on every failed verification, expire at max attempts, and consume/update user in one transaction.
- [x] Invalidate older challenges when issuing a new code; align in-memory behavior.
- [x] Define retention constants/config for sessions, verification challenges, idempotency states, contact sessions, notifications/events/audit.
- [x] Add bounded batch cleanup methods and a PostgreSQL advisory-lock worker with graceful shutdown.
- [x] Add idempotency failed/completed retention and response body cap/skip semantics.
- [x] Run profile/idempotency/worker integration tests.

Rollback point: verification migration/service/store, cleanup repository/runner and app wiring.

## 5. Runtime Hardening

- [x] Make memory limiter bounded with scheduled cleanup and counters; preserve concurrency safety.
- [x] Return integer `Retry-After` and stable Problem Details for every limiter.
- [x] Apply distinct IP/user policies to login, OAuth, verification, contacts, orders, reports and model audit.
- [x] Add versioned encryption/fingerprint keyring config and select decrypt key by record version.
- [x] Bind AES-GCM AAD to record ID, field type and key version; add dry-run re-encryption command with batch cursor.
- [x] Parse and validate pgxpool size/lifetime/idle/health settings and database session timeouts.
- [x] Add pool stats and background-task/request context timeouts.
- [x] Complete CSP, frame, permissions and production header policy with tests.
- [x] Add structured request/operation metrics/logs without sensitive fields.

Rollback point: limiter, keyring, database config, headers and metrics independently.

## 6. CI, Security Scans, And Documentation

- [x] Add Go format/vet/test/race/govulncheck jobs with pinned versions.
- [x] Add frontend frozen install/typecheck/test/build/audit using existing scripts.
- [x] Add Gitleaks, Trivy filesystem/image, SBOM and Docker build jobs with pinned versions.
- [x] Add temporary PostgreSQL migration/integration job and release gates tied to commit/tag.
- [x] Ensure checks do not print environment file contents.
- [x] Update/add deployment, security, operations, backup/restore and release checklist docs with placeholder values only.
- [x] Run local equivalents and record real PASS/FAIL/SKIPPED results.

Rollback point: each workflow job and each documentation file.

## 7. P2 Feasibility Gate

- [x] Measure large backend/frontend files after P0/P1.
- [x] Split only modules touched by the hardening work where an extraction reduces current duplication and preserves tests.
- [x] Generate OpenAPI TypeScript contracts only if one source can replace current duplicate types without a broad call-site rewrite.
- [x] Otherwise record explicit follow-up tasks and residual risk; do not block release or claim completion.

### P2 Feasibility Result

- `frontend/src/api/generated/openapi/types.gen.ts` is 11,464 generated lines.
  It is owned by `frontend/openapi-ts.config.mjs` and the byte-for-byte drift
  check, so it must not be manually split.
- `frontend/src/lib/api.ts` is 5,599 handwritten lines but was not changed by
  this hardening task. Splitting it here would create a broad call-site rewrite
  outside the release-blocking scope.
- Hardening touched `backend/internal/store/postgres/api_market.go` (1,773
  lines), `api_quota.go` (1,773), and `api_order.go` (1,323). Their encrypted
  field handling already reuses the shared contact codec; the remaining size is
  dominated by domain-specific transactional repository operations. No
  extraction found in this pass reduces current duplication without moving or
  weakening transaction boundaries.
- `backend/internal/server/postgres_integration_test.go` is 1,913 lines and is
  test-only. Splitting it does not change production maintainability enough to
  justify release risk.
- Decision: keep these files unchanged for this release. Treat the handwritten
  API facade and domain repository files as post-launch, independently tested
  refactor candidates; they are not release blockers.

## Validation Commands

```text
git diff --check
cd backend && gofmt -l .
cd backend && go vet ./...
cd backend && go test -count=1 ./...
cd backend && go test -race -count=1 ./...
node scripts/check-openapi-routes.mjs
node scripts/check-migrations-doc.mjs
cd frontend && pnpm install --frozen-lockfile
cd frontend && pnpm typecheck
cd frontend && pnpm test
cd frontend && VITE_API_MODE=real pnpm build
docker compose -f compose.yaml -f compose.prod.yaml config
docker build --build-arg GIT_COMMIT=<test-sha> backend
```

Integration and scanner commands will be added only when their exact repository scripts/actions exist; no nonexistent command will be reported as executed.
