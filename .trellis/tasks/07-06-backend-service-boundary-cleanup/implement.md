# Backend service boundary cleanup implementation plan

## Checklist

- [x] Add a `CarpoolService` interface in `backend/internal/server/server.go` with the carpool listing/application/membership methods currently used by `carpool_handler.go`.
- [x] Add a constructor-facing aggregate interface so `NewServer` still receives one application service value.
- [x] Add `carpools CarpoolService` to `Server` and assign it in `NewServer`.
- [x] Remove carpool listing/application/membership methods from the legacy `Service` interface.
- [x] Update `backend/internal/server/carpool_handler.go` to call `s.carpools` for carpool operations.
- [x] Add a legacy facade comment above `core.Service`.
- [x] Run source scans confirming carpool handler no longer calls `s.app` for carpool methods and `server.Service` no longer lists carpool handler methods.
- [x] Run `gofmt` on touched Go files.
- [x] Run `go test ./...` in `backend`.
- [x] Run `git diff --check`.

## Validation Commands

```bash
rg -n "s\\.app\\.(CreateCarpool|PublishCarpool|UpdateCarpool|SubmitCarpool|PublicCarpool|MyCarpool|AdminCarpool|CreateCarpoolApplication|AcceptCarpool|RejectCarpool|CancelCarpool|WithdrawCarpool|ConfirmCarpool|OwnerCarpool|EndCarpool)" backend/internal/server
rg -n "CreateCarpoolListing|PublicCarpoolListings|MyCarpoolApplications|OwnerCarpoolMemberships|EndCarpoolMembershipWithIdempotency" backend/internal/server/server.go
gofmt -w backend/internal/server/server.go backend/internal/server/carpool_handler.go backend/internal/module/core/service.go
go test ./...
git diff --check
```

## Validation Results

- Docker `gofmt` completed for `backend/internal/server/server.go`, `backend/internal/server/carpool_handler.go`, and `backend/internal/module/core/service.go`.
- Docker `go test ./...` passed in `backend`.
- `rg` source scans confirmed `carpool_handler.go` no longer calls `s.app` for carpool operations and `server.Service` no longer owns the migrated carpool method set.
- `git diff --check` passed.
- Backend directory-structure spec now records the focused server-side service interface pattern for future handler migrations.

## Risk Points

- `NewServer` is used heavily in backend tests; keep the constructor shape as a single argument plus options.
- Do not remove carpool forwarding methods from `core.Service` in this child because review/favorite/search resolver contracts still depend on facade methods.
- Do not change pagination behavior in carpool list handlers; that belongs to the database pagination child task.
