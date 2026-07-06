# Backend service boundary cleanup

## Goal

Start Phase 5 backend service boundary cleanup with one independently verifiable domain slice: carpool transport dependencies. The change should stop carpool handlers from depending on the giant `server.Service` method set, document `core.Service` as a legacy compatibility facade, and create a repeatable pattern for future domain-specific service interfaces.

## Requirements

- Introduce a carpool-specific server-side service interface that contains only the carpool listing/application/membership methods used by `backend/internal/server/carpool_handler.go`.
- Keep `backend/internal/server.NewServer` source-compatible for existing callers by accepting a single application service value, while internally storing carpool behavior behind the new domain-specific field.
- Remove carpool listing/application/membership methods from the giant `backend/internal/server.Service` interface once the handlers use the new carpool field.
- Update `backend/internal/server/carpool_handler.go` to call the carpool-specific dependency instead of `s.app` for carpool domain operations.
- Add a short comment on `backend/internal/module/core.Service` explaining that it is a legacy compatibility facade over domain services and should not be expanded for new behavior.
- Preserve existing route paths, request/response DTOs, OpenAPI behavior, idempotency behavior, and handler pagination behavior in this child task.
- Do not migrate API-market, database-level pagination, repository contracts, OpenAPI files, or smoke scripts in this child task.

## Acceptance Criteria

- [x] `server.Service` no longer declares carpool listing/application/membership handler methods.
- [x] A carpool-specific interface exists in the server package and `Server` stores it separately from the remaining legacy app facade.
- [x] `backend/internal/server/carpool_handler.go` uses the carpool-specific dependency for carpool domain operations.
- [x] `backend/internal/module/core.Service` has an explicit legacy facade comment.
- [x] No route registration paths or OpenAPI files change.
- [x] `go test ./...` passes for `backend`.

## Notes

- Source requirement: `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, Phase 5.
- The selected migrated domain is `carpool` because Phase 5.2 explicitly names carpool/API-market as the preferred first domain, and carpool already has a dedicated `internal/module/carpool.Service`.
- Current evidence: `backend/internal/server/server.go` defines a giant `Service` interface; `backend/internal/server/carpool_handler.go` calls `s.app` for 24 carpool listing/application/membership operations.
- Validation evidence: Docker `gofmt` completed, Docker `go test ./...` passed in `backend`, source scans passed, and `git diff --check` passed.
