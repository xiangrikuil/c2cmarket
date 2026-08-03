# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory contains guidelines for backend development. Fill in each file with your project's specific conventions.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Active |
| [API Contracts](./api-contracts.md) | HTTP, session, CSRF, idempotency, and slice contracts | Active |
| [Identity And Session](./identity-session.md) | linux.do/account/API-market avatar projection, email time presentation, and logout cache consistency | Active |
| [Limited API Packages](./api-limited-packages.md) | Cross-layer publishing, recommendation, snapshot, inventory, and expiry contract | Active |
| [OAuth Identity And Administrator Bootstrap](./auth-identity.md) | Immutable provider identity ownership and proven create-only administrator bootstrap | Active |
| [Authentication Session Renewal](./auth-sessions.md) | Seven-day idle expiry, renewal throttling, cookie sync, and absolute expiry | Active |
| [Administrator User Directory](./admin-user-directory.md) | Server pagination, safe account detail, and transactional account governance | Active |
| [Registered-User Growth And Umami Analytics](./growth-analytics.md) | PostgreSQL KPI truth, Shanghai cohorts, first-touch attribution, opaque Umami identity, and administrator dashboard | Active |
| [Verification And Data Lifecycle](./verification-data-lifecycle.md) | HMAC email challenges, finite idempotency, and bounded PostgreSQL retention | Active |
| [Limited API Quota Offers](./api-quota-offers.md) | Fixed quota inventory, rounds, owner sales lifecycle projection, orders, credentials, and concurrency | Active |
| [Public API Order Numbers](./api-order-public-numbers.md) | Immutable commercial references, collision retry, UUID boundaries, projections, search, and responsive display | Active |
| [API Service Promotions](./api-service-promotions.md) | Administrator schedules, eligibility, category-grid projection, service commercial facts, snapshots, audit, and idempotency | Active |
| [Referral Rewards And Promotion Benefits](./promotion-rewards.md) | API-service referrals, first-publication qualification, coupon lifecycle, reward projection, administration, and audit | Active |
| [Reputation Facts](./reputation.md) | Truthful transaction facts, role/scope aggregation, and exclusions | Active |
| [Database Guidelines](./database-guidelines.md) | PostgreSQL migration patterns and schema conventions | Active |
| [Deployment Contract](./deployment-contract.md) | CI, GHCR, VPS release, backup, and environment isolation requirements | Active |
| [Error Handling](./error-handling.md) | Problem Details and domain error handling | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Active |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging and secret handling rules | Active |
| [Safe Outbound HTTP](./outbound-http.md) | Public HTTPS validation, DNS rebinding defense, bounded clients, and secret-safe errors | Active |
| [Reproducible Release And Contract Drift](./release-contract.md) | Fixed-commit archives/images, build metadata, image-only production Compose, and generated OpenAPI types | Active |
| [Runtime Security And Observability](./runtime-operations.md) | API/Pages response headers, protected metrics, bounded labels, and operator-route exposure | Active |

---

## How to Fill These Guidelines

## Pre-Development Checklist

Before editing backend code, read:

1. [Directory Structure](./directory-structure.md)
2. [API Contracts](./api-contracts.md)
3. [Identity And Session](./identity-session.md) for OAuth, profile identity, email time, or logout work
4. [OAuth Identity And Administrator Bootstrap](./auth-identity.md) when touching OAuth identity ownership, provider bindings, OAuth permissions, or first-admin bootstrap
5. [Authentication Session Renewal](./auth-sessions.md) when touching session creation, validation, cookies, revocation, or expiry
6. [Administrator User Directory](./admin-user-directory.md) when touching administrator account discovery, safe detail, status, permissions, or account-governance audit records
7. [Registered-User Growth And Umami Analytics](./growth-analytics.md) when touching growth KPIs, attribution, analytics identity/events, activity facts, cohorts, or the administrator growth dashboard
8. [Verification And Data Lifecycle](./verification-data-lifecycle.md) when touching email challenges, idempotency expiry/replay, retention SQL, or maintenance runners
9. [Limited API Packages](./api-limited-packages.md) when touching fixed packages, package inventory, order snapshots, or expiry
10. [Limited API Quota Offers](./api-quota-offers.md) when touching quota batches, offers, rounds, orders, inventory, or credential delivery
11. [Public API Order Numbers](./api-order-public-numbers.md) when touching API order creation, migration, identifiers, DTOs, search, notifications, or visible order references
12. [API Service Promotions](./api-service-promotions.md) when touching promotion schedules, eligibility, public promotion reads, analytics contracts, or administrator promotion actions
13. [Referral Rewards And Promotion Benefits](./promotion-rewards.md) when touching referral attribution, first-publication rewards, promotion coupons, reward projection, or growth-promotion administration
14. [Reputation Facts](./reputation.md) when touching reputation facts, exclusions, scoring inputs, or public reputation DTOs
15. [Database Guidelines](./database-guidelines.md)
16. [Deployment Contract](./deployment-contract.md) for CI/CD, images, Compose release, backup, or VPS work
17. [Error Handling](./error-handling.md)
18. [Quality Guidelines](./quality-guidelines.md)
19. [Logging Guidelines](./logging-guidelines.md)
20. [Safe Outbound HTTP](./outbound-http.md) when adding or changing outbound HTTP requests whose destination is stored, configured, or otherwise variable
21. [Reproducible Release And Contract Drift](./release-contract.md) when touching source packaging, Docker/Compose releases, build metadata, `/version`, OpenAPI, or generated API types
22. [Runtime Security And Observability](./runtime-operations.md) when touching response headers, `frontend/public/_headers`, `/metrics`, observability labels, or operator-route exposure
23. [C2CMarket Product Context](../guides/product-context.md)
24. [Maintainability Contract](../guides/maintainability-contract.md)

## Quality Check

Backend changes must run the package's local verification command:

```bash
cd backend && go test ./...
```

For contract-affecting work, also run product boundary scans over changed backend/OpenAPI docs and verify generated or hand-written OpenAPI/migration files against the conventions in this directory.

---

**Language**: All documentation should be written in **English**.
