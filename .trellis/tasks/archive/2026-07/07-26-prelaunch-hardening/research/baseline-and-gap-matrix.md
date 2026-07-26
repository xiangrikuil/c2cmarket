# Baseline And Gap Matrix

Date: 2026-07-26
Author: Codex

## Baseline

- Branch: `docs/open-source-readme`
- Commit: `0f14ad75d9ec7e658d49830533c7c603c7c4d849`
- Worktree: 233 uncommitted changes at session start.
- Backend baseline: `go test -count=1 ./...` passed.
- Contract baseline: OpenAPI route guard passed with 270 method/path pairs.
- Migration baseline: 61 migrations documented; `ExpectedMigrationVersion` is 61.
- Diff baseline: staged and unstaged `git diff --check` passed.
- Frontend baseline: initial `pnpm` calls failed before project execution because login-shell initialization selected Node 14. With an explicit Node `v24.13.0` path, `pnpm typecheck` passed and Vitest passed 48 files / 189 tests.

## P0 Evidence

| Area | Current evidence | Gap |
| --- | --- | --- |
| OAuth | `auth_identities` unique key exists | `UpsertOAuthUser` upserts `users` by username and updates `auth_identities.user_id` |
| Bootstrap | Password uses Argon2id | Repository reuses username, grants admin, and overwrites password; no provenance |
| SSRF | Adapter has total timeout and body `LimitReader` | Default transport, arbitrary scheme/host, redirects enabled, no IP validation/rebinding defense |
| Proxy | Trusted CIDR gate and spoofing tests exist | Standardized IP is local to Server and Cloudflare-specific path/chain semantics need review |
| Ingress | Production removes PostgreSQL ports | Backend host-wide port inherited from base Compose |
| Packaging | Some generated paths filtered | Untracked files are explicitly packaged; `.env*` not in archive forbidden pattern |
| Versioning | Readiness checks migration 61 | No app version, Git SHA, build time or commit-bound Docker metadata |

## P1 Evidence

| Area | Current evidence | Gap |
| --- | --- | --- |
| Verification | Challenge table has `attempt_count` | Raw SHA-256, no failed-attempt increment/lockout, old challenges remain active |
| Idempotency | 24h processing expiry and startup cleanup | Completed rows never expire; cleanup only processing and only at startup; body unbounded |
| Sessions | Idle/absolute renewal contract in migration 53 | No periodic expired/revoked cleanup |
| Limiter | Mutex, IP+user keys, window reset | Unbounded map, opportunistic full scan, no metrics or `Retry-After` header |
| Contact crypto | AES-GCM, HMAC fingerprint, versions stored | Single key only, decrypt ignores version, no AAD or re-encryption command |
| Database | Readiness ping and migration check | No explicit pool sizes/lifetimes/health period or PostgreSQL session timeouts |
| Headers | `nosniff`, Referrer-Policy, HSTS | Missing CSP, frame and permissions policies |
| Observability | Request ID and request logger exist | Missing requested metrics and consistent sensitive-field redaction contracts |
| CI | Backend/front-end/route/migration basics | Missing format, vet, race, vuln, secrets, Trivy, SBOM, migration DB and release gate |

## Overlap Risks

- Auth, app, database, server and migration files already contain uncommitted work from session renewal, reputation and quota tasks.
- Migration numbers 51-61 are untracked in the worktree; new migrations must start after the actual highest current file and must not renumber or modify those files.
- Backend Dockerfile and OpenAPI are already modified.
- The implementation must inspect the latest diff immediately before each edit and preserve concurrent user changes.

## Tool Availability

Project instructions prefer `sequential-thinking`, `code-index`, `shrimp-task-manager` and Exa. Deferred-tool search did not expose callable versions in this session. Planning therefore uses Trellis phase scripts, direct repository evidence via `rg`, local Git inspection, `trellis mem`, and local test commands. No synthetic output is attributed to unavailable tools.
