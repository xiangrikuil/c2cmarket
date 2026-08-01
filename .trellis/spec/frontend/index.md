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
| [Owner API Service Sales Lifecycle](../backend/api-quota-offers.md#scenario-owner-api-service-sales-lifecycle-projection) | Server-filtered owner read model, channel states, default views, and republishing behavior | Active |
| [Reputation Presentation](./reputation.md) | Nullable real facts and public reputation wording | Active |
| [Hook Guidelines](./hook-guidelines.md) | Custom hooks, data fetching patterns | Active |
| [State Management](./state-management.md) | Local state, global state, server state | Active |
| [Identity And Session](../backend/identity-session.md) | Cross-layer account/API-market avatar, email time, and logout cache contract | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Active |
| [Type Safety](./type-safety.md) | Type patterns, validation | Active |
| [Nuxt SSR and Cloudflare Worker](./nuxt-ssr-deployment.md) | Hybrid rendering, SEO, sitemap, runtime env, and Worker deployment contracts | Active |
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
7. [Owner API Service Sales Lifecycle](../backend/api-quota-offers.md#scenario-owner-api-service-sales-lifecycle-projection) when touching the owner service list, `salesSummary`, sales filters, or limited-package republishing
8. [Reputation Presentation](./reputation.md) when touching trust, completion, review, cancellation, or dispute facts
9. [Hook Guidelines](./hook-guidelines.md)
10. [State Management](./state-management.md)
11. [Identity And Session](../backend/identity-session.md) for account-shell, merchant avatar, or logout work
12. [Quality Guidelines](./quality-guidelines.md)
13. [Type Safety](./type-safety.md)
14. [Nuxt SSR and Cloudflare Worker](./nuxt-ssr-deployment.md) for rendering, SEO, sitemap, or deployment work
15. [Reproducible Release And Contract Drift](../backend/release-contract.md) when touching OpenAPI or generated API types
16. [Runtime Security And Observability](../backend/runtime-operations.md) when touching `frontend/public/_headers`, frontend asset origins, or API connection origins
17. [C2CMarket Product Context](../guides/product-context.md)
18. [Maintainability Contract](../guides/maintainability-contract.md)

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
