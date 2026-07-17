# Backend Deployment Contract

> Executable CI, GHCR, and VPS release requirements for the backend.

Date: 2026-07-17

Executor: Codex

## Scenario: Release a tested backend commit to staging or production

### 1. Scope / Trigger

Apply this contract whenever changing GitHub Actions release jobs, backend
image metadata, Compose deployment configuration, VPS release scripts,
database migration sequencing, or production backup wiring.

The frontend remains owned by Cloudflare Workers Builds. This contract owns
only the Go backend image and the Compose files, migrations, and scripts needed
to run it on the VPS.

### 2. Signatures

The reusable workflow accepts exactly these inputs:

```text
deploy_environment: "staging" | "production"
release_tag: "staging" | "production"
git_sha: 40 lowercase hexadecimal Git commit characters
```

The remote installer and deployment entry points are:

```bash
scripts/install-vps-release.sh <environment> <git-sha> <image-ref> <archive-path>
scripts/deploy-vps-backend.sh <environment> <image-ref>
```

The deploy script owns the environment mapping; callers must not supply an
arbitrary Compose project, env path, or port:

| Environment | Compose project | Shared env | Port | Current link |
| --- | --- | --- | --- | --- |
| staging | `c2c-staging` | `/opt/c2cmarket/shared/.env.staging` | 8081 | `/opt/c2cmarket/staging-current` |
| production | `c2c-prod` | `/opt/c2cmarket/shared/.env.production` | 8080 | `/opt/c2cmarket/current` |

### 3. Contracts

- `.github/workflows/ci.yml` runs existing backend and frontend checks for all
  pull requests and for pushes to `staging` and `main`.
- A release job may run only after both CI jobs succeed. Pull requests never
  receive deployment secrets and never deploy.
- The image name is `ghcr.io/xiangrikuil/c2cmarket-backend`. GitHub Actions may
  publish readable `staging` or `production` aliases, but VPS deployment must
  use the immutable full-SHA tag.
- The image must be built from `backend/Dockerfile` and carry the OCI source
  repository and revision labels.
- GitHub environment secrets are `VPS_HOST`, `VPS_USER`,
  `VPS_SSH_PRIVATE_KEY`, and `VPS_SSH_KNOWN_HOSTS`. `production` owns the
  required-reviewer gate.
- The release archive contains `compose.yaml`, `compose.prod.yaml`,
  `backend/migrations`, and the install, deploy, and production-backup scripts.
  The VPS does not run `git pull` or build application source.
- `compose.yaml` retains `build.context` for local development and exposes
  `image: ${BACKEND_IMAGE:-c2cmarket-backend:local}`. A VPS release pulls the
  SHA image and starts it with `--no-build`.
- Production must finish the existing PostgreSQL dump, checksum, and R2 upload
  before migrations. Staging must not invoke the production backup.
- The installer changes the current symlink only after migrations, backend
  startup, `/health`, and `/readyz` all succeed.

### 4. Validation & Error Matrix

| Condition | Required behavior |
| --- | --- |
| Environment is not `staging` or `production` | Exit 2 before running Compose |
| Git SHA is not 40 lowercase hex characters | Exit 2 before extraction or deployment |
| Image is not the repository's matching full-SHA tag | Exit 2; never pull or start it |
| Shared env file is missing | Exit non-zero before Compose mutation |
| Compose expansion fails | Exit non-zero before database backup or migration |
| Production backup or R2 upload fails | Exit non-zero; do not run migration |
| Image pull or migration fails | Exit non-zero; do not update the current symlink |
| Health or readiness exhausts retries | Print Compose status and exit non-zero |
| Current path exists as a regular file/directory | Refuse to overwrite it with a symlink |
| SSH identity or verified known-hosts data is missing | Fail in the runner before SCP |

Database migrations are never automatically rolled down. A failed release may
leave its version directory and uploaded archive for diagnosis, but it must not
claim success by changing the current link.

### 5. Good / Base / Bad Cases

- Good: a tested `staging` push builds `<sha>`, deploys `c2c-staging` on 8081,
  passes both loopback probes, and then changes `staging-current`.
- Base: a tested `main` push publishes `<sha>`, waits for production approval,
  completes the R2 backup, deploys `c2c-prod` on 8080, and then changes
  `current`.
- Bad: a workflow deploys `:latest`, reuses a personal root key, disables SSH
  host verification, builds on the VPS, migrates before backup, or changes the
  current link before readiness succeeds.

### 6. Tests Required

- Parse both workflow files as YAML and run an Actions-aware linter when one is
  already available in the trusted local toolchain.
- Run `bash -n` for the installer, deployment, backup, and release tests.
- Run `scripts/test-vps-release.sh` and assert fixed ports, staging backup
  exclusion, production backup-before-migration, `--no-build`, error
  propagation, and current-link behavior.
- Expand production and staging Compose configurations with their real ignored
  env files and `config --quiet`.
- Build the local backend image to prove the default `build` path still works.
- Run `go test ./...`, frontend typecheck/build/tests, OpenAPI route checks,
  migration documentation checks, and `git diff --check` before handoff.

### 7. Wrong vs Correct

#### Wrong

```bash
docker compose up -d --build backend
ln -sfn /opt/c2cmarket/releases/production/new /opt/c2cmarket/current
```

This rebuilds unverified source on the VPS and changes the success pointer
without proving backup, migration, health, or readiness.

#### Correct

```bash
scripts/deploy-vps-backend.sh \
  production \
  ghcr.io/xiangrikuil/c2cmarket-backend:<40-character-git-sha>
ln -sfn \
  /opt/c2cmarket/releases/production/<40-character-git-sha> \
  /opt/c2cmarket/current
```

The second command is permitted only after the deployment script exits zero.
The normal GitHub workflow enforces that order through
`scripts/install-vps-release.sh`.
