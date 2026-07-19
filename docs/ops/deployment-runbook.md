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
- `CONTACT_ENCRYPTION_KEY`
- `CONTACT_FINGERPRINT_KEY`
- `CONTACT_KEY_VERSION`
- `SMTP_USERNAME`
- `SMTP_PASSWORD`
- `MAIL_FROM_ADDRESS`
- `VITE_API_BASE_URL`
- `NUXT_PUBLIC_SITE_URL`
- `NUXT_PUBLIC_API_BASE_URL`
- `NUXT_API_BASE_URL`
- Optional Umami tracker fields: `VITE_UMAMI_ENABLED`, `VITE_UMAMI_SCRIPT_URL`, `VITE_UMAMI_WEBSITE_ID`, `VITE_UMAMI_DOMAINS`, `VITE_UMAMI_HOST_URL`

Production must keep:

```text
APP_ENV=production
ENABLE_DEV_AUTH=false
OAUTH_PROVIDER_MODE=oauth2
EMAIL_PROVIDER=aliyun_directmail
VITE_API_MODE=real
VITE_ENABLE_MOCK=false
```

`OAUTH_PROVIDER_MODE=fake` is only for local automated smoke. `/api/v1/auth/dev-session` is only for development/test.
`EMAIL_PROVIDER=development` is only for local development/test. It exposes `devCode` for automation and must not be used in production.

`FRONTEND_ORIGIN` is the primary browser origin for cookie-authenticated requests
and OAuth callback redirects. Production requires it to be an absolute HTTPS
origin and automatically adds it to the CORS allowlist. `ALLOWED_ORIGINS` can add
other explicit origins. CORS must never use `*` with session cookies.

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
VITE_UMAMI_ENABLED=true
VITE_UMAMI_SCRIPT_URL=https://<umami-origin>/script.js
VITE_UMAMI_WEBSITE_ID=<website-id>
VITE_UMAMI_DOMAINS=<frontend-domain>
VITE_UMAMI_HOST_URL=https://<umami-origin>
```

Only public tracker configuration belongs in `VITE_*`. Do not expose Umami API keys,
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

## Local or Manual First-Deploy Validation

The commands in this section validate a source checkout before GHCR automation
is enabled. The normal VPS path does not keep a Git checkout or build source;
use the branch-driven release under **Regular Release** for the VPS.

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

Build and start the backend:

```bash
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app build backend
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app up -d backend
```

Check process health and database/migration readiness:

```bash
curl -fsS http://127.0.0.1:${BACKEND_PORT:-8080}/health
curl -fsS http://127.0.0.1:${BACKEND_PORT:-8080}/readyz
```

`/readyz` must report PostgreSQL readiness and `schemaDirty=false`.
The expected schema version in the current backend is `52`.

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
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Strict-Transport-Security: max-age=31536000; includeSubDomains   # production only
```

CSP is not generated by the Go API because the backend does not serve the
Nuxt frontend. Configure frontend response policy at the Cloudflare Worker
boundary according to the actual asset and analytics origins.

OAuth token exchange and userinfo requests use a dedicated 10-second HTTP client
timeout and a 1 MiB response-body read limit. Do not log OAuth client secrets,
provider tokens, raw userinfo, session cookies, or CSRF tokens when debugging
login failures.

The current rate limiter is in-process and windowed. It protects OAuth, search,
API purchase intent creation, direct contact reads, report/appeal creation, and
development-only contact/session entries. Exceeded requests return
`application/problem+json` with `code=RATE_LIMITED` and HTTP `429`.

Main list endpoints support `limit` and opaque `cursor`; default page size is
20 and max page size is 100. Clients should persist and pass `nextCursor` without
parsing it.

Idempotency processing rows can be retried when the row has expired and the
request hash matches. Same-key different-body requests still return
`IDEMPOTENCY_KEY_REUSED`; non-expired processing rows still return
`IDEMPOTENCY_IN_PROGRESS`. Startup performs a conservative cleanup of stale
processing rows older than the configured expiry window.

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
Production builds intentionally fail when `VITE_ENABLE_MOCK=true`, and must not
ship a mock/demo fallback.

When Umami is enabled, verify the browser loads the configured tracker script and
that custom events appear in Umami Events. Network checks should show requests to
the Umami collector after route views or success actions, but the request payload
must not contain raw search text, contact details, report descriptions, or IDs.

## Source Package

Create a clean source archive for release handoff:

```bash
scripts/package-source.sh
```

The script writes to `output/` and verifies that the archive excludes `.git/`,
`output/`, `tmp/`, `.DS_Store`, `__MACOSX/`, `node_modules/`, `dist/`, `build/`,
and `coverage/`.

## Smoke Validation

For local development/test environments with fake OAuth and dev auth enabled:

```bash
API_BASE_URL=http://127.0.0.1:8080 \
node scripts/run-smokes.mjs
```

The runner is intentionally serial and stops on first failure. It covers auth, official price, API market, carpool, profile, announcements, demands, favorites, reviews, reports, notifications, and search.

For real production OAuth, do not use fake OAuth smoke identities. Use health/readiness checks plus a controlled login with the real provider and run only smoke scripts that are safe for the target environment and seeded data policy.

## Regular Release

The normal backend release is branch-driven:

1. Open a feature PR into `staging`; the `ci` workflow runs backend, contract,
   migration-documentation, release-script, frontend type/build, and frontend
   test gates.
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
