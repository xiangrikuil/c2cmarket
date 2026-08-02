# API Service Promotion Contract

Date: 2026-08-02
Author: Codex

## Scenario: Administrator-operated API market promotion pool

### 1. Scope / Trigger

- `internal/module/apipromotion` owns administrator-created promotion schedules, dynamic status, placement capacity, and stop facts.
- API service review, publication, inventory, payment configuration, orderability, reputation, and badges remain owned by their existing modules.
- Promotion schedules use an independent table, while the buyer surface injects one matching campaign into the existing free-quota or fixed-package grid. Promotions never change the backend natural ordering or recommendation score and do not add merchant routes, payment fields, or analytics tables.

### 2. Signatures

```text
GET  /api/v1/api-service-promotions
GET  /api/v1/admin/api-service-promotions
GET  /api/v1/admin/api-service-promotions/availability
POST /api/v1/admin/api-service-promotions
POST /api/v1/admin/api-service-promotions/{id}/stop
```

Create and stop are administrator-only. They require `Idempotency-Key`; stop also requires `If-Match`. UUIDs are validated before persistence, and create/stop reasons are trimmed, required, and limited to 500 Unicode code points.

### 3. Contracts

#### Eligibility And Status

- Configuration hard blocks are non-approved or administratively unavailable services, non-active owners, and current seller/all restrictions for `api_service_publish` or `all`.
- Ordinary unresolved disputes are administrator warnings, not automatic merchant-fault decisions or hard blocks.
- Displayability additionally uses the public orderable predicate: online, accepting orders, valid payment settings/window, and available non-expired metered allowance or enabled fixed-package stock.
- Dynamic status order is `stopped`, `scheduled`, `finished`, `suppressed`, then `serving`. A suppressed campaign resumes automatically when displayability returns before its end.
- A request captures one timestamp and uses it for schedule status, restrictions, quota expiry, and displayability decisions.

#### Capacity And Concurrency

- `api_market_top` has capacity 3 and schedules use half-open ranges `[starts_at, ends_at)`.
- Capacity is the peak number of existing non-stopped campaigns simultaneously active anywhere inside the proposed range. It is not the number of campaigns that merely intersect the range; staggered non-concurrent schedules do not accumulate.
- Creation takes a placement-scoped PostgreSQL transaction advisory lock before eligibility, peak capacity, same-service overlap, and insertion checks.
- The same service cannot have overlapping non-stopped administrator campaigns or used reward-activation intervals. Administrator availability and create both query the two sources; adjacent ranges where one ends exactly when the next starts are valid.

#### Atomicity And Projection

- Create and stop commit the promotion row, administrator audit row, and completed idempotency response in one PostgreSQL transaction.
- Replays return the original response and restore the resource `ETag`; a retry must not add promotion or audit rows.
- Public reads return only currently serving campaigns and at most three items. The deterministic daily ordering uses campaign ID plus the Asia/Shanghai date with the campaign ID as a tie-breaker.
- Public promotion DTOs embed `PublicAPIService` and must not expose administrator IDs, create/stop reasons, owner IDs, contact fields, or future promotion-payment fields.

#### API Service Commercial Facts

- New or revised services require exactly one account pool: `gpt_pro_20x`, `gpt_pro_5x`, `gpt_plus`, or `custom`. A custom public label is 2-40 Unicode code points; preset pools reject a custom label.
- Historical services keep database `NULL` pool and performance values. Public DTOs omit those values, and new snapshots encode explicit JSON `null`; neither backend nor frontend may infer a pool or concurrency from title, model, or distribution system.
- Storage and API names use `declared_max_concurrency` and `declaredMaxConcurrency`. Public copy identifies it as merchant-declared and states that the platform has not measured it. The old `recommendedConcurrency` key is allowed only at a tested historical JSON read boundary, never on new writes.
- `merchantRefundCommitment` is an explicit seller boolean. Enabled commitments use fixed version `api-merchant-refund-v1`; the server owns the wording and the platform does not escrow, fund, execute, or guarantee the refund.
- API intent pricing snapshots and limited-quota offer/order snapshots freeze pool code/label, declared maximum concurrency, commitment boolean, rule version, and the applicable expiry fact. Later service edits never rewrite an existing snapshot.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Non-administrator calls an administrator endpoint | `403 PERMISSION_DENIED` |
| Invalid UUID, placement, time range, or reason longer than 500 code points | `422 VALIDATION_FAILED` |
| Service or promotion does not exist | `404 OBJECT_NOT_FOUND` |
| Hard eligibility failure, same-service overlap, peak capacity full, or an ended/stopped campaign | `409 INVALID_STATE_TRANSITION` |
| Stop version differs from `If-Match` | `412 VERSION_CONFLICT` |
| Stop omits `If-Match` | `428 PRECONDITION_REQUIRED` |
| Storage fails | Problem Details failure; never empty authoritative success |

### 5. Good / Base / Bad Cases

- Good: three schedules run sequentially; a fourth range spanning all three sees peak occupancy 1 and can be inserted because its resulting peak is 2.
- Good: a schedule becomes `suppressed` when stock reaches zero and resumes automatically if stock returns before `ends_at`.
- Good: an active reward activation makes same-service administrator availability report overlap and makes transactional create reject the range without consuming administrator capacity.
- Base: no serving schedules returns an empty public list and preserves the natural market unchanged.
- Base: a historical service with no pool or performance declaration returns no fabricated public value and freezes explicit JSON `null` values in a new order snapshot.
- Bad: reject a proposal because three staggered rows intersect it even though they are never concurrent.
- Bad: add `is_featured`, payment, compensation, badge, or ranking fields to `api_services`.
- Bad: trust the availability preview without transactionally repeating eligibility, capacity, and overlap checks during create.

### 6. Tests Required

Run `go test ./...`, `go vet ./...`, the OpenAPI route/type drift checks, the migration documentation check, and the PostgreSQL integration suite when `C2C_TEST_DATABASE_URL` is available. Regression coverage must include boundary status, eligibility, concurrent capacity, staggered peak capacity, same-service overlap, public privacy, audit de-duplication, and replayed `ETag`.

### 7. Wrong vs Correct

#### Wrong

```sql
SELECT count(*)
FROM api_service_promotions
WHERE starts_at < :ends_at AND ends_at > :starts_at;
```

Using this total as capacity rejects staggered schedules that intersect the proposal at different times.

#### Correct

```text
candidate points = proposed starts_at + each overlapping campaign starts_at
peak = max(active campaigns at every candidate point)
allow create only when peak < 3, then insert under the placement transaction lock
```
