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
| [Database Guidelines](./database-guidelines.md) | PostgreSQL migration patterns and schema conventions | Active |
| [Deployment Contract](./deployment-contract.md) | CI, GHCR, VPS release, backup, and environment isolation requirements | Active |
| [Error Handling](./error-handling.md) | Problem Details and domain error handling | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Active |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging and secret handling rules | Active |

---

## How to Fill These Guidelines

## Pre-Development Checklist

Before editing backend code, read:

1. [Directory Structure](./directory-structure.md)
2. [API Contracts](./api-contracts.md)
3. [Identity And Session](./identity-session.md) for OAuth, profile identity, email time, or logout work
4. [Database Guidelines](./database-guidelines.md)
5. [Deployment Contract](./deployment-contract.md) for CI/CD, images, Compose release, backup, or VPS work
6. [Error Handling](./error-handling.md)
7. [Quality Guidelines](./quality-guidelines.md)
8. [Logging Guidelines](./logging-guidelines.md)
9. [C2CMarket Product Context](../guides/product-context.md)
10. [Maintainability Contract](../guides/maintainability-contract.md)

## Quality Check

Backend changes must run the package's local verification command:

```bash
cd backend && go test ./...
```

For contract-affecting work, also run product boundary scans over changed backend/OpenAPI docs and verify generated or hand-written OpenAPI/migration files against the conventions in this directory.

---

**Language**: All documentation should be written in **English**.
