<p align="center">
  <img src="./frontend/public/c2cmarket-logo-mark.svg" alt="C2CMarket" width="80" height="80">
</p>

<h1 align="center">C2CMarket</h1>

<p align="center">
  A community marketplace for linux.do users, covering subscription carpools, API services, and public pricing records.
</p>

<p align="center">
  <a href="https://c2cmarket.shop"><strong>Open C2CMarket</strong></a> ·
  <a href="./docs/openapi/c2c-market-api-v1.yaml">API contract</a> ·
  <a href="./docs/ops/deployment-runbook.md">Deployment</a> ·
  <a href="./CONTRIBUTING.md">Contributing</a>
</p>

<p align="center">
  <a href="./README.md">简体中文</a> · <a href="./README_EN.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/xiangrikuil/c2cmarket/actions/workflows/ci.yml"><img src="https://github.com/xiangrikuil/c2cmarket/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-green.svg" alt="MIT License"></a>
  <a href="https://linux.do"><img src="https://img.shields.io/badge/community-linux.do-1D4ED8?logo=discourse&logoColor=white" alt="linux.do community"></a>
  <img src="https://img.shields.io/badge/Go-1.26.6-00ADD8?logo=go&logoColor=white" alt="Go 1.26.6">
  <img src="https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white" alt="Vue 3">
</p>

<p align="center">
  <a href="https://c2cmarket.shop">
    <img src="./.github/assets/c2cmarket-home.png" alt="Anonymous C2CMarket home page" width="100%">
  </a>
</p>

> [!NOTE]
> C2CMarket is under development. APIs, database migrations, and deployment settings may change before 1.0. Review the [release checklist](./docs/release-checklist.md) before a production deployment.

## Why C2CMarket

linux.do users already offer and seek subscription seats and API service quota, but listings are scattered across posts and separate services. Buyers and sellers have no single place to check profiles, orders, reviews, and disputes. It is hard to assess the other party before a deal or trace what happened when something goes wrong.

Some ChatGPT and Claude users have spare subscription seats or API quota; others need more. Relay and community-run services meet part of that demand, but their sources, rules, and availability vary. C2CMarket brings these listings into one market, where both sides can review public profiles and transaction history before deciding whether to trade off-platform.

## About C2CMarket

C2CMarket brings subscription carpools, API services, buyer requests, and public pricing records into one marketplace. Users can browse or publish listings. The platform records applications, orders, reviews, and disputes; communication and payment happen off-platform.

The platform does not process payments, provide escrow or fulfillment guarantees, or proxy upstream API traffic. This boundary applies to public pages, order flows, and administration tools.

## Features

- Browse subscription carpools, API services, buyer requests, and public pricing records.
- Publish and manage carpool or API service listings, then track applications, orders, and fulfillment states.
- Review public profiles, ratings, reports, and dispute records before working with another user.
- Use notifications, unified search, and administration tools for routine operations.

## Technology

| Layer | Technology |
| --- | --- |
| Frontend | Nuxt 4, Vue 3, TypeScript, Pinia, TanStack Query, Tailwind CSS |
| Backend | Go 1.26.6, chi, pgx |
| Database | PostgreSQL 18, versioned SQL migrations |
| Deployment | Docker Compose, Cloudflare Workers, VPS/Caddy, GHCR |
| Integrations | linux.do OAuth 2.0, Alibaba Cloud DirectMail SMTP, optional Umami |

## Quick start

### Requirements

- Docker and Docker Compose
- Node.js `>=24.11 <25`
- pnpm `>=10 <11`
- Go 1.26.6 when running the backend outside Docker

### 1. Clone and configure

```bash
git clone https://github.com/xiangrikuil/c2cmarket.git
cd c2cmarket
cp .env.example .env
```

### 2. Start PostgreSQL and apply migrations

```bash
docker compose up -d postgres
docker compose --profile migrate run --rm migrate
```

### 3. Start the backend

```bash
docker compose --profile app up -d --build backend
```

The backend listens on `http://127.0.0.1:8080` by default. Health endpoints:

```text
GET /health
GET /readyz
GET /version
```

### 4. Start the frontend

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend dev
```

Open `http://127.0.0.1:5173`. The development command connects to the local backend through the Nuxt same-origin proxy. For a frontend-only demo, run `pnpm --dir frontend dev:mock`.

Stop the local services with:

```bash
docker compose --profile app down
```

## Local verification

Before opening a pull request, run the checks related to your change:

```bash
cd backend && go test ./...
cd ..
pnpm --dir frontend typecheck
pnpm --dir frontend test
node scripts/check-openapi-routes.mjs
node scripts/check-openapi-types.mjs
node scripts/check-migrations-doc.mjs
node scripts/check-compose-exposure.mjs
```

A production-mode frontend build also requires explicit site and API URLs:

```bash
NUXT_PUBLIC_API_MODE=real \
NUXT_PUBLIC_SITE_URL=https://c2cmarket.shop \
NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
NUXT_API_BASE_URL=https://api.c2cmarket.shop \
pnpm --dir frontend build
```

The full business smoke suite requires a running backend:

```bash
API_BASE_URL=http://127.0.0.1:8080 node scripts/run-smokes.mjs
```

## Repository layout

```text
.
├── frontend/          Nuxt application
├── backend/           Go HTTP API and database migrations
├── docs/openapi/      OpenAPI contract
├── docs/ops/          Deployment and operations guides
├── scripts/           Contract checks, release tools, and smoke tests
├── compose.yaml       Local development services
└── compose.prod.yaml  Production Compose overrides
```

## Documentation

| Document | Contents |
| --- | --- |
| [OpenAPI contract](./docs/openapi/c2c-market-api-v1.yaml) | HTTP APIs, requests, and responses |
| [System architecture](./docs/project-architecture-api-db-overview-2026-06-23.md) | Frontend, backend, API, and database relationships |
| [Deployment guide](./docs/ops/deployment-runbook.md) | Environment setup, migrations, and releases |
| [Workers/VPS deployment](./docs/ops/cloudflare-workers-vps-backends.md) | Cloudflare and backend topology |
| [Production operations](./docs/operations.md) | Routine checks and incident handling |
| [Backup and restore](./docs/backup-restore.md) | PostgreSQL backup and recovery |
| [Release checklist](./docs/release-checklist.md) | Checks before and after a release |

## Product boundaries

C2CMarket is not a payment processor, escrow service, account custodian, fulfillment guarantor, or API proxy. It does not accept third-party account passwords, cookies, sessions, verification codes, recovery codes, or panel owner credentials. An API order may store one buyer-specific delivery credential and show it to the order participants only after the seller confirms off-platform payment.

Cost sharing, member invitations, and usage patterns may be restricted by the relevant provider terms. C2CMarket is not affiliated with, authorized by, or endorsed by linux.do, OpenAI, or any other third-party service provider. Users are responsible for reviewing the applicable terms.

## Contributing

Issues and pull requests are welcome. Read the [contribution guide](./CONTRIBUTING.md) before starting, and keep each change focused and independently verifiable.

## License

C2CMarket is available under the [MIT License](./LICENSE).
