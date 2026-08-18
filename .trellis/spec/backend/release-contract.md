# Reproducible Release And Contract Drift

Date: 2026-07-26
Author: Codex
Updated: 2026-08-08

## Scenario: Fixed-Commit Backend Release And OpenAPI Type Snapshot

### 1. Scope / Trigger

- Trigger: changing source packaging, backend Docker builds, production or
  staging Compose, build metadata, `/version`, the OpenAPI document, or
  generated frontend OpenAPI types.
- This contract prevents a release from silently including dirty or untracked
  worktree files and prevents production from rebuilding a different image
  from the operator's current checkout.

### 2. Signatures

```text
scripts/package-source.sh [git-ref] [archive-name.tar.gz]
scripts/build-backend-image.sh <git-ref> <version> <image>
GET /version
node scripts/check-openapi-types.mjs
```

`GET /version` returns:

```json
{
  "service": "c2c-market-backend",
  "version": "0.1.0",
  "gitCommit": "<full resolved commit>",
  "buildTime": "<RFC3339 commit time>",
  "expectedMigrationVersion": 113
}
```

### 3. Contracts

- Both release scripts require a worktree with no staged, unstaged, or
  untracked non-ignored files and resolve the supplied ref with
  `<ref>^{commit}`.
- Source packages use `git archive` from the resolved commit and `gzip -n`.
  They write a SHA-256 sidecar and exclude local history, generated output,
  caches, dependencies, and every `.env*` except the three root examples.
- Portable `mktemp` templates end with `XXXXXX`. A suffix after the placeholder
  can become a literal filename on BSD/macOS and makes concurrent packaging
  collide.
- Release Docker context comes only from the resolved commit archive.
  `APP_VERSION`, the full commit, and the commit time are injected with Go
  ldflags and repeated in OCI version, revision, and created labels.
- The reusable GHCR publisher resolves the commit time from its exact
  `git_sha` checkout and passes the same `APP_VERSION`, `GIT_COMMIT`, and
  `BUILD_TIME` inputs to `backend/Dockerfile`; setting only OCI labels is not
  sufficient because it leaves `/version` on development defaults.
- Local Go builds return explicit `development` / `unknown` metadata.
  `expectedMigrationVersion` is read directly from
  `database.ExpectedMigrationVersion`, never copied into an ldflag.
- Development Compose retains `backend.build`. Production and staging remove
  it and require `BACKEND_IMAGE`; deployment uses `up --no-build`.
- `frontend/openapi-ts.config.mjs` is the single generator configuration.
  Generated files under `frontend/src/api/generated/openapi/` are mechanical
  snapshots and must not be hand-edited.
- `scripts/check-openapi-types.mjs` loads the committed generator config,
  regenerates into a temporary directory, and compares the exact file set and
  bytes with the committed snapshot.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Staged, unstaged, or untracked worktree content | Release script exits non-zero before a final artifact is written |
| Ref does not resolve to a commit | Release script exits non-zero |
| Archive name is not a `.tar.gz` basename | Packaging exits non-zero |
| Final archive name already exists | Packaging exits non-zero without overwriting it |
| Archive contains a forbidden path or environment file | Packaging exits non-zero without a final archive |
| Two packages run concurrently with distinct final names | Both use distinct temporary files and succeed |
| Version contains unsupported characters or image is empty | Image build exits non-zero |
| Production/staging omits `BACKEND_IMAGE` | Compose configuration exits non-zero |
| Production/staging retains a backend build context | Compose exposure check exits non-zero |
| OpenAPI parse/reference warning or generated file drift | OpenAPI type check exits non-zero |
| Image labels differ from requested release metadata | Image build exits non-zero |
| Reusable publisher omits binary build arguments or matching OCI labels | Release workflow contract check exits non-zero |
| Running `/version.gitCommit` differs from the immutable image SHA | VPS deployment exits non-zero before current-link promotion |

### 5. Good / Base / Bad Cases

- Good: build `c2cmarket-backend:0.1.0` from tag `v0.1.0`, set the exact image
  as staging `BACKEND_IMAGE`, validate `/version` and labels, then promote the
  same image reference to production.
- Base: local `docker compose --profile app up -d --build backend` uses
  development metadata and remains suitable for local work only.
- Bad: run `docker compose ... --profile app build backend` with the production
  overlay; this rebuilds from an operator checkout and bypasses commit
  identity.
- Bad: edit `types.gen.ts` to fix a type; regeneration will discard the edit
  and the drift check must fail until the OpenAPI source is corrected.

### 6. Tests Required

```bash
bash -n scripts/package-source.sh scripts/build-backend-image.sh
node scripts/test-package-source.mjs
cd backend && go test -count=1 ./internal/buildinfo ./internal/server
node scripts/check-openapi-routes.mjs
node scripts/check-openapi-types.mjs
node scripts/check-compose-exposure.mjs
ruby scripts/check-release-workflow.rb
scripts/test-vps-release.sh
scripts/package-source.sh HEAD c2cmarket-source-check.tar.gz
scripts/build-backend-image.sh HEAD 0.0.0-test c2cmarket-backend:release-check
```

Assertions:

- Two concurrent archives of the same commit both succeed, have the same
  SHA-256, and contain no forbidden path.
- Dirty-tree failures leave no named archive.
- `/version` fields match the injected metadata and
  `database.ExpectedMigrationVersion`.
- Image OCI labels match `/version`.
- Reusable publishing uses the exact workflow SHA and its commit time for both
  build arguments and OCI labels.
- Streamed VPS installation survives a deployment child that consumes stdin,
  and rejects a runtime commit that differs from the image SHA.
- Production and staging expanded Compose configs contain `image` but no
  backend `build`.
- OpenAPI regeneration has the same file set and bytes as the snapshot.

### 7. Wrong vs Correct

#### Wrong

```bash
docker compose \
  --env-file .env.production \
  -f compose.yaml \
  -f compose.prod.yaml \
  --profile app build backend
```

#### Correct

```bash
scripts/build-backend-image.sh v0.1.0 0.1.0 c2cmarket-backend:0.1.0
# Set BACKEND_IMAGE=c2cmarket-backend:0.1.0 in the target environment file.
docker compose \
  --env-file .env.production \
  -f compose.yaml \
  -f compose.prod.yaml \
  --profile app up --no-build -d backend
curl -fsS http://127.0.0.1:8080/version
```

## Scenario: Exact-SHA CI Release Gate

### 1. Scope / Trigger

- Trigger: changing `.github/workflows/ci.yml`, toolchain versions, dependency
  scans, Docker/SBOM jobs, production environment examples, or the set of
  checks required before a release.
- The workflow is a release contract. A green subset is not evidence that an
  exact commit is releasable.

### 2. Signatures

```text
.github/workflows/ci.yml
jobs.release-gate.needs
bash scripts/ci-postgres-integration.sh
docker exec <postgres-container> pg_isready --host 127.0.0.1
node scripts/check-security-headers.mjs
node scripts/check-compose-exposure.mjs
gitleaks git --config .gitleaks.toml --log-opts="origin/staging..HEAD"
```

Required release jobs:

```text
backend
backend-race
contracts
postgres-integration
frontend
secret-scan
filesystem-scan
image
release-gate
```

### 3. Contracts

- The workflow uses repository-read permission by default and pins third-party
  actions by full commit SHA. Scanner and generator versions are explicit.
- Go comes from `backend/go.mod`; Node and pnpm match the supported frontend
  toolchain. Frontend installation is frozen and the production-like build
  uses `NUXT_PUBLIC_API_MODE=real` with both public and server API base URLs.
- Backend format, vet, tests, race, and `govulncheck` are independent evidence.
  PostgreSQL 18 integration migrates four empty databases through
  `database.ExpectedMigrationVersion` with `dirty=false`, verifies the current
  Version 82 rollback through Versions 82, 81, 80, 79, 78, and 77 to Version 76 before reapplying
  all six migrations, and verifies the Version 65→current legacy-constraint upgrade
  path in a fifth isolated database.
- PostgreSQL integration readiness must probe `127.0.0.1` from inside the
  container. The official PostgreSQL image starts a temporary Unix-socket-only
  initialization server before restarting into the final TCP server; a
  socket-default `pg_isready` success is not evidence that migration commands
  can safely begin.
- Contract checks cover routes, generated OpenAPI files, migrations, security
  headers, Compose exposure, commit-only source packaging, and whitespace.
- Gitleaks scans full Git history. Allowlists for public identifiers must scope
  `targetRules`, `paths`, and an anchored `regexTarget = "secret"` expression.
  A line-target allowlist is forbidden because it can suppress an unrelated
  secret found on the same line. Trivy scans both the repository filesystem and
  the exact-commit backend image for HIGH/CRITICAL findings. Syft produces a
  non-empty SPDX JSON SBOM for that image.
- The image job requires a clean checkout whose `HEAD` equals `GITHUB_SHA`,
  then calls `scripts/build-backend-image.sh` with that SHA. Production and
  staging consume a prebuilt `BACKEND_IMAGE`; they never build from a deployment
  checkout.
- `release-gate` uses `if: always()`, depends on every required job, and fails
  unless every dependency result is `success`. It rechecks the exact SHA and
  clean checkout.
- CI and local gates must not print real environment files or secret values.
  Tests use repository examples and explicit test-only values.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Required job fails, is cancelled, or is skipped | `release-gate` fails |
| Workflow action is tag-only or unpinned | Static review/actionlint blocks the change |
| Checkout SHA differs from `GITHUB_SHA` | Image and final release checks fail |
| Checkout has tracked or untracked release input | Image/release clean-source check fails |
| `pg_isready` observes only the temporary initialization Unix socket | Keep retrying; migration and database creation may start only after the loopback TCP probe succeeds. |
| PostgreSQL migration is dirty or not current | Integration gate fails |
| OpenAPI generated snapshot differs | Contracts job fails |
| High/critical filesystem or image finding exists | Trivy job fails |
| Gitleaks detects a non-allowlisted secret | Secret-scan job fails |
| An allowed public identifier shares a line with an unrelated generic key | Only the exact identifier is allowlisted; the unrelated key still fails the secret scan |
| SBOM is missing or empty | Image job fails |
| Production Compose retains `build` or public PostgreSQL | Compose guard fails |

### 5. Good / Base / Bad Cases

- Good: all eight prerequisite jobs succeed for one full SHA, PostgreSQL
  readiness proves the final loopback TCP service is accepting connections,
  the image labels and SBOM identify that SHA, and `release-gate` succeeds.
- Good: a public order-number fixture is allowlisted by exact detected value,
  while a generic key beside it remains reportable.
- Base: a pull request runs the same gates without publishing or deploying.
- Bad: `pg_isready` omits `--host`, reports the initialization socket ready,
  and `createdb` races the PostgreSQL entrypoint restart.
- Bad: treating a green unit-test job as release approval while image scan or
  PostgreSQL integration was skipped.
- Bad: allowlisting an entire source line because it contains one public
  identifier, thereby hiding another finding on that line.
- Bad: rebuilding on the server from a mutable branch after CI scanned a
  different image.

### 6. Tests Required

```bash
actionlint .github/workflows/ci.yml
bash scripts/ci-postgres-integration.sh
cd backend && go test -count=1 ./...
cd backend && go test -race -count=1 ./...
cd frontend && pnpm install --frozen-lockfile
cd frontend && pnpm typecheck && pnpm test
cd frontend && \
  NUXT_PUBLIC_API_MODE=real \
  NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
  NUXT_API_BASE_URL=https://api.c2cmarket.shop \
  pnpm build
node scripts/check-openapi-routes.mjs
node scripts/check-openapi-types.mjs
node scripts/check-migrations-doc.mjs
node scripts/check-security-headers.mjs
node scripts/check-compose-exposure.mjs
node scripts/test-package-source.mjs
gitleaks git --config .gitleaks.toml --log-opts="origin/staging..HEAD"
```

The PostgreSQL gate must pass from both cold and warm image starts, migrate all
empty databases to `ExpectedMigrationVersion` with `dirty=false`, and verify
the supported Version 65→current path plus the current two-migration rollback
and reapply path. Its readiness command must contain `--host 127.0.0.1`.

Local release evidence additionally scans Git history, the filesystem, and the
exact-commit image, then generates and parses a non-empty SPDX JSON SBOM. Every
new Gitleaks allowlist also needs a negative probe proving that an unrelated
generic key on the same line remains detected.

### 7. Wrong vs Correct

#### Wrong

```yaml
release-gate:
  needs: [backend]
```

This approves a commit without frontend, integration, secret, filesystem, image,
or SBOM evidence.

#### Correct

```yaml
release-gate:
  if: ${{ always() }}
  needs:
    - backend
    - backend-race
    - contracts
    - postgres-integration
    - frontend
    - secret-scan
    - filesystem-scan
    - image
```

#### Wrong: probe the entrypoint's temporary Unix socket

```bash
docker exec "${postgres_container}" \
  pg_isready --quiet --username "${POSTGRES_USER}" --dbname postgres
```

#### Correct: wait for the final loopback TCP server

```bash
docker exec "${postgres_container}" \
  pg_isready --quiet --host 127.0.0.1 \
    --username "${POSTGRES_USER}" --dbname postgres
```
