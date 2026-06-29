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
| [Hook Guidelines](./hook-guidelines.md) | Custom hooks, data fetching patterns | Active |
| [State Management](./state-management.md) | Local state, global state, server state | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Active |
| [Type Safety](./type-safety.md) | Type patterns, validation | Active |

---

## Pre-Development Checklist

Before editing frontend code, read:

1. [Directory Structure](./directory-structure.md)
2. [Component Guidelines](./component-guidelines.md)
3. [Hook Guidelines](./hook-guidelines.md)
4. [State Management](./state-management.md)
5. [Quality Guidelines](./quality-guidelines.md)
6. [Type Safety](./type-safety.md)
7. [C2CMarket Product Context](../guides/product-context.md)
8. [Maintainability Contract](../guides/maintainability-contract.md)

## Quality Check

Frontend changes must run local type/build verification:

```bash
pnpm --dir frontend exec vue-tsc -b --pretty false
pnpm --dir frontend exec vite build
```

For product-facing changes, also scan for product-boundary wording and verify real backend mode does not silently fall back to mock success data.

---

**Language**: All documentation should be written in **English**.
