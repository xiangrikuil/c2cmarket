# Directory Structure

> How backend code is organized in this project.

---

## Overview

C2CMarket backend is a Go service built on the standard `net/http` stack with `github.com/go-chi/chi/v5` for routing. Keep transport parsing, application behavior, and domain error definitions in separate packages.

---

## Directory Layout

```
backend/
├── cmd/api/                  # process entrypoint and PORT binding
├── internal/app/             # dependency composition for config, store, service, server
├── internal/server/          # chi router, route registration, handlers, server DTOs
├── internal/middleware/      # request ID, session cookie, CSRF, idempotency HTTP helpers
├── internal/response/        # JSON and Problem Details response helpers
├── internal/validator/       # strict JSON, If-Match, request hash, shared request parsing
├── internal/database/        # pgxpool construction and PostgreSQL readiness checks
├── internal/module/          # business modules and core compatibility facade
├── internal/config/          # environment config and production startup guards
├── internal/domain/          # shared domain errors and stable error codes
├── internal/health/          # readiness status contract shared by stores and HTTP
├── internal/store/postgres/  # PostgreSQL repository implementation split by domain
└── migrations/               # PostgreSQL-oriented SQL contract baseline

docs/openapi/
└── c2c-market-api-v1.yaml    # OpenAPI 3.1 contract for implemented endpoints
```

---

## Module Organization

- Add new endpoint handlers in `internal/server` unless the target business module already owns a handler adapter.
- Keep process dependency wiring in `internal/app`; do not put business state machines there.
- Keep `internal/module/core` as a compatibility facade only. It may keep legacy service method names during the transition, but business state machines belong in module-owned services.
- `internal/module/auth`, `internal/module/idempotency`, `internal/module/contact`, `internal/module/catalog`, `internal/module/officialprice`, `internal/module/carpool`, `internal/module/apimarket`, `internal/module/apiintent`, `internal/module/profile`, and `internal/module/announcement` own their models, repository contracts, and module services. `internal/module/core` delegates to these services.
- New module work should prefer focused files under `internal/module/<domain>` for handler, service, repository interface, model, DTO, and errors when that domain is touched.
- Keep stable error codes and error constructors in `internal/domain`.
- Keep environment parsing and production startup guards in `internal/config`.
- Keep process/database readiness contracts in `internal/health`; concrete stores should return health status without importing HTTP packages.
- Do not let handlers mutate persistence directly when the action has state-machine, idempotency, or product-boundary logic.
- In-memory storage for no-repository local tests belongs inside the owning module service. It must not be a production fallback after a PostgreSQL repository exists.
- PostgreSQL connection lifecycle belongs to `internal/database` plus `internal/store/postgres`; HTTP handlers should receive only small interfaces such as readiness checkers.
- The service layer must depend on focused repository interfaces such as auth, idempotency, catalog, carpool, API service, API purchase intent, contact, official price, profile, and announcement repositories. Do not reintroduce one broad repository interface as the primary dependency of new code.

---

## Naming Conventions

- Package names are short and responsibility-based: `server`, `app`, `domain`, `database`, `response`, `validator`.
- JSON DTO fields are camelCase.
- Database table and column names are snake_case.
- Endpoint route keys used for idempotency must be stable strings matching the OpenAPI path shape.
- Public resource IDs returned by the runtime must be UUID strings to match PostgreSQL `uuid` primary and foreign keys. Session cookies, CSRF tokens, and idempotency keys remain opaque tokens and must not be modeled as public UUID identifiers.

---

## Examples

- `backend/cmd/api/main.go`: process startup, config load, app construction, and HTTP listen.
- `backend/internal/app/app.go`: dependency composition, PostgreSQL store construction, and server wiring.
- `backend/internal/server/routes.go`: chi route tree for implemented endpoints.
- `backend/internal/server/*_handler.go`: server-level HTTP adapters while module handlers are still being migrated.
- `backend/internal/module/{auth,idempotency,contact,catalog,officialprice,carpool,apimarket,apiintent,profile,announcement}`: module-owned services and repository contracts.
- `backend/internal/module/core/service.go`: compatibility facade that keeps legacy service method names and delegates to module services.
- `backend/internal/module/core/persistence.go`: focused repository aliases and repository aggregation used during the transition.
- `backend/internal/domain/errors.go`: Problem Details error codes and typed app errors.
- `backend/internal/config/config.go`: `PORT`, `APP_ENV`, `DATABASE_URL`, and `ENABLE_DEV_AUTH` parsing.
- `backend/internal/database/postgres.go`: `pgxpool` construction and `schema_migrations` readiness query.
- `backend/internal/store/postgres/*.go`: PostgreSQL repository methods split by domain.
