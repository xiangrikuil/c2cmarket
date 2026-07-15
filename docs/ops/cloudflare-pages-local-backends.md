# Cloudflare Pages with Local Production and Staging Backends

Date: 2026-07-15
Operator: Codex

## Target topology

| Environment | Frontend | Backend | Local port | Compose project | Database |
| --- | --- | --- | --- | --- | --- |
| Production | `https://c2cmarket.shop` | `https://api.c2cmarket.shop` | `127.0.0.1:8080` | `c2c-prod` | isolated production volume |
| Staging | `https://staging.c2cmarket.shop` | `https://api-staging.c2cmarket.shop` | `127.0.0.1:8081` | `c2c-staging` | isolated, initially empty volume |

Cloudflare Pages serves both frontends. One locally managed Cloudflare Tunnel exposes only the two API hostnames. Do not route `c2cmarket.shop`, `www.c2cmarket.shop`, or `staging.c2cmarket.shop` through the Tunnel.

## 1. Prepare local environment files

```bash
cp .env.production.example .env.production
cp .env.staging.example .env.staging
```

Replace every `CHANGE_ME` value. The two environments must use different database passwords, contact-encryption keys, contact-fingerprint keys, key versions, bootstrap passwords, and linux.do OAuth clients. They may use the same verified Alibaba Cloud DirectMail SMTP account. Keep:

```text
# .env.production
FRONTEND_ORIGIN=https://c2cmarket.shop
ALLOWED_ORIGINS=https://c2cmarket.shop
OAUTH_REDIRECT_URL=https://api.c2cmarket.shop/api/v1/auth/oauth/callback
BACKEND_PORT=8080

# .env.staging
FRONTEND_ORIGIN=https://staging.c2cmarket.shop
ALLOWED_ORIGINS=https://staging.c2cmarket.shop
OAUTH_REDIRECT_URL=https://api-staging.c2cmarket.shop/api/v1/auth/oauth/callback
BACKEND_PORT=8081
MAIL_FROM_NAME=C2CMarket Staging
```

Both backends deliberately run with `APP_ENV=production`, real OAuth, DirectMail, secure cookies, and development authentication disabled. `.env.production` and `.env.staging` are ignored by Git.

## 2. Start the two isolated backend stacks

Production:

```bash
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml up -d postgres
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile migrate run --rm migrate
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app up -d --build backend
```

Staging:

```bash
docker compose -p c2c-staging --env-file .env.staging -f compose.yaml -f compose.prod.yaml up -d postgres
docker compose -p c2c-staging --env-file .env.staging -f compose.yaml -f compose.prod.yaml --profile migrate run --rm migrate
docker compose -p c2c-staging --env-file .env.staging -f compose.yaml -f compose.prod.yaml --profile app up -d --build backend
```

The Compose project name scopes container names, networks, and the `c2c_postgres_data` volume. Never run either stack without its `-p` value. The staging stack starts from a fresh database unless its `c2c-staging_c2c_postgres_data` volume already exists.

Verify locally:

```bash
curl -fsS http://127.0.0.1:8080/health
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8081/health
curl -fsS http://127.0.0.1:8081/readyz
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml ps
docker compose -p c2c-staging --env-file .env.staging -f compose.yaml -f compose.prod.yaml ps
```

## 3. Configure the Cloudflare Tunnel

Create or reuse one locally managed Tunnel, then create DNS routes for API hostnames only:

```bash
cloudflared tunnel login
cloudflared tunnel create c2cmarket-local
cloudflared tunnel route dns c2cmarket-local api.c2cmarket.shop
cloudflared tunnel route dns c2cmarket-local api-staging.c2cmarket.shop
```

If the Tunnel currently owns `c2cmarket.shop` or `www.c2cmarket.shop`, delete those public-hostname routes and their Tunnel CNAME records before attaching the root domain to Pages. Otherwise Cloudflare can return error `1033` even when the Pages deployment is healthy.

Install the repository example as the local Tunnel configuration:

```bash
mkdir -p ~/.cloudflared
cp deploy/cloudflared/config.yml.example ~/.cloudflared/config.yml
```

Replace the Tunnel UUID, macOS username, and credentials-file path, then validate and run. The example pins the edge transport to `http2` because this Mac's local network has shown QUIC connections timing out after a network transition while TCP port 7844 remains available:

```bash
cloudflared tunnel ingress validate
cloudflared tunnel --protocol http2 run c2cmarket-local
```

For login-time operation on a Mac that remains powered on, install the repository LaunchAgent after the configuration works interactively. Do not also enable `brew services cloudflared`: Homebrew's default service starts the binary without `tunnel run` and exits with status 1 for this named-tunnel setup.

```bash
mkdir -p ~/Library/LaunchAgents ~/Library/Logs
sed "s|/Users/CHANGE_ME|$HOME|g" deploy/launchd/com.cloudflare.cloudflared.plist.example > ~/Library/LaunchAgents/com.cloudflare.cloudflared.plist
launchctl bootout gui/$(id -u)/com.cloudflare.cloudflared 2>/dev/null || true
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.cloudflare.cloudflared.plist
launchctl kickstart -k gui/$(id -u)/com.cloudflare.cloudflared
launchctl print gui/$(id -u)/com.cloudflare.cloudflared
```

Cloudflare documents `auto`, `quic`, and `http2` as supported edge transports. When QUIC/UDP is unstable but TCP port 7844 works, forcing `http2` is the supported recovery path: <https://developers.cloudflare.com/tunnel/advanced/run-parameters/#protocol>.

Confirm public routing:

```bash
curl -fsS https://api.c2cmarket.shop/health
curl -fsS https://api.c2cmarket.shop/readyz
curl -fsS https://api-staging.c2cmarket.shop/health
curl -fsS https://api-staging.c2cmarket.shop/readyz
```

## 4. Configure Cloudflare Pages

Use `main` as the production branch and a long-lived `staging` branch for staging. The Pages build settings are:

```text
Root directory: /
Build command: npx --yes pnpm@10.11.1 --dir frontend install --frozen-lockfile && npx --yes pnpm@10.11.1 --dir frontend build
Build output directory: frontend/dist
```

Set branch-scoped frontend variables:

| Scope | `VITE_API_MODE` | `VITE_API_BASE_URL` |
| --- | --- | --- |
| Production (`main`) | `real` | `https://api.c2cmarket.shop` |
| Preview (`staging`) | `real` | `https://api-staging.c2cmarket.shop` |

Attach `c2cmarket.shop` to the production Pages deployment. Enable preview deployments for `staging`, add `staging.c2cmarket.shop` as a custom domain, and point its proxied CNAME to `staging.<pages-project>.pages.dev` as instructed by Pages. Keep `www` either redirected to the root Pages domain or remove it if unused.

## 5. Configure linux.do OAuth

Create two linux.do OAuth applications so staging activity cannot change the production client configuration:

```text
Production callback:
https://api.c2cmarket.shop/api/v1/auth/oauth/callback

Staging callback:
https://api-staging.c2cmarket.shop/api/v1/auth/oauth/callback
```

Store each client ID and client secret only in its local ignored env file. After the API exchanges the code and creates the API-domain session cookie, it redirects to the matching `FRONTEND_ORIGIN`. The `returnTo` value remains a relative frontend path and cannot switch the redirect to another host.

## 6. Protect staging with Cloudflare Access

Create two self-hosted Access applications:

1. `staging.c2cmarket.shop/*`
2. `api-staging.c2cmarket.shop/*`

Give both applications an Allow policy restricted to the approved email address. On the staging API application, enable **Bypass OPTIONS requests to origin** so browser CORS preflight reaches the Go backend; the backend still accepts only `https://staging.c2cmarket.shop` through its exact `ALLOWED_ORIGINS` list.

Access authorization cookies are scoped per hostname. Before testing the SPA, open `https://staging.c2cmarket.shop` and `https://api-staging.c2cmarket.shop/health` in the same browser and complete Access authentication on both. The frontend already sends credentialed API requests, so subsequent API calls can include both the Access cookie and backend session cookie.

Do not put Cloudflare Access in front of the production frontend or production API unless that becomes an explicit product requirement.

## 7. Configure production backups to R2

Create a private R2 bucket such as `c2cmarket-backups`, create a narrowly scoped R2 API token for that bucket, and configure an `rclone` S3 remote named `c2cmarket-r2`. Keep its credentials in the user-level rclone config, not in this repository.

In the R2 dashboard, add a lifecycle rule that expires objects under `postgres/production/` after 30 days. Test one backup manually:

```bash
scripts/backup-production-postgres.sh
rclone lsf c2cmarket-r2:c2cmarket-backups/postgres/production/
```

The script writes a PostgreSQL custom-format dump and SHA-256 file locally, then uploads both. If upload fails, the local files remain. Local files older than seven days are deleted only after a successful upload; R2 retention is controlled by the 30-day lifecycle rule.

At least monthly, download one R2 dump, verify `pg_restore --list` can read it,
and restore it into a temporary PostgreSQL database that is not connected to
either running stack. Record the object name, restore duration, and row-count
spot checks before deleting the temporary database.

Install the daily 03:30 macOS job:

```bash
cp deploy/launchd/com.c2cmarket.postgres-backup.plist.example ~/Library/LaunchAgents/com.c2cmarket.postgres-backup.plist
plutil -lint ~/Library/LaunchAgents/com.c2cmarket.postgres-backup.plist
launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/com.c2cmarket.postgres-backup.plist
launchctl kickstart -k "gui/$(id -u)/com.c2cmarket.postgres-backup"
```

Replace every `/Users/CHANGE_ME` path before loading the job. Inspect `~/Library/Logs/c2cmarket-postgres-backup*.log` and confirm the first object exists in R2.

## 8. Release and rollback

For a regular backend release, rebuild and migrate staging first, validate OAuth and core flows, then repeat against production. A backend restart may cause the accepted 1–3 minute maintenance window.

```bash
docker compose -p c2c-staging --env-file .env.staging -f compose.yaml -f compose.prod.yaml --profile app build backend
docker compose -p c2c-staging --env-file .env.staging -f compose.yaml -f compose.prod.yaml --profile migrate run --rm migrate
docker compose -p c2c-staging --env-file .env.staging -f compose.yaml -f compose.prod.yaml --profile app up -d backend

docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app build backend
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile migrate run --rm migrate
docker compose -p c2c-prod --env-file .env.production -f compose.yaml -f compose.prod.yaml --profile app up -d backend
```

Before a migration-bearing production release, run and verify the R2 backup. Roll back application source/image independently for each Compose project. Review migration down scripts before any database rollback.

## 9. Troubleshooting order

1. Local `/health` and `/readyz` on ports 8080/8081.
2. `docker compose ... ps` and backend/PostgreSQL logs for the correct project.
3. `cloudflared tunnel ingress validate` and connector status. A public HTTP `530` response means Cloudflare has no healthy connector; if the browser labels that response as CORS because the error page has no allow-origin header, repair the Tunnel before changing backend CORS.
4. DNS: Pages owns frontend hosts; Tunnel owns API hosts.
5. Access: authenticate both staging hosts and verify OPTIONS bypass on the API app.
6. CORS: confirm each backend has exactly its matching frontend origin.
7. OAuth: confirm client, secret, and callback all belong to the same environment.
8. Backup: confirm local dump size, rclone remote, R2 object, and lifecycle rule.

For a Tunnel `530`, inspect the persistent service and recent transport failures:

```bash
launchctl print gui/$(id -u)/com.cloudflare.cloudflared
tail -n 100 ~/Library/Logs/com.cloudflare.cloudflared.err.log
curl -i https://api.c2cmarket.shop/readyz
```

Repeated `QUIC stream: timeout: no recent network activity` or `Failed to dial a quic connection` messages, while the HTTP/2 connectivity pre-check passes, require `protocol: http2` in `~/.cloudflared/config.yml` followed by `launchctl kickstart -k gui/$(id -u)/com.cloudflare.cloudflared`. Cloudflare's current connectivity-precheck guidance is <https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/troubleshoot-tunnels/connectivity-prechecks/>.
