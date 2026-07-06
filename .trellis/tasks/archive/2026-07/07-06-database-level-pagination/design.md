# Database-level pagination design

## Boundary

- Backend-only pagination migration for the seven Phase 6 prioritized repository paths.
- No route path changes, no OpenAPI schema changes, no frontend changes, and no search pagination changes.
- Existing offset-based `paginateSlice` remains for list endpoints outside this child.

## Current Shape

- Handlers call service methods that return `[]T`.
- Handlers call `writePaginatedJSON`, which decodes an offset cursor and slices in memory.
- PostgreSQL repositories sort rows but load the full result set.

## Target Shape

- Add shared backend pagination contracts in `internal/domain`:

```go
type PageRequest struct {
	Limit  int
	Cursor string
}

type Page[T any] struct {
	Items      []T
	NextCursor *string
}
```

- Add `parsePageRequest` / `writePageJSON` helpers in `internal/server` for database-backed pages.
- Add PostgreSQL keyset cursor helpers in `internal/store/postgres`:
  - decode base64url JSON cursor into `(time, id)`
  - encode the last returned item key as the next cursor
  - return `422 VALIDATION_FAILED` for invalid cursor input
- Use `LIMIT $n` with `limit+1`; return only the first `limit` rows and produce `nextCursor` only when the extra row exists.

## Ordering

- Carpool listings, API services, reports, and feedback tickets: preserve existing `updated_at DESC` ordering and add `id DESC` as the deterministic tie-breaker.
- Notifications: preserve existing `created_at DESC` ordering and add `id DESC` as the deterministic tie-breaker.
- Cursor predicate for descending order:

```sql
AND (sort_time, id) < ($cursor_time, $cursor_id::uuid)
```

## Compatibility

- HTTP response shape remains `listResponse[T]` with `items` and `nextCursor`.
- Query parameters remain `limit` and `cursor`.
- Existing cursors are opaque and may change internal encoding; clients are not allowed to inspect them.
- `MarkAllNotificationsRead` may still need all currently visible notifications for its existing response. That path can explicitly request the maximum page size or keep an internal full-list helper if preserving semantics requires it.

## Rollback

- Restore affected repository/service method signatures to return `[]T`.
- Revert handlers to `writePaginatedJSON`.
- Remove keyset cursor helpers and tests if unused.
