# Search index query alignment implementation plan

## Checklist

- [x] Update `backend/internal/store/postgres/search.go` predicates to use
      existing trigram expression-index shapes.
- [x] Add a forward migration for the merchant-profile display-name trigram
      expression required by the public search contract.
- [x] Add `scripts/explain-search.sql` with representative EXPLAIN checks and
      expected index comments.
- [x] Update backend database/search guidelines if the verification workflow is
      worth preserving for future agents.
- [x] Run formatting if Go code needs it.
- [x] Run `cd backend && go test ./...`.
- [x] Run `git diff --check`.
- [x] Mark this task and the parent roadmap acceptance item complete after
      validation.

## Validation Commands

```bash
cd backend && go test ./...
git diff --check
```

Optional local PostgreSQL check after migrations are applied:

```bash
psql "$DATABASE_URL" -f scripts/explain-search.sql
```

## Risk Points

- Search predicates must not weaken public visibility constraints.
- API service store-alias matching must not expose owner usernames.
- Trigram expression matching is sensitive to SQL expression shape, so avoid
  helper abstractions that obscure the final SQL text.
