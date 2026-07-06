# Database-level pagination implementation plan

## Checklist

- [x] Add shared `domain.PageRequest` and `domain.Page[T]`.
- [x] Add server helpers for parsing page request query params and writing already-paginated pages.
- [x] Add PostgreSQL keyset cursor encode/decode/page helpers with focused tests.
- [x] Migrate carpool public/admin listing repository, service, core facade, server interface, and handlers.
- [x] Migrate API-market owner/admin service list repository, service, core facade, server interface, and handlers.
- [x] Migrate notifications list repository, service, core facade, server interface, and handler while preserving mark-all behavior.
- [x] Migrate admin reports list repository, service, core facade, server interface, and handler.
- [x] Migrate feedback tickets by submitter repository/service and any caller path.
- [x] Run source scans proving the selected handlers no longer use `writePaginatedJSON` after full-list repository calls.
- [x] Run `gofmt` on touched Go files.
- [x] Run Docker `go test ./...` in `backend`.
- [x] Run `git diff --check`.

## Validation Commands

```bash
rg -n "writePaginatedJSON\\(w, r, (toCarpoolListingResponses|toAPIServices|toNotification|toReport|toFeedback)" backend/internal/server
rg -n "ListPublicCarpoolListings\\(ctx context.Context\\)|ListAdminCarpoolListings\\(ctx context.Context\\)|ListAPIServicesByOwner\\(ctx context.Context, ownerUserID string\\)|ListAdminAPIServices\\(ctx context.Context\\)|ListNotifications\\(ctx context.Context, userID string\\)|ListAdminReports\\(ctx context.Context\\)|ListFeedbackTicketsBySubmitter\\(ctx context.Context, submitterUserID string\\)" backend/internal
docker run --rm -v /Users/lixinjian/Crypto/c2cmarket:/work -w /work/backend golang:1.26-alpine go test ./...
git diff --check
```

## Validation Results

- Docker `gofmt` completed for touched Go files.
- Docker `go test ./...` passed in `backend`.
- `git diff --check` passed.
- Source scans confirmed the selected repository methods no longer use the old unbounded signatures.
- Targeted handlers now parse a page request and write already-paginated `domain.Page` responses.
- OpenAPI cursor parameter wording now describes the cursor as opaque without exposing offset/keyset internals.
- Backend API/database specs now record the repository-level `domain.PageRequest` / `domain.Page[T]` pagination convention.

## Risk Points

- Do not change route names, response DTOs, auth, CSRF, or idempotency behavior.
- Keep cursor values opaque; avoid documenting internal cursor payload as an API contract.
- Some existing in-memory tests and fake services may need signature updates.
- Preserve list ordering exactly except for adding `id DESC` as a deterministic tie-breaker.
