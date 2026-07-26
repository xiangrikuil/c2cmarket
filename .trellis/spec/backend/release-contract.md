# Reproducible Release And Contract Drift

Date: 2026-07-26
Author: Codex

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
  "expectedMigrationVersion": 62
}
```

### 3. Contracts

- Both release scripts require a worktree with no staged, unstaged, or
  untracked non-ignored files and resolve the supplied ref with
  `<ref>^{commit}`.
- Source packages use `git archive` from the resolved commit and `gzip -n`.
  They write a SHA-256 sidecar and exclude local history, generated output,
  caches, dependencies, and every `.env*` except the three root examples.
- Release Docker context comes only from the resolved commit archive.
  `APP_VERSION`, the full commit, and the commit time are injected with Go
  ldflags and repeated in OCI version, revision, and created labels.
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
| Version contains unsupported characters or image is empty | Image build exits non-zero |
| Production/staging omits `BACKEND_IMAGE` | Compose configuration exits non-zero |
| Production/staging retains a backend build context | Compose exposure check exits non-zero |
| OpenAPI parse/reference warning or generated file drift | OpenAPI type check exits non-zero |
| Image labels differ from requested release metadata | Image build exits non-zero |

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
scripts/package-source.sh HEAD c2cmarket-source-check.tar.gz
scripts/build-backend-image.sh HEAD 0.0.0-test c2cmarket-backend:release-check
```

Assertions:

- Two archives of the same commit have the same SHA-256 and contain no
  forbidden path.
- Dirty-tree failures leave no named archive.
- `/version` fields match the injected metadata and
  `database.ExpectedMigrationVersion`.
- Image OCI labels match `/version`.
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
