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
- Only `dispute_status=none` permits opening the current dispute workflow. Every active dispute phase pauses buyer completion and delivery-review auto-completion.
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
| Active dispute exists during buyer completion or auto-completion | Action remains blocked and order fulfillment status stays unchanged |
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
- Frontend helper tests: all six labels are exhaustive, only `none` permits opening, active phases block completion, and unknown values fail.
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
- Future enforcement that reuses `api_service_publish` must be described as limiting new orders, publishing, and restoring. Existing orders continue fulfillment, delivery, after-sales work, and disputes and must not consult that restriction.

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
