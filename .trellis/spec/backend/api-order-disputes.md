# API Order Dispute Lifecycle

Date: 2026-08-09
Author: Codex

## Scenario: Dispute Phase Projection Remains Separate From Order Fulfillment

### 1. Scope / Trigger

- Trigger: changing API order dispute creation, administrator resolution, order DTOs, dispute status constraints, order completion gates, or participant/admin dispute presentation.
- The dispute case is the moderation record. `api_orders.dispute_status` is the participant-facing order projection. Neither field replaces `api_orders.status`.

### 2. Signatures

```text
dispute_cases.status:
  negotiating | open | waiting_info | resolved | closed

api_orders.dispute_status:
  none | negotiating | open | awaiting_fulfillment |
  fulfillment_confirmation | closed

apiorder.IsDisputeActive(status string) bool
apiorder.Service.CloseDisputeProjection(
  ctx, disputeCaseID, actorUserID, requestID string,
) *domain.AppError

ApiOrder.disputeStatus:
  none | negotiating | open | awaiting_fulfillment |
  fulfillment_confirmation | closed
```

### 3. Contracts

- `api_orders.status` continues to represent payment, delivery, completion, and cancellation only. A dispute projection update must not change that field or any payment, delivery, credential, completion, or cancellation fact.
- Buyer, seller, and administrator order reads return the same `disputeStatus` for the same order.
- `open` means platform review. `closed` means the dispute record has reached a final closed projection. Reserved remediation phases must not be rendered as platform review.
- Only `dispute_status=none` permits opening the current dispute workflow. Every active dispute phase pauses every ordinary transaction mutation on that order: payment submission/instruction reads, buyer cancellation, seller payment confirmation or issue reporting, delivery submission, buyer completion, payment-timeout cancellation, delivery reminder, and delivery-review auto-completion. Dispute negotiation, remedy, and closure mutations remain available.
- Administrator resolution or closure of an API order dispute that has no pending remediation converges the linked order projection to `closed` in the same PostgreSQL transaction and increments the order version once.
- In-memory mode performs the same projection through the report-to-apiorder callback. PostgreSQL mode updates it only inside the administrator dispute transaction.
- `api_order.dispute_closed` is an order audit event, but its `from_status` and `to_status` remain the unchanged order transaction status. The event kind and note carry the dispute meaning.
- Replaying an already converged close operation does not increment the order version or append a second close event.
- Frontend status parsing rejects unknown non-empty values. Missing legacy values normalize to `none`; `resolved` must never be guessed into an order projection.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Linked order is missing or references another dispute | `409 INVALID_STATE_TRANSITION`; administrator mutation rolls back in PostgreSQL |
| Order projection is `none` during close convergence | `409 INVALID_STATE_TRANSITION` |
| Order projection is already `closed` | Idempotent success; no new version or event |
| Active dispute exists during an ordinary order action or timeout materialization | Action remains blocked and order transaction status, inventory, event, and notification state stay unchanged |
| Frontend receives an unknown dispute projection | Fail explicitly instead of presenting a misleading phase |

### 5. Good / Base / Bad Cases

- Good: an administrator resolves an API order dispute, and buyer, seller, and administrator reads all return `disputeStatus=closed` while the order remains `payment_submitted`.
- Base: an undisputed order returns `disputeStatus=none` and keeps the dispute entry action available while the order state permits it.
- Bad: an administrator resolution writes `closed` into `api_order_events.to_status`; consumers then parse a dispute phase as an order transaction status.
- Bad: the UI interprets every value other than `open` as eligible for a new dispute.

### 6. Tests Required

- Migration test: both check constraints contain the documented values and neither migration rewrites business rows.
- API order unit test: close convergence changes only projection metadata, increments once, and keeps event transaction statuses unchanged.
- Route regression: after administrator resolution, buyer and seller detail DTOs both return `closed` with the unchanged order status.
- PostgreSQL store test: projection convergence occurs inside the administrator transaction and locks the linked order relationship.
- Frontend/helper tests: all six labels are exhaustive, only `none` permits opening, active phases block every ordinary action and timeout, and unknown values fail.
- Page regression: buyer, seller, and administrator details use the shared label and description helpers.
- Gates: full Go test/vet, focused Vitest, Nuxt typecheck, OpenAPI generated-type check, route parity, migration documentation, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
appendEvent(order, EventDisputeClosed, order.DisputeStatus, DisputeStatusClosed)
```

This stores dispute phases in fields whose contract is `ApiOrderStatus`.

#### Correct

```go
appendEvent(order, EventDisputeClosed, order.Status, order.Status)
```

The event kind records the dispute transition while transaction status fields retain their original meaning.

## Scenario: Completed API Orders Have A Frozen 24-Hour Reporting Grace Period

### 1. Scope / Trigger

- Trigger: changing API-order validity snapshots, completed-order dispute creation, dispute occurrence evidence, participant/admin order DTOs, or frontend dispute eligibility.
- The grace period permits reporting a failure that occurred during purchased service validity. It does not extend API validity and does not promise, execute, or guarantee a refund.

### 2. Signatures

```text
POST /api/v1/me/api-orders/{id}/dispute
POST /api/v1/owner/api-orders/{id}/dispute
  { issueCode, requestedResolution, requestedAmountCny?, reason, issueOccurredAt? }

ApiOrder:
  afterSalesExpiresAt?: RFC3339 timestamp
  canOpenDispute: boolean
  disputeEligibilityReason:
    eligible | order_cancelled | dispute_exists |
    after_sales_expired | completed_validity_unknown

dispute_cases.issue_occurred_at timestamptz NULL

apiorder.ValidityExpiresAt(order) *time.Time
apiorder.WithAfterSalesProjection(order, now) Order
apiorder.ValidateDisputeOccurrence(order, raw, now) (*time.Time, *domain.AppError)
```

### 3. Contracts

- The authoritative frozen validity end is `packageExpiresAt`, then `quotaExpiresAtSnapshot`, then `pricingSnapshot.serviceValidityExpiresAt`.
- When a validity end exists, `afterSalesExpiresAt = validityEnd + exactly 24 hours`. Eligibility is open only while `now < afterSalesExpiresAt`; the exact boundary is expired.
- Cancelled orders and orders with any dispute projection other than `none` are always ineligible. A completed historical order without a usable frozen validity end remains ineligible instead of receiving an invented deadline.
- A completed eligible order requires `issueOccurredAt`. It must be RFC 3339, no later than the authoritative validity end, and no later than the server's current time. Non-completed orders may omit it for compatibility.
- Opening the completed-order dispute writes `issue_occurred_at`, creates the existing negotiation record, and updates only the dispute projection. The completed transaction status, completion source, credential, payment, delivery, and validity facts remain unchanged.
- In-memory and PostgreSQL mutations use one authoritative `now` for timeout materialization, eligibility, occurrence validation, dispute persistence, and the response projection.
- Buyer, seller, and administrator order DTOs return the same server-derived deadline, eligibility, and stable reason. Participant responses obtain frozen contact evidence through the authorized intent read; administrator responses contain no raw contacts or credentials.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| `now < validityEnd + 24h`, no dispute, not cancelled | `canOpenDispute=true`, reason `eligible` |
| `now >= validityEnd + 24h` | `canOpenDispute=false`, reason `after_sales_expired`; dispute mutation returns `409 INVALID_STATE_TRANSITION` |
| Completed order has no usable frozen validity end | `canOpenDispute=false`, reason `completed_validity_unknown` |
| Order is cancelled | `canOpenDispute=false`, reason `order_cancelled` |
| Any dispute projection already exists | `canOpenDispute=false`, reason `dispute_exists` |
| Completed eligible order omits `issueOccurredAt` | `422 VALIDATION_FAILED`, field `issueOccurredAt`, reason `required` |
| Occurrence time is malformed or in the future | `422 VALIDATION_FAILED`, field `issueOccurredAt`, reason `invalid` or `future` |
| Occurrence time is later than frozen validity | `422 VALIDATION_FAILED`, field `issueOccurredAt`, reason `after_validity` |

### 5. Good / Base / Bad Cases

- Good: validity ends at T, the buyer reports at `T+23h`, and `issueOccurredAt=T-1m`; the dispute opens while the order remains completed.
- Base: an incomplete, non-cancelled legacy order without a validity snapshot retains the existing dispute path and may omit occurrence time.
- Base: a completed historical order without validity facts remains readable but has no dispute action or fabricated deadline.
- Bad: accepting a report at exactly `T+24h`, extending credential/API usability until that time, or describing the grace period as guaranteed compensation.
- Bad: reconstructing validity from the current mutable service, browser time, or a later merchant edit.

### 6. Tests Required

- Unit tests cover validity-source priority, `T+24h-epsilon`, exact `T+24h`, cancellation, existing disputes, unknown completed validity, missing/malformed/future/post-validity occurrence time, and one-clock in-memory mutation behavior.
- Router/OpenAPI tests assert participant/admin projection parity, conditional occurrence validation, persistence in dispute detail, and no administrator contact/credential leakage.
- PostgreSQL integration covers completed-order dispute creation before the deadline, atomic rollback after the deadline or invalid occurrence, and unchanged order completion/credential facts.
- Frontend adapter tests assert real and Mock paths carry server-equivalent projection fields; order-detail tests assert the occurrence input and grace-period copy.
- Gates: full Go test/vet, full Vitest, OpenAPI generated-type check, Nuxt typecheck/build, migration upgrade/rollback/re-upgrade, desktop/mobile browser checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```ts
const canOpenDispute = order.status !== 'completed'
const deadline = addHours(new Date(), 24)
```

#### Correct

```ts
const canOpenDispute = order.canOpenDispute
const deadline = order.afterSalesExpiresAt
```

The backend computes eligibility from frozen order facts; the browser renders it and never starts a new validity or grace clock.

## Scenario: Structured Participant Negotiation Requires Bilateral Confirmation

### 1. Scope / Trigger

- Trigger: changing the API-order dispute request, participant messages, settlement proposals, participant escalation, administrator evidence, or seller restrictions related to dispute enforcement.
- This workflow applies only to `target_type=api_order`. It is an auditable, non-real-time negotiation record, not a general chat channel.

### 2. Signatures

```text
POST /api/v1/me/api-orders/{id}/dispute
POST /api/v1/owner/api-orders/{id}/dispute
GET  /api/v1/me/disputes/{id}
POST /api/v1/me/disputes/{id}/messages
POST /api/v1/me/disputes/{id}/settlement-proposals
POST /api/v1/me/disputes/{id}/settlement-proposals/{proposalId}/confirm
POST /api/v1/me/disputes/{id}/settlement-proposals/{proposalId}/reject
POST /api/v1/me/disputes/{id}/escalate

dispute_cases:
  issue_code text
  requested_resolution text
  requested_amount_cny numeric(20,2) nullable

api_order_dispute_messages:
  id, dispute_case_id, sender_user_id, body, request_id, created_at

api_order_dispute_settlement_proposals:
  id, dispute_case_id, proposed_by_user_id, resolution, amount_cny,
  terms, status, accepted_by_user_id, accepted_at,
  rejected_by_user_id, rejected_at, request_id,
  created_at, updated_at, version
```

### 3. Contracts

- Opening a dispute requires `issueCode`, `requestedResolution`, a credential-free `reason`, and `requestedAmountCny` only for `partial_refund`. The amount must be a plain positive decimal with at most two fractional digits and must not exceed the linked order total.
- Opening creates `dispute_cases.status=negotiating`, `api_orders.dispute_status=negotiating`, and the initial immutable message in one transaction. It does not change `api_orders.status`.
- Messages are append-only plain text. No update/delete endpoint exists, and PostgreSQL rejects direct update/delete while still permitting parent-row cascade cleanup.
- A new proposal makes the prior pending proposal `superseded` but preserves its history. Proposal creation is the proposer's confirmation.
- Only the other order participant may confirm or reject the same pending `proposalId`. Confirmation atomically changes the proposal to `accepted`, the case to `closed`, and the order dispute projection to `closed`; rejection changes only the proposal to `rejected` and leaves the case negotiating.
- Either participant may escalate while negotiating. Escalation supersedes any pending proposal and atomically changes the case and order dispute projection to `open`. After escalation, participant proposal creation, confirmation, rejection, and unilateral closure are forbidden; messages remain allowed in `open` and `waiting_info`.
- A seller may reject a proposal or request platform review. Only an administrator may decide that the original request is invalid or close a platform-reviewed dispute without participant agreement.
- Participant reads return `404 OBJECT_NOT_FOUND` to outsiders and for non-API disputes submitted to these negotiation mutation routes. Mutations authenticate and validate CSRF before decoding JSON.
- Participant mutations lock the idempotency row, dispute case, linked order, and affected proposal in that order. Business writes and the completed idempotency record commit together.
- Administrator-created enforcement that reuses `api_service_publish` must be described as limiting new orders, publishing, and restoring. Existing orders continue unless that specific order independently has an active dispute; an active dispute pauses its ordinary transaction flow under the separate projection contract.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Missing/unknown issue or resolution | `422 VALIDATION_FAILED` with the matching field |
| Partial refund amount is empty, non-decimal, non-positive, has more than two decimals, or exceeds the order total | `422 VALIDATION_FAILED` on `requestedAmountCny` / `amountCny` |
| Message, terms, or reason contains a secret | `422 SECRET_CONTENT_DETECTED` |
| Outsider or unsupported non-API dispute mutation | `404 OBJECT_NOT_FOUND` |
| Proposer confirms/rejects their own proposal | `409 INVALID_STATE_TRANSITION` |
| Proposal is no longer pending | `409 INVALID_STATE_TRANSITION` |
| Participant acts on a proposal after escalation or closure | `409 INVALID_STATE_TRANSITION` |
| Linked case/order projection is inconsistent | `409 INVALID_STATE_TRANSITION`; transaction rolls back |
| Same idempotency key and request hash is replayed | Return the original result without another message, event, or version increment |

### 5. Good / Base / Bad Cases

- Good: the buyer proposes a partial refund of `25.50`; the seller confirms that exact proposal; the proposal, case, and order projection close atomically while the order remains `payment_submitted`.
- Base: the seller rejects a pending continuation proposal with an optional reason; the rejected proposal remains visible in history and the dispute remains `negotiating`.
- Good: either participant escalates; pending proposals become `superseded`, both participants see `open`, and the administrator sees the immutable negotiation evidence.
- Bad: the proposer calls confirm on their own proposal and closes the case without the counterparty.
- Bad: product copy says the seller restriction "only stops new orders" even though `api_service_publish` also blocks publishing and restoring.

### 6. Tests Required

- API-order unit tests: structured request validation, plain-decimal amount grammar, order-total limit, and initial `negotiating` projection.
- Report unit tests: immutable messages, proposal replacement, self-confirm rejection, counterparty confirmation, rejection without closure, and escalation lockout.
- Router tests: buyer/seller parity, outsider `404`, authentication-before-decode, initial message attribution, bilateral close, escalation, and unchanged order lifecycle status.
- PostgreSQL/source tests: lock order, idempotency completion before commit, one pending proposal constraint, append-only trigger, and non-destructive migration rollback.
- Frontend tests: structured `null` amount when not partial, exhaustive status labels, immutable timeline, visible proposal history, no unilateral seller closure wording, and platform escalation controls.
- Gates: full Go test/vet, full Vitest, Nuxt typecheck and production build, OpenAPI generated-type and route checks, migration documentation, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
seller rejects request -> case closed as request_invalid
api_service_publish copy -> "only stops new orders"
```

This grants a participant administrator authority and misstates the enforcement boundary.

#### Correct

```text
seller rejects proposal -> proposal=rejected, case=negotiating
seller escalates         -> case=open, order.dispute_status=open
administrator rules invalid -> administrator workflow may close the case
api_service_publish copy -> "limits new orders, publishing, and restoring; existing orders continue"
```

## Scenario: Post-Ruling Remedies Produce Sanction Evidence Only After Confirmed Overdue

### Contracts

- An administrator resolves an API-order dispute either without a remedy, which closes the case and order projection, or with a new append-only remedy, which keeps the case `resolved` and projects the order to `awaiting_fulfillment`.
- Only the responsible party may move `pending` to `claimed_fulfilled`; this projects `fulfillment_confirmation` and does not close the case. Only the beneficiary may confirm or contest. Confirmation closes both projections, while contesting preserves the remedy history and reopens both projections for administrator review.
- A beneficiary response timeout is exactly 48 hours and closes with `confirmation_expired`. Its public result must state that the beneficiary did not respond and the platform did not verify payment or fulfillment.
- Ordinary administrator close is forbidden while the latest remedy is `pending` or `claimed_fulfilled`. Reaching `due_at` alone does not create fault; only the administrator `mark_overdue` action may record `overdue` and close both projections.
- API-order outcomes with `responsible` or `shared` responsibility, source-linked restrictions, and aggregate fault counts are eligible only when the latest remedy is `overdue`. No-remedy and administrator-invalid-claim decisions may create only non-fault outcomes such as `not_responsible` or `undetermined`.

### Tests Required

- Cover exact due/confirmation boundaries, active-remedy close rejection, contest reopening both projections, neutral timeout wording, and overdue-only outcome, restriction, and aggregate reputation evidence.

## Scenario: Confirmed Overdue Remedies Produce Explicit Tiered Seller Sanctions

### 1. Scope / Trigger

- Trigger: changing API-order remedy sanctions, restriction evidence, seller API-order creation, administrator sanction UI, seller restriction visibility, or the rolling breach window.
- The system recommends a sanction from durable facts. Only an administrator mutation creates the restriction; remedy expiry or outcome creation must never apply it automatically.

### 2. Signatures

```text
GET  /api/v1/admin/disputes/{id}/sanction-recommendation
POST /api/v1/admin/disputes/{id}/sanction
  If-Match: "<subject-user-version>"
  Idempotency-Key: <opaque key>
  { "internalReason": "2..2000 characters" }

GET /api/v1/me/reputation
  -> { ruleVersion, items, activeRestrictions[] }

user_restrictions.source_dispute_remedy_id uuid
  REFERENCES api_order_dispute_remedies(id) ON DELETE RESTRICT
  UNIQUE WHERE source_dispute_remedy_id IS NOT NULL

reputation.RecommendedAPIOrderSanctionDays(count int) int
  0 -> 0, 1 -> 7, 2 -> 30, 3+ -> 90
```

### 3. Contracts

- Eligibility requires an API-order dispute, its latest remedy in administrator-confirmed `overdue`, an active `responsible|shared` outcome, and the same user as outcome subject, remedy responsible party, and order seller.
- The 180-day count uses `overdue_at >= now - 180 days`, includes the current qualifying fact, and counts only overdue remedies whose responsible user is the linked API-order seller. Historical facts remain counted even if a later appeal reverses their outcome; the durable overdue fact is not rewritten.
- Recommendation reads use one read-only repeatable-read PostgreSQL snapshot. Apply locks and revalidates the dispute, latest remedy, active outcome, API order, subject user version, and existing remedy-linked restriction before calculating the current tier.
- Apply always creates `role_scope=seller`, `action_code=api_service_publish`, a fixed-expiry restriction, both outcome/remedy evidence links, one governance event, one public seller notification to `/my/reputation`, and one completed idempotency result in the same transaction.
- A remedy evidence link is immutable audit ownership. Upstream deletion is restricted, and the partial unique index prevents a second sanction even after expiry or revocation.
- Generic administrator restriction creation must reject API-order outcome sources. It must not accept or write `sourceDisputeRemedyId`; the dedicated sanction endpoint owns this evidence.
- Active `api_service_publish|all` restrictions for seller/all roles block new normal API orders and limited-quota API orders inside their PostgreSQL transactions before contact locking, inventory reservation, intent advancement, or order insertion.
- The same administrator-created restriction continues to block API-service submission, publication, restoration, public visibility, and promotion. Existing orders do not consult the restriction gate; only an independently active dispute pauses the affected order's ordinary transaction actions.
- `/me/reputation.activeRestrictions` is a public-safe projection: restriction type, role, action, reason code, public reason, start, and optional end only. It must not expose IDs, internal reasons, administrator IDs, source evidence IDs, versions, or governance records.
- Product copy must name all three limited abilities: new orders, publishing, and restoring. It must also state that existing orders continue; copy that says the restriction only stops new orders is false.
- After `412 VERSION_CONFLICT` or a state conflict, the administrator UI refetches the recommendation and clears explicit confirmation before another apply attempt.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Non-API dispute or no latest overdue remedy | Recommendation is ineligible with a granular reason code; apply returns `409 INVALID_STATE_TRANSITION` |
| Missing/inactive/non-fault outcome | Recommendation is ineligible; no restriction, event, notification, or completed idempotency result |
| Subject, remedy responsible party, and order seller differ | Recommendation is ineligible with `responsible_seller_required` |
| Missing or stale subject-user `If-Match` | `428 PRECONDITION_REQUIRED` or `412 VERSION_CONFLICT` |
| Internal reason is outside 2..2000 characters | `422 VALIDATION_FAILED` on `internalReason` |
| Same remedy is submitted again | Exact completed replay returns the original response; any second sanction attempt creates no new restriction |
| Seller has an active matching restriction during new order creation | `403 REPUTATION_ACTION_RESTRICTED`; inventory, intent, contact, and order state remain unchanged |
| Restriction is expired or revoked | New order, submission, publication, and restoration gates allow the action again |

### 5. Good / Base / Bad Cases

- Good: a seller's second administrator-confirmed overdue remedy inside 180 days recommends 30 days; an administrator confirms once, and the restriction, user version, event, notification, and idempotency completion commit atomically.
- Good: the seller is restricted after an order already exists. New normal and quota orders fail, while the existing buyer can still pay and the seller can still deliver and handle the dispute.
- Base: an administrator rules the buyer request invalid or the responsible party fulfills on time. No overdue fact exists, so no sanction recommendation is eligible.
- Bad: count an unresolved complaint, a due timestamp alone, or a beneficiary confirmation timeout as a seller breach.
- Bad: create the restriction automatically when `mark_overdue` runs, accept a remedy ID through the generic restriction form, or show internal governance fields on the seller reputation page.

### 6. Tests Required

- Migration tests: expected version 88, `ON DELETE RESTRICT`, remedy-source partial uniqueness, overdue lookup index, and executable down migration.
- Service tests: every ineligible reason, exact 180-day boundary, outside-window exclusion, `0/1/2/3+` tier mapping, historical-fact counting, subject/seller parity, duplicate remedy application, and in-memory/PostgreSQL parity.
- Store tests: repeatable-read recommendation snapshot, apply lock/revalidation order, unique conflict, and atomic restriction/event/notification/idempotency writes.
- Order tests: both normal and limited-quota restriction gates precede contact, inventory, intent, and order side effects; existing-order mutations have no added gate.
- Route/OpenAPI tests: administrator authority, CSRF, idempotency, subject-user `If-Match`, fresh `ETag`, dedicated route parity, generic request exclusion, and generated types.
- Frontend tests: recommendation/refetch flow, explicit confirmation, conflict reset, accurate three-ability copy, public active-restriction projection, and no internal-field leakage.
- Gates: full Go test/vet, full Vitest, Nuxt typecheck and real-mode production build, migration/OpenAPI/generated-type checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
mark remedy overdue -> automatically create a 30-day restriction
generic restriction request -> accepts sourceDisputeRemedyId
restriction copy -> "only stops new orders"
new quota order -> claim inventory -> check seller restriction
```

#### Correct

```text
mark remedy overdue -> durable sanction evidence only
GET recommendation -> one repeatable-read snapshot
administrator POST -> revalidate -> explicit fixed-tier restriction
restriction copy -> "limits new orders, publishing, and restoring; existing orders continue"
new quota order -> check seller restriction -> claim inventory -> create order
```
