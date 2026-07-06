# Search index query alignment

## Goal

Align PostgreSQL trigram search indexes with the global search SQL predicates so
the database can use the existing expression indexes for public-safe search
fields, and document a repeatable EXPLAIN workflow for future maintenance.

## Confirmed Facts

- The source maintenance prompt defines this as Phase 7 and requires matching
  `pg_trgm` expression indexes to the actual `internal/store/postgres/search.go`
  SQL.
- `backend/migrations/000024_search_trigram_indexes.up.sql` creates GIN trigram
  expression indexes for API services/models, carpool listings, demands,
  product plans, API model catalog/providers, users, linux.do bindings, and
  merchant profiles.
- `backend/internal/store/postgres/search.go` currently searches many of those
  fields with separate `OR LOWER(column) ILIKE` predicates instead of equivalent
  indexed expressions.
- `backend/migrations/000036_search_trigram_alignment.up.sql` now narrows the
  merchant-profile trigram expression to display-name-only search to match the
  public store-alias API service contract.
- Search is public, read-only, and must keep the existing visibility predicates
  and response DTOs.

## Requirements

- Update global search SQL predicates so indexed public-safe text groups use
  expressions equivalent to the trigram indexes where those indexes support the
  current search path.
- Preserve existing result types, sort/order behavior, visibility filters, and
  sensitive-data exclusions for public search.
- Do not add a search table, broaden public visibility, expand
  `backend/internal/server.Service`, or introduce production mock fallback.
- Avoid broad index churn. Add or change an index only if it directly supports a
  current search predicate and is documented.
- Add `scripts/explain-search.sql` or equivalent developer documentation showing
  how to verify the relevant trigram indexes with PostgreSQL `EXPLAIN`.
- Keep backend tests passing.

## Acceptance Criteria

- [x] Search SQL uses expression predicates matching the current trigram indexes
      for carpool listings, demands, API services, API service models, product
      plans, users, linux.do bindings, and merchant profile display/search text
      where those tables participate in global search.
- [x] Search behavior remains public-safe: no contact values, owner contact IDs,
      hidden store-alias owner usernames, admin internals, credential-looking
      fields, payment, escrow, guarantee, or fulfillment material are returned.
- [x] `scripts/explain-search.sql` documents a practical psql workflow for
      checking index usage.
- [x] `cd backend && go test ./...` passes.

## Notes

- Source requirement:
  `/Users/lixinjian/Downloads/c2cmarket-codex-maintenance-prompt.md`, Phase 7.
