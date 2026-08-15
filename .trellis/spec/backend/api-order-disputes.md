# API Order Dispute Lifecycle

Date: 2026-08-09
Author: Codex
Updated: 2026-08-15

## Scenario: Dispute Phase Projection Remains Separate From Order Fulfillment

### 1. Scope / Trigger

- Trigger: changing API order dispute creation, administrator resolution, order DTOs, dispute status constraints, order completion gates, or participant/admin dispute presentation.
- The dispute case is the moderation record. `api_orders.dispute_status` is the participant-facing order projection. Neither field replaces `api_orders.status`.

### 2. Signatures

```text
dispute_cases.status:
  open | waiting_info | resolved | closed | withdrawn | self_resolved

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
- `open` means platform review. New finalization clears the active projection to `none` and preserves the case through `latest_dispute_case_id`; `closed` remains a legacy read value only. Reserved remediation phases must not be rendered as platform review.
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

- Migration test: both check constraints contain the documented values and migration 107 rewrites only the legacy projection fields required for direct platform handling.
- API order unit test: close convergence changes only projection metadata, increments once, and keeps event transaction statuses unchanged.
- Route regression: after administrator resolution, buyer and seller detail DTOs both clear the active case, retain the latest case/history projection, and keep the order status unchanged.
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
- Opening the completed-order dispute writes `issue_occurred_at`, creates the direct platform-handling record, and updates only the dispute projection. The completed transaction status, completion source, credential, payment, delivery, and validity facts remain unchanged.
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

## Scenario: API-Order Disputes Enter Platform Handling Directly

### 1. Scope / Trigger

- Trigger: changing API-order dispute creation, participant response/closure actions, directed supplements, order-list dispute projections, or legacy negotiation presentation.
- This workflow applies only to `target_type=api_order`. Buyer and seller may communicate through WeChat, email, or linux.do, but the platform does not record which external channel they used or require proof that negotiation ended.

### 2. Signatures

```text
POST /api/v1/me/api-orders/{id}/dispute
POST /api/v1/owner/api-orders/{id}/dispute
GET  /api/v1/me/disputes/{id}
POST /api/v1/me/disputes/{id}/response
POST /api/v1/me/disputes/{id}/withdraw
POST /api/v1/me/disputes/{id}/self-resolve
POST /api/v1/me/disputes/{id}/supplements
POST /api/v1/me/disputes/{id}/remedy/claim
POST /api/v1/me/disputes/{id}/remedy/confirm
POST /api/v1/me/disputes/{id}/remedy/contest

dispute_cases.status:
  open | waiting_info | resolved | closed | withdrawn | self_resolved

dispute_cases:
  applicant_statement text
  respondent_response text nullable
  responded_at timestamptz nullable
  next_actor text
  next_user_id uuid nullable
  due_at timestamptz nullable
  fact_snapshot jsonb

participant order list projection:
  disputeStatus, disputeCaseId, disputeNextActor,
  disputeDueAt, disputeNeedsAction, disputeRemedyAction
```

### 3. Contracts

- Opening requires `issueCode`, `requestedResolution`, a credential-free `reason`, and `requestedAmountCny` only for `partial_refund`. It atomically creates `status=open`, freezes the reason as `applicant_statement`, records an immutable `fact_snapshot`, and projects `next_actor=respondent` with `due_at=opened_at+48h`. It never changes `api_orders.status`.
- The respondent may submit one immutable formal response with optional private image evidence. Success records `respondent_response/responded_at` and moves only the action projection to `next_actor=admin`, `due_at=NULL`; a second response is rejected.
- Before an administrator ruling, only the applicant may end the case. `withdraw` records an applicant withdrawal; `self-resolve` records that the parties resolved the issue offline. Neither outcome assigns responsibility or adverse reputation facts.
- `waiting_info` targets exactly one participant through `next_actor`, `next_user_id`, and a 48-hour `due_at`. Only that participant may submit the requested supplement. Completion returns the case to `open`, `next_actor=admin`, and clears the deadline.
- Platform rulings, remedy fulfillment, counterparty confirmation/contest, lateness review, and appeals remain available. Participant response, withdrawal, and self-resolution do not bypass an existing ruling or remedy.
- New message, settlement-proposal, proposal-confirm/reject, and escalation routes do not exist. Historical rows in `api_order_dispute_messages` and `api_order_dispute_settlement_proposals` remain readable in the participant detail as legacy history and have no write controls.
- Buyer and seller order lists keep fulfillment `status` unchanged and render the dispute as a second status. `disputeNeedsAction` is true only when the current viewer matches the action owner; every linked case exposes an independent case-page route.
- In-memory and PostgreSQL flows must produce the same `next_actor`, deadline, remedy action, and viewer-specific needs-action projection. Changing only action/deadline metadata must not increment the API-order business version.
- Participant reads return `404 OBJECT_NOT_FOUND` to outsiders. Every mutation authenticates and validates CSRF before decoding JSON, and idempotent writes complete with the business transaction.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Missing/unknown issue or requested resolution | `422 VALIDATION_FAILED` on the matching field |
| Invalid or excessive partial-refund amount | `422 VALIDATION_FAILED` on `requestedAmountCny` |
| Applicant statement, response, supplement, or reason contains a secret | `422 SECRET_CONTENT_DETECTED` |
| Outsider or unsupported non-API dispute mutation | `404 OBJECT_NOT_FOUND` |
| Applicant attempts the respondent response, or respondent answers twice | `409 INVALID_STATE_TRANSITION`; response fields remain unchanged |
| Respondent attempts withdraw/self-resolve | `409 INVALID_STATE_TRANSITION` |
| Applicant attempts withdraw/self-resolve after ruling/remedy creation | `409 INVALID_STATE_TRANSITION` |
| Non-targeted participant submits a directed supplement | `409 INVALID_STATE_TRANSITION`; actor and deadline remain unchanged |
| Legacy message/proposal/escalation route is called | `404`; no legacy row or event is written |
| Same idempotency key and request hash is replayed | Return the original result without another event or version increment |

### 5. Good / Base / Bad Cases

- Good: a buyer opens a dispute; both order lists keep the original fulfillment status, show a dispute badge, and only the seller sees “纠纷待你处理” until the one formal response is submitted.
- Good: the administrator requests evidence from the buyer; the buyer receives a 48-hour deadline, submits text and images, and the next actor returns to the administrator.
- Base: the applicant confirms a WeChat refund and chooses `self-resolve`; the case remains in history without a platform responsibility finding.
- Base: an upgraded case still displays old messages and proposals, but neither participant can add a message, submit a proposal, or request escalation.
- Bad: opening creates `negotiating`, asks which chat application was used, or requires the applicant to certify that negotiation failed.
- Bad: the UI changes the order fulfillment status to `disputed`, or shows both participants that the same action is waiting on them.

### 6. Tests Required

- API-order and report unit tests cover direct-open creation, 48-hour respondent deadline, immutable fact snapshot, one formal response, applicant-only closure, targeted supplements, remedy transitions, and in-memory projection parity.
- Router tests cover buyer/seller creation, outsider `404`, authentication-before-decode, second-response conflict, applicant closure, legacy mutation route `404`, administrator ruling, and unchanged order fulfillment status.
- Migration tests execute 106->107, 107 down, and 107 up; verify legacy negotiating/waiting/remedy mappings; and verify down migration refuses to discard new-flow data.
- PostgreSQL integration scans both buyer and seller order-list queries with actual rows and validates actor/deadline/needs-action projections.
- Frontend tests cover all six case labels, independent list badges/filters/links, one response with image evidence, applicant closure, directed supplement, remedy actions, and read-only legacy history.
- Gates: full Go test/vet, full Vitest, Nuxt typecheck/build, OpenAPI regeneration, migration documentation, browser smoke checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
create -> negotiating -> messages/proposals -> confirm negotiation ended -> open
order.status = disputed
```

#### Correct

```text
create -> open, next_actor=respondent, due_at=opened_at+48h
response -> open, next_actor=admin, due_at=NULL
request info -> waiting_info, targeted participant, due_at=now+48h
supplement -> open, next_actor=admin, due_at=NULL
order.status remains the payment/delivery lifecycle status
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
- Initial dispute and formal-response evidence is visible to both order participants and administrators. Directed supplement evidence is visible only to its submitter and administrators. Appeal evidence is visible only to the appellant and administrators. Evidence already bound to legacy messages or escalation remains readable under its original visibility.
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
- Bad: a formal response commits and evidence binding then fails, leaving the response without the evidence the request claimed to attach.

### 6. Tests Required

- Image unit tests cover JPEG/PNG/WebP normalization, QR rejection, byte limits, and oversized declared dimensions rejected before full pixel decode.
- Service/handler tests cover session, CSRF, strict multipart fields, exact redaction confirmation, participant ownership, no-store content, administrator route projection, quarantine version/idempotency, and unavailable object-storage capability.
- PostgreSQL integration tests cover one-time atomic binding for every usage, source/dispute ownership, visibility classes, immediate quarantine denial, lifecycle claims, object-key non-disclosure, and rollback after a rejected business action.
- Migration tests cover up/down/up for evidence assets and bindings, typed-source constraints, cleanup indexes, and current expected version 107.
- OpenAPI generation and route checks must keep upload fields, `2..800` quarantine reason, response paths, generated requiredness, and registered routes aligned.

### 7. Wrong vs Correct

#### Wrong

```text
decode pixels -> check dimensions -> write formal response -> bind evidence
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

- Migration tests: current expected version 107, `ON DELETE RESTRICT`, remedy-source partial uniqueness, the unreversed `late_confirmed` lookup index, reversal fields, and executable down/up migration paths.
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
