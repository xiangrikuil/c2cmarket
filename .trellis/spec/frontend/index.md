# Frontend Development Guidelines

> Best practices for frontend development in this project.

---

## Overview

This directory contains the current project conventions for the Vue 3 + Vite frontend. The specs describe actual patterns already used in `frontend/src`, especially the API facade, backend adapters, TanStack Query hooks, shadcn-vue primitives, and product-boundary copy rules.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Active |
| [Component Guidelines](./component-guidelines.md) | Component patterns, props, composition | Active |
| [Marketplace UI Guidelines](./marketplace-ui-guidelines.md) | Authoritative site-wide visual, layout, marketplace hierarchy, and browser acceptance contract | Active |
| [Limited API Quota Offers](./api-quota-offers.md) | Quota market, purchase, owner management, adapters, and responsive behavior | Active |
| [Reputation Presentation](./reputation.md) | Nullable real facts and public reputation wording | Active |
| [Hook Guidelines](./hook-guidelines.md) | Custom hooks, data fetching patterns | Active |
| [State Management](./state-management.md) | Local state, global state, server state | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Active |
| [Type Safety](./type-safety.md) | Type patterns, validation | Active |
| [Reproducible Release And Contract Drift](../backend/release-contract.md) | OpenAPI generator configuration and generated type drift contract | Active |

---

## Pre-Development Checklist

Before editing frontend code, read:

1. [Directory Structure](./directory-structure.md)
2. [Component Guidelines](./component-guidelines.md)
3. [Marketplace UI Guidelines](./marketplace-ui-guidelines.md) for any product-facing UI change
4. [Limited API Quota Offers](./api-quota-offers.md) when touching quota market, purchase, owner management, adapters, or mock flows
5. [Reputation Presentation](./reputation.md) when touching trust, completion, review, cancellation, or dispute facts
6. [Hook Guidelines](./hook-guidelines.md)
7. [State Management](./state-management.md)
8. [Quality Guidelines](./quality-guidelines.md)
9. [Type Safety](./type-safety.md)
10. [Reproducible Release And Contract Drift](../backend/release-contract.md) when touching OpenAPI or generated API types
11. [C2CMarket Product Context](../guides/product-context.md)
12. [Maintainability Contract](../guides/maintainability-contract.md)

## Quality Check

Frontend changes must run local type/build verification:

```bash
pnpm --dir frontend exec vue-tsc -b --pretty false
pnpm --dir frontend exec vite build
```

For product-facing changes, also scan for product-boundary wording and verify real backend mode does not silently fall back to mock success data.

---

**Language**: All documentation should be written in **English**.
