# API Order Dispute Lifecycle

Date: 2026-08-09
Author: Codex
Updated: 2026-08-18

## Scenario: Dispute Phase Projection Remains Separate From Order Fulfillment

### 1. Scope / Trigger

- Trigger: changing API order dispute creation, administrator resolution, order DTOs, dispute status constraints, order completion gates, or participant/admin dispute presentation.
- The dispute case is the moderation record. `api_orders.dispute_status` is the participant-facing order projection. Neither field replaces `api_orders.status`.

### 2. Signatures

```text
dispute_cases.status:
  pending_seller_response | pending_applicant_decision |
  voluntary_fulfillment | open | waiting_info |
  resolved | closed | withdrawn | self_resolved

api_orders.dispute_status:
  none | negotiating | pending_seller_response |
  pending_applicant_decision | open |
  awaiting_fulfillment | fulfillment_confirmation | closed

apiorder.IsDisputeActive(status string) bool
apiorder.Service.CloseDisputeProjection(
  ctx, disputeCaseID, actorUserID, requestID string,
) *domain.AppError

ApiOrder.disputeStatus:
  none | negotiating | pending_seller_response |
  pending_applicant_decision | open |
  awaiting_fulfillment | fulfillment_confirmation | closed
```

### 3. Contracts

- `api_orders.status` continues to represent payment, delivery, completion, and cancellation only. A dispute projection update must not change that field or any payment, delivery, credential, completion, or cancellation fact.
- Buyer, seller, and administrator order reads return the same `disputeStatus` for the same order.
- `pending_seller_response` and `pending_applicant_decision` are pre-platform after-sales phases; `open` alone means platform review. New finalization clears the active projection to `none` and preserves the case through `latest_dispute_case_id`; `negotiating` and `closed` remain legacy read values only. Remediation phases must not be rendered as platform review.
- Only `dispute_status=none` permits opening the current dispute workflow. Every active dispute phase pauses every ordinary transaction mutation on that order: payment submission/instruction reads, buyer cancellation, seller payment confirmation or issue reporting, delivery submission, buyer completion, payment-timeout cancellation, delivery reminder, and delivery-review auto-completion. Formal response, directed supplement, applicant closure, remedy, and administrator mutations remain available when their own state guards allow them.
- Administrator resolution or closure of an API order dispute that has no pending remediation clears `dispute_case_id`, sets `dispute_status=none`, preserves `latest_dispute_case_id`, and increments the order version once in the same PostgreSQL transaction.
- In-memory mode performs the same projection through the report-to-apiorder callback. PostgreSQL mode updates it only inside the administrator dispute transaction.
- `api_order.dispute_closed` is an order audit event, but its `from_status` and `to_status` remain the unchanged order transaction status. The event kind and note carry the dispute meaning.
- Replaying an already converged finalization does not increment the order version or append a second close event.
- Frontend status parsing rejects unknown non-empty values. Missing legacy values normalize to `none`; `resolved` must never be guessed into an order projection.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Linked order is missing or references another dispute | `409 INVALID_STATE_TRANSITION`; administrator mutation rolls back in PostgreSQL |
| Order projection is `none` during close convergence | `409 INVALID_STATE_TRANSITION` |
| Case is inactive and only `latest_dispute_case_id` references it | Idempotent success; no new version or event |
| Active dispute exists during an ordinary order action or timeout materialization | Action remains blocked and order transaction status, inventory, event, and notification state stay unchanged |
| Frontend receives an unknown dispute projection | Fail explicitly instead of presenting a misleading phase |

### 5. Good / Base / Bad Cases

- Good: an administrator resolves an API order dispute, and buyer, seller, and administrator reads return `disputeStatus=none`, `hasDisputeHistory=true`, and the same `latestDisputeCaseId` while the order remains `payment_submitted`.
- Base: an undisputed order returns `disputeStatus=none` and keeps the dispute entry action available while the order state permits it.
- Bad: an administrator resolution writes `closed` into `api_order_events.to_status`; consumers then parse a dispute phase as an order transaction status.
- Bad: the UI interprets every value other than `open` as eligible for a new dispute.

### 6. Tests Required

- Migration test: both check constraints contain the documented values and migration 108 adds the seller-first phases without changing order fulfillment status.
- API order unit test: close convergence changes only projection metadata, increments once, and keeps event transaction statuses unchanged.
- Route regression: after administrator resolution, buyer and seller detail DTOs both clear the active case, retain the latest case/history projection, and keep the order status unchanged.
- PostgreSQL store test: projection convergence occurs inside the administrator transaction and locks the linked order relationship.
- Frontend/helper tests: every documented projection label is exhaustive, only `none` permits opening, active phases block every ordinary action and timeout, and unknown values fail.
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
- Opening the completed-order dispute writes `issue_occurred_at`, creates a seller-first after-sales record, and updates only the dispute projection. The completed transaction status, completion source, credential, payment, delivery, and validity facts remain unchanged.
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

## Scenario: API-Order After-Sales Requests Use Seller-First Resolution

### 1. Scope / Trigger

- Trigger: changing API-order after-sales creation, seller decisions, applicant withdrawal or platform intervention, voluntary remedies, directed supplements, order-list projections, or timeout maintenance.
- This workflow applies only to `target_type=api_order`. Buyer and seller may communicate through WeChat, email, or linux.do, while the platform records only formal after-sales actions and evidence.

### 2. Signatures

```text
POST /api/v1/me/api-orders/{id}/dispute
GET  /api/v1/me/disputes/{id}
POST /api/v1/me/disputes/{id}/seller-decision
  { decision: accepted | rejected, reason, evidenceAssetIds? }
POST /api/v1/me/disputes/{id}/platform-intervention
  { reason, evidenceAssetIds? }
POST /api/v1/me/disputes/{id}/withdraw
POST /api/v1/me/disputes/{id}/supplements
POST /api/v1/me/disputes/{id}/remedy/claim
POST /api/v1/me/disputes/{id}/remedy/confirm
POST /api/v1/me/disputes/{id}/remedy/contest

dispute_cases.status:
  pending_seller_response | pending_applicant_decision |
  voluntary_fulfillment | open | waiting_info |
  resolved | closed | withdrawn

dispute_cases:
  applicant_statement text
  seller_decision text nullable
  seller_decision_reason text nullable
  seller_decided_at timestamptz nullable
  seller_response_late boolean
  applicant_decision_due_at timestamptz nullable
  next_actor text
  next_user_id uuid nullable
  due_at timestamptz nullable
  fact_snapshot jsonb

participant order list projection:
  disputeStatus, disputeCaseId, disputeNextActor,
  disputeDueAt, disputeNeedsAction, disputeRemedyAction
```

### 3. Contracts

- Only the buyer may open an API-order after-sales request. Opening requires `issueCode`, `requestedResolution`, a credential-free `reason`, and `requestedAmountCny` only for `partial_refund`. It atomically creates `status=pending_seller_response`, freezes the reason and fact snapshot, sets `next_actor=respondent`, and sets `due_at=opened_at+24h`. It never changes `api_orders.status`.
- The seller may submit one immutable `accepted` or `rejected` decision with a reason and optional private evidence. The seller may still respond after the 24-hour deadline while the buyer has not withdrawn or committed platform intervention; the case records `seller_response_late=true`. Concurrent seller decisions permit exactly one winner.
- Rejection moves to `pending_applicant_decision`, sets `next_actor=applicant`, and gives the buyer three days to withdraw or request platform intervention. Expiry closes the case neutrally without assigning responsibility, sanctions, adverse facts, or appeal eligibility.
- Acceptance creates a neutral `seller_acceptance` remedy and moves to voluntary fulfillment. The seller claims fulfillment with evidence; the buyer then has 24 hours to confirm or request platform intervention. Confirmation or expiry closes neutrally and creates no platform responsibility outcome.
- Only the applicant may withdraw, and only before platform review or a committed remedy outcome. Withdrawal is the sole participant cancellation operation; `self-resolve` does not have a route or an available action.
- The applicant may request platform intervention after seller rejection, after the initial 24-hour response deadline, after an accepted remedy becomes overdue, or while reviewing a claimed voluntary remedy. Intervention atomically moves the case to `open`, sets `next_actor=admin`, clears voluntary deadlines, and prevents any later seller-decision write.
- `waiting_info` targets exactly one participant through `next_actor`, `next_user_id`, and a 48-hour `due_at`. Only that participant may submit the requested supplement. Completion returns the case to `open`, `next_actor=admin`, and clears the deadline.
- Platform rulings, remedy fulfillment, counterparty confirmation/contest, lateness review, and appeals remain available. Pre-platform seller decisions and applicant withdrawal cannot bypass an existing ruling or remedy.
- Message, formal-response, self-resolution, settlement-proposal, proposal-confirm/reject, and legacy escalation routes do not exist. Historical messages and settlement proposals remain readable as legacy history and have no write controls.
- Buyer and seller order lists keep fulfillment `status` unchanged and render the dispute as a second status. `disputeNeedsAction` is true only when the current viewer matches the action owner; every linked case exposes an independent case-page route.
- The backend-provided `availableActions` list is authoritative for participant controls. The frontend must not infer seller-decision, withdrawal, intervention, remedy, or appeal eligibility from labels or elapsed browser time.
- In-memory and PostgreSQL flows must produce the same decisions, `next_actor`, deadlines, remedy action, and viewer-specific needs-action projection. Changing only action/deadline metadata must not increment the API-order business version.
- Participant reads return `404 OBJECT_NOT_FOUND` to outsiders. Every mutation authenticates and validates CSRF before decoding JSON, and idempotent writes complete with the business transaction.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Missing/unknown issue or requested resolution | `422 VALIDATION_FAILED` on the matching field |
| Invalid or excessive partial-refund amount | `422 VALIDATION_FAILED` on `requestedAmountCny` |
| Applicant statement, seller decision, supplement, or reason contains a secret | `422 SECRET_CONTENT_DETECTED` |
| Seller calls the removed owner dispute-creation path | `404 OBJECT_NOT_FOUND`; no case or order event is created |
| Outsider or unsupported non-API dispute mutation | `404 OBJECT_NOT_FOUND` |
| Applicant attempts a seller decision, or seller decides twice | `409 INVALID_STATE_TRANSITION`; decision fields remain unchanged |
| Seller attempts withdrawal or platform intervention | `409 INVALID_STATE_TRANSITION` |
| Applicant withdraws after platform intervention or ruling/remedy commitment | `409 INVALID_STATE_TRANSITION` |
| Seller responds after platform intervention wins a concurrent commit | `409 INVALID_STATE_TRANSITION`; platform-review projection remains unchanged |
| Buyer requests intervention before any allowed trigger | `409 INVALID_STATE_TRANSITION`; deadlines remain unchanged |
| Non-targeted participant submits a directed supplement | `409 INVALID_STATE_TRANSITION`; actor and deadline remain unchanged |
| Legacy response/self-resolution/message/proposal/escalation route is called | `404`; no legacy row or event is written |
| Same idempotency key and request hash is replayed | Return the original result without another event or version increment |

### 5. Good / Base / Bad Cases

- Good: a buyer applies, the seller accepts within 24 hours, submits refund evidence, and the buyer confirms; the case closes without responsibility or appeal facts while order fulfillment status remains unchanged.
- Good: the seller rejects, the buyer requests platform intervention within three days, and the administrator receives the frozen request, seller decision, and evidence.
- Base: the seller responds at hour 25 before the buyer intervenes; the response succeeds with `seller_response_late=true`.
- Base: the seller never responds; after 24 hours the buyer sees intervention, while neutral expiry occurs only after the applicant-decision window.
- Bad: the seller opens the original request, either party cancels it, or the UI infers actions from a status label instead of `availableActions`.
- Bad: opening creates `negotiating`, asks which chat application was used, or requires proof that external negotiation failed.
- Bad: the UI changes the order fulfillment status to `disputed`, or shows both participants that the same action is waiting on them.

### 6. Tests Required

- API-order and report unit tests cover buyer-only creation, 24-hour seller deadline, accept/reject, late response, immutable fact snapshot, applicant-only withdrawal/intervention, voluntary remedies, targeted supplements, and in-memory projection parity.
- Router tests cover removed seller creation, outsider `404`, authentication-before-decode, duplicate seller decision, applicant withdrawal/intervention, removed legacy routes, administrator ruling, and unchanged order fulfillment status.
- Migration tests execute `107 -> 108`, `108 down`, and `108 up`; verify new statuses, constraints, neutral finalization, and timestamp expressions.
- PostgreSQL integration validates accept/reject, remedy evidence, late response, concurrent decisions, applicant-decision expiry, voluntary-confirmation expiry, and no voluntary reputation or appeal outcome.
- Frontend tests cover independent list badges/filters/links, seller decisions with evidence, applicant actions, voluntary remedy actions, directed supplements, and read-only legacy history.
- Gates: full Go test/vet, full Vitest, Nuxt typecheck/build, OpenAPI regeneration, migration documentation, browser smoke checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
seller creates request -> chat/proposals -> either party cancels -> open
order.status = disputed
```

#### Correct

```text
buyer creates -> pending_seller_response, next_actor=respondent, due_at=opened_at+24h
seller accepts -> voluntary remedy -> seller claim -> buyer confirm or intervene
seller rejects -> pending_applicant_decision -> buyer withdraws or intervenes within 3d
seller timeout -> buyer may intervene; seller may still answer until intervention commits
intervention -> open, next_actor=admin, due_at=NULL
request info -> waiting_info, targeted participant, due_at=now+48h
supplement -> open, next_actor=admin, due_at=NULL
order.status remains the payment/delivery lifecycle status
```

## Scenario: Administrator Detail Replays Formal After-Sales Activity

### 1. Scope / Trigger

- Trigger: changing administrator or participant dispute detail DTOs, platform-intervention events, evidence bindings, directed supplements, remedies, or the administrator dispute timeline.
- Administrator review must receive the complete formal after-sales record without restoring the removed participant chat workflow.

### 2. Signatures

```text
GET /api/v1/admin/disputes/{id}
GET /api/v1/me/disputes/{id}

AdminDispute.platformInterventionReason?: string
MyDisputeDetail.platformInterventionReason?: string

dispute_events:
  entity_type = dispute
  entity_id = dispute case id
  action = platform_intervention_requested
  reason = immutable applicant reason

evidence grouping:
  dispute_initial     + source_id=dispute id
  formal_response     + source_id=dispute id
  platform_escalation + source_id=dispute id
  info_supplement     + source_id=supplement id
  remedy_claim        + source_id=remedy id
  remedy_contest      + source_id=remedy id
```

### 3. Contracts

- Administrator detail exposes the buyer application, seller accept/reject decision and reason, applicant platform-intervention reason, directed supplements, remedies, and their private evidence. The UI orders these formal facts by their business timestamps.
- `platformInterventionReason` is a read projection of the latest immutable `platform_intervention_requested` event for the case. PostgreSQL reads recover it from `dispute_events`; in-memory intervention writes set the same response projection immediately. Do not add a duplicate mutable column.
- Evidence is grouped under its formal action by both `usage` and `sourceId`. Evidence already assigned to an action renders once. Unmatched evidence remains visible under an explicit fallback section rather than being discarded.
- Participant identity labels derive from stable user IDs and the case's buyer/seller fields. They must not depend on the current administrator viewer or array position.
- Historical `messages` and `settlementProposals` render only in a separate read-only legacy section. No current participant message, proposal, or chat-image write control may be introduced.
- OpenAPI is the source of generated frontend DTO types. Update both administrator and participant schemas, regenerate, and run the drift guard.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| No platform-intervention event exists | Omit or return an empty `platformInterventionReason`; detail remains readable |
| Several intervention events exist in historical data | Project the latest by `created_at DESC, id DESC` |
| Evidence `usage` and `sourceId` match a formal action | Render it beneath that action exactly once |
| Evidence cannot be matched to a known action | Render it in the fallback evidence section |
| Legacy message/proposal exists | Render read-only under legacy history; expose no mutation control |
| Private evidence content is unavailable or quarantined | Existing authorized content endpoint returns its stable error; the detail timeline still renders |

### 5. Good / Base / Bad Cases

- Good: a seller rejects with images, the buyer requests intervention with a reason and images, and the administrator sees both actions, actors, times, text, and correctly grouped media.
- Good: the administrator requests a supplement and later creates a remedy; submission, claim, contest, confirmation, and their evidence appear in chronological context.
- Base: a pre-event historical dispute has no intervention reason but remains fully administrable with its available formal fields and legacy history.
- Bad: the administrator sees only the latest status and cannot inspect why the seller rejected or why the buyer escalated.
- Bad: the frontend merges old free-form messages into the current formal timeline or restores a participant chat composer.
- Bad: unmatched evidence is silently omitted because its source type is unfamiliar.

### 6. Tests Required

- Report service test asserts in-memory intervention returns the submitted immutable reason.
- Handler/router tests assert administrator and participant DTOs include `platformInterventionReason` without changing existing dispute state or order status.
- PostgreSQL integration writes an intervention event, reloads administrator detail, and asserts the reason is recovered from event history.
- Frontend regression asserts the administrator detail includes application, seller decision, intervention, supplement, remedy, grouped evidence, fallback evidence, and read-only legacy sections.
- Gates: full Go test/vet, full Vitest, Nuxt typecheck and real-mode production build, OpenAPI route/type drift checks, browser desktop/mobile smoke checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
admin detail = current status + generic evidence gallery
platform intervention reason = new mutable dispute_cases column
legacy messages = current chat timeline with reply controls
```

#### Correct

```text
immutable formal facts + typed evidence bindings -> chronological admin timeline
latest platform_intervention_requested event -> platformInterventionReason projection
legacy messages/proposals -> separate read-only history
unmatched evidence -> explicit fallback gallery
```

## Scenario: API-Order Dispute Image Evidence Is Private, Bound Once, And Retained For A Bounded Period

### 1. Scope / Trigger

- Trigger: changing dispute evidence upload, image processing, evidence attachment to a dispute action, participant/admin dispute reads, quarantine, object storage, or evidence cleanup.
- Evidence is an append-only private fact attached to an API order. It must not be copied into public events, notifications, audit payloads, or idempotency response metadata.

### 2. Signatures

```text
POST /api/v1/me/api-orders/{id}/dispute-evidence
  multipart/form-data:
    kind: allowed evidence kind
    redactionConfirmed: true
    files: 1..3 JPEG | PNG | WebP inputs, each <= 5 MiB and <= 4096x4096

GET  /api/v1/me/dispute-evidence/{id}/content
GET  /api/v1/admin/dispute-evidence/{id}/content
POST /api/v1/admin/dispute-evidence/{id}/quarantine
  If-Match: "<asset-version>"
  Idempotency-Key: <opaque key>

evidenceAssetIds: 0..3 distinct UUIDs bound by the owning business action
```

### 3. Contracts

- Upload requires session, CSRF, an allowed `kind`, exactly one `redactionConfirmed=true`, and one to three files. The request body is bounded before multipart parsing; each file is bounded before decoding.
- JPEG, PNG, and WebP inputs are decoded only after their header dimensions pass the `1..4096` check. QR-bearing images are rejected. Accepted inputs are re-encoded by the server as JPEG or PNG, and stored with the hash and dimensions of that output.
- Only the linked API-order buyer or seller may upload. A successful upload creates private, unbound assets with a 24-hour expiry. Upload does not create a dispute event or reveal an object-store key.
- A business mutation binds each asset atomically and at most once. The uploader, API order, allowed usage, source type, source ID, and dispute ownership must all match the action. A failed mutation leaves every asset unbound and reusable until expiry.
- Initial request and seller-decision evidence is visible to both order participants and administrators. Directed supplement evidence is visible only to its submitter and administrators. Appeal evidence is visible only to the appellant and administrators. Evidence already bound to legacy messages or escalation remains readable under its original visibility.
- Participant content reads use the `/me` route; administrator DTOs use the `/admin` content route. Every content response is authorized from current binding visibility, uses `private, no-store` plus `nosniff`, and never returns quarantined or deletion-pending content.
- Administrator quarantine requires current version, CSRF, idempotency, and a credential-free reason of 2..800 characters. Quarantine makes content unreadable immediately and schedules destruction after seven days.
- Unbound uploads are eligible for cleanup after 24 hours. Bound evidence for a terminal dispute is eligible after 90 days only when no active remedy or submitted appeal still requires it. Cleanup first claims durable deletion state, then removes the object and finalizes the database fact idempotently.
- Object keys, raw bytes, content paths, credentials, account values, and image descriptions must not enter events, notifications, moderation audit payloads, logs, or completed idempotency bodies for the binding action.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Missing/false redaction confirmation, unknown multipart field, zero files, or more than three files | `422 VALIDATION_FAILED` on the matching upload field |
| Unsupported content, QR code, file over 5 MiB, or dimensions outside `1..4096` | `422 VALIDATION_FAILED`; no object or asset row remains |
| Outsider uploads or reads, or participant reads another visibility class | `404 OBJECT_NOT_FOUND` without confirming asset existence |
| Asset belongs to another order/uploader, is expired/quarantined, or has already been bound | Business mutation fails atomically; no partial bindings or source row |
| Stale quarantine `If-Match` | `412 VERSION_CONFLICT`; asset remains readable under its prior state |
| Quarantined/deletion-pending asset content read | `404 OBJECT_NOT_FOUND` for participants and administrators |
| Object-store write or database insert fails during upload | Best-effort object rollback; no successful asset response |

### 5. Good / Base / Bad Cases

- Good: a buyer uploads two redacted PNG files, opens a dispute with both IDs, and both participants can read the bound content while no object key appears in events or notifications.
- Base: an upload is never attached; after 24 hours cleanup claims and destroys it without affecting an order or dispute.
- Good: an administrator quarantines one bound image; it becomes unreadable immediately and is physically destroyed after the seven-day quarantine period.
- Bad: the server fully decodes a compressed image before checking declared dimensions, or returns a participant content URL in an administrator DTO.
- Bad: a seller decision commits and evidence binding then fails, leaving the decision without the evidence the request claimed to attach.

### 6. Tests Required

- Image unit tests cover JPEG/PNG/WebP normalization, QR rejection, byte limits, and oversized declared dimensions rejected before full pixel decode.
- Service/handler tests cover session, CSRF, strict multipart fields, exact redaction confirmation, participant ownership, no-store content, administrator route projection, quarantine version/idempotency, and unavailable object-storage capability.
- PostgreSQL integration tests cover one-time atomic binding for every usage, source/dispute ownership, visibility classes, immediate quarantine denial, lifecycle claims, object-key non-disclosure, and rollback after a rejected business action.
- Migration tests cover up/down/up for evidence assets and bindings, typed-source constraints, cleanup indexes, and current expected version 108.
- OpenAPI generation and route checks must keep upload fields, `2..800` quarantine reason, response paths, generated requiredness, and registered routes aligned.

### 7. Wrong vs Correct

#### Wrong

```text
decode pixels -> check dimensions -> write seller decision -> bind evidence
admin DTO -> /api/v1/me/dispute-evidence/{id}/content
quarantine -> content remains readable until cleanup
```

#### Correct

```text
decode header -> check dimensions -> decode/re-encode pixels
business transaction -> validate source and all assets -> write source + bindings atomically
admin DTO -> /api/v1/admin/dispute-evidence/{id}/content
quarantine -> deny reads immediately -> destroy after bounded retention
```

## Scenario: API-Order History, Remedy Progress, Lateness, And Appeals Are Independent Facts

### 1. Scope / Trigger

- Trigger: changing dispute creation/closure, settlement acceptance, remedy actions, lateness decisions, finality, appeals, reputation reversal, or the API-order dispute projection.
- The case history, current active projection, remedy progress, lateness decision, commercial result, and appeal reversal are separate facts. No one field may stand in for another.

### 2. Signatures

```text
dispute_cases:
  api_order_id uuid
  active boolean
  final_reason text
  appeal_expires_at timestamptz
  adversely_affected_user_ids uuid[]

api_orders:
  dispute_case_id uuid nullable          # active projection
  latest_dispute_case_id uuid nullable   # newest historical case

api_order_dispute_remedies.status:
  pending | claimed_fulfilled | confirmed | contested |
  confirmation_expired | cancelled

api_order_dispute_remedies.lateness_status:
  not_due | on_time | late_unreviewed | late_confirmed | late_excused

POST /api/v1/admin/disputes/{id}/remedy/confirm-lateness
POST /api/v1/admin/disputes/{id}/remedy/excuse-lateness

appeal window = finalization time + exactly 30 days
```

### 3. Contracts

- A database partial unique index permits at most one `active=true` case per API order. Closed cases remain immutable history; closure clears `api_orders.dispute_case_id`, keeps `latest_dispute_case_id`, and permits a later independent case while after-sales eligibility remains open.
- An administrator decision or accepted actionable settlement creates an append-only remedy. If an initial order credential already exists, a continuation remedy cannot submit, replace, revoke, or version another credential.
- Remedy fulfillment progress and lateness are independent. Reaching `due_at` records `late_unreviewed`; an administrator may set `late_confirmed` or `late_excused` without changing `remedy.status`, closing the dispute, changing the commercial outcome, or blocking a later fulfillment claim.
- Only an unreversed `late_confirmed` fact may support responsibility, reputation, or a seller sanction. `late_unreviewed` and `late_excused` are never penalty inputs.
- Finalization writes a non-empty `final_reason`, `appeal_expires_at`, and the exact `adversely_affected_user_ids`. Only a listed user may appeal, and only while `now < appeal_expires_at`.
- Appeal approval reverses only records whose subject is the appellant: dispute outcomes, linked restrictions, and that appellant's confirmed-lateness fact. It does not rewrite the original dispute/remedy timeline or another affected user's records.
- Lateness reversal appends a private dispute-level event with `entity_type=dispute` and `action=remedy_lateness_reversed`. `dispute_events.entity_type=remedy` is invalid under the existing schema.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| A second active case is created for one API order | `409 INVALID_STATE_TRANSITION` or unique-conflict mapping; no partial writes |
| Lateness decision is requested before `due_at` or without `late_unreviewed` | `409 INVALID_STATE_TRANSITION` |
| `confirm_lateness` / `excuse_lateness` succeeds | Only lateness fields, audit/event facts, and version change; remedy progress and case finality remain unchanged |
| Responsible party claims after `due_at` | Claim remains allowed; progress becomes `claimed_fulfilled`, while confirmed/excused lateness remains unchanged |
| User is absent from `adversely_affected_user_ids` | Appeal creation/approval is rejected without revealing another subject's records |
| Appeal is submitted at or after `appeal_expires_at` | `409 INVALID_STATE_TRANSITION` |
| Approved appellant has no matching outcome/restriction/lateness row | Idempotent directed no-op for that record class; never select another subject's row |

### 5. Good / Base / Bad Cases

- Good: an administrator orders a refund, creates a pending remedy, confirms it late, and the responsible party later claims fulfillment; lateness remains a separate reviewed fact until an appellant-specific approval reverses it.
- Base: a no-action mutual agreement closes the active case, clears the active order projection, and preserves the case through `latest_dispute_case_id`.
- Bad: confirming lateness changes `remedy.status=cancelled`, closes the case, or sets a commercial result.
- Bad: appeal approval loads the first outcome for the case without filtering `subject_user_id=appellant_user_id`.

### 6. Tests Required

- Migration tests cover the one-active partial unique index, history projection, progress/lateness constraints, 30-day finality fields, directed reversal fields, and executable down/up paths.
- Service tests cover actionable settlement remedy creation, no second credential, exact due/appeal boundaries, late claim after confirmation/excusal, and progress/finality preservation.
- PostgreSQL integration covers one active plus multiple historical cases, appellant-filtered reversal for multi-subject cases, sanctions excluding reversed lateness, and the valid dispute event entity/action pair.
- Cross-layer tests cover OpenAPI route/type parity, administrator controls, participant history display, and stable error mapping.
- Run full Go test/vet, PostgreSQL migration/integration gate, full Vitest, Nuxt typecheck/build, OpenAPI generated-type/route checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
confirm_lateness -> remedy.status=cancelled -> dispute closed
approve appeal -> reverse first outcome for dispute
event entity_type=remedy
```

#### Correct

```text
confirm_lateness -> lateness_status=late_confirmed only
late fulfillment claim -> status=claimed_fulfilled, lateness unchanged
approve appeal -> reverse rows WHERE subject_user_id=appellant_user_id
event entity_type=dispute, action=remedy_lateness_reversed
```

## Scenario: Confirmed Late Remedies Produce Explicit Tiered Seller Sanctions

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

- Eligibility requires an API-order dispute, its latest remedy with unreversed `lateness_status=late_confirmed`, an active `responsible|shared` outcome, and the same user as outcome subject, remedy responsible party, and order seller.
- The 180-day count uses `lateness_decided_at >= now - 180 days`, includes the current qualifying fact, and counts only unreversed `late_confirmed` remedies whose responsible user is the linked API-order seller. Appeal-reversed lateness facts remain in history but are excluded from recommendation and breach counts.
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
| Non-API dispute or no latest unreversed `late_confirmed` remedy | Recommendation is ineligible with a granular reason code; apply returns `409 INVALID_STATE_TRANSITION` |
| Missing/inactive/non-fault outcome | Recommendation is ineligible; no restriction, event, notification, or completed idempotency result |
| Subject, remedy responsible party, and order seller differ | Recommendation is ineligible with `responsible_seller_required` |
| Missing or stale subject-user `If-Match` | `428 PRECONDITION_REQUIRED` or `412 VERSION_CONFLICT` |
| Internal reason is outside 2..2000 characters | `422 VALIDATION_FAILED` on `internalReason` |
| Same remedy is submitted again | Exact completed replay returns the original response; any second sanction attempt creates no new restriction |
| Seller has an active matching restriction during new order creation | `403 REPUTATION_ACTION_RESTRICTED`; inventory, intent, contact, and order state remain unchanged |
| Restriction is expired or revoked | New order, submission, publication, and restoration gates allow the action again |

### 5. Good / Base / Bad Cases

- Good: a seller's second unreversed administrator-confirmed late remedy inside 180 days recommends 30 days; an administrator confirms once, and the restriction, user version, event, notification, and idempotency completion commit atomically.
- Good: the seller is restricted after an order already exists. New normal and quota orders fail, while the existing buyer can still pay and the seller can still deliver and handle the dispute.
- Base: an administrator rules the buyer request invalid, excuses lateness, reverses the lateness on appeal, or the responsible party fulfills on time. No eligible `late_confirmed` fact exists, so no sanction recommendation is eligible.
- Bad: count an unresolved complaint, a due timestamp alone, or a beneficiary confirmation timeout as a seller breach.
- Bad: create the restriction automatically when `confirm_lateness` runs, count appeal-reversed lateness, accept a remedy ID through the generic restriction form, or show internal governance fields on the seller reputation page.

### 6. Tests Required

- Migration tests: current expected version 108, `ON DELETE RESTRICT`, remedy-source partial uniqueness, the unreversed `late_confirmed` lookup index, reversal fields, and executable down/up migration paths.
- Service tests: every ineligible reason, exact 180-day boundary, outside-window exclusion, `0/1/2/3+` tier mapping, historical-fact counting, subject/seller parity, duplicate remedy application, and in-memory/PostgreSQL parity.
- Store tests: repeatable-read recommendation snapshot, apply lock/revalidation order, unique conflict, and atomic restriction/event/notification/idempotency writes.
- Order tests: both normal and limited-quota restriction gates precede contact, inventory, intent, and order side effects; existing-order mutations have no added gate.
- Route/OpenAPI tests: administrator authority, CSRF, idempotency, subject-user `If-Match`, fresh `ETag`, dedicated route parity, generic request exclusion, and generated types.
- Frontend tests: recommendation/refetch flow, explicit confirmation, conflict reset, accurate three-ability copy, public active-restriction projection, and no internal-field leakage.
- Gates: full Go test/vet, full Vitest, Nuxt typecheck and real-mode production build, migration/OpenAPI/generated-type checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
confirm remedy lateness -> automatically create a 30-day restriction
generic restriction request -> accepts sourceDisputeRemedyId
restriction copy -> "only stops new orders"
new quota order -> claim inventory -> check seller restriction
```

#### Correct

```text
confirm remedy lateness -> durable sanction evidence only
GET recommendation -> one repeatable-read snapshot
administrator POST -> revalidate -> explicit fixed-tier restriction
restriction copy -> "limits new orders, publishing, and restoring; existing orders continue"
new quota order -> check seller restriction -> claim inventory -> create order
```

## Scenario: Active Disputes Produce Tiered Commerce Restrictions And Live Action Notifications

### 1. Scope / Trigger

- Trigger: changing seller API-service publication, restoration, order creation, dispute aggregation, participant deadlines, notification classification, or the seller commerce-status UI.
- Temporary dispute restrictions protect new commerce only. They are calculated from current API-order dispute facts and remain independent from administrator-created `user_restrictions`.

### 2. Signatures

```text
GET /api/v1/owner/api-orders/commerce-status

CommerceRestrictionLevel:
  normal | service_limited | account_limited

CommerceReasonCode:
  service_multiple_buyers | seller_response_overdue |
  account_multiple_buyers | remedy_fulfillment_overdue

SellerCommerceStatus:
  level, activeDisputeCount, activeBuyerCount,
  blockingDisputeCount, affectedServiceIds[], reasonCodes[],
  disputes[], nextReleaseAt?

apiorder.EvaluateSellerCommerce(facts, now) SellerCommerceStatus
SellerCommerceStatus.BlocksService(apiServiceID) bool

notification.Notification:
  ActionRequired bool
  ActionDueAt *time.Time
```

### 3. Contracts

- Every active dispute freezes only its linked order. One ordinary dispute, one seller rejection awaiting the buyer, one claimed remedy awaiting buyer confirmation, or one buyer request for platform review does not by itself block unrelated commerce.
- Active buyer counts are distinct by buyer user ID. Two distinct buyers with active disputes on the same API service produce `service_limited` for that service only. Three distinct buyers across the seller account produce `account_limited` for every service.
- A seller-response deadline reached in `pending_seller_response` produces `service_limited` for the linked service. A fulfillment deadline reached in `awaiting_fulfillment` produces `account_limited`.
- Closed, withdrawn, self-resolved, neutral-expiry, and incomplete facts do not count. Recalculation immediately lowers or clears temporary restrictions when active facts fall below a threshold.
- Publication, restoration, opening sales, fixed-quota publication, and new normal/quota order creation call the same target-service-aware backend decision. Draft edits, reads, and fulfillment of already-created orders remain available.
- Formal reputation restrictions are checked independently and keep their existing error code. Closing an ordinary dispute never removes or bypasses an administrator restriction.
- Buyer decision and remedy-confirmation windows are exactly 24 hours. A reminder may be emitted once when no more than two hours remain. Expiry closes neutrally and must not claim buyer confirmation or platform verification of off-platform payment or delivery.
- `dispute.remedy_claimed` and `dispute.remedy_confirmation_due` project as `交易待办` only while the current notification recipient is the beneficiary of an active `claimed_fulfilled` remedy and `now < confirmation_due_at`. After confirmation, contest, expiry, or the exact deadline, the historical notification projects as `交易通知` even if it remains unread.
- Notification read state is not action-completion state. PostgreSQL derives `ActionRequired` from current dispute/remedy rows on every notification read; in-memory mode clears the corresponding action flag on transition and at its deadline.
- Dispute notifications link to `/my/disputes/{disputeId}`. Seller pages show the returned restriction level, reasons, affected services, cases, action owner, and deadline; they do not infer sellability from labels.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| One active ordinary dispute or one active buyer | Linked order remains frozen; other sales remain enabled |
| Same buyer opens multiple active disputes | Count once for account and once per affected service |
| Two distinct buyers affect one service | `service_limited`; only that service is blocked |
| Three distinct buyers affect any seller services | `account_limited`; all new API commerce is blocked |
| Seller response is overdue | Linked service is blocked; unrelated services remain enabled |
| Seller remedy fulfillment is overdue | Entire seller account is blocked from new API commerce |
| Buyer requests platform review once | Case enters review; no new account/service restriction from that action alone |
| Buyer confirms, contests, or reaches confirmation deadline | Prior confirmation notification is no longer `交易待办` |
| Temporary restriction and formal reputation restriction overlap | Both remain independently effective; ordinary closure clears only the temporary calculation |

### 5. Good / Base / Bad Cases

- Good: the seller submits complete handling evidence, the case waits for the buyer, and all unrelated services continue accepting orders while the seller sees that no action is currently required.
- Good: two different buyers dispute one API service, so only that service is paused; after one case closes, recalculation restores it.
- Base: one buyer has three active orders in dispute. The seller sees all three cases, but the account buyer count is one and no threshold restriction is created solely from repetition.
- Bad: classify every unread remedy notification as a current to-do, leaving completed confirmation work pinned in the high-priority queue.
- Bad: let the browser count dispute rows or decide whether a service can sell, or let one review-button click pause the entire seller account.

### 6. Tests Required

- Pure aggregation tests cover single ordinary disputes, same-buyer de-duplication, two buyers on one service, three buyers across services, seller-response overdue, fulfillment overdue, and closed/incomplete exclusion.
- PostgreSQL/store tests assert aggregation uses real order ownership and service IDs and every publication/order gate calls the shared status decision before side effects.
- Notification tests assert actionable and completed category projection, recipient/remedy matching, exact-deadline expiry, unrelated-dispute isolation, reminder idempotency, and the `/my/disputes/{id}` route.
- Handler/OpenAPI tests cover the commerce-status route, response fields, session requirement, generated types, and route parity.
- Frontend tests cover seller status rendering, publication controls, role-specific dispute filters/actions, mobile confirmation controls, and explicit off-platform verification disclaimers.
- Gates: full Go test/vet, full Vitest, Nuxt typecheck and real-mode build, OpenAPI generated-type/route checks, desktop/mobile browser inspection, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
has any active dispute -> block every seller service
notification unread=true -> transaction to-do
buyer clicked platform review -> block seller account
```

#### Correct

```text
active dispute facts -> distinct buyers per service/account -> highest tier
current claimed remedy + matching beneficiary + before deadline -> transaction to-do
historical event after confirm/contest/expiry -> transaction notice
formal restriction -> independent reputation gate
```
