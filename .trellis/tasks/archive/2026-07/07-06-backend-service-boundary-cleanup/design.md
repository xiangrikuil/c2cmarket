# Backend service boundary cleanup design

## Boundary

- Backend-only transport/service-boundary cleanup.
- First migrated domain slice: carpool listing/application/membership handlers.
- No route, OpenAPI, database, repository, pagination, or frontend changes.
- `core.Service` remains the concrete application facade for now so existing tests and app wiring continue to compile.

## Current Shape

- `backend/internal/server/server.go` defines one large `Service` interface used by `Server.app`.
- `backend/internal/server/carpool_handler.go` calls `s.app` for 24 carpool methods.
- `backend/internal/module/core.Service` forwards those methods to `internal/module/carpool.Service` and also implements other domain resolver interfaces.

## Target Shape

- Split a `carpoolService` interface inside the server package.
- Add an internal constructor-facing aggregate, for example:

```go
type applicationService interface {
  Service
  carpoolService
}
```

- Keep `NewServer(service applicationService, options ...ServerOptions)` as a single-service constructor from the caller's perspective.
- Store the aggregate as:

```go
type Server struct {
  app      Service
  carpools carpoolService
}
```

- Remove carpool listing/application/membership methods from `Service` and put them in `carpoolService`.
- Change `carpool_handler.go` call sites from `s.app.<CarpoolMethod>` to `s.carpools.<CarpoolMethod>`.

## Compatibility

- `*core.Service` already implements all carpool methods, so existing `NewServer(core.NewService(...))` calls keep compiling.
- Non-carpool handlers keep using `s.app` unchanged.
- Review submission remains on the review service boundary in this child; it depends on carpool memberships internally but is owned by `review_handler.go`.
- `core.Service` retains its carpool forwarding methods because other domain services use it as an internal resolver and because a broader removal would be a separate migration.

## Rollback

- Restore carpool methods to `Service`.
- Remove `carpoolService` and `Server.carpools`.
- Change `s.carpools` call sites back to `s.app`.
