# Frontend Development Guidelines

> Best practices for frontend development in this project.

---

## Overview

This directory contains the current project conventions for the Nuxt 4 + Vue 3 frontend. The specs describe actual patterns already used in `frontend/src`, especially hybrid rendering, the API facade, backend adapters, TanStack Query hooks, shadcn-vue primitives, and product-boundary copy rules.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Active |
| [Component Guidelines](./component-guidelines.md) | Component patterns, props, composition | Active |
| [Marketplace UI Guidelines](./marketplace-ui-guidelines.md) | Authoritative site-wide visual, layout, marketplace hierarchy, and browser acceptance contract | Active |
| [Functional Motion Guidelines](./motion-guidelines.md) | Transaction dialogs, keyed list updates, status feedback, timing, and reduced-motion contracts | Active |
| [Limited API Packages](../backend/api-limited-packages.md) | Cross-layer package publishing, cards, recommendation, ordering, and lifecycle contract | Active |
| [Limited API Quota Offers](./api-quota-offers.md) | Quota market, purchase, owner management, adapters, and responsive behavior | Active |
| [API Health And Quota Policy](../backend/api-health-quota-policy.md) | Probe administration, public health presentation, SKU quota rules, and order snapshots | Active |
| [API Probe Connections And Model Tester State](./type-safety.md#scenario-api-probe-connections-and-model-tester-state) | Reusable seller connections, service binding, HTTP acknowledgement, order import, temporary credentials, and batch model tests | Active |
| [Capability-Driven Navigation](./type-safety.md#scenario-capability-driven-student-seller-and-administrator-navigation) | Student, linux.do, probe, seller, and administrator menu/route/query boundaries | Active |
| [Transaction Contacts](../backend/transaction-contacts.md) | Explicit transaction contact selection, account-email disclosure, and snapshot boundaries | Active |
| [Unified Operation Audit](../backend/operation-audit.md) | Administrator filters, stable cursor pages, safe DTO projection, and no real-to-mock fallback | Active |
| [Public API Order Numbers](../backend/api-order-public-numbers.md) | Full public references, UUID route separation, normalized search, Mock migration, and responsive display | Active |
| [API Order Dispute Lifecycle](../backend/api-order-disputes.md) | Shared dispute projection labels, participant/admin consistency, remediation actions, and governance copy | Active |
| [API Service Promotions](./api-service-promotions.md) | Category-grid promotion injection, shared-card disclosure, administrator preflight, analytics, and DTO boundaries | Active |
| [Referral Rewards And Promotion Benefits](./promotion-rewards.md) | Invitation capture, user wallet, administrator controls, reward placement, poster, and analytics privacy | Active |
| [Registered-User Growth And Umami Analytics](../backend/growth-analytics.md) | Administrator growth dashboard, generated DTOs, registration attribution, normalized events, and opaque Umami identity | Active |
| [Owner API Service Sales Lifecycle](../backend/api-quota-offers.md#scenario-owner-api-service-sales-lifecycle-projection) | Server-filtered owner read model, channel states, default views, and republishing behavior | Active |
| [Reputation Presentation](./reputation.md) | Nullable real facts and public reputation wording | Active |
| [Hook Guidelines](./hook-guidelines.md) | Custom hooks, data fetching patterns | Active |
| [State Management](./state-management.md) | Local state, global state, server state | Active |
| [Identity And Session](../backend/identity-session.md) | Cross-layer account/API-market avatar, email time, and logout cache contract | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Active |
| [Type Safety](./type-safety.md) | Type patterns, validation | Active |
| [API Model Catalog Pricing Sync And Activation](../backend/api-model-catalog-sync.md) | Administrator-reviewed models.dev sync, activation, query invalidation, and responsive dialog boundaries | Active |
| [Nuxt SSR and Cloudflare Worker](./nuxt-ssr-deployment.md) | Hybrid rendering, development first-page performance, SEO, sitemap, runtime env, and Worker deployment contracts | Active |
| [Reproducible Release And Contract Drift](../backend/release-contract.md) | OpenAPI generator configuration and generated type drift contract | Active |
| [Runtime Security And Observability](../backend/runtime-operations.md) | Cloudflare Worker response-header and API-origin policy contract | Active |

---

## Pre-Development Checklist

Before editing frontend code, read:

1. [Directory Structure](./directory-structure.md)
2. [Component Guidelines](./component-guidelines.md)
3. [Marketplace UI Guidelines](./marketplace-ui-guidelines.md) for any product-facing UI change
4. [Functional Motion Guidelines](./motion-guidelines.md) when touching transaction animation, list updates, dialogs, status feedback, or reduced-motion behavior
5. [Limited API Packages](../backend/api-limited-packages.md) when touching fixed package publishing, cards, recommendation, ordering, or expiry
6. [Limited API Quota Offers](./api-quota-offers.md) when touching quota market, purchase, owner management, adapters, or mock flows
7. [API Health And Quota Policy](../backend/api-health-quota-policy.md) when touching probe administration, public health facts, quota rules, or order snapshots
8. [API Probe Connections And Model Tester State](./type-safety.md#scenario-api-probe-connections-and-model-tester-state) when touching seller connections, service binding, HTTP acknowledgement, order imports, or temporary model tests
9. [Capability-Driven Navigation](./type-safety.md#scenario-capability-driven-student-seller-and-administrator-navigation) when touching authenticated menus, routes, owner queries, student/linux.do mocks, probes, or administrator access
10. [Transaction Contacts](../backend/transaction-contacts.md) when touching contact settings, merchant/carpool contact selection, account-email disclosure, or contact audit display
11. [Unified Operation Audit](../backend/operation-audit.md) when touching `/admin/logs`, operation-audit filters/cursors, recent-operation cards, safe DTOs, or real/mock behavior
12. [Public API Order Numbers](../backend/api-order-public-numbers.md) when touching API order identifiers, adapters, lists, details, search, notifications, or Mock migration
13. [API Order Dispute Lifecycle](../backend/api-order-disputes.md) when touching API order dispute labels, actions, messages, remediation, or governance copy
14. [API Service Promotions](./api-service-promotions.md) when touching category-grid promotion injection, administrator scheduling, promotion analytics, or promotion DTO adapters
15. [Referral Rewards And Promotion Benefits](./promotion-rewards.md) when touching invite capture, benefits/wallet UI, growth-promotion administration, poster generation, reward placement, or benefit analytics
16. [Registered-User Growth And Umami Analytics](../backend/growth-analytics.md) when touching `/admin/growth`, growth queries/DTOs, registration attribution, auth funnel events, normalized page events, or Umami identity
17. [Owner API Service Sales Lifecycle](../backend/api-quota-offers.md#scenario-owner-api-service-sales-lifecycle-projection) when touching the owner service list, `salesSummary`, sales filters, or limited-package republishing
18. [Reputation Presentation](./reputation.md) when touching trust, completion, review, cancellation, or dispute facts
19. [Hook Guidelines](./hook-guidelines.md)
20. [State Management](./state-management.md)
21. [Identity And Session](../backend/identity-session.md) for account-shell, merchant avatar, or logout work
22. [Quality Guidelines](./quality-guidelines.md)
23. [Type Safety](./type-safety.md)
24. [API Model Catalog Pricing Sync And Activation](../backend/api-model-catalog-sync.md) when touching `/admin/api-models`, models.dev preview/apply, bulk activation, or public model catalog invalidation
25. [Nuxt SSR and Cloudflare Worker](./nuxt-ssr-deployment.md) for rendering, SEO, sitemap, or deployment work
26. [Reproducible Release And Contract Drift](../backend/release-contract.md) when touching OpenAPI or generated API types
27. [Runtime Security And Observability](../backend/runtime-operations.md) when touching `frontend/public/_headers`, frontend asset origins, or API connection origins
28. [C2CMarket Product Context](../guides/product-context.md)
29. [Maintainability Contract](../guides/maintainability-contract.md)

## Quality Check

Frontend changes must run local type/build verification:

```bash
pnpm --dir frontend typecheck
NUXT_PUBLIC_API_MODE=real \
NUXT_PUBLIC_SITE_URL=https://c2cmarket.shop \
NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
NUXT_API_BASE_URL=https://api.c2cmarket.shop \
pnpm --dir frontend build
```

For product-facing changes, also scan for product-boundary wording and verify real backend mode does not silently fall back to mock success data.

---

**Language**: All documentation should be written in **English**.
