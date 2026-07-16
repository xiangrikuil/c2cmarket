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
| [Limited API Packages](./api-limited-packages.md) | Cross-layer publishing, recommendation, snapshot, inventory, and expiry contract | Active |
| [Database Guidelines](./database-guidelines.md) | PostgreSQL migration patterns and schema conventions | Active |
| [Error Handling](./error-handling.md) | Problem Details and domain error handling | Active |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Active |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging and secret handling rules | Active |

---

## How to Fill These Guidelines

## Pre-Development Checklist

Before editing backend code, read:

1. [Directory Structure](./directory-structure.md)
2. [API Contracts](./api-contracts.md)
3. [Database Guidelines](./database-guidelines.md)
4. [Error Handling](./error-handling.md)
5. [Quality Guidelines](./quality-guidelines.md)
6. [Logging Guidelines](./logging-guidelines.md)
7. [C2CMarket Product Context](../guides/product-context.md)
8. [Maintainability Contract](../guides/maintainability-contract.md)

## Quality Check

Backend changes must run the package's local verification command:

```bash
cd backend && go test ./...
```

For contract-affecting work, also run product boundary scans over changed backend/OpenAPI docs and verify generated or hand-written OpenAPI/migration files against the conventions in this directory.

---

**Language**: All documentation should be written in **English**.
