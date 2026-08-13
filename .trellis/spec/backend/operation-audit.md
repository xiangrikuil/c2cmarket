# Unified Administrator Operation Audit

Date: 2026-08-12
Author: Codex

## Scenario: Allowlisted Projection Of Authoritative Operation Facts

### 1. Scope / Trigger

- Trigger: adding a business mutation/event, changing an audit/access table, extending `GET /api/v1/admin/audit-logs`, changing administrator audit filters, or introducing retention/cleanup for a covered source.
- Owners: domain-specific repositories, `internal/module/operationaudit`, `internal/store/postgres/operation_audit.go`, migration `000092`, OpenAPI administrator audit schemas, and the `/admin/logs` frontend.
- Goal: preserve one authoritative fact per successful operation and safely project heterogeneous histories without a second global write table or arbitrary JSON exposure.

### 2. Signatures

```http
GET /api/v1/admin/audit-logs
  ?sourceKind=&domain=&action=&actorKind=&actorUserId=
  &targetType=&targetId=&outcome=&from=&to=&search=&limit=&cursor=
```

```go
type Repository interface {
    ListOperationAudit(ctx context.Context, query operationaudit.Query) ([]operationaudit.Entry, *domain.AppError)
}

type CursorPosition struct {
    OccurredAt time.Time
    SourceKind string
    EventID    string
}
```

```text
sourceKind:
  admin | moderation | domain | api_order |
  contact_session_access | api_intent_contact_access |
  api_order_access | probe

actorKind: user | admin | system
outcome: succeeded | status_changed | accessed
```

### 3. Contracts

- Domain-specific tables remain authoritative: `admin_audit_logs`, `moderation_audit_logs`, approved `domain_events`, `api_order_events`, three dedicated contact/payment access logs, and `api_probe_connection_events`. There is no second global audit-write table.
- Every covered business command commits its mutation, exactly one approved event/audit fact, and idempotency completion in the same PostgreSQL transaction. Failed validation, capability denial, version conflict, rollback, and replay do not create another success fact.
- Network calls and email/webhook side effects run outside the database transaction. A replayed completed command returns its stored completion without repeating the external effect.
- `ActionRegistry` is the only display allowlist. A database event appears only when `(sourceKind, action, targetType)` is registered with a fixed domain, label, outcome, summary, allowed actor kinds, and optional server-owned detail template.
- Dual-written governance facts have one display authority. For example, user account/administrator-permission changes use `admin_audit_logs`; matching compatibility `domain_events` are excluded to prevent duplicate rows.
- Response entries contain only IDs, safe current actor username, fixed registry text, sanitized request ID, timestamps, and a server-constructed relative detail path. Arbitrary metadata, before/after/reason/note, request bodies, contacts, email, payment instructions, credentials/fingerprints, delivery, evidence, cookies, session/CSRF/OAuth/Turnstile tokens, and provider errors never cross the reader.
- Deleted actor/target rows do not erase the event. Missing joins return the fact with absent display data. Detail paths are generated only from fixed templates and valid UUID target IDs; database URLs are ignored.
- Pagination is descending and stable on `(created_at, source_kind, source_event_id)`. The cursor is opaque, versioned, and bound to that tuple. Every UNION branch applies source-relevant time/cursor/filter conditions before `LIMIT + 1`.
- Default query window is 30 days; maximum is 90 days; default limit is 20 and maximum 100. Search is at most 100 bytes and operates only over explicitly safe columns.
- The API and frontend require `admin.access`. Real API failures never fall back to session storage or mock entries. The UI states that retention differs by source and does not promise permanent completeness.
- Authentication, Turnstile, CSRF, capability, and rate-limit failures belong to bounded structured runtime telemetry, not the persistent business audit reader.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Anonymous or missing `admin.access` | `401`/`403 CAPABILITY_REQUIRED`; no audit query |
| Unknown source/domain/actor/outcome | `422 VALIDATION_FAILED` on the offending filter |
| Action/target type is absent from or incompatible with the registry filters | `422 VALIDATION_FAILED` |
| Invalid UUID actor/target, RFC3339 time, limit, or cursor | `422 VALIDATION_FAILED` |
| `from > to` or range exceeds 90 days | `422 VALIDATION_FAILED` |
| Unknown database event action/target tuple | Omit it; never project raw fallback text/metadata |
| Actor kind is invalid for the registered source/action | Omit the corrupt row |
| Duplicate admin/domain representation exists | Return only the registry-designated authority |
| Same completed command key is replayed | Return the stored completion; event count remains one |
| Mutation/event/completion insert fails | Roll back all three; reader returns no success entry |

### 5. Good / Base / Bad Cases

- Good: a seller creates a probe; one connection row, one safe probe event, and one completed idempotency entry commit. Replaying the key returns the same response and does not call the provider again.
- Good: an administrator changes a user's admin grant; compatibility domain notification facts may still exist, but the unified reader shows one authoritative admin entry.
- Base: an event's actor was deleted; the reader still returns actor kind/ID and fixed summary with an empty current username.
- Base: a new domain event is deployed before its registry entry; it remains stored for its domain consumers but is invisible in the unified admin reader.
- Bad: copy every event into `operation_logs`, creating two truths with independent transaction/retention behavior.
- Bad: serialize `metadata_json`, governance reason, contact values, credentials, or request bodies into the response and rely on the frontend to hide fields.
- Bad: use offset pagination over a multi-source UNION or construct a detail URL from a database string.

### 6. Tests Required

- Registry unit tests assert every allowed tuple is unique, has fixed safe text/domain/outcome, uses a valid actor matrix/detail template, and disallows unknown prefix-based discovery.
- Repository tests assert every UNION branch has allowlist/filter/time/cursor predicates, deterministic tuple ordering, `LIMIT + 1`, safe joins, and no raw metadata/value columns in the projection.
- PostgreSQL integration tests insert mixed sources at identical timestamps and page without duplicate/omission; combine filters; preserve deleted actors/targets; and prove dual-written governance returns once.
- Every covered mutation has success, fault-injection, and replay assertions over mutation/event/idempotency row counts. External-work tests assert rollback sends nothing and completed replay schedules nothing again.
- Privacy golden scans serialize representative entries and reject contact/payment/credential/delivery/evidence/request/session/token fields and arbitrary database JSON.
- Handler/OpenAPI/generated/frontend tests assert `admin.access`, exact enums/DTO fields, stable validation, no legacy before/after/reason fields, shared recent-operation projection, no real-to-mock fallback, and responsive filter/pagination behavior.
- Performance verification uses representative mixed data and `EXPLAIN (ANALYZE, BUFFERS)` to confirm source indexes plus bounded time/cursor/limit behavior.

### 7. Wrong vs Correct

#### Wrong

```sql
SELECT event_type AS action, metadata_json AS details
FROM domain_events
ORDER BY created_at DESC;
```

This auto-publishes unknown actions and arbitrary sensitive metadata, cannot resolve duplicate authorities, and has unstable same-timestamp pagination.

#### Correct

```text
domain-specific transaction
  -> mutation + approved authoritative event + idempotency completion

allowlisted UNION reader
  -> fixed labels/summaries + safe IDs/time + composite cursor
```

Keep writers domain-owned, require an explicit registry tuple before display, and make the backend projection the final privacy boundary.
