# Backend Deployment Contract

> Executable CI, GHCR, and VPS release requirements for the backend.

Date: 2026-07-18

Executor: Codex

Updated: 2026-08-01

## Scenario: Release a tested backend commit to staging or production

### 1. Scope / Trigger

Apply this contract whenever changing GitHub Actions release jobs, backend
image metadata, Compose deployment configuration, VPS release scripts,
database migration sequencing, production backup wiring, or the private SSH
path used by GitHub-hosted deployment runners.

The frontend remains owned by Cloudflare Workers Builds. This contract owns
only the Go backend image and the Compose files, migrations, and scripts needed
to run it on the VPS.

### 2. Signatures

The reusable image-publishing workflow accepts exactly these inputs:

```text
release_tag: "staging" | "production"
git_sha: 40 lowercase hexadecimal Git commit characters
```

The remote installer and deployment entry points are:

```bash
scripts/install-vps-release.sh <environment> <git-sha> <image-ref> <archive-path>
scripts/deploy-vps-backend.sh <environment> <image-ref>
```

The deployment network identities are:

```text
VPS Tailscale address: 100.67.1.59
VPS device tag: tag:c2c-prod
GitHub Actions device tag: tag:c2c-ci
Allowed private path: tag:c2c-ci -> tag:c2c-prod:tcp/22
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
  repository, version, revision, and created labels. The reusable publisher
  derives the build time from the exact release commit and passes
  `APP_VERSION`, `GIT_COMMIT`, and `BUILD_TIME` as Docker build arguments so
  `/version` and the OCI labels describe the same artifact.
- GitHub environment secrets are `VPS_HOST`, `VPS_USER`,
  `VPS_SSH_PRIVATE_KEY`, and `VPS_SSH_KNOWN_HOSTS`. `production` owns the
  required-reviewer gate.
- GitHub repository secrets are `TS_OAUTH_CLIENT_ID` and `TS_OAUTH_SECRET`.
  The Tailscale OAuth client may write auth keys only and must force
  `tag:c2c-ci`; it must not receive policy, device, DNS, user, or general API
  write scopes.
- `VPS_HOST` is the VPS Tailscale address, not its public origin address, and
  `VPS_SSH_KNOWN_HOSTS` binds that private address to the host public key read
  through a previously trusted SSH session.
- Each direct staging or production deployment job joins the tailnet before
  configuring SSH. It uses the full-SHA-pinned `tailscale/github-action`,
  advertises only `tag:c2c-ci`, and waits for `VPS_HOST` with the action's
  `ping` input before SCP or SSH.
- The tailnet policy grants tailnet administrators and `tag:c2c-ci` access to
  `tag:c2c-prod` on `tcp/22` only. Web ports and the public `8443` service are
  not reachable through the CI tag. Tailscale SSH remains disabled; deployment
  continues to use the existing OpenSSH `deploy` key.
- The reusable workflow only publishes the GHCR image. The top-level
  `.github/workflows/ci.yml` owns separate deployment jobs whose environment
  names are the literal values `staging` and `production`; those direct jobs
  read the environment-scoped SSH secrets. Deployment jobs and environment
  secrets must not cross a `workflow_call` boundary.
- The release archive contains `compose.yaml`, `compose.prod.yaml`,
  `backend/migrations`, and the install, deploy, and production-backup scripts.
  The VPS does not run `git pull` or build application source.
- `compose.yaml` retains `build.context` for local development and exposes
  `image: ${BACKEND_IMAGE:-c2cmarket-backend:local}`. A VPS release pulls the
  SHA image and starts it with `--no-build`.
- Production must finish the existing PostgreSQL dump, checksum, and R2 upload
  before migrations. Staging must not invoke the production backup.
- The installer is streamed to the VPS through `bash -s`. Any deployment child
  it launches must receive `/dev/null` as stdin so Docker or Compose cannot
  consume the unparsed remainder of the installer.
- A deployment is successful only when `/health` and `/readyz` pass and
  `/version.gitCommit` equals the full SHA from `BACKEND_IMAGE`.
- The installer changes the current symlink only after migrations, backend
  startup, health, readiness, and runtime commit verification all succeed.

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
| Health, readiness, or runtime commit verification exhausts retries | Print Compose status and exit non-zero |
| `/version.gitCommit` differs from the immutable image tag | Exit non-zero and do not update the current symlink |
| A streamed installer child reads stdin | The child sees EOF; the installer still executes release promotion |
| Current path exists as a regular file/directory | Refuse to overwrite it with a symlink |
| SSH identity or verified known-hosts data is missing | Fail in the runner before SCP |
| Tailscale OAuth credentials are missing or invalid | Fail while joining the tailnet; never attempt SCP or SSH |
| The CI node cannot reach `VPS_HOST` | The action's bounded ping fails before archive upload |
| The OAuth client requests a tag other than `tag:c2c-ci` | Tailnet authorization rejects node creation |
| A CI-tagged node connects to a production port other than `tcp/22` | Tailnet policy denies the connection |
| Deployment job is moved into a reusable workflow | Reject in tests; environment secrets must be read by direct `ci.yml` jobs |

Database migrations are never automatically rolled down. A failed release may
leave its version directory and uploaded archive for diagnosis, but it must not
claim success by changing the current link.

### 5. Good / Base / Bad Cases

- Good: a tested `staging` push builds `<sha>`, deploys `c2c-staging` on 8081,
  joins Tailscale as `tag:c2c-ci`, reaches the VPS private address, passes both
  loopback probes, reports the same `<sha>` from `/version`, and then changes
  `staging-current`.
- Base: a tested `main` push publishes `<sha>`, waits for production approval,
  completes the R2 backup, deploys `c2c-prod` on 8080, and then changes
  `current`.
- Bad: a workflow deploys `:latest`, uses the public VPS address, reuses a
  personal root key, disables SSH host verification, gives the Tailscale OAuth
  client broad scopes, builds on the VPS, migrates before backup, or changes
  the current link before readiness succeeds.

### 6. Tests Required

- Parse both workflow files as YAML and run an Actions-aware linter when one is
  already available in the trusted local toolchain.
- Assert that `release-backend.yml` contains no VPS secrets or environment
  binding, while `ci.yml` contains literal staging and production deployment
  jobs that reference all four environment secrets.
- Run `bash -n` for the installer, deployment, backup, and release tests.
- Run `scripts/test-vps-release.sh` and assert fixed ports, staging backup
  exclusion, production backup-before-migration, `--no-build`, error
  propagation, runtime commit matching, streamed-installer stdin isolation,
  and current-link behavior.
- Run `ruby scripts/check-release-workflow.rb` and assert the reusable
  publisher injects matching binary build arguments and OCI labels from the
  exact workflow inputs. The same checker must assert both direct deployment
  jobs use the pinned Tailscale action before SSH, pass the two OAuth secret
  references, advertise `tag:c2c-ci`, ping `VPS_HOST`, and pin the Tailscale
  client version.
- Run a real staging deployment from a GitHub-hosted runner after changing the
  private network contract. Assert the ephemeral CI node receives
  `tag:c2c-ci`, SCP and SSH succeed against the private address, and the node is
  removed when the job ends.
- Expand production and staging Compose configurations with their real ignored
  env files and `config --quiet`.
- Build the local backend image to prove the default `build` path still works.
- Run `go test ./...`, frontend typecheck/build/tests, OpenAPI route checks,
  migration documentation checks, and `git diff --check` before handoff.

### 7. Wrong vs Correct

#### Wrong: read environment secrets inside a reusable workflow

```yaml
on: workflow_call

jobs:
  deploy-staging:
    environment:
      name: staging
    env:
      VPS_HOST: ${{ secrets.VPS_HOST }}
```

GitHub may create a deployment record for `staging` while resolving these
environment secrets to empty values. A literal environment name inside the
called workflow does not repair the caller/callee secret boundary.

#### Correct: publish through reuse and deploy from the top-level workflow

```yaml
publish-staging:
  uses: ./.github/workflows/release-backend.yml
  with:
    release_tag: staging
    git_sha: ${{ github.sha }}

deploy-staging:
  needs: publish-staging
  environment:
    name: staging
  env:
    VPS_HOST: ${{ secrets.VPS_HOST }}
```

The top-level staging and production jobs may share their step sequence through
a YAML anchor, but their environment names, secret references, conditions, and
concurrency groups remain explicit and independently testable.

#### Wrong: deploy to the public origin before joining the tailnet

```yaml
env:
  VPS_HOST: 192.236.230.132
steps:
  - run: scp release.tar.gz "${VPS_USER}@${VPS_HOST}:/tmp/"
```

This depends on a public SSH rule and cannot restrict GitHub-hosted runners by
source address because their egress addresses change.

#### Correct: join with the CI tag before private SSH

```yaml
- name: Connect deployment runner to Tailscale
  uses: tailscale/github-action@780049a30b6ff5c378a9e7b389d15ece7a204888 # v4.1.3
  with:
    oauth-client-id: ${{ secrets.TS_OAUTH_CLIENT_ID }}
    oauth-secret: ${{ secrets.TS_OAUTH_SECRET }}
    tags: tag:c2c-ci
    ping: ${{ secrets.VPS_HOST }}
    version: 1.98.10
```

The environment-scoped `VPS_HOST` then resolves to the trusted Tailscale
address, while the existing `deploy` key and host verification remain in use.

#### Wrong: rebuild or claim success before readiness

```bash
docker compose up -d --build backend
ln -sfn /opt/c2cmarket/releases/production/new /opt/c2cmarket/current
```

This rebuilds unverified source on the VPS and changes the success pointer
without proving backup, migration, health, or readiness.

#### Correct: deploy the immutable image before changing the current link

```bash
scripts/deploy-vps-backend.sh \
  production \
  ghcr.io/xiangrikuil/c2cmarket-backend:<40-character-git-sha>
ln -sfn \
  /opt/c2cmarket/releases/production/<40-character-git-sha> \
  /opt/c2cmarket/current
```

The deployment script exits zero only after `/version.gitCommit` matches the
image SHA. The second command is permitted only after that zero exit. The
normal GitHub workflow enforces this order through
`scripts/install-vps-release.sh`, which launches the deployment child with
stdin redirected from `/dev/null`.
