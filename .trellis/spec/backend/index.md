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
| [OAuth Identity And Administrator Bootstrap](./auth-identity.md) | Immutable provider identity ownership and proven create-only administrator bootstrap | Active |
| [Authentication Session Renewal](./auth-sessions.md) | Seven-day idle expiry, renewal throttling, cookie sync, and absolute expiry | Active |
| [Limited API Quota Offers](./api-quota-offers.md) | Fixed quota inventory, rounds, orders, credentials, and concurrency | Active |
| [Reputation Facts](./reputation.md) | Truthful transaction facts, role/scope aggregation, and exclusions | Active |
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
3. [OAuth Identity And Administrator Bootstrap](./auth-identity.md) when touching OAuth identity ownership, provider bindings, OAuth permissions, or first-admin bootstrap
4. [Authentication Session Renewal](./auth-sessions.md) when touching session creation, validation, cookies, revocation, or expiry
5. [Limited API Quota Offers](./api-quota-offers.md) when touching quota batches, offers, rounds, orders, inventory, or credential delivery
6. [Reputation Facts](./reputation.md) when touching reputation facts, exclusions, scoring inputs, or public reputation DTOs
7. [Database Guidelines](./database-guidelines.md)
8. [Error Handling](./error-handling.md)
9. [Quality Guidelines](./quality-guidelines.md)
10. [Logging Guidelines](./logging-guidelines.md)
11. [C2CMarket Product Context](../guides/product-context.md)
12. [Maintainability Contract](../guides/maintainability-contract.md)

## Quality Check

Backend changes must run the package's local verification command:

```bash
cd backend && go test ./...
```

For contract-affecting work, also run product boundary scans over changed backend/OpenAPI docs and verify generated or hand-written OpenAPI/migration files against the conventions in this directory.

---

**Language**: All documentation should be written in **English**.
