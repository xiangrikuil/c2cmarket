# C2CMarket Deployment Runbook

## Scope

This runbook covers the current deployable shape of C2CMarket: Go backend, PostgreSQL, SQL migrations, Nuxt 4/Nitro Cloudflare Worker frontend, OAuth login, Alibaba Cloud DirectMail email verification, health/readiness checks, and local smoke validation.

C2CMarket does not deploy payment, escrow, guarantee, credential custody, API proxying, external push, SMS, webhook, or automatic credential delivery services. Production email is limited to profile email verification through Alibaba Cloud DirectMail.

## Required Inputs

Create a production env file from the template:

```bash
cp .env.production.example .env.production
```

Replace every `CHANGE_ME` value before production use:

- `POSTGRES_PASSWORD`
- `OAUTH_CLIENT_ID`
- `OAUTH_CLIENT_SECRET`
- `OAUTH_AUTHORIZE_URL`
- `OAUTH_TOKEN_URL`
- `OAUTH_USERINFO_URL`
- `OAUTH_REDIRECT_URL`
- `FRONTEND_ORIGIN`
- `ALLOWED_ORIGINS`
- `TRUSTED_PROXIES` after observing the backend container's immediate Caddy/Docker bridge peer
- `CONTACT_ENCRYPTION_KEY`
- `CONTACT_FINGERPRINT_KEY`
- `CONTACT_KEY_VERSION`
- `CONTACT_ENCRYPTION_KEYRING` and `CONTACT_FINGERPRINT_KEYRING`; each must
  contain `CONTACT_KEY_VERSION`, and old referenced versions must remain
- `EMAIL_VERIFICATION_PEPPER` with at least 32 bytes and a value distinct from the contact keys
- `METRICS_BEARER_TOKEN` with at least 32 bytes and a value distinct from all
  encryption, fingerprint, OAuth, SMTP, and verification secrets
- `DB_MAX_CONNS`, `DB_MIN_CONNS`, connection lifetime/idle/health periods, and
  statement/lock/idle-in-transaction timeouts sized to the PostgreSQL budget
- `C2C_BOOTSTRAP_ADMIN_USERNAME` and `C2C_BOOTSTRAP_ADMIN_PASSWORD` for an empty-database first deploy only
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `MAIL_FROM_ADDRESS`
- `NUXT_PUBLIC_API_MODE`
- `NUXT_PUBLIC_SITE_URL`
- `NUXT_PUBLIC_API_BASE_URL`
- `NUXT_API_BASE_URL`
- Optional Umami tracker fields: `NUXT_PUBLIC_UMAMI_ENABLED`, `NUXT_PUBLIC_UMAMI_SCRIPT_URL`, `NUXT_PUBLIC_UMAMI_WEBSITE_ID`, `NUXT_PUBLIC_UMAMI_DOMAINS`, `NUXT_PUBLIC_UMAMI_HOST_URL`

Production must keep:

```text
APP_ENV=production
ENABLE_DEV_AUTH=false
OAUTH_PROVIDER_MODE=oauth2
EMAIL_PROVIDER=aliyun_directmail
EMAIL_VERIFICATION_PEPPER=<distinct 32-byte minimum secret>
NUXT_PUBLIC_API_MODE=real
```

`OAUTH_PROVIDER_MODE=fake` is only for local automated smoke. `/api/v1/auth/dev-session` is only for development/test.
`EMAIL_PROVIDER=development` is only for local development/test. It exposes `devCode` for automation and must not be used in production.

Administrator Bootstrap is create-only:

- For an empty database with no administrator, set both Bootstrap variables for the first backend start. After the administrator is created, clear both variables and restart the backend.
- For an existing database that already has an administrator, clear both variables before starting a release containing migration 62. Existing administrators predate the `initial-admin-v1` marker and are intentionally not claimed or modified automatically.
- Never leave only `C2C_BOOTSTRAP_ADMIN_USERNAME` configured; startup rejects a username without a password.
- Do not create the marker manually or use Bootstrap to promote an existing normal or OAuth user.

`FRONTEND_ORIGIN` is the primary browser origin for cookie-authenticated requests
and OAuth callback redirects. Production requires it to be an absolute HTTPS
origin and automatically adds it to the CORS allowlist. `ALLOWED_ORIGINS` can add
other explicit origins. CORS must never use `*` with session cookies.

Model audit outbound access is fail closed:

- Targets must use absolute public HTTPS base URLs without credentials, query
  strings, or fragments.
- The backend validates every DNS answer when a target is saved and resolves it
  again before each new connection. Any private, loopback, link-local, metadata,
  special-use, or mixed public/private result rejects the target.
- `MODEL_AUDIT_ALLOWED_HOSTS` is an optional comma-separated list of exact DNS
  hosts or IP literals. Wildcards and ports are invalid. An empty value permits
  any host that passes the public-address policy.
- Configure an explicit list when production audits use a known provider set.
  Existing saved targets are not rewritten; unsafe targets fail when next used.

Client IP forwarding is also fail closed:

- The Compose backend port is published only on host loopback. Do not add a
  second public backend mapping for health checks or Tunnel traffic.
- Production and staging use `TRUST_X_FORWARDED_FOR=true` only after
  `TRUSTED_PROXIES` contains the smallest exact IP or CIDR for the backend
  container's immediate peer.
- A host-managed `cloudflared` process can appear inside the backend container
  as a Docker bridge gateway. Cloudflare public edge ranges are not the
  immediate peer and must not be copied into `TRUSTED_PROXIES`.
- The backend prefers a valid `CF-Connecting-IP`, otherwise strips trusted XFF
  hops from right to left, then falls back to `X-Real-IP` and the direct peer.
  Untrusted peers cannot influence the resolved address.

For the first observation, temporarily override proxy-header trust, send one
request through the public Tunnel with a recognizable request ID, and inspect
the normalized direct peer:

```bash
TRUST_X_FORWARDED_FOR=false TRUSTED_PROXIES= \
  docker compose -p c2c-prod --env-file .env.production \
  -f compose.yaml -f compose.prod.yaml --profile app up --no-build -d backend
curl -fsS -H 'X-Request-Id: ingress-peer-observation-prod' \
  https://api.c2cmarket.shop/health
docker compose -p c2c-prod --env-file .env.production \
  -f compose.yaml -f compose.prod.yaml logs --since 5m backend \
  | grep 'request_id=ingress-peer-observation-prod'
```

Set `TRUSTED_PROXIES` in `.env.production` to the resulting `client_ip` as an
exact address (`/32` for IPv4 or `/128` for IPv6 is also valid), restore
`TRUST_X_FORWARDED_FOR=true`, and recreate the backend. Repeat independently
for staging; do not assume both Compose networks use the same gateway.

DirectMail settings:

```text
SMTP_HOST=smtpdm.aliyun.com
SMTP_PORT=465
SMTP_USERNAME=<verified DirectMail SMTP account>
SMTP_PASSWORD=<DirectMail SMTP password>
MAIL_FROM_ADDRESS=<verified sender address>
MAIL_FROM_NAME=C2CMarket
```

If the Aliyun DirectMail SMTP account or sender address is not ready yet, keep the `CHANGE_ME` placeholders in the template but do not start production; the backend intentionally fails fast when SMTP credentials are missing.

Optional Umami analytics:

```text
NUXT_PUBLIC_UMAMI_ENABLED=true
NUXT_PUBLIC_UMAMI_SCRIPT_URL=https://<umami-origin>/script.js
NUXT_PUBLIC_UMAMI_WEBSITE_ID=<website-id>
NUXT_PUBLIC_UMAMI_DOMAINS=<frontend-domain>
NUXT_PUBLIC_UMAMI_HOST_URL=https://<umami-origin>
```

Only public tracker configuration belongs in `NUXT_PUBLIC_*`. Do not expose Umami API keys,
admin credentials, share URLs, report URLs, or dashboard-only URLs to the frontend.
The frontend custom events intentionally send low-cardinality product, price bucket,
seat bucket, result bucket, entity type, and reason-code fields only. They must not
include raw search terms, URL query strings, user IDs, contact values, report text,
linux.do identifiers, payment instructions, API keys, tokens, sessions, cookies, or
panel credentials.

## Cloudflare Workers and VPS Backends

The current release topology serves production and staging frontends from
Nuxt/Nitro Cloudflare Workers, with static assets bound to the same Workers,
and runs two isolated backend stacks on the RackNerd VPS. Cloudflare proxied A
records reach Caddy with Full (strict) TLS;
Caddy routes the API hostnames to loopback-only ports 8080 and 8081. Follow
[`cloudflare-workers-vps-backends.md`](./cloudflare-workers-vps-backends.md) for
the authoritative hostnames, Compose project names, Caddy/automatic-TLS contract,
Access policy, OAuth callbacks, and systemd R2 backup procedure.

The VPS owns only the two API origins. The production and staging Workers keep
`c2cmarket.shop` and `staging.c2cmarket.shop`.

## Build Release Artifacts

Release inputs must resolve to a commit and the worktree must have no staged,
unstaged, or untracked non-ignored files:

```bash
RELEASE_REF=v0.1.0
RELEASE_VERSION=0.1.0
RELEASE_IMAGE=c2cmarket-backend:0.1.0
scripts/package-source.sh "${RELEASE_REF}"
scripts/build-backend-image.sh \
  "${RELEASE_REF}" "${RELEASE_VERSION}" "${RELEASE_IMAGE}"
docker image inspect \
  --format '{{json .Config.Labels}}' "${RELEASE_IMAGE}"
```

The source command writes a deterministic archive plus `.sha256` under
`output/`. The image command builds only from the resolved commit archive and
sets OCI version, revision, and created labels. Set `BACKEND_IMAGE` in the
target environment file to exactly `${RELEASE_IMAGE}` before running Compose.

## Local or Manual First-Deploy Validation

The commands in this section validate immutable release artifacts before GHCR
automation is enabled. The normal VPS path does not keep a Git checkout or
build source; use the branch-driven release under **Regular Release**.

Validate Compose configuration:

```bash
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml config
```

Start PostgreSQL:

```bash
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml up -d postgres
```

Run migrations:

```bash
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile migrate run --rm migrate
```

Before starting the backend, apply the Bootstrap rule above. A new empty
installation may keep the one-time values for its first start. An upgraded
installation with an existing administrator must have both values empty.

Start the prebuilt backend image:

```bash
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app up --no-build -d backend
```

Check process health and database/migration readiness:

```bash
curl -fsS http://127.0.0.1:${BACKEND_PORT:-8080}/health
curl -fsS http://127.0.0.1:${BACKEND_PORT:-8080}/readyz
curl -fsS http://127.0.0.1:${BACKEND_PORT:-8080}/version
```

`/readyz` must report PostgreSQL readiness and `schemaDirty=false`.
The expected schema version in the current backend is `89`.
`/version` must report the release version, full resolved Git commit, commit
time, and `expectedMigrationVersion=89`; the first three values must match the
image labels inspected above.

After a successful empty-database Bootstrap, clear both Bootstrap variables,
recreate the backend container, and verify `/readyz` again. Keeping the secret
configured is unnecessary even though a proven marker rerun is idempotent.

## Backend Hardening Checks

The backend process uses explicit HTTP server timeouts:

```text
ReadHeaderTimeout = 5s
ReadTimeout       = 15s
WriteTimeout      = 30s
IdleTimeout       = 60s
```

In `APP_ENV=production`, session and OAuth state cookies must include
`Secure=true`, `HttpOnly=true`, and `SameSite=Lax`. Logout and OAuth state clear
cookies must use the same Path/Secure/SameSite shape so browsers can remove
them.

The backend sets:

```text
Content-Security-Policy: default-src 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'none'
Permissions-Policy: camera=(), geolocation=(), microphone=(), payment=(), usb=()
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Strict-Transport-Security: max-age=31536000; includeSubDomains   # production only
```

The API policy denies browser content loading. The Cloudflare Workers frontend
applies the policy from `frontend/public/_headers`, including explicit
production/staging API `connect-src` values. `style-src 'unsafe-inline'` is
currently limited to Vue runtime style bindings; scripts do not allow
`unsafe-eval` or wildcard/HTTP sources. Run
`node scripts/check-security-headers.mjs` for every policy change.

OAuth token exchange and userinfo requests use a dedicated 10-second HTTP client
timeout and a 1 MiB response-body read limit. Do not log OAuth client secrets,
provider tokens, raw userinfo, session cookies, or CSRF tokens when debugging
login failures.

The current rate limiter is bounded, in-process, and windowed. It protects OAuth, search,
API purchase intent creation, direct contact reads, report/appeal creation, and
development-only contact/session entries. Exceeded requests return
`application/problem+json` with `code=RATE_LIMITED`, HTTP `429`, and an integer
`Retry-After`. Size each instance consistently and monitor active/max keys.

Main list endpoints support `limit` and opaque `cursor`; default page size is
20 and max page size is 100. Clients should persist and pass `nextCursor` without
parsing it.

Idempotency processing, failed, and completed rows expire after fifteen
minutes, one hour, and seven days. A key can be reused after its row expires.
Responses larger than 64 KiB are not cached; resource-backed responses are
rebuilt, while a generic result that cannot be rebuilt returns
`IDEMPOTENCY_RESULT_NOT_REPLAYABLE` without repeating the mutation.

The PostgreSQL lifecycle runner executes immediately and then every
`MAINTENANCE_INTERVAL` (default `15m`). Each data type processes at most
`MAINTENANCE_BATCH_SIZE` rows per run under one advisory transaction lock.
Defaults retain terminal sessions for 7 days, consumed/expired verification
challenges for 24 hours, read notifications for 90 days, and unread
notifications plus unreferenced domain events for 365 days. Ended contact
windows are marked `expired`; contact ciphertext/history, contact access logs,
administrator audits, and dispute audits are not deleted.

`GET /metrics` exposes Prometheus/OpenMetrics data for HTTP route/status/latency,
PostgreSQL pool/readiness/migrations/slow queries, limiter, contact decrypt,
safe outbound rejection, maintenance, SSE/realtime, OAuth/email failures, and
idempotency conflicts. Production requires `Authorization: Bearer
<METRICS_BEARER_TOKEN>`. Keep the route outside public ingress and add a
network or authenticated monitoring-proxy restriction. `/health` remains
public liveness; treat `/readyz`, `/version`, and `/metrics` as operator routes.
See [`../operations.md`](../operations.md) for alert guidance.

API purchase intent direct-contact disclosure writes
`api_purchase_intent_contact_access_logs` rows with intent ID, viewer user ID,
viewed side, request ID, and timestamp. The log table does not store plaintext
contact values. Responses that include full contact values must keep
`Cache-Control: no-store`.

## Frontend Build

Build the frontend in real-backend mode:

```bash
NUXT_PUBLIC_API_MODE=real \
NUXT_PUBLIC_SITE_URL=https://c2cmarket.shop \
NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
NUXT_API_BASE_URL=https://api.c2cmarket.shop \
pnpm --dir frontend build
```

The build uses Nitro's `cloudflare_module` preset and must produce
`frontend/.output/server/index.mjs` and
`frontend/.output/public`. Validate both Worker configurations before publishing:

```bash
pnpm --dir frontend exec wrangler deploy --dry-run --config ../wrangler.jsonc
pnpm --dir frontend exec wrangler deploy --dry-run --config ../wrangler.staging.jsonc
```

Publish production or staging from the repository root with the matching
Wrangler config. Both configs run the Nitro server entry and bind `.output/public`
as Worker assets; there is no `dist/` deployment or `index.html` SPA fallback:

```bash
pnpm --dir frontend exec wrangler deploy --config ../wrangler.jsonc
pnpm --dir frontend exec wrangler deploy --config ../wrangler.staging.jsonc
```

After publishing, verify public SSR HTML, a private route's `X-Robots-Tag`, a
missing public detail's HTTP 404, `/sitemap.xml`, and `/robots.txt` on the target
hostname. Production robots allow public market crawling; staging and
`workers.dev` must return `Disallow: /`.
Production builds require `NUXT_PUBLIC_API_MODE=real` and must not ship a
mock/demo fallback.

When Umami is enabled, verify the browser loads the configured tracker script and
that custom events appear in Umami Events. Network checks should show requests to
the Umami collector after route views or success actions, but the request payload
must not contain raw search text, contact details, report descriptions, or IDs.

## Source Package

Create a clean source archive for a fixed release ref:

```bash
scripts/package-source.sh v0.1.0
```

The script rejects a dirty worktree, resolves the ref to a commit, writes a
deterministic archive and `.sha256` to `output/`, and verifies that the archive
excludes `.git/`, `.codex/`, Trellis task/workspace history, `output/`, `tmp/`,
dependency/build/cache directories, and all environment files except the three
root examples.

## Smoke Validation

For local development/test environments with fake OAuth and dev auth enabled:

```bash
API_BASE_URL=http://127.0.0.1:8080 \
node scripts/run-smokes.mjs
```

The runner is intentionally serial and stops on first failure. It covers auth, official price, API market, carpool, profile, announcements, favorites, reviews, reports, notifications, and search.

For real production OAuth, do not use fake OAuth smoke identities. Use health/readiness checks plus a controlled login with the real provider and run only smoke scripts that are safe for the target environment and seeded data policy.

## Regular Release

The normal backend release is branch-driven:

1. Open a feature PR into `staging`; the `ci` workflow runs backend, race,
   PostgreSQL, contract, migration-documentation, release-script, frontend
   type/build/test, secret/filesystem scan, image scan, SBOM, and exact-SHA
   release gates.
2. Merging `staging` publishes the exact tested commit to
   `ghcr.io/xiangrikuil/c2cmarket-backend:<git-sha>` and deploys it to
   `c2c-staging` / port 8081 through the GitHub `staging` environment.
3. Validate staging OAuth, email, CORS, health/readiness, and safe core flows.
4. Open the `staging` to `main` PR. After merge and CI, the immutable production
   image is published; the GitHub `production` environment waits for reviewer
   approval.
5. Production deployment uploads a PostgreSQL backup to R2 before migrations,
   then pulls the GHCR image, applies migrations, starts the backend with
   `--no-build`, and verifies `/health` and `/readyz` before changing
   `/opt/c2cmarket/current`.
6. Cloudflare Workers Builds independently publishes the frontend for its
   configured `staging` or `main` branch.

Record the source checksum, image digest/OCI labels, Trivy results, and SBOM
for the release SHA. After deployment, verify `/health`, `/readyz`, `/version`,
authenticated `/metrics`, origin allowlists, response headers, and cookie
behavior from the deployed frontend origin.

The authoritative first-time GitHub/VPS setup, secret names, release directory
layout, and manual recovery commands are in
[`cloudflare-workers-vps-backends.md`](./cloudflare-workers-vps-backends.md).

## Rollback

To roll back only the application version, select the previous successful
40-character Git SHA and run the deployment script from its release directory:

```bash
OLD_SHA=<40-character-git-sha>
/opt/c2cmarket/releases/production/${OLD_SHA}/scripts/deploy-vps-backend.sh \
  production \
  ghcr.io/xiangrikuil/c2cmarket-backend:${OLD_SHA}
ln -sfn /opt/c2cmarket/releases/production/${OLD_SHA} /opt/c2cmarket/current
```

If migrations have already run, inspect `backend/migrations/*.down.sql` before rollback. Do not run destructive down migrations against production data without a database backup and explicit operator approval.

Migration 63 invalidates legacy bind-email challenges and may discard cached
idempotency response bodies larger than 64 KiB. Its down migration cannot
restore those values, so clients must request a new verification code after
rollback and operators must not treat the down migration as data restoration.

Migration 64 records the ciphertext format required to distinguish legacy
no-AAD rows from `aad_v1`. After any contact re-encryption apply batch, do not
drop this metadata or old keyring entries during rollback. Prefer running the
previous application against the forward schema when compatible; do not
automatically execute down migrations.

Migration 65 removes the prelaunch demand schema. Its down migration recreates
empty tables and indexes only; it cannot restore deleted demand rows.

## Troubleshooting

- Backend does not start: check `APP_ENV`, OAuth env keys, contact crypto env keys, DirectMail env keys, and `DATABASE_URL`.
- Email verification startup/config errors: check `EMAIL_PROVIDER=aliyun_directmail`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, and `MAIL_FROM_ADDRESS`.
- Production backend rejects startup: check `FRONTEND_ORIGIN` / `ALLOWED_ORIGINS`.
- `/readyz` fails: check PostgreSQL container health, `schema_migrations`, and migration dirty state.
- Login fails before redirect: check `OAUTH_REDIRECT_URL` matches the provider app configuration and public backend URL.
- Browser requests fail with `CSRF_TOKEN_INVALID` before handler logic: check request `Origin` is in `ALLOWED_ORIGINS`.
- Contact detail response is cached by an intermediary: verify `Cache-Control: no-store` reaches the browser for carpool contact reads and API purchase intent buyer/owner detail reads.
- Mutations fail with `CSRF_TOKEN_INVALID`: refresh `/api/v1/auth/session` and verify the frontend sends `X-CSRF-Token`.
- Admin routes return `PERMISSION_DENIED`: verify `user_permissions.permission='admin'` for the logged-in user.

## Contact Retention Notes

Deleting a contact method retires that user-facing method, but historical
carpool sessions and API purchase intents keep frozen encrypted contact method
version references for dispute/audit review. Authorized reads can still decrypt
those frozen versions within the business rules and must be served with
`Cache-Control: no-store`; API purchase intent contact reads also write
non-plaintext access logs.

This release does not physically destroy historical ciphertext or implement key
destruction. A future retention task should add explicit `destroyed_at` fields,
operator approval, and a key-rotation/destruction runbook before deleting
historical encrypted values.
