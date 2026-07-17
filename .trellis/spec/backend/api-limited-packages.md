# Limited API Package Contract

> Cross-layer contract for fixed-price, fixed-duration API quota packages. This file is the shared source of truth for backend, OpenAPI, persistence, frontend adapters, recommendation logic, and marketplace UI.

---

## Scenario: Publish, Recommend, And Fulfill Limited API Packages

### 1. Scope / Trigger

- Trigger: any change to `fixed_package` API services, package publishing, package-model associations, package inventory, purchase-intent/order snapshots, package expiry, or package recommendation UI.
- One API service has exactly one billing mode. Existing `metered_usd_quota` behavior remains independent from `fixed_package`.
- A marketplace row/card represents one package, not one service or merchant.
- Limited packages remain inside `/api-market`; publishing remains inside `/api-market/new`. Do not add a separate top-level marketplace route.
- Package value uses merchant-declared multipliers. UI copy must label the estimate `按商家声明估算`; the platform does not claim to verify the multiplier.

### 2. Signatures

```text
POST  /api/v1/owner/api-services
PATCH /api/v1/owner/api-services/{id}
GET   /api/v1/api-services
GET   /api/v1/api-services/{id}
POST  /api/v1/api-services/{id}/purchase-intents
POST  /api/v1/me/api-purchase-intents/{id}/orders
POST  /api/v1/me/api-orders/{id}/cancel
POST  /api/v1/owner/api-orders/{id}/confirm-payment
POST  /api/v1/owner/api-orders/{id}/submit-delivery

Frontend routes:
  /api-market?panel=packages
  /api-market/{serviceId}?package={packageId}
  /api-market/new
```

```text
api_service_packages:
  panel_allowance  numeric(18,6) NOT NULL
  stock_total      integer NOT NULL
  stock_available  integer NOT NULL

api_service_package_models:
  PRIMARY KEY (api_service_package_id, api_service_model_id)
  FOREIGN KEY (api_service_id, api_service_package_id)
  FOREIGN KEY (api_service_id, api_service_model_id)

api_orders:
  package_stock_reserved boolean NOT NULL DEFAULT false
  package_expires_at     timestamptz NULL
```

`api_service_models.merchant_multiplier` is a positive decimal string. It is not forced to `1.0000`; values such as `0.0100`, `1.0000`, and `1.2000` are valid.

### 3. Contracts

#### Publish Request And Stable Updates

`APIServiceRequest.billingMode` is `fixed_package`. The request includes service-level `models[]` and one or more `packages[]`.

```text
models[]:
  modelCatalogId: string UUID, required and unique
  modelPriceVersionId: string UUID, optional
  merchantMultiplier: positive DecimalString, default 1.0000
  enabled: boolean

packages[]:
  id: string UUID, update-only and optional
  name: non-empty string
  priceCny: positive DecimalString
  panelAllowance: positive DecimalString
  durationDays: 1 | 3 | 7 | 30
  stockTotal: integer >= 0
  description: non-empty, non-secret string
  enabled: boolean
  sortOrder: integer
  modelCatalogIds: non-empty unique subset of enabled service models
```

- Creation ignores client-supplied package IDs and allocates server IDs.
- Update preserves IDs for retained models and packages. A non-empty package ID must already belong to the current service.
- Packages omitted during update are disabled, not deleted, so intents/orders can still resolve their references.
- Updating total stock preserves reserved and consumed units:

```text
newStockAvailable = oldStockAvailable + newStockTotal - oldStockTotal
```

  Reject the update when the result is negative.
- Package-model rows are reconciled inside the same service transaction after stable service-model and package reconciliation.

#### Public Response And Recommendation

`PublicAPIService` exposes `packages`, merchant identity fields (`merchantDisplayName`, `merchantProfileSlug`, `merchantAvatarUrl`), `completed30d`, `unresolvedDisputes`, `responseMedianMinutes`, and `updatedAt`.

```text
APIServicePackage:
  id, name, priceCny, panelAllowance, durationDays
  stockTotal, stockAvailable, description, enabled, sortOrder
  models[]

APIServicePackageModel:
  serviceModelId, modelCatalogId, modelPriceVersionId
  modelNameSnapshot, providerSnapshot, merchantMultiplier
```

- Exact model names and versions are displayed from snapshots, including names such as `GPT-5.5` and `GPT-5.6`.
- Package results are not shown until both an exact `modelCatalogId` and a duration in `1 | 3 | 7 | 30` are selected.
- Candidates must be publicly orderable `fixed_package` services with an enabled, in-stock package matching the exact model and duration.
- Declared unit cost for the selected model is:

```text
declaredUnitCost = priceCny * merchantMultiplier / panelAllowance
```

- Comprehensive recommendation is the only ranking mode:

```text
value       = 100 * bestDeclaredUnitCost / declaredUnitCost
fulfillment = 100 * (completed30d + 2) / (completed30d + unresolvedDisputes + 4)
response    = 50 when responseMedianMinutes is null,
              otherwise 100 * 60 / (60 + max(0, responseMedianMinutes))
freshness   = 100 * exp(-max(0, ageDays) / 30)
score       = 0.60 * value + 0.25 * fulfillment + 0.10 * response + 0.05 * freshness
```

- Sort by descending score, lower declared unit cost, higher available stock, newer service update, then package ID.
- Cards use two columns on desktop and one on mobile, maintain stable dimensions, and show no more than three model chips plus `+N`.
- Package cards consume the same `merchantAvatarUrl` projection as other API-market cards. They render an image when present and the merchant/store-name initial only when absent.
- Opening a card navigates to `/api-market/{serviceId}?package={packageId}`. Detail preselects that valid package and fixes the intent/order amount to its CNY price.

#### Inventory, Snapshots, And Expiry

- Intent creation freezes `selectedPackageSnapshot` with package ID, name, price, panel allowance, duration, description, enabled/sort order, and every package model's service-model ID, catalog ID, price-version ID, model name/provider snapshots, and merchant multiplier.
- Order creation copies the intent snapshot. Later package/model edits must not reprice or rewrite existing intents/orders.
- Order creation atomically reserves one unit with the order insert, intent transition, events, notifications, and idempotency completion:

```sql
UPDATE api_service_packages
SET stock_available = stock_available - 1
WHERE id = $1
  AND api_service_id = $2
  AND enabled = true
  AND stock_available > 0;
```

- A newly created fixed-package order sets `package_stock_reserved=true`.
- Buyer cancellation or payment timeout releases exactly one unit only while `package_stock_reserved=true`, then clears the flag in the same transaction.
- Payment confirmation clears the flag without increasing stock. Delivery, completion, and later disputes never restore the unit.
- Delivery submission reads the frozen duration from `selectedPackageSnapshot` and sets:

```text
packageExpiresAt = deliverySubmittedAt + durationDays calendar days
```

  Package validity never starts at intent creation, order creation, or payment time.

### 4. Validation & Error Matrix

| Condition | HTTP / result | Stable field/code |
| --- | --- | --- |
| `fixed_package` has no packages or no service models | 422 | `VALIDATION_FAILED`, `packages` or `models` required |
| Price, panel allowance, or multiplier is non-positive/invalid | 422 | `VALIDATION_FAILED`, matching nested field `invalid` |
| Duration is not 1, 3, 7, or 30 | 422 | `VALIDATION_FAILED`, `packages.N.durationDays` invalid |
| Stock total is negative | 422 | `VALIDATION_FAILED`, `packages.N.stockTotal` invalid |
| Package model subset is empty | 422 | `VALIDATION_FAILED`, `packages.N.modelCatalogIds` required |
| Package references a disabled/unselected model | 422 | `VALIDATION_FAILED`, nested model ID invalid |
| Duplicate model, package ID, or package-model ID | 422 | `VALIDATION_FAILED`, matching field `duplicate` |
| Update supplies an unknown/foreign package ID | 422 | `VALIDATION_FAILED`, `packages` / `invalid_id` |
| New stock total is below already reserved/consumed units | 422 | `VALIDATION_FAILED`, `packages` / `stock_below_committed` |
| Intent selects a missing, disabled, or sold-out package | 422 | `VALIDATION_FAILED`, `selectedPackageId` invalid |
| Concurrent order loses the final-stock reservation race | 409 | `INVALID_STATE_TRANSITION`; refresh/retry message |
| Delivery snapshot is missing/invalid for a fixed package | 409 | `INVALID_STATE_TRANSITION`; delivery is rejected |
| Package expiry is set without fixed-package delivery | Database rejection | `ck_api_orders_package_expiry` |
| Reserved flag is true outside an unpaid fixed-package state | Database rejection | `ck_api_orders_package_stock_reservation` |

### 5. Good/Base/Bad Cases

- Good: a merchant publishes 1-, 3-, 7-, and 30-day packages, enables exact models, and uses `0.0100` for a relay multiplier while a self-hosted listing keeps `1.0000`.
- Good: two buyers race for the last unit; exactly one order commits and the other receives a stock conflict.
- Good: a 3-day package is delivered at time T; its frozen expiry is T plus 3 calendar days even if the merchant edits or disables the package later.
- Base: a package with total 12 and available 8 is edited to total 10; available becomes 6, preserving four reserved/consumed units.
- Base: missing response history receives neutral response score 50; no fabricated response time is persisted or returned.
- Bad: recreate every package on edit, changing IDs and breaking existing intent/order references.
- Bad: restore package stock after payment confirmation, delivery, completion, or dispute.
- Bad: rank different models or durations together, or present the declared multiplier as platform-verified value.

### 6. Tests Required

- Migration checks: version 51 is documented; package stock/allowance/duration constraints and package-model ownership foreign keys exist; non-1 multipliers such as `1.2000` insert successfully.
- API-market domain tests: allowed durations, positive decimals, exact model subsets, duplicate/foreign IDs, stable package/model IDs, disabled omissions, and stock-delta rejection.
- Intent tests: package availability and a full immutable snapshot containing exact model name, multiplier, and model price-version ID.
- Order tests: last-unit reservation, cancellation and timeout release exactly once, payment-confirmation consumption, no later release, and delivery-based expiry from the frozen snapshot.
- PostgreSQL integration: reservation/update/release occurs in the order transaction and cannot oversell under competing writes.
- OpenAPI/router checks: publish/public response fields, snapshot/order lifecycle fields, route parity, and strict YAML parsing.
- Frontend unit tests: adapter mapping, mock lifecycle parity, exact model/duration filtering, all score components, deterministic tie breakers, and sold-out exclusion.
- Merchant projection tests: both `public_profile` and `store_alias` preserve their correct avatar boundary through storage, API response, frontend adapter, and shared card component.
- Frontend gates: `vue-tsc`, Vitest, real-backend production build, plus desktop/mobile browser checks for package cards, query preselection, fixed order amount, publish controls, and viewport overflow.
- Metered-quota regression tests must continue passing after every package change.

### 7. Wrong vs Correct

#### Wrong: Replace Rows And Reset Inventory

```text
DELETE all packages for service
INSERT request packages with new IDs and stock_available = stock_total
```

This breaks durable references and resurrects already reserved or sold units.

#### Correct: Reconcile Stable IDs And Apply A Delta

```text
retain known IDs
disable omitted packages
newStockAvailable = oldStockAvailable + newStockTotal - oldStockTotal
reject when newStockAvailable < 0
```

#### Wrong: Re-read Mutable Package Data At Delivery

```text
packageExpiresAt = deliverySubmittedAt + currentPackage.durationDays
```

#### Correct: Use The Order Snapshot

```text
durationDays = parse(order.selectedPackageSnapshot).durationDays
packageExpiresAt = deliverySubmittedAt + durationDays
```
