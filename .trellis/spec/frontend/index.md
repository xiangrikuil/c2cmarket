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
| [Limited API Packages](../backend/api-limited-packages.md) | Cross-layer package publishing, cards, recommendation, ordering, and lifecycle contract | Active |
| [Hook Guidelines](./hook-guidelines.md) | Custom hooks, data fetching patterns | Active |
| [State Management](./state-management.md) | Local state, global state, server state | Active |
| [Identity And Session](../backend/identity-session.md) | Cross-layer account/API-market avatar, email time, and logout cache contract | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Active |
| [Type Safety](./type-safety.md) | Type patterns, validation | Active |
| [Nuxt SSR and Cloudflare Worker](./nuxt-ssr-deployment.md) | Hybrid rendering, SEO, sitemap, runtime env, and Worker deployment contracts | Active |

---

## Pre-Development Checklist

Before editing frontend code, read:

1. [Directory Structure](./directory-structure.md)
2. [Component Guidelines](./component-guidelines.md)
3. [Marketplace UI Guidelines](./marketplace-ui-guidelines.md) for any product-facing UI change
4. [Hook Guidelines](./hook-guidelines.md)
5. [State Management](./state-management.md)
6. [Identity And Session](../backend/identity-session.md) for account-shell, merchant avatar, or logout work
7. [Quality Guidelines](./quality-guidelines.md)
8. [Type Safety](./type-safety.md)
9. [C2CMarket Product Context](../guides/product-context.md)
10. [Maintainability Contract](../guides/maintainability-contract.md)
11. [Nuxt SSR and Cloudflare Worker](./nuxt-ssr-deployment.md) for rendering, SEO, sitemap, or deployment work

## Quality Check

Frontend changes must run local type/build verification:

```bash
pnpm --dir frontend typecheck
VITE_API_MODE=real \
NUXT_PUBLIC_SITE_URL=https://c2cmarket.shop \
NUXT_PUBLIC_API_BASE_URL=https://api.c2cmarket.shop \
NUXT_API_BASE_URL=https://api.c2cmarket.shop \
pnpm --dir frontend build
```

For product-facing changes, also scan for product-boundary wording and verify real backend mode does not silently fall back to mock success data.

---

**Language**: All documentation should be written in **English**.
