# C2CMarket Maintenance Hardening Report

日期：2026-07-06
执行者：Codex / xiangrikuil

## Completed Changes

- Phase 0 safety net: CI now covers backend tests, frontend install/typecheck/build/test, and OpenAPI route parity.
- P0 auth hardening: password writes use `argon2id_v1`; legacy `sha256_salted_v1` is verify/rehash only; fixed admin password seed was removed in favor of explicit bootstrap.
- P0 request/proxy hardening: strict JSON decoding, request body limits, trusted proxy handling, and rate-limit bypass tests were added.
- Runtime hardening: `/readyz` checks PostgreSQL migration state, HTTP server timeouts/shutdown/logging baseline are in place, and `ExpectedMigrationVersion` now tracks migration 36.
- Toolchain hardening: frontend dependencies no longer use `latest`; Node/pnpm engines and CI toolchain are pinned.
- Frontend real/mock isolation: real backend mode no longer silently creates dev sessions or falls back to mock success data for migrated paths.
- Backend service boundaries: domain services/repository contracts own business behavior; `core.Service` remains a compatibility facade.
- Database pagination: prioritized public/owner/admin lists use repository-level pagination instead of handler-side full-load slicing.
- Search alignment: PostgreSQL search predicates now match trigram expression indexes where applicable, with `scripts/explain-search.sql` for EXPLAIN verification.
- Final cleanup: migration doc drift check, source packaging script, architecture/runbook updates, backend client critical-path tests, and this report were added.

## Verification Results

| Check | Result |
| --- | --- |
| `node scripts/check-migrations-doc.mjs` | Pass: 36 migrations documented, latest version 36. |
| `scripts/package-source.sh c2cmarket-source-check.tar.gz` | Pass: archive created under `output/` and forbidden paths were absent. |
| `node scripts/check-openapi-routes.mjs` with Node 24 runtime | Pass: 211 method/path pairs. |
| `docker run --rm -e GOPROXY=https://goproxy.cn,direct -v ... golang:1.26-alpine go test ./...` | Pass. Plain `proxy.golang.org` runs timed out from this environment; the mirror avoided dependency download timeouts. |
| `node node_modules/vue-tsc/bin/vue-tsc.js -b --pretty false` with Node 24 runtime | Pass. |
| `VITE_API_MODE=real node node_modules/vite/bin/vite.js build` with Node 24 runtime | Pass. Rolldown reported upstream `@vueuse/core` pure-annotation warnings but completed successfully. |
| `node node_modules/vitest/vitest.mjs run` with Node 24 runtime | Pass: 7 test files, 9 tests. |
| `git diff --check` | Pass. |

Local note: the shell `pnpm` command is version 11.7.0 and the shell Node is
22.22.1, while the project requires pnpm `>=10 <11` and Node `>=24.11 <25`.
For local verification in this session, the Codex bundled Node 24.14.0 runtime
ran the project-local `vue-tsc`, `vite`, and `vitest` binaries directly. CI is
configured for Node 24.11 and pnpm 10.

## Remaining Technical Debt

- Production still needs real OAuth provider configuration, TLS/domain setup,
  static frontend hosting, backup/restore drills, monitoring/alerting, log
  retention, secret management, and key rotation procedures.
- Frontend mock/demo code remains intentionally available for local mode. Before
  a strict production-only frontend split, remove or quarantine mock wording and
  demo-only state paths in a dedicated task.
- Local Docker Go verification may need a region-appropriate `GOPROXY` to avoid
  `proxy.golang.org` TLS timeouts.
- Vite/Rolldown currently emits non-fatal pure-annotation warnings from
  `@vueuse/core`; track upstream dependency behavior before treating this as a
  build failure.
- Production smoke scripts that use fake OAuth/dev auth are not safe to run
  against production. Real production validation needs a controlled
  non-destructive checklist.

## Next Steps

- Run migrations through version 36 in the target database and verify `/readyz`.
- Execute admin bootstrap once, then remove bootstrap secrets from the runtime
  environment.
- Configure the frontend static host or reverse proxy with CSP, TLS, SPA
  fallback, and API origin routing.
- Add production observability: structured logs, metrics, alerts, and backup
  restore drills.
- Plan a separate frontend real-only cleanup if mock/demo mode should be removed
  from production source bundles.

## Deployment Checklist

- [ ] production database migrated to latest version
- [ ] `/readyz` returns healthy
- [ ] admin bootstrap completed and bootstrap secret removed
- [ ] password algorithm is `argon2id_v1` for new credentials
- [ ] CORS origins configured
- [ ] CSRF enabled
- [ ] trusted proxy configured or disabled
- [ ] rate limiting enabled
- [ ] frontend built with `VITE_ENABLE_MOCK=false`
- [ ] frontend built with `VITE_API_MODE=real` or `VITE_API_BASE_URL`
- [ ] CSP configured at static host or reverse proxy
- [ ] backup and restore tested
- [ ] logs do not contain secrets or contact plaintext
