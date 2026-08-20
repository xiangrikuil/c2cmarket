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
| [Carpool Lightweight Matching](./carpool-lightweight-matching.md) | Direct acceptance, condition snapshots, recruitment control, membership contact access, and review exclusion | Active |
| [Announcement Publication Lifecycle](./announcement-lifecycle.md) | Immediate/scheduled publication times, atomic persistence, status projection, and user-detail metadata | Active |
| [API Model Catalog Pricing Sync And Activation](./api-model-catalog-sync.md) | Administrator-reviewed models.dev preview/apply, price versions, activation, and responsive admin workflow | Active |
| [Identity And Session](./identity-session.md) | linux.do/account/API-market avatar projection, email time presentation, and logout cache consistency | Active |
| [Limited API Packages](./api-limited-packages.md) | Cross-layer publishing, recommendation, snapshot, inventory, and expiry contract | Active |
| [OAuth Identity And Administrator Bootstrap](./auth-identity.md) | Immutable provider identity ownership and proven create-only administrator bootstrap | Active |
| [Authentication Sessions](./auth-sessions.md) | Turnstile gates, renewal, student registration, deterministic capabilities, linux.do linking, and fixed development personas | Active |
| [Administrator Registration Invite Codes](./registration-invite-codes.md) | Admin-issued single-use registration codes, transactional consumption, OAuth boundaries, cleanup, and audit retention | Active |
| [Transaction Contacts](./transaction-contacts.md) | Explicit per-transaction WeChat or verified-email selection, snapshots, and privacy boundaries | Active |
| [Unified Operation Audit](./operation-audit.md) | Authoritative domain events, atomic idempotent writes, allowlisted safe projection, and stable administrator history | Active |
| [Restricted-Account Governance Appeals](./account-governance-appeals.md) | Existing-identity-only OAuth, dedicated fixed-expiry sessions, and appeal/account-status isolation | Active |
| [Administrator User Directory](./admin-user-directory.md) | Server pagination, safe account detail, and transactional account governance | Active |
| [Registered-User Growth And Umami Analytics](./growth-analytics.md) | PostgreSQL KPI truth, Shanghai cohorts, first-touch attribution, opaque Umami identity, and administrator dashboard | Active |
| [Verification And Data Lifecycle](./verification-data-lifecycle.md) | HMAC email challenges, finite idempotency, and bounded PostgreSQL retention | Active |
| [Limited API Quota Offers](./api-quota-offers.md) | Fixed quota inventory, rounds, owner sales lifecycle projection, orders, credentials, and concurrency | Active |
| [API Health And Quota Policy](./api-health-quota-policy.md) | Reusable real-model probes, 24-hour health, latency calibration, quota rules, and immutable order snapshots | Active |
| [Public API Order Numbers](./api-order-public-numbers.md) | Immutable commercial references, collision retry, UUID boundaries, projections, search, and responsive display | Active |
| [API Order Dispute Lifecycle](./api-order-disputes.md) | Dispute phases, order projection, participant/admin consistency, remediation, and governance boundaries | Active |
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
3. [Carpool Lightweight Matching](./carpool-lightweight-matching.md) when touching carpool listings, applications, memberships, contacts, reviews, reputation, or migration 111+
4. [API Model Catalog Pricing Sync And Activation](./api-model-catalog-sync.md) when touching the API model catalog, models.dev ingestion, price versions, model activation, or the public model picker
5. [Identity And Session](./identity-session.md) for OAuth, profile identity, email time, or logout work
6. [OAuth Identity And Administrator Bootstrap](./auth-identity.md) when touching OAuth identity ownership, provider bindings, OAuth permissions, or first-admin bootstrap
7. [Authentication Sessions](./auth-sessions.md) when touching session creation, validation, student registration, capability projection, linux.do linking, cookies, revocation, or expiry
8. [Administrator Registration Invite Codes](./registration-invite-codes.md) when touching required-code registration, administrator code management, password/OAuth/WeChat account creation, or registration-code cleanup
9. [Transaction Contacts](./transaction-contacts.md) when touching contact CRUD, transaction contact selection, account-email reuse, seller capabilities, linux.do projection, or contact audit events
10. [Unified Operation Audit](./operation-audit.md) when touching covered mutations/events, audit/access tables, administrator operation history, privacy projection, cursor filters, or retention semantics
11. [Restricted-Account Governance Appeals](./account-governance-appeals.md) when touching restricted OAuth, dedicated appeal sessions, account-governance appeals, or their standalone frontend
12. [Administrator User Directory](./admin-user-directory.md) when touching administrator account discovery, safe detail, status, permissions, or account-governance audit records
13. [Registered-User Growth And Umami Analytics](./growth-analytics.md) when touching growth KPIs, attribution, analytics identity/events, activity facts, cohorts, or the administrator growth dashboard
14. [Verification And Data Lifecycle](./verification-data-lifecycle.md) when touching email challenges, idempotency expiry/replay, retention SQL, or maintenance runners
15. [Limited API Packages](./api-limited-packages.md) when touching fixed packages, package inventory, order snapshots, or expiry
16. [Limited API Quota Offers](./api-quota-offers.md) when touching quota batches, offers, rounds, orders, inventory, or credential delivery
17. [API Health And Quota Policy](./api-health-quota-policy.md) when touching probe authorization, execution, health projection, quota rules, or order snapshots
18. [Public API Order Numbers](./api-order-public-numbers.md) when touching API order creation, migration, identifiers, DTOs, search, notifications, or visible order references
19. [API Order Dispute Lifecycle](./api-order-disputes.md) when touching API order dispute states, order projections, messages, remediation, or seller governance
20. [API Service Promotions](./api-service-promotions.md) when touching promotion schedules, eligibility, public promotion reads, analytics contracts, or administrator promotion actions
21. [Referral Rewards And Promotion Benefits](./promotion-rewards.md) when touching referral attribution, first-publication rewards, promotion coupons, reward projection, or growth-promotion administration
22. [Reputation Facts](./reputation.md) when touching reputation facts, exclusions, scoring inputs, or public reputation DTOs
23. [Database Guidelines](./database-guidelines.md)
24. [Deployment Contract](./deployment-contract.md) for CI/CD, images, Compose release, backup, or VPS work
25. [Error Handling](./error-handling.md)
26. [Quality Guidelines](./quality-guidelines.md)
27. [Logging Guidelines](./logging-guidelines.md)
28. [Safe Outbound HTTP](./outbound-http.md) when adding or changing outbound HTTP requests whose destination is stored, configured, or otherwise variable
29. [Reproducible Release And Contract Drift](./release-contract.md) when touching source packaging, Docker/Compose releases, build metadata, `/version`, OpenAPI, or generated API types
30. [Runtime Security And Observability](./runtime-operations.md) when touching response headers, `frontend/public/_headers`, `/metrics`, observability labels, or operator-route exposure
31. [C2CMarket Product Context](../guides/product-context.md)
32. [Maintainability Contract](../guides/maintainability-contract.md)
33. [Announcement Publication Lifecycle](./announcement-lifecycle.md) when touching announcement publication, timing, status derivation, audit persistence, Mock parity, or user-detail time metadata

## Quality Check

Backend changes must run the package's local verification command:

```bash
cd backend && go test ./...
```

For contract-affecting work, also run product boundary scans over changed backend/OpenAPI docs and verify generated or hand-written OpenAPI/migration files against the conventions in this directory.

---

**Language**: All documentation should be written in **English**.
