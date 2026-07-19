<p align="center">
  <img src="./frontend/public/c2cmarket-logo-mark.svg" alt="C2CMarket" width="88" height="88">
</p>

<h1 align="center">C2CMarket</h1>

<p align="center">
  A community marketplace for subscription carpools, API services, demand posts, and official price references.
</p>

<p align="center">
  <a href="./README.md">简体中文</a> · <a href="./README_EN.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/xiangrikuil/c2cmarket/actions/workflows/ci.yml"><img src="https://github.com/xiangrikuil/c2cmarket/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-green.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26">
  <img src="https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white" alt="Vue 3">
</p>

> [!IMPORTANT]
> C2CMarket is under active development. APIs, database migrations, and deployment configuration may change. Review the configuration, business rules, and applicable local requirements before deploying to production.

## Overview

C2CMarket is a decoupled web application for community-driven listings and transaction coordination. It helps users publish, discover, and manage subscription carpools, API services, demand posts, and official price records. It also includes order tracking, notifications, reviews, reports, and an administration console.

The platform focuses on discovery, matching, off-platform communication, and reputation records. It does not process in-platform payments, provide escrow or fulfillment guarantees, or proxy upstream API traffic.

## Features

- **Subscription carpools**: listings, applications, contact windows, join confirmation, completion, exit, and owner management.
- **API service marketplace**: publishing, review, availability, orders, payment confirmation, and fulfillment status tracking.
- **Demand posts**: publishing, moderation, public discovery, closing, and reopening.
- **Official prices**: maintained reference records for publicly available official pricing.
- **Community reputation**: public profiles, favorites, reviews, reports, disputes, and appeals.
- **Notification center**: announcements, business notifications, unread state, and email reminders.
- **Administration**: users, product catalog, listings, services, orders, announcements, feedback, and audit records.
- **Unified search**: public carpools, API services, demand posts, price records, and profiles.

## Technology

| Layer | Technology |
| --- | --- |
| Frontend | Nuxt 4, Vue 3, TypeScript, Pinia, TanStack Query, Tailwind CSS |
| Backend | Go 1.26, chi, pgx |
| Database | PostgreSQL 18, versioned SQL migrations |
| Infrastructure | Docker Compose, Cloudflare Workers, VPS/Caddy, GHCR, GitHub Actions |
| Integrations | linux.do OAuth 2.0, Alibaba Cloud DirectMail SMTP, optional Umami |

## Repository layout

```text
.
├── frontend/              Nuxt 4 hybrid-rendered application
├── backend/               Go HTTP API
│   ├── cmd/api/           Service entry point
│   ├── internal/          Domain modules and infrastructure
│   └── migrations/        PostgreSQL migrations
├── docs/openapi/          OpenAPI contract
├── docs/ops/              Deployment and operations guides
├── scripts/               Contract checks and smoke tests
├── compose.yaml           Local development services
└── compose.prod.yaml      Production Compose overrides
```

## Quick start

### Requirements

- Docker and Docker Compose
- Node.js `>=24.11 <25`
- pnpm `>=10 <11`
- Go 1.26 when running the backend outside Docker

### 1. Clone and configure

```bash
git clone https://github.com/xiangrikuil/c2cmarket.git
cd c2cmarket
cp .env.example .env
```

`.env.example` contains development defaults only. Never commit real credentials.

### 2. Start PostgreSQL and apply migrations

```bash
docker compose up -d postgres
docker compose --profile migrate run --rm migrate
```

### 3. Start the backend

```bash
docker compose --profile app up -d --build backend
```

The backend listens on `http://127.0.0.1:8080` by default:

```text
GET /health
GET /readyz
```

### 4. Start the frontend

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend dev
```

Open `http://127.0.0.1:3000`. The Nuxt development server uses runtime configuration to reach the local backend.

Stop the local services with:

```bash
docker compose --profile app down
```

## Local verification

Run these checks before opening a pull request:

```bash
cd backend && go test ./...
cd ..
pnpm --dir frontend typecheck
NUXT_PUBLIC_API_MODE=real \
NUXT_PUBLIC_SITE_URL=https://c2cmarket.shop \
NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
NUXT_API_BASE_URL=https://api.c2cmarket.shop \
pnpm --dir frontend build
pnpm --dir frontend test
node scripts/check-openapi-routes.mjs
node scripts/check-migrations-doc.mjs
```

Production frontend builds require real mode plus both the public and server-side API URLs.

With the backend running, execute the end-to-end smoke suite when the change affects business workflows:

```bash
API_BASE_URL=http://127.0.0.1:8080 node scripts/run-smokes.mjs
```

## Configuration and deployment

- Local configuration: [`.env.example`](./.env.example)
- Production configuration: [`.env.production.example`](./.env.production.example)
- Staging configuration: [`.env.staging.example`](./.env.staging.example)
- API contract: [`docs/openapi/c2c-market-api-v1.yaml`](./docs/openapi/c2c-market-api-v1.yaml)
- Deployment guide: [`docs/ops/deployment-runbook.md`](./docs/ops/deployment-runbook.md)
- Workers/VPS deployment: [`docs/ops/cloudflare-workers-vps-backends.md`](./docs/ops/cloudflare-workers-vps-backends.md)

Production requires real OAuth, independent encryption keys, an HTTPS frontend origin, PostgreSQL, and valid SMTP configuration. Do not reuse development defaults from the example files.

## Product boundaries and disclaimer

C2CMarket is not a payment, escrow, account custody, fulfillment guarantee, or API proxy platform. It must not store or transfer third-party account passwords, cookies, sessions, verification codes, recovery codes, or panel owner credentials.

Cost sharing, member invitations, and usage patterns for third-party subscriptions may be restricted by the relevant provider terms and may result in account restrictions, service interruptions, privacy exposure, or financial loss. This project is not officially affiliated with, authorized by, or guaranteed by linux.do, OpenAI, or any other third-party provider. Users are responsible for reviewing applicable terms and accepting the associated risks.

## Contributing

Issues and pull requests are welcome. Read the [contribution guide](./CONTRIBUTING.md) before starting, and keep each change focused and independently verifiable.

## License

C2CMarket is available under the [MIT License](./LICENSE).
