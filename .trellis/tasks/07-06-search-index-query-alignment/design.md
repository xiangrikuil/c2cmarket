# Search index query alignment design

## Scope

This task keeps the global search feature shape intact. It changes only the
PostgreSQL search predicates and developer verification material needed to align
those predicates with trigram expression indexes.

## Boundaries

- `backend/internal/store/postgres/search.go` remains the PostgreSQL repository
  implementation for global search aggregation.
- `backend/internal/module/search` service validation, result DTOs, handler
  routes, and OpenAPI response shape stay unchanged.
- Visibility predicates remain owned by the existing public list predicates:
  active official prices, active carpools, active demands, API services matching
  `publicAPIServiceOrderablePredicate`, active users, and public-profile API
  merchants.
- Store-alias API services may match public merchant display names but must not
  match or expose the hidden owner username or create a separate merchant result
  for that store alias.

## Query Alignment

- Replace separate field-level `LOWER(field) ILIKE` checks with grouped
  expression checks that match the existing GIN trigram indexes, for example:
  `lower(title || ' ' || summary || ' ' || access_arrangement)`.
- Keep additional unindexed predicates only where they preserve existing public
  behavior and there is no current trigram index for that field group.
- Add a narrow forward migration for merchant profile search so the index matches
  the current display-name-only public search contract instead of slug plus
  display name.
- For API service model matching, use an `EXISTS` predicate over
  `api_service_models` with the indexed
  `lower(model_name_snapshot || ' ' || provider_snapshot)` expression instead of
  filtering only on a lateral aggregate string.

## Verification Script

Add `scripts/explain-search.sql` as a psql-oriented developer script. It should:

- Set a sample pattern once.
- Run representative `EXPLAIN` statements for the global search predicates.
- Include comments naming the expected indexes.
- Use `SET enable_seqscan = off` only to make local verification deterministic;
  production planner settings remain unchanged.

## Compatibility

The runtime SQL changes are read-only. The only migration change is a forward
index-expression alignment for merchant profiles; it must update
`ExpectedMigrationVersion` and migration docs with the same commit.
