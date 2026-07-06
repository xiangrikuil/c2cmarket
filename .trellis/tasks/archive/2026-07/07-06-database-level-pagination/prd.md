# Database-level pagination

## Goal

Complete Phase 6 database-level pagination for the prioritized backend list endpoints. The change should stop the selected PostgreSQL repositories from loading all rows before server-side slicing, while preserving existing HTTP response shape `{ items, nextCursor }` and public route compatibility.

## Requirements

- Migrate the prioritized list paths from repository-wide `[]T` loading plus `server.paginateSlice` to repository-level `limit/cursor` pagination:
  - `ListPublicCarpoolListings`
  - `ListAdminCarpoolListings`
  - `ListAPIServicesByOwner`
  - `ListAdminAPIServices`
  - `ListNotifications`
  - `ListAdminReports`
  - `ListFeedbackTicketsBySubmitter`
- Introduce a shared backend page contract with a request limit, opaque cursor, returned items, and optional next cursor.
- Use PostgreSQL `LIMIT page_size + 1` for migrated repository queries.
- Use stable keyset ordering matching the existing list sort: `updated_at/id` for resources currently sorted by update time and `created_at/id` for notifications.
- Keep HTTP route paths, query parameter names, and response body shape compatible with the current OpenAPI list contract.
- Keep cursors opaque to clients; clients must pass `nextCursor` back without inspecting it.
- Preserve in-memory pagination helpers for endpoints not migrated in this child.
- Do not migrate search, frontend adapters, or unrelated repository lists in this child task.

## Acceptance Criteria

- [x] Migrated repository interfaces accept a page request instead of returning unbounded lists.
- [x] Migrated PostgreSQL queries use keyset filters when a cursor is present and `LIMIT limit+1`.
- [x] Migrated handlers call repository-backed pagination and no longer call `writePaginatedJSON` for the selected endpoints.
- [x] The response JSON remains `{ items, nextCursor }` for selected HTTP routes.
- [x] Tests cover first page, next cursor, last page, invalid cursor, and stable keyset cursor encoding/decoding.
- [x] `go test ./...` passes for `backend`.

## Notes

- Source requirement: `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, Phase 6.
- Current evidence: `backend/internal/server/pagination.go` paginates slices with offset cursors, while the prioritized repositories return unbounded `[]T` lists.
- Validation evidence: Docker `go test ./...` passed in `backend`, `git diff --check` passed, and source scans confirmed old unbounded prioritized repository signatures are gone.
- OpenAPI cursor parameter wording was updated without changing route paths, parameters, or response schemas.
