# Public API Order Number Contract

Date: 2026-08-02
Executor: Codex

## Scenario: Immutable Commercial API Order Numbers

### 1. Scope / Trigger

- Trigger: work that creates, migrates, reads, searches, displays, or notifies about API orders.
- The public number is a customer-service and transaction reference. It is not a database identity, route key, authorization token, or sequence counter.

### 2. Signatures

```go
const OrderNumberAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
const OrderNumberSuffixLength = 10

func apiorder.GenerateOrderNo(createdAt time.Time) (string, error)
```

```sql
api_orders.order_no text NOT NULL
CONSTRAINT ck_api_orders_order_no_format
  CHECK (order_no ~ '^API-[0-9]{8}-[ABCDEFGHJKMNPQRSTUVWXYZ23456789]{10}$')
CONSTRAINT ux_api_orders_order_no UNIQUE (order_no)
TRIGGER trg_api_orders_order_no_immutable
```

```text
HTTP response: orderNo: string (required)
HTTP create request: no orderNo field
Internal identity and route parameter: api_orders.id UUID
Public format: API-YYYYMMDD-XXXXXXXXXX
Example: API-20260802-K7M4P9Q2XZ
```

```ts
function matchesApiOrderSearch(query: string, values: readonly string[]): boolean
```

### 3. Contracts

- The date segment is the order `created_at` day in `Asia/Shanghai`, not UTC and not the migration execution date.
- New numbers use `crypto/rand` with rejection sampling over the 31-character alphabet. Exclude ambiguous `0/O` and `1/I/L` characters.
- Both normal API-service orders and limited-quota-offer orders use the same generator and insertion helper.
- Insertions retry at most eight times, and only when `ON CONFLICT ON CONSTRAINT ux_api_orders_order_no DO NOTHING` reports an `order_no` collision. Intent uniqueness and every other database error retain their existing error mapping.
- Migration 75 backfills historical rows deterministically from the row UUID, original creation date, and a bounded collision attempt. Rollback and re-apply must reproduce the same values before constraints are restored.
- `api_orders.id` remains the primary key, foreign-key target, permission lookup, mutation target, and frontend route parameter. `orderNo` is display and search data only.
- `orderNo` is required in the domain model, HTTP DTO, OpenAPI schema, generated frontend type, real-backend adapter, and frontend `ApiOrder` model.
- Buyer lists, merchant lists, order details, administrator summaries, and order notifications show the complete public number. A `ShortId` component may provide copy behavior but must use `full=true`; a parent container must wrap instead of truncating the number on narrow screens.
- UI search matches ordinary text case-insensitively and matches order numbers after removing non-alphanumeric characters, so `api20260802k7m4p9q2xz` finds `API-20260802-K7M4P9Q2XZ`.
- Legacy Mock orders receive a matching number once and immediately persist the migrated record to session storage. A refresh must not assign another public number.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Generated suffix contains an ambiguous or unsupported character | Reject through generator/migration tests; the database format constraint blocks the row. |
| Random source fails | Return an internal creation error and roll back the order transaction. |
| Candidate collides on `ux_api_orders_order_no` | Generate another candidate and retry, up to eight attempts. |
| Insert fails on `ux_api_orders_intent` | Return `409 API_PURCHASE_INTENT_HAS_ORDER`; do not treat it as a number collision. |
| Eight number collisions occur | Return an internal error and roll back all order side effects. |
| Code tries to update `order_no` | PostgreSQL raises `23514` through `ck_api_orders_order_no_immutable`. |
| Client sends `orderNo` on create | The request contract does not accept or use the value. |
| Search omits hyphens or uses lowercase | Match the same order in buyer, merchant, and administrator views. |
| Narrow screen cannot fit service title and number on one line | Wrap the number to a new line without clipping it or creating page-level horizontal overflow. |

### 5. Good / Base / Bad Cases

- Good: an order created at `2026-08-01T16:30:00Z` receives an `API-20260802-...` number because Shanghai is already on the next day; the buyer sees and copies the full value while the route remains `/my/api-orders/<uuid>`.
- Base: a historical order is backfilled once, rollback/re-apply produces the same number, and existing events and relations still reference its UUID.
- Bad: expose `API-444D72`, derive the public value from a visible UUID suffix, use a sequential daily counter, or navigate by `orderNo`.

### 6. Tests Required

- Generator unit tests: exact Shanghai date, alphabet, rejection sampling, random-source failure, format, and sample uniqueness.
- PostgreSQL insertion tests: first-candidate collision retry, retry exhaustion, and non-number errors preserved.
- Migration tests: column/constraint/trigger contract plus real PostgreSQL `74 -> 75 -> 74 -> 75`, format, uniqueness, Shanghai dates, deterministic hash, and immutable-update rejection.
- Projection tests: domain to HTTP DTO, OpenAPI generated type, administrator summary, and notification text.
- Frontend tests: real adapter mapping, Mock creation and legacy persistence, full display/copy surfaces, normalized search, and UUID routes.
- Browser acceptance at desktop and `390x844`: buyer list, buyer detail, and administrator list show the complete number; no horizontal overflow.
- Required commands: `go test ./...`, `go vet ./...`, `pnpm --dir frontend test`, `pnpm --dir frontend typecheck`, OpenAPI route/type drift checks, migration documentation check, and real-mode production build.

### 7. Wrong vs Correct

#### Wrong

```go
order.OrderNo = "API-" + order.ID.String()[len(order.ID.String())-6:]
```

```sql
INSERT INTO api_orders (...) VALUES (...);
-- Catch every 23505 and retry as though the public number collided.
```

#### Correct

```go
orderNo, err := apiorder.GenerateOrderNo(order.CreatedAt)
if err != nil {
    return err
}
order.OrderNo = orderNo
```

```sql
INSERT INTO api_orders (..., order_no)
VALUES (..., $41)
ON CONFLICT ON CONSTRAINT ux_api_orders_order_no DO NOTHING;
```
## Participant Identity Projection

- API purchase-intent and order read models resolve the current `users.username` alongside participant UUIDs; usernames are presentation data and never replace UUID authorization, routing, filtering, or audit keys.
- Buyer views expose the merchant username, merchant views expose the buyer username, and administrator views expose both usernames while retaining full copyable user IDs.
- Frontend participant labels prefer `@username` and fall back to the existing shortened UUID only when a username is unavailable. Pages must not issue per-row profile requests.
