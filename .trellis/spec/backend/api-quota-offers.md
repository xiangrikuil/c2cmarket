# Limited API Quota Offer Contract

Date: 2026-07-19
Author: Codex

## Scenario: Fixed, Time-Limited API Quota Offers

### 1. Scope / Trigger

- Trigger: changes to limited API quota batches, offers, sale rounds, inventory units, quota orders, pre-imported delivery credentials, public offer projections, or their OpenAPI routes.
- This contract is separate from the legacy Sub2API free-amount purchase path. Both reuse `api_services` and `api_orders`, but only limited offers use `purchase_kind='limited_quota_offer'` and authoritative inventory units.
- Primary owners: `internal/module/apiquota`, `internal/store/postgres/api_quota.go`, `internal/server/api_quota_handler.go`, migrations `000051` and `000052`, and `docs/openapi/c2c-market-api-v1.yaml`.

### 2. Signatures

```text
GET  /api/v1/api-quota-offers
GET  /api/v1/api-quota-offers/{id}
POST /api/v1/api-quota-offers/{id}/orders

GET/POST /api/v1/owner/api-services/{id}/quota-batches
GET/POST /api/v1/owner/api-quota-batches/{id}/offers
GET/POST /api/v1/owner/api-quota-batches/{id}/rounds
POST     /api/v1/owner/api-quota-batches/{id}/{publish|pause|resume|archive}
POST     /api/v1/owner/api-quota-offers/{id}/credentials/import
GET      /api/v1/owner/api-quota-offers/{id}/credentials/summary
```

```text
apiquota.Manager.CreateBatch/CreateOffer/CreateRound/PublishBatch
apiquota.Manager.CreateOrderWithIdempotency
postgres.Store.CreateAPIQuotaOrderWithIdempotency

api_quota_batches
api_quota_offers
api_quota_sale_rounds
api_quota_allocations
api_quota_inventory_units
api_quota_round_claims
api_quota_credentials
```

### 3. Contracts

- A batch is the seller-declared external USD allowance source. An offer is the buyer-visible fixed USD/CNY product. A sale round limits scheduled copies. Do not collapse these objects into `api_services` or legacy `api_service_packages`.
- `sale_cutoff_at <= expires_at - interval '1 hour'`. No new order may be created at or after either boundary.
- `model_multiplier` must be positive and defaults to `1.0000`, but it is independent of seller identity and distribution system. Sub2API limited offers may declare another positive multiplier. Legacy Sub2API free-amount services retain their existing fixed-one rule.
- Publishing locks the batch, validates all planned USD and credential capacity, reserves the full planned USD allowance, creates one inventory row per copy, and activates allocations in one transaction.
- Purchase claims one available inventory row with `FOR UPDATE SKIP LOCKED`. A scheduled purchase also inserts the unique `(sale_round_id, buyer_user_id)` claim. The intent, order, snapshots, inventory/credential reservation, events, notifications, and completed idempotency record commit together.
- Scheduled orders freeze a five-minute payment window. Continuous limited offers and legacy free-amount orders freeze ten minutes.
- Pending-payment cancellation or timeout releases an eligible inventory unit and pre-imported credential. The scheduled buyer claim remains, so the buyer cannot re-enter that round. Payment-submitted and later states do not release inventory.
- Public current/next rounds must be offer-specific: a round is projected only when an active allocation exists for both the round and current offer.
- Public cards and orders expose fixed USD, CNY, derived CNY/USD, multiplier, cutoff, expiry, distribution system, delivery ETA/mode, and seller-declared TTFT/concurrency confirmation. `performanceDisclaimer` remains `商户自报，平台未测速`.
- Credential CSV templates are mutually exclusive: `api_base_url,api_key,instructions` or `panel_login_url,username,password,instructions`. Import is all-or-nothing, at most 5000 rows, encrypted at rest, fingerprint-deduplicated, and never returned by public/list/summary/event/log payloads.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Cutoff later than expiry minus one hour | `422 VALIDATION_FAILED` on `saleCutoffAt` |
| Non-positive USD, CNY, or multiplier | `422 VALIDATION_FAILED` on the field |
| Scheduled order before start | `409 API_QUOTA_NOT_STARTED` |
| Round absent, closed, or ended | `409 API_QUOTA_ROUND_ENDED` |
| No matching available inventory unit | `409 API_QUOTA_SOLD_OUT` |
| Same buyer retries any offer in the same round | `409 API_QUOTA_BUYER_ROUND_LIMIT` |
| Batch lacks declared USD at publish | `409 API_QUOTA_INSUFFICIENT_ALLOWANCE` |
| Pre-imported credential capacity is insufficient | `409 API_QUOTA_CREDENTIAL_UNAVAILABLE` |
| Batch cutoff/expiry reached | `409 API_QUOTA_BATCH_EXPIRED` |
| Same idempotency key with another body | Existing `IDEMPOTENCY_KEY_REUSED` contract |

### 5. Good / Base / Bad Cases

- Good: one round allocates 10 copies of `$50` and 5 copies of `$100`; 15 different buyers can succeed, while one buyer can claim only one of the two offers.
- Good: a Sub2API limited offer defaults to `1.0000` but persists and freezes `1.2500` when the seller declares it.
- Base: a continuous offer creates a ten-minute order without a sale-round ID.
- Bad: a public `$50` card displays the next round that only allocates `$100` inventory.
- Bad: cancellation deletes the round claim and lets the same buyer reclaim released stock.
- Bad: raw CSV keys or passwords appear in idempotency caches, public responses, summaries, notifications, or logs.

### 6. Tests Required

- Unit: cutoff boundary, positive multiplier independent of distribution system, 1000-copy round input, orderability, CSV headers/duplicates/row limit.
- PostgreSQL: publish rollback, `SKIP LOCKED` inventory claims, offer-specific round projection, release/retire behavior, credential reserve/deliver, idempotent replay, and cross-offer round limit.
- Capacity: at least 1500 different buyers compete for 1000 copies; assert exactly 1000 successes, 500 expected sold-out results, no duplicate orders/credentials, no negative inventory, and no unexpected 5xx.
- HTTP/OpenAPI: route parity, Problem Details codes, five/ten-minute snapshots, private `no-store` behavior, and no raw credential leakage.
- Required commands: `go test ./...`, `node scripts/check-openapi-routes.mjs`, and `node scripts/check-migrations-doc.mjs`.

### 7. Wrong vs Correct

#### Wrong

```sql
CHECK (distribution_system <> 'sub2api' OR model_multiplier = 1.0000)

SELECT * FROM api_quota_sale_rounds
WHERE batch_id = current_batch
ORDER BY starts_at LIMIT 1;
```

This couples a new product contract to the legacy service rule and can project another offer's round.

#### Correct

```sql
CHECK (model_multiplier > 0)

SELECT * FROM api_quota_sale_rounds r
WHERE r.batch_id = current_batch
  AND EXISTS (
    SELECT 1 FROM api_quota_allocations a
    WHERE a.sale_round_id = r.id
      AND a.offer_id = current_offer
      AND a.status = 'active'
  );
```

The offer freezes the seller-declared multiplier and only displays rounds that actually allocate that offer.

## Scenario: Fixed Beijing Rush Slots And Atomic Publication

### 1. Scope / Trigger

- Trigger: changes to fixed rush slots, `system_slot_key`, simplified owner publication, slot-filtered public offers, or owner pause/archive behavior for a system slot.
- Reuse the existing batch, offer, round, allocation, inventory, claim, credential, and order tables. A fixed slot is a constrained scheduled round, not a second flash-sale domain.

### 2. Signatures

```text
GET  /api/v1/api-quota-sale-slots
GET  /api/v1/api-quota-offers?slotKey=<YYYY-MM-DD@HH:00>
POST /api/v1/owner/api-services/{id}/quota-rush-offers

apiquota.SystemSaleSlots/ResolveSystemSaleSlot/ResolveOpenSystemSaleSlot
apiquota.Manager.CreateRushOfferWithIdempotency
postgres.Store.CreateSystemRushOfferWithIdempotency

api_quota_sale_rounds.system_slot_key text NULL
```

The owner create route accepts `multipart/form-data` with one JSON `payload` part and an optional `file` part. The payload fields are `sourceType`, `sourceLabel`, `name`, `usdAllowance`, `priceCny`, `modelMultiplier`, `copies`, `deliveryMode`, `deliveryEtaMinutes`, `slotKey`, `expiresAt`, `sourceConfirmedAt`, and optional `deliveryKind`.

### 3. Contracts

- Fixed sessions use `Asia/Shanghai` at `09:00`, `13:00`, and `20:00`, last 30 minutes, and close registration one hour before start. The server returns 21 slots covering Beijing today plus six following calendar days.
- A slot key is `YYYY-MM-DD@HH:00`. The server derives `startsAt`, `endsAt`, and `registrationClosesAt`; clients must not submit or derive those timestamps.
- Simplified publication creates exactly one batch, one scheduled offer, one system round, one allocation, optional credential rows, and one inventory row per copy in one PostgreSQL transaction. Any failure rolls everything back.
- `sale_cutoff_at` equals the slot end. `expiresAt` must be at least one hour after slot end. Scheduled orders keep the existing five-minute frozen payment window even when it ends after the slot.
- Pre-imported delivery requires one normalized credential row per copy before publication. Manual delivery rejects a credential kind or file.
- Before registration closes, owner archive retires available inventory and credentials, closes active/planned allocations, returns their USD allowance, cancels the scheduled system round, and archives its offers in the same transaction.
- At or after registration close, owners cannot pause or archive a system-slot batch. Historical rounds with `system_slot_key IS NULL` retain the existing advanced-management behavior.
- Global latest-migration validation belongs to `check-migrations-doc.mjs`; quota migration tests assert only the migration files and schema fragments owned by the quota feature.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Slot key is not one of the server-generated seven-day fixed slots | `422 VALIDATION_FAILED`, field `slotKey` |
| Slot registration has closed | `409 INVALID_STATE_TRANSITION`, reason `registration_closed` |
| `copies` is outside `1..5000` | `422 VALIDATION_FAILED`, field `copies` |
| Expiry is earlier than slot end plus one hour | `422 VALIDATION_FAILED`, field `expiresAt` |
| Pre-imported delivery has no file or fewer rows than copies | `422 VALIDATION_FAILED`, field `file` |
| Manual delivery includes a CSV | `422 VALIDATION_FAILED`, field `file` |
| Multipart has duplicate/unknown parts or exceeds the size limit | `422 VALIDATION_FAILED` or `413` |
| Owner pauses or archives at/after registration close | `409 INVALID_STATE_TRANSITION` |

### 5. Good / Base / Bad Cases

- Good: at Beijing `07:59:59`, a seller publishes two `$50` copies into the `09:00` slot; both inventory rows and two pre-imported credentials become available atomically.
- Good: before `08:00`, archive retires both copies and credentials, returns `$100`, cancels the round, and removes the offer from public sale.
- Base: an old scheduled round with a null system key remains manageable through the advanced owner flow.
- Bad: a client submits `10:15`, provides its own end time, or publishes after the one-hour registration cutoff.
- Bad: archive changes only the batch status while leaving available inventory, credentials, or allocated allowance behind.

### 6. Tests Required

- Unit: fixed hours, Beijing day rollover, registration-close/start/end boundaries, invalid keys, seven-day range, expiry minimum, copy limit, credential count, and idempotent replay.
- HTTP/OpenAPI: slot list, `slotKey` filter, strict multipart parsing, credential non-leakage, `no-store`, and route parity.
- PostgreSQL: atomic publication rollback and successful archive cleanup asserting inventory, credentials, allocation allowance, round, offer, and batch state.
- Required commands: `go test ./...`, `node scripts/check-openapi-routes.mjs`, and `node scripts/check-migrations-doc.mjs`.

### 7. Wrong vs Correct

#### Wrong

```go
startsAt, _ := time.Parse(time.RFC3339, request.StartsAt)
createBatch()
createOffer()
createRound()
publishBatch()
```

#### Correct

```go
slot, appErr := ResolveOpenSystemSaleSlot(input.SlotKey, now)
if appErr != nil {
    return appErr
}
return repo.CreateSystemRushOfferWithIdempotency(ctx, entry, publicationFrom(slot), credentials, now, buildCompletion)
```

Only the server resolves a fixed slot, and the repository commits the complete publication or no publication.
