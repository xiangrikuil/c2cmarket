# Limited API Quota Offer Contract

Date: 2026-07-19
Updated: 2026-08-14
Author: Codex

## Scenario: Fixed, Time-Limited API Quota Offers

### 1. Scope / Trigger

- Trigger: changes to limited API quota batches, offers, sale rounds, inventory units, quota orders, merchant contact snapshots, after-sales eligibility, pre-imported delivery credentials, public offer projections, or their OpenAPI routes.
- This contract is separate from the legacy Sub2API free-amount purchase path. Both reuse `api_services` and `api_orders`, but only limited offers use `purchase_kind='limited_quota_offer'` and authoritative inventory units.
- Primary owners: `internal/module/apiquota`, `internal/store/postgres/api_quota.go`, `internal/server/api_quota_handler.go`, migrations `000054` through `000056` and `000094`, and `docs/openapi/c2c-market-api-v1.yaml`.

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
api_purchase_intent_owner_contact_snapshots
api_orders.quota_expires_at_snapshot
```

### 3. Contracts

- A batch is the seller-declared external USD allowance source. An offer is the buyer-visible fixed USD/CNY product. A sale round limits scheduled copies. Do not collapse these objects into `api_services` or legacy `api_service_packages`.
- `sale_cutoff_at <= expires_at - interval '1 hour'`. No new order may be created at or after either boundary.
- `model_multiplier` must be positive and remains in offers and orders as an immutable pricing snapshot. First-party publication clients derive it from the selected API service's default multiplier and do not expose an offer-level override. The persistence contract remains independent of seller identity and distribution system.
- Publishing locks the batch, validates all planned USD and credential capacity, reserves the full planned USD allowance, creates one inventory row per copy, and activates allocations in one transaction.
- A seller with any active API-order dispute cannot publish or resume a quota batch, create an atomic rush offer, submit/publish/restore the base API service, or receive a new normal/quota order. All write paths return `409 ACTIVE_API_ORDER_DISPUTE` before contact, inventory, intent, order, or publication side effects. Closing every active dispute immediately restores eligibility unless an independent reputation restriction remains.
- Purchase claims one available inventory row with `FOR UPDATE SKIP LOCKED`. A scheduled purchase also inserts the unique `(sale_round_id, buyer_user_id)` claim. The intent, order, snapshots, inventory/credential reservation, events, notifications, and completed idempotency record commit together.
- The same purchase transaction locks every ordered contact configured on the base API service and freezes each immutable merchant contact version on the generated intent. The first frozen contact continues populating legacy single-contact fields; no current profile value is read to repair historical orders.
- Scheduled orders freeze a five-minute payment window. Continuous limited offers and legacy free-amount orders freeze ten minutes.
- Pending-payment cancellation or timeout releases an eligible inventory unit and pre-imported credential. The scheduled buyer claim remains, so the buyer cannot re-enter that round. Payment-submitted and later states do not release inventory.
- Public current/next rounds must be offer-specific: a round is projected only when an active allocation exists for both the round and current offer.
- Public cards expose fixed USD, CNY, derived CNY/USD, multiplier, cutoff, expiry, distribution system, delivery ETA/mode, merchant-declared concurrency, SKU quota policy, and service-level platform health. Current public projections do not expose seller-declared TTFT or `performanceDisclaimer`; historical orders retain the frozen declaration for transaction explanation.
- Every offer owns explicit 5h/daily USD limits. New writes allow only `limited` with a positive amount or `unlimited` without an amount. Order creation freezes the offer policy into dedicated columns and the self-describing JSON snapshot; historical `unspecified` is never inferred as unlimited.
- New offer and rush-offer creation accepts only `deliveryMode=manual`. Supplying
  `preimported` returns a stable field-level validation error before persistence. Existing
  pre-imported offers, credential imports, orderability, reservations, and fulfillment remain
  readable and operational; response enums continue to describe both historical modes.
- Order creation freezes the API service account-pool code/label, merchant-declared maximum concurrency, merchant refund commitment, `api-merchant-refund-v1` rule version, and batch expiry inside both pricing and offer snapshots. Historical nullable pool/concurrency values remain explicit JSON `null` and are never inferred.
- `quota_expires_at_snapshot` is the authoritative validity end for limited-quota after-sales. A first dispute may be opened only while `now < quotaExpiresAtSnapshot + 24h`; completed-order `issueOccurredAt` must be no later than the frozen quota expiry. Reporting grace does not extend the quota, credential, sale, or refund entitlement.
- Historical pre-imported credential CSV templates remain mutually exclusive:
  `api_base_url,api_key,instructions` or `panel_login_url,username,password,instructions`. Import is
  all-or-nothing, at most 5000 rows, encrypted at rest, fingerprint-deduplicated, and never
  returned by public/list/summary/event/log payloads. New publication requests expose no CSV/file
  input.

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
| New standard or rush offer uses `deliveryMode=preimported` | `422 VALIDATION_FAILED`, field `deliveryMode`, reason `new_preimported_not_allowed` |
| Any configured merchant contact is missing, disabled, or no longer owned when buying | `409 MERCHANT_CONTACT_UNAVAILABLE`; inventory, claim, intent, and order writes roll back |
| Seller has any active API-order dispute | `409 ACTIVE_API_ORDER_DISPUTE`; publication/restoration/new-order writes leave no business residue |
| Completed limited-quota report is at or after frozen expiry plus 24 hours | `409 INVALID_STATE_TRANSITION`, reason `after_sales_expired` |
| Completed limited-quota report occurrence is after frozen expiry | `422 VALIDATION_FAILED`, field `issueOccurredAt`, reason `after_validity` |

### 5. Good / Base / Bad Cases

- Good: one round allocates 10 copies of `$50` and 5 copies of `$100`; 15 different buyers can succeed, while one buyer can claim only one of the two offers.
- Good: a limited offer created for a `1.2500` API service persists and freezes `1.2500` without asking the seller to configure the multiplier again.
- Good: a rush order atomically freezes WeChat and linux.do contact versions, and a failure occurring before quota expiry can be reported during the following 24 hours without extending the quota.
- Base: a continuous offer creates a ten-minute order without a sale-round ID.
- Base: a historical pre-imported offer can still import credentials and complete an existing
  pre-imported order; this compatibility does not reopen pre-imported publication.
- Bad: a public `$50` card displays the next round that only allocates `$100` inventory.
- Bad: hiding `preimported` from response enums and making historical orders impossible to explain.
- Bad: cancellation deletes the round claim and lets the same buyer reclaim released stock.
- Bad: raw CSV keys or passwords appear in idempotency caches, public responses, summaries, notifications, or logs.
- Bad: reserve inventory before validating/freeze-locking every merchant contact, re-read a current contact for order detail, or restart the after-sales clock in the browser.

### 6. Tests Required

- Unit: cutoff boundary, positive multiplier and commercial-fact snapshots independent of distribution system, historical explicit-null snapshots, stable standard/rush pre-imported rejection, 1000-copy round input, historical orderability, and CSV headers/duplicates/row limit.
- PostgreSQL: publish rollback, `SKIP LOCKED` inventory claims, offer-specific round projection, release/retire behavior, credential reserve/deliver, idempotent replay, and cross-offer round limit.
- PostgreSQL contact/after-sales integration: all selected contacts freeze in service order inside the purchase transaction; a contact failure leaves inventory and claims untouched; frozen quota expiry controls the exact reporting boundary.
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

The offer freezes the API service's seller-declared default multiplier and only displays rounds that actually allocate that offer.

#### Wrong: Rebuild Limited-Quota Evidence From Mutable State

```text
contacts = currentService.ownerContacts
afterSalesDeadline = browserNow + 24h
```

#### Correct: Use Frozen Intent And Order Facts

```text
contacts = intent.ownerContactSnapshots ordered by sort_order
afterSalesDeadline = order.quotaExpiresAtSnapshot + 24h
```

## Scenario: Fixed Beijing Rush Slots And Atomic Publication

### 1. Scope / Trigger

- Trigger: changes to fixed rush slots, `system_slot_key`, simplified owner publication, slot-filtered public offers, or owner pause/archive behavior for a system slot.
- Reuse the existing batch, offer, round, allocation, inventory, claim, credential, and order tables. A fixed slot is a constrained scheduled round, not a second flash-sale domain.

### 2. Signatures

```text
GET  /api/v1/api-quota-sale-slots
GET  /api/v1/api-quota-offers?slotKey=<YYYY-MM-DD@HH:00>
POST /api/v1/owner/api-services/{id}/quota-rush-offers
POST /api/v1/owner/api-quota-rounds/{id}/confirm-fulfillment

apiquota.SystemSaleSlots/ResolveSystemSaleSlot/ResolveOpenSystemSaleSlot
apiquota.Manager.CreateRushOfferWithIdempotency
postgres.Store.CreateSystemRushOfferWithIdempotency

api_quota_sale_rounds.system_slot_key text NULL
api_quota_sale_rounds.fulfillment_confirmed_at timestamptz NULL
```

The owner create route accepts `multipart/form-data` with one JSON `payload` part. The payload fields are `sourceType`, `sourceLabel`, `name`, `usdAllowance`, `priceCny`, `modelMultiplier`, `copies`, `deliveryMode=manual`, `deliveryEtaMinutes`, `slotKey`, `expiresAt`, and `sourceConfirmedAt`.
First-party clients populate `modelMultiplier` from the selected API service default; it is a required snapshot field, not a second seller-editable setting.

### 3. Contracts

- Fixed sessions use `Asia/Shanghai` at `20:00`, last 30 minutes, and close registration one hour before start. The server returns seven slots covering Beijing today plus six following calendar days.
- A slot key is `YYYY-MM-DD@HH:00`. The server derives `startsAt`, `endsAt`, and `registrationClosesAt`; clients must not submit or derive those timestamps.
- Simplified publication creates exactly one batch, one manual-delivery scheduled offer, one system round, one allocation, and one inventory row per copy in one PostgreSQL transaction. Any failure rolls everything back.
- `sale_cutoff_at` equals the slot end. `expiresAt` must be at least one hour after slot end. Scheduled orders keep the existing five-minute frozen payment window even when it ends after the slot.
- New rush publication rejects `preimported`, credential kind, and file input. Historical system-slot offers that were already pre-imported keep their credential and archive behavior.
- Before registration closes, owner archive retires available inventory and credentials, closes active/planned allocations, returns their USD allowance, cancels the scheduled system round, and archives its offers in the same transaction.
- At or after registration close, owners cannot pause or archive a system-slot batch. Historical rounds with `system_slot_key IS NULL` retain the existing advanced-management behavior.
- A seller may publish at most 10 copies in one system slot across all of their offers. Publication serializes by seller and slot, then sums planned/active allocations in PostgreSQL before writing inventory.
- A system-slot round is orderable only after the seller confirms fulfillment during `[startsAt-30m, startsAt)`. Confirmation rechecks seller eligibility, service/probe/payment readiness, and the published batch. Public projections and the order transaction enforce the same fact; historical custom rounds with `system_slot_key IS NULL` do not require it.
- Global latest-migration validation belongs to `check-migrations-doc.mjs`; quota migration tests assert only the migration files and schema fragments owned by the quota feature.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Slot key is not one of the server-generated seven-day fixed slots | `422 VALIDATION_FAILED`, field `slotKey` |
| Slot registration has closed | `409 INVALID_STATE_TRANSITION`, reason `registration_closed` |
| `copies` is outside `1..10` | `422 VALIDATION_FAILED`, field `copies` |
| Existing planned/active copies plus requested copies exceed 10 for the seller and slot | `409 VALIDATION_FAILED`, field `copies`, reason `slot_limit` |
| Expiry is earlier than slot end plus one hour | `422 VALIDATION_FAILED`, field `expiresAt` |
| `deliveryMode=preimported` | `422 VALIDATION_FAILED`, field `deliveryMode`, reason `new_preimported_not_allowed` |
| Request includes a credential kind or CSV part | `422 VALIDATION_FAILED` as an unknown/retired creation field |
| Multipart has duplicate/unknown parts or exceeds the size limit | `422 VALIDATION_FAILED` or `413` |
| Owner pauses or archives at/after registration close | `409 INVALID_STATE_TRANSITION` |
| Seller confirms before `startsAt-30m` or at/after `startsAt` | `409 INVALID_STATE_TRANSITION` |
| System round has no fulfillment confirmation | Public response uses `fulfillment_confirmation_required`; order creation returns `409 INVALID_STATE_TRANSITION` |

### 5. Good / Base / Bad Cases

- Good: before Beijing `19:00`, a seller publishes two manually delivered `$50` copies into the `20:00` slot; both inventory rows become available atomically, then the seller confirms fulfillment at `19:30`.
- Good: before `19:00`, archive retires both copies and credentials, returns `$100`, cancels the round, and removes the offer from public sale.
- Base: an old scheduled round with a null system key remains manageable through the advanced owner flow.
- Bad: a client submits a non-20:00 slot, provides its own end time, publishes 11 copies, or publishes after the one-hour registration cutoff.
- Bad: the frontend hides an unconfirmed round while the public query or order transaction still permits purchase.
- Bad: archive changes only the batch status while leaving available inventory, credentials, or allocated allowance behind.

### 6. Tests Required

- Unit: fixed 20:00 hour, Beijing day rollover, registration-close/start/end boundaries, invalid keys, seven-day range, expiry minimum, 10-copy limit, confirmation window, stable pre-imported rejection, and idempotent replay.
- HTTP/OpenAPI: slot list, `slotKey` filter, strict payload-only multipart parsing, retired file/kind rejection, `no-store`, and route parity.
- PostgreSQL: atomic publication rollback, concurrent seller/slot copy aggregation, confirmation readiness recheck, unconfirmed-order rejection, and successful archive cleanup asserting inventory, credentials, allocation allowance, round, offer, and batch state.
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

## Scenario: API Order Deadlines, Pending Capacity, And Late Payment

### 1. Scope / Trigger

- Trigger: changes to API-order creation, payment submission/confirmation, delivery, timeout cancellation, late-payment recovery, flexible/limited channel publication, or their API/UI projections.
- PostgreSQL time and frozen order facts are authoritative. Overdue flags are projections and do not add a primary order status.

### 2. Signatures

```text
POST /api/v1/me/api-orders/{id}/report-late-payment
POST /api/v1/owner/api-orders/{id}/resolve-late-payment

api_orders.merchant_confirm_due_at timestamptz NULL
api_orders.delivery_due_at timestamptz NULL
api_orders.late_payment_status reported|not_received|received_refund_pending NULL
api_orders.late_payment_reported_at/late_payment_resolved_at timestamptz NULL
api_quota_sale_rounds.fulfillment_confirmed_at timestamptz NULL
```

### 3. Contracts

- An on-time payment submission freezes `merchant_confirm_due_at = payment_submitted_at + 10 minutes`. Seller payment confirmation freezes `delivery_due_at = paid_confirmed_at + delivery ETA snapshot`; limited quota uses its frozen 1-10 minute ETA and other API orders use 10 minutes.
- Boundaries use `[start,end)`: at the due timestamp the projection is overdue. Paid/payment-submitted orders are never auto-cancelled and never release inventory because a seller deadline elapsed.
- Normal first delivery is rejected at or after the frozen absolute quota expiry. A reporting grace period never extends product validity.
- Order creation takes a buyer advisory transaction lock before inventory reservation. A buyer may have no more than one `pending_payment` order for the same product and no more than three pending API orders globally across ordinary and limited-quota flows.
- Only a `payment_timeout` cancellation may be reported, and only while `now < cancelled_at + 24h`. A report and its seller resolution are independent facts: they never revive the order, inventory unit, credential reservation, or round claim.
- Seller resolution accepts only `not_received` or `received_refund_pending`. It records an event but does not claim that an off-platform refund has completed.
- A flexible-quota service stops accepting orders when `quota_expires_at <= now + 24h`. Flexible quota and published/paused, unexpired limited quota are mutually exclusive channels for one API service.
- New dispute requests reject `continue_fulfillment`; historical disputes containing it remain readable.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Buyer already has a pending order for the product | `409 INVALID_STATE_TRANSITION` |
| Buyer already has three pending API orders | `409 INVALID_STATE_TRANSITION` |
| Delivery occurs at/after frozen quota expiry | `409 INVALID_STATE_TRANSITION` |
| Late-payment report is not for `payment_timeout`, is repeated, or reaches the 24-hour boundary | `409 INVALID_STATE_TRANSITION` |
| Seller resolution status is absent, `reported`, or unknown | `422 VALIDATION_FAILED`, field `status` |
| Flexible quota has 24 hours or less remaining | Service is not orderable; order transaction rejects the stale attempt |
| Publishing/resuming one channel while the other is sellable | `409 INVALID_STATE_TRANSITION` |
| New dispute requests `continue_fulfillment` | `422 VALIDATION_FAILED`, field `requestedResolution` |

### 5. Good / Base / Bad Cases

- Good: a buyer submits payment at `10:00`, the seller confirms at `10:05`, and a five-minute limited-quota ETA freezes delivery due at `10:10`.
- Good: a timed-out buyer reports at `cancelledAt+23:59:59`; the seller records `received_refund_pending`, while the original inventory remains with its current owner.
- Base: a historical order has null deadline and late-payment fields and remains readable without fabricated overdue facts.
- Bad: restore a timed-out order or stock because the buyer reports an off-platform transfer.
- Bad: release inventory when the seller misses a confirmation/delivery deadline, or calculate authority from browser time.

### 6. Tests Required

- Unit: exact merchant/delivery deadline boundaries, exact 24-hour report boundary, allowed seller resolutions, and historical null projections.
- PostgreSQL: both order creation paths enforce same-product/global pending limits under concurrent requests; deadline writes, late-payment facts, idempotency completion, inventory non-resurrection, channel mutual exclusion, and expiry delivery rejection commit atomically.
- HTTP/OpenAPI: both late-payment routes, version/idempotency headers, generated type drift, response projections, and rejection of `continue_fulfillment` on new disputes.
- Frontend: deadline/overdue display, persistent transfer warning, report/resolve dialogs, historical dispute label, and no client-time authorization.

### 7. Wrong vs Correct

#### Wrong

```text
if browserNow > paymentSubmittedAt + 10m: releaseInventory()
if buyerReportsTransfer: reopenOrderAndReserveStock()
```

#### Correct

```text
merchantConfirmOverdue = serverNow >= merchantConfirmDueAt
latePaymentReport -> append fact/event only; keep order, inventory, credential, and round claim unchanged
```

Deadlines explain seller performance; they do not rewrite the settled inventory state machine.

## Scenario: Owner API Service Sales Lifecycle Projection

### 1. Scope / Trigger

- Trigger: changes to the owner API-service list, limited-offer expiry visibility, sales-state filters, `salesSummary`, `healthSummary`, or the frontend service-selection workflow.
- `APIService` remains the long-lived integration and merchant resource. A limited quota batch or offer expiring must not delete, archive, or relabel the base service as the package itself.
- Primary owners: `internal/module/apimarket`, `internal/store/postgres/api_market_owner_sales.go`, `internal/server/api_market_handler.go`, the owner OpenAPI list schema, and `frontend/src/pages/MyApiServicesPage.vue`.

### 2. Signatures

```text
GET /api/v1/owner/api-services?salesView=active|expired|paused|draft|all

apimarket.Manager.OwnerServices(ctx, user, OwnerServiceFilter, PageRequest)
postgres.Store.ListAPIServicesByOwner(ctx, ownerUserID, OwnerServiceFilter, PageRequest)

OwnerAPIServiceListItem = APIService + required salesSummary + required healthSummary
salesSummary.overallState =
  selling|upcoming|paused|sold_out|expired|draft|offline|archived
salesSummary.channels[].kind = flexible_quota|limited_quota
healthSummary = ServiceHealthSummary
```

### 3. Contracts

- Missing `salesView` defaults to `active`. `active` contains `selling` and `upcoming`; `draft` contains `draft` and `offline`; `all` preserves every state.
- The server filters before keyset pagination. PostgreSQL must derive channels and `overallState` in the same LATERAL/CTE projection used by `WHERE`; the frontend must not filter a partially loaded page or request batches/offers once per service.
- One service may expose both channels in historical/owner projections, but only one may currently be sellable: opening flexible quota requires no published/paused unexpired limited batch, and publishing limited quota requires flexible ordering to be closed. Overall priority is `selling > upcoming > paused > sold_out > expired > draft > offline > archived`.
- A limited channel uses batch, offer, allocation, round, inventory, credential, cutoff, and expiry facts. It must not derive state from `APIService.billingMode`.
- The owner list response requires both `salesSummary` and `healthSummary`. The handler loads health summaries once for the page's deduplicated service IDs; the frontend must not issue one private probe-config request per row.
- Missing probe configuration returns `no_sample/unconfigured`. A health-summary dependency failure is fail-open for the service list and returns `no_sample/temporarily_unavailable` with 12 no-sample slots; it must not omit the field or block service management.
- Owner detail and administrator service responses keep their existing schemas. Public service responses retain their separate public health projection and must not expose the owner-only `salesSummary` read model or private probe configuration.
- Normal `/my/api-services` starts with `active`. `/my/api-services?intent=quota` starts with `all` so expired, offline, and otherwise reusable base services remain selectable.
- An expired limited channel keeps a republish action. Historical batches, offers, orders, and the base service remain queryable.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing `salesView` | Normalize to `active` |
| Unknown `salesView` | `422 VALIDATION_FAILED`, field `salesView` |
| Current or future published sale exists | `selling` or `upcoming`; include in `active` |
| Cutoff or expiry reached with no higher-priority channel | `expired`; exclude from `active`, include in `expired` and `all` |
| Service or selected sale plan is paused | `paused`; include in `paused` and `all` |
| Service is reviewing, draft, or offline | `draft` or `offline`; include in `draft` and `all` |
| Limited sale expired while flexible quota sells | Preserve both historical/current channels; overall remains `selling` |
| Flexible and unexpired limited channels would both be sellable | Reject the write before projection; never publish this owner state |
| Probe is not configured | Required `healthSummary` with `no_sample/unconfigured` |
| Health-summary loading fails | Return the service page with `no_sample/temporarily_unavailable` for each item |

### 5. Good / Base / Bad Cases

- Good: one service shows `自由额度 / 销售中` and `限时额度包 / 已过期` together, stays in the active view, and requires closing flexible ordering before limited-package republishing.
- Good: a scheduled offer between valid rounds remains `upcoming` with `nextStartsAt` and stays on the default page.
- Base: a service with no sales channel uses its service lifecycle fallback and remains reachable through `draft` or `all`.
- Base: an unconfigured probe remains manageable and shows `未配置` from the required owner-list health summary.
- Good: two owner services trigger one batched summary load and both receive their matching health projection.
- Bad: deleting the base `APIService` when the last batch expires.
- Bad: returning `salesSummary` on the public API-service list or calculating expiry from browser time.
- Bad: loading owner services, then issuing one batch/offer request per row and filtering only those loaded rows.
- Bad: displaying `状态未知` in real mode because the owner list omitted `healthSummary`, or fetching private probe configuration once per service card.

### 6. Tests Required

- Domain: filter normalization, complete state priority, exact expiry boundaries, no-channel fallback, and two-channel coexistence.
- PostgreSQL: multiple services/batches/offers/rounds, current and future inventory, cutoff transition, no duplicate service rows, and stable cursor/limit behavior. Run against a dedicated database through `C2C_TEST_DATABASE_URL`; a missing variable must be reported as a skip, not as executed coverage.
- HTTP/OpenAPI: default `active`, explicit `all`, invalid filter Problem Details, required sales and health summaries, one batched health load, unconfigured and temporarily-unavailable fallbacks, and no public/admin/detail leakage.
- Frontend: generated-type drift, required adapter mapping, query key includes `salesView`, mock/real parity, all status labels, `intent=quota`, and expired republish action.
- Browser: `1440x900` and `390x844`; assert desktop buttons/mobile Select, no page or table overflow, no overlap, and no console warning/error.

### 7. Wrong vs Correct

#### Wrong

```ts
const expired = new Date(service.expiresAt) <= new Date()
const services = (await getOwnerServices()).filter(service => !expired)
for (const service of services) {
  service.offers = await getQuotaOffers(service.id)
}
```

#### Correct

```sql
SELECT api_services.*, sales.overall_state, sales.channels
FROM api_services
JOIN LATERAL (/* authoritative channel aggregation */) sales ON true
WHERE api_services.owner_user_id = $1
  AND ($2 = 'all' OR ($2 = 'active' AND sales.overall_state IN ('selling', 'upcoming')))
ORDER BY api_services.updated_at DESC, api_services.id DESC
LIMIT $3;
```

```go
summaries := server.loadAPIHealthSummaries(ctx, apiServiceIDs(page.Items))
items := toOwnerAPIServiceListItemResponses(page.Items, summaries)
```

The list filter and pagination use one authoritative sales projection, health enrichment is one
fail-open batch per page, and the UI only maps the returned states to labels and actions.
