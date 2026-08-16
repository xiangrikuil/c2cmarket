# Reputation Facts

Date: 2026-07-24
Executor: Codex
Updated: 2026-08-15

## Scenario: Truthful Reputation Facts And Transaction Exclusions

### 1. Scope / Trigger

- Trigger: backend, PostgreSQL, OpenAPI, or profile work that reads transaction history for reputation, exposes reputation facts, or excludes/restores a transaction from reputation calculation.
- This contract owns raw facts only. Tier, state, confidence, scoring rules, reviews, dispute responsibility, and account restrictions are separate reputation contracts.

### 2. Signatures

```text
reputation.Repository.AggregateFacts(ctx, userIDs, now)
  -> map[userID]RawFacts

reputation.Service.ExcludeTransaction(ctx, adminActor, input)
  -> TransactionExclusion

reputation.Service.RestoreTransaction(ctx, adminActor, input)
  -> TransactionExclusion

RawFacts:
  buyer|seller:
    carpool|api|overall:
      completedCount
      completedCountLast90Days
      roleResponsibilityCancellationCount
      unknownResponsibilityCancellationCount
      unresolvedDisputeCount
      sourceDataUpdatedAt

PostgreSQL:
  reputation_transaction_exclusions
  reputation_transaction_exclusion_events
```

### 3. Contracts

- Carpool completion comes only from `carpool_memberships.status='completed'`. API normal-completion facts come only from `api_orders.commercial_outcome='normal_fulfillment'`; refund, partial refund, continued fulfillment, pending, and unverified closure must not inflate normal completion counts.
- Purchase intents, accepted applications, payment submission, delivery submission, or other intermediate states must not be inferred as completed transactions.
- Buyer and seller facts remain separate. Carpool and API facts remain separate. `overall` is the service-layer sum of the two business scopes for one role.
- The recent completion window is 90 days from the repository `now` argument. Compatibility DTO field names must not change the calculation window.
- A responsibility cancellation is counted only when durable status/event data identifies that role as responsible. The system executor and the business-responsible participant are separate concepts; a system-created timeout event must not be rewritten with a participant actor.
- An API order cancelled with `cancel_reason='payment_timeout'` is buyer responsibility even though `api_order.payment_timeout_cancelled` has no actor. The seller receives neither a responsible nor an unknown cancellation fact for that order.
- An expired `accepted_reserved` carpool application uses `carpool_join_confirmations` as responsibility evidence. A missing buyer confirmation is buyer responsibility, a missing owner confirmation is seller responsibility, and both missing confirmations create one responsibility cancellation for each role. A participant who confirmed is not affected.
- Historical cancellation without durable status, event, reason, or confirmation evidence increments the unknown count and must not be guessed. An impossible expired carpool application with both confirmations is also unknown for both participants.
- Unresolved disputes are `open|waiting_info` cases mapped through the actual transaction participants. A dispute is not a responsibility decision.
- An active transaction exclusion removes all facts from that transaction for both participants. Restore makes the facts eligible again without rewriting the transaction terminal state.
- Exclusion events are append-only and record administrator, action, reason code, reason, and time. Restore updates the current exclusion row and appends a new event.
- List/profile callers pass all user IDs to one repository call. Per-row reputation queries are forbidden.
- Public response fields are nullable when the fact source is unavailable or not yet calculated. A fixed zero is allowed only when the repository successfully proves no matching facts.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Empty/duplicate user IDs | Trim, de-duplicate, and return an empty map when no IDs remain |
| Non-admin exclusion/restore | `403 PERMISSION_DENIED` |
| Unsupported transaction type | `422 VALIDATION_FAILED`, field `transactionType` |
| Non-UUID transaction ID | `422 VALIDATION_FAILED`, field `transactionId` |
| Invalid reason code | `422 VALIDATION_FAILED`, field `reasonCode` |
| Empty reason | `422 VALIDATION_FAILED`, field `reason` |
| Restore without active exclusion | `409 INVALID_STATE_TRANSITION` |
| Required reputation repository unavailable for a mutation | `500 INTERNAL_ERROR`, never fake success |

### 5. Good/Base/Bad Cases

- Good: a completed carpool membership contributes one buyer/carpool completion and one seller/carpool completion; only an API order with `commercial_outcome=normal_fulfillment` contributes API normal-completion facts.
- Good: an administrator excludes a disputed API order, both participants lose that order's facts, and restore makes them visible again while preserving two audit events.
- Base: a requested user has no matching terminal transactions; a successful batch query returns explicit zero facts for every role/scope.
- Bad: count a purchase intent as an API completion, assign every expired reservation to the buyer without checking confirmations, use the system timeout executor as the event actor, or run one SQL query per profile row.

### 6. Tests Required

- Unit tests must cover empty input, duplicate IDs, role/scope merge, 90-day window behavior, unknown cancellation, API payment-timeout responsibility, role-specific carpool confirmation expiry, and exclusion validation.
- Repository SQL tests must assert terminal predicates, participant joins, the shared cumulative/window responsibility matrix, active-exclusion predicates, and one UUID-array batch parameter.
- PostgreSQL integration must apply the complete migration chain, aggregate an empty user, exclude/restore a transaction, and prove exclusion events reject update/delete.
- Profile/DTO tests must prove repository-backed zero is distinct from unavailable `null`.
- Run `go test -count=1 ./...`, OpenAPI YAML parsing, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
for _, userID := range userIDs {
    profile.Reputation = queryReputation(userID)
}
```

```sql
SELECT count(*) FROM api_purchase_intents WHERE status = 'ordered';
```

#### Correct

```go
facts, appErr := reputationService.AggregateFacts(ctx, userIDs)
```

```sql
SELECT ... FROM api_orders
WHERE commercial_outcome = 'normal_fulfillment'
  AND (exclusion.id IS NULL OR exclusion.restored_at IS NOT NULL);
```

#### Wrong

```sql
-- A system-created timeout event has no participant actor, so this makes the
-- cancellation unknown for both roles.
WHERE event.event_type = 'api_order.cancelled'
  AND event.actor_user_id = participants.user_id;
```

```sql
-- Expiry alone does not prove which participant missed confirmation.
WHERE application.status = 'expired'
  AND participants.role = 'buyer';
```

#### Correct

```sql
WHERE cancellation.actor_cancelled
   OR (
     participants.role = 'buyer'
     AND api_order.cancel_reason = 'payment_timeout'
   );
```

```sql
WHERE application.status = 'expired'
  AND (
    (participants.role = 'buyer' AND NOT confirmations.buyer_confirmed)
    OR (participants.role = 'seller' AND NOT confirmations.owner_confirmed)
  );
```

## Scenario: Dispute Outcomes And Role-Scoped Action Restrictions

### 1. Scope / Trigger

- Trigger: backend, PostgreSQL, OpenAPI, authentication, contact disclosure, review submission, or transaction work that assigns dispute responsibility or blocks a user action for reputation reasons.
- Governance owns reversible dispute outcomes and role/action restrictions. Account activation remains an authentication contract, and tier calculation remains a separate reputation-engine contract.

### 2. Signatures

```text
POST /api/v1/admin/disputes/{id}/reputation-outcome
POST /api/v1/admin/users/{id}/reputation-restrictions
POST /api/v1/admin/reputation-restrictions/{id}/revoke

Required mutation headers:
  Cookie: c2c_session=<admin session>
  X-CSRF-Token: <session token>
  Idempotency-Key: <opaque key>
  If-Match: "<version>"

reputation.Service.CheckActionAllowed(ctx, userID, role, action)
  -> nil | REPUTATION_ACTION_RESTRICTED

PostgreSQL:
  dispute_cases.subject_user_id
  dispute_reputation_outcomes
  user_restrictions
  reputation_governance_events
```

`role` is `buyer|seller`. A stored `role_scope` may also be `all`.

`action` is one of:

```text
carpool_publish
carpool_apply
carpool_accept
api_service_publish
api_order_create
contact_view
review_submit
```

A stored `action_code` may also be `all`.

### 3. Contracts

- `dispute_cases.subject_user_id` is the only unresolved-dispute reputation subject. Reporters, witnesses, administrators, and other participants must not receive caution merely because they are linked to the case.
- An unresolved `open|waiting_info` dispute is caution evidence only. It cannot create a responsibility outcome or an action restriction by itself.
- A dispute outcome may be created only for a resolved dispute and is unique per dispute. It records `subject_user_id`, `responsibility`, `severity`, `role_scope`, administrator, public/internal reasons, and a version.
- Outcome creation uses the dispute version from `If-Match`, updates the dispute subject, appends an audit event, and completes idempotency in the same PostgreSQL transaction.
- Restriction creation uses the target user's version from `If-Match`. Revocation uses the restriction version. Successful responses and exact idempotent replays return the corresponding fresh `ETag`.
- A restriction is active only when all conditions hold:

```text
restriction.user_id == userID
restriction.role_scope in {role, "all"}
restriction.action_code in {action, "all"}
restriction.starts_at <= now
restriction.ends_at is null OR now < restriction.ends_at
restriction.revoked_at is null
```

- Action checks run before the protected side effect. Contact checks run before decrypting or returning contact values and before inserting a contact access log.
- `contact_view` resolves the current viewer as buyer or seller from the resource relationship; it must not use one fixed role for every viewer.
- Creating or publishing a carpool, applying, accepting, publishing an API service, creating an API order, viewing participant contacts, and submitting a review must call the shared action contract with the matching role/action pair.
- Approving an appeal reverses only the appellant-subject outcome, restrictions sourced from that outcome, and any appellant-owned confirmed-lateness fact, in the same transaction as the appeal state change. Multi-subject cases must filter every lookup by `subject_user_id=appellant_user_id`; reversal must never select an arbitrary case-level row.
- Dispute appeal authorization and reputation outcome creation serialize on the dispute row. A submitted or approved appeal blocks creating a later outcome, so an appeal cannot be invalidated by changing the dispute subject or by reapplying a reversed responsibility decision.
- `reputation_governance_events` is append-only. Administrators create outcomes/restrictions or reverse/revoke them through domain actions; there is no API that directly edits a user's reputation tier.
- Password login, OAuth callback, and existing session validation independently reject any account whose `account_status` is not `active`. Account status must not be inferred from reputation restrictions.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Missing/non-admin session or invalid CSRF | `401`/`403` Problem Details before mutation |
| Missing `Idempotency-Key` | `400 VALIDATION_FAILED` |
| Same key with a different request hash | `409 IDEMPOTENCY_KEY_REUSED` |
| Missing `If-Match` | `428 PRECONDITION_REQUIRED` |
| Stale dispute/user/restriction version | `412 VERSION_CONFLICT` |
| Outcome requested for unresolved dispute | `409 INVALID_STATE_TRANSITION` |
| Duplicate outcome for one dispute | `409 INVALID_STATE_TRANSITION` |
| Unknown role, action, responsibility, severity, or invalid period | `422 VALIDATION_FAILED` with the matching field |
| Protected action matches an active restriction | `403 REPUTATION_ACTION_RESTRICTED` with `public_reason` |
| Restriction is expired, not started, revoked, or belongs to another role/action | Allow the action |
| Appeal appellant is not the outcome/restriction/lateness subject | Do not reverse that row; never fall back to another subject in the case |
| Appellant-owned lateness was already reversed | Idempotent no-op; retain the original timeline and reversal audit |
| Password, OAuth, or session user is not `active` | `403 ACCOUNT_RESTRICTED`; do not create/continue a session |
| Update/delete of a governance event | PostgreSQL rejects the mutation |

### 5. Good/Base/Bad Cases

- Good: a resolved dispute receives one seller/high outcome, a seller `api_service_publish` restriction blocks only that action, and an approved appeal reverses both records while preserving their history.
- Good: a multi-subject case has buyer and seller outcomes; approving the buyer's appeal reverses only buyer-owned outcome/restriction/lateness records.
- Good: a restricted buyer requests a contact window; the API returns `403`, contains no plaintext contact value, and does not append a contact access log.
- Base: an unresolved dispute with a subject increments caution facts but does not block either role. A time-limited restriction stops applying exactly at `ends_at`.
- Bad: penalize a report author because they appear in `primary_user_id`, apply a buyer restriction to seller actions, or decrypt contacts before checking the restriction.

### 6. Tests Required

- Service tests must cover exact role/action matching, `all` wildcards, start/end boundaries, manual revocation, admin validation, and idempotent replay `ETag`.
- Core/contact tests must cover every protected action call site and prove contact disclosure checks happen before plaintext response and audit insertion.
- Authentication tests must cover non-active OAuth, password, and existing-session paths.
- Router tests must cover admin authority, CSRF, `Idempotency-Key`, `If-Match`, stale versions, and the three OpenAPI routes.
- PostgreSQL integration must apply the complete migration chain through the current expected version and prove unresolved-outcome rejection, active/expired/revoked restrictions, appellant-filtered multi-subject appeal reversal, reversed lateness exclusion, valid dispute-event entity types, append-only events, and restricted contact non-disclosure.
- Run `go test -count=1 ./...`, `go vet ./...`, OpenAPI YAML parsing, runtime/OpenAPI route parity, migration-doc validation, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
if dispute.PrimaryUserID == userID {
    blockEveryTransaction(userID)
}
contacts := decryptContactValues(session)
checkRestriction(userID)

outcome := loadFirstOutcomeForCase(dispute.ID)
reverse(outcome)
```

#### Correct

```go
if appErr := reputationService.CheckActionAllowed(
    ctx,
    userID,
    resolvedRole,
    reputation.ActionContactView,
); appErr != nil {
    return nil, appErr
}
contacts := decryptContactValues(session)

outcome := loadActiveOutcome(dispute.ID, appeal.AppellantUserID)
reverseOnlyAppellantOwnedFacts(outcome, appeal.AppellantUserID)
```

## Scenario: Verified Bidirectional Transaction Reviews

### 1. Scope / Trigger

- Trigger: backend, PostgreSQL, OpenAPI, profile, or reputation work that creates, publishes, removes, aggregates, or displays verified transaction reviews.
- This contract owns review truth and visibility. Reputation tier calculation may consume published review facts later, but must not infer reviews from transaction completion or sealed rows.

### 2. Signatures

```text
transaction_type:
  carpool_membership | api_order

reviewer_role / reviewee_role:
  buyer | seller

review status:
  sealed | published | removed

API-order commercial outcome:
  pending | cancelled_unpaid | normal_fulfillment | full_refund |
  partial_refund | continued_fulfillment | closed_unverified

API-order review window:
  [commercial_outcome_updated_at, commercial_outcome_updated_at + 14 days)

GET /api/v1/me/reviews:
  ReviewCenterRow.allowedTags[] = { code, label, polarity }
  ReviewCenterRow.commercialOutcome
  ReviewCenterRow.reviewPaused
  ReviewCenterRow does not expose counterparty submission state

POST|PUT /api/v1/me/transactions/{transactionType}/{transactionId}/review:
  { rating: 1..5, tags: tag-code[0..5], note: string[0..600] }

PostgreSQL:
  transaction_reviews
  transaction_review_revisions
```

### 3. Contracts

- A verified review points to one completed carpool membership or one API order with a reviewable commercial outcome: `normal_fulfillment|full_refund|partial_refund|continued_fulfillment`. Purchase intents, applications, payment/delivery submission, `pending`, `cancelled_unpaid`, and `closed_unverified` are not review sources.
- API-order review eligibility and deadline are independent of `api_orders.status` and `completed_at`. Mutable rows snapshot `commercial_outcome`; when a dispute finalizes, unfrozen rows refresh to the new outcome and `commercial_outcome_updated_at + 14 days`.
- Any active API-order dispute pauses review creation, sealed-review editing, deadline auto-publication, public/reputation aggregation, and deadline-driven recalculation. Published/frozen rows remain immutable. Closing the active dispute resumes only mutable rows under the final commercial outcome.
- Buyer and seller may each create one review of the other participant. Direction and role are preserved so future reputation aggregation can keep buyer and seller behavior separate.
- The first review remains sealed. The author can edit it before the deadline, but the counterparty and public surfaces cannot read its rating, tags, or note.
- Before publication, ordinary-user responses must not expose whether the counterparty submitted. Do not return a counterparty-submission boolean or a received sealed row; both are observable signals even when content is null.
- When both participants submit, both reviews publish and freeze atomically. When only one submits, eligible reads publish and freeze it at the 14-day deadline.
- Published content is immutable. Administrator removal is a visible-state transition with actor, reason, version, and append-only revision; it cannot rewrite the frozen content.
- An active `reputation_transaction_exclusions` row makes the transaction ineligible and removes its published reviews from public reads. Restore makes the transaction eligible again without rewriting review history.
- Only `published` and non-removed reviews are public verified facts. Sealed, removed, excluded, mock, inferred, or unavailable reviews must not affect public review counts or future score inputs.
- Rating is required. A review is valid when it has at least one allowed tag or a non-empty note; validation friction must not vary by star value.
- Tags use a backend-owned typed catalog. New writes persist stable codes, review-center rows return scenario-filtered `allowedTags`, and submission validates transaction type plus reviewer/reviewee roles. Public DTOs convert codes and supported historical Chinese aliases to current labels.
- Historical `与描述不符` remains readable but displays as `实际体验与描述有差异`. Adding or renaming a code requires updating reputation SQL classification and its regression test in the same change.
- Public review lists expose each raw rating. Bayesian/weighted rating remains a reputation-engine input and is not a second public review average.
- Review notes reject contact- and credential-shaped content. Reviews remain experience notes and do not prove payment, fulfillment, dispute responsibility, refund eligibility, guarantee, or platform endorsement.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Unsupported type, malformed ID, rating/tag/note validation failure | `422 VALIDATION_FAILED` |
| Rating outside 1-5 | `422 VALIDATION_FAILED`, field `rating` |
| More than five tags or a tag not allowed for the resolved roles | `422 VALIDATION_FAILED`, field `tags` |
| Both tags and trimmed note are empty | `422 VALIDATION_FAILED`, field `content` |
| Current user is not a participant | `404 OBJECT_NOT_FOUND` |
| Carpool is not completed, API commercial outcome is not reviewable, or transaction is actively excluded | `409 INVALID_STATE_TRANSITION` |
| API order has an active dispute | Return `reviewPaused=true`; reject create/edit and skip auto-publication/aggregation without exposing a received sealed row |
| Submission/edit at or after the deadline | `409 INVALID_STATE_TRANSITION` |
| Edit targets a published, removed, or otherwise frozen review | `409 INVALID_STATE_TRANSITION` |
| Received review is sealed | Omit the row so submission itself is not observable |
| Non-admin removal | `403 PERMISSION_DENIED` |
| Missing/stale removal precondition | `428 PRECONDITION_REQUIRED` / `412 VERSION_CONFLICT` |

### 5. Good/Base/Bad Cases

- Good: an API buyer and seller both submit; both reviews become verified public facts with opposite role directions in one commit.
- Good: a confirmed full refund starts a new 14-day review window at `commercial_outcome_updated_at`; both participants may review, but the order does not count as a normal fulfillment.
- Good: an active dispute pauses an author's sealed review; it is neither editable nor auto-published nor aggregated until the dispute closes.
- Good: seller-to-buyer `quick_payment` is stored as a code, rendered as `付款及时`, and included in the positive reputation-tag aggregate.
- Good: one carpool review reaches its deadline, materializes as published, and remains immutable while its revision history records the transition.
- Base: an administrator removes abusive published text. Public reads omit it, while frozen content and audited removal history remain stored.
- Bad: return `counterpartySubmitted`, return a received sealed placeholder row, accept a tag in the wrong role direction, or display weighted rating as the ordinary public average.

### 6. Tests Required

- Service and repository tests must cover both transaction types, both directions, no counterparty-submission signal, commercial-outcome eligibility, the outcome-time 14-day boundary, active-dispute pause, tag-or-note validation, scenario tag rejection, historical aliases, active exclusion, and post-publication immutability.
- PostgreSQL integration must prove confirmed refunds remain reviewable without counting as normal completion, active disputes pause create/edit/auto-publication/reputation aggregation, mutable deadline refresh, second-submit atomic publication, append-only revisions, legacy review preservation, and audited removal.
- Public-profile tests must prove only published, non-removed, non-excluded rows appear and role/type fields survive the API boundary.
- Run the full backend suite, vet, OpenAPI parsing and route parity, migration documentation checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
response.CounterpartySubmitted = counterpartyReview != nil
response.Items = append(response.Items, receivedSealedPlaceholder)

deadline := order.CompletedAt.Add(14 * 24 * time.Hour)
if order.Status == "completed" { allowReview() }
```

#### Correct

```go
if row.Direction == DirectionReceived && row.Visibility == VisibilitySealed {
    continue
}
if transaction.ReviewPaused {
    skipMutationPublicationAndAggregation()
}
deadline := ReviewDeadlineForAPIOrder(order.CommercialOutcomeUpdatedAt)
row.AllowedTags = AllowedTags(row.TransactionType, row.ReviewerRole, row.RevieweeRole)
```

## Scenario: Versioned Reputation Snapshots, History, And Cache Invalidation

### 1. Scope / Trigger

- Trigger: backend, PostgreSQL, OpenAPI, profile, carpool, or API-market work that calculates or exposes buyer/seller reputation tiers, risk states, confidence, metrics, progress, badges, or history.
- The evaluator owns derived meaning. Handlers and frontend code display its output and must not reproduce thresholds.

### 2. Signatures

```text
reputation.Service.GetMany(ctx, userIDs, role, scope)
reputation.Service.GetUserScope(ctx, userID, scope)
reputation.Service.GetUserReputation(ctx, userID)
reputation.Service.RecalculateUser(ctx, userID)
reputation.Service.RecalculateAll(ctx)
reputation.Service.History(ctx, userID, limit)

GET  /api/v1/reputation/rules
GET  /api/v1/users/{username}/reputation?scope=overall|carpool|api
GET  /api/v1/me/reputation
GET  /api/v1/admin/users/{id}/reputation?limit=1..100
POST /api/v1/admin/users/{id}/reputation/recalculate
POST /api/v1/admin/reputation/recalculate

PostgreSQL:
  user_reputation_states
  user_reputation_history
```

### 3. Contracts

- `reputation-v2` calculates six independent snapshots per user: buyer/seller crossed with overall/carpool/API. The version upgrade changes timeout responsibility attribution only; tier, state, confidence, and scoring thresholds remain unchanged.
- A cached `reputation-v1` snapshot is stale. The next list, detail, profile, or explicit recalculation read rebuilds and persists it as `reputation-v2` from current durable facts.
- `tier`, `state`, `confidence`, metrics, progress, warnings, and badges come from one pure evaluator and one versioned rule set.
- Verified review ratings use a Bayesian prior weight of 5. The platform prior is the same role/scope public-review average, or 4.0 while that sample has fewer than 20 reviews.
- Common positive and negative tags use only published, non-removed reviews from non-excluded transactions. Return at most five per polarity, ordered by count descending and tag ascending.
- Review-count and weighted-rating progress is passive evidence with `status=unavailable`; it never exposes a remaining review count or an action.
- Responsibility-cancellation progress may expose the minimum additional fault-free completions `N` satisfying `F / (C + F + N) <= R`, rounded up. This is a mathematical condition, not an upgrade prediction.
- A snapshot is valid only when its rule version matches, `dirty_at` is null, source facts are not newer, and `now < next_recalculation_at` when a boundary exists.
- `next_recalculation_at` covers review deadlines, restriction starts/ends, 90/365-day windows, and `reliable_since + 90 days`.
- Saving a new key or a tier/state change appends history. An unchanged forced recalculation updates the cache without adding history. Initial history preserves null `fromTier` and `fromState`.
- List surfaces call `GetMany` once for all owner/applicant IDs. Repository or recalculation failure returns unavailable/null; it must not synthesize zero or a fixed tier.
- PostgreSQL AFTER trigger functions branch on `TG_OP`: INSERT reads only `NEW`, DELETE reads only `OLD`, and UPDATE handles both old and new participants. Return `NULL` because the row result is ignored.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Unknown role or scope | `422 VALIDATION_FAILED` with the matching field |
| Missing engine repository | `500 INTERNAL_ERROR`; no fabricated snapshot |
| Public username or admin target does not exist | `404 OBJECT_NOT_FOUND` |
| Non-admin reads audit or triggers recalculation | `403 PERMISSION_DENIED` |
| Admin history limit outside `1..100` | `422 VALIDATION_FAILED` |
| Dirty, due, source-stale, or old-rule snapshot | Recalculate from facts before returning |
| Batch recalculation fails | Return an explicit error; do not return mixed fake defaults |
| Attempt to update/delete history | PostgreSQL rejects the mutation |

### 5. Good/Base/Bad Cases

- Good: one completed API order contributes independently to buyer/API and seller/API, while a verified seller review affects only seller/API and seller/overall review metrics.
- Good: an order exclusion marks both participants dirty; the next read removes the order and review from every affected snapshot.
- Base: a real aggregate query proves a user has no facts and returns six `insufficient` snapshots with zero counts and nullable rates.
- Bad: calculate a card tier in a handler, query reputation once per list row, treat one five-star review as reliable, or keep returning a cached snapshot after its time boundary.
- Bad: write `COALESCE(NEW.column, OLD.column)` in a multi-operation trigger. On INSERT or DELETE, the absent transition record is unassigned and can fail at runtime.

### 6. Tests Required

- Evaluator tests cover 0/3/10/30 completions, exact 5% cancellation, one five-star review, Bayesian prior behavior, risk overrides, continuity interruption, recent completion, passive review evidence, and fault-free completion math.
- Service tests prove one aggregate/load/save batch, six role/scope snapshots, stale-only writes, rule-version invalidation, and bounded full rebuilds.
- Apply migrations 1 through the current expected version to a new isolated database.
- PostgreSQL integration executes empty and populated reputation aggregation, writes six snapshots, verifies list/detail equality, checks tag polarity, excludes a real order, and proves unchanged forced rebuilds do not duplicate history.
- Trigger integration covers INSERT/UPDATE/DELETE dirty events and asserts unrelated users remain clean.
- Handler/OpenAPI tests cover public, self, admin audit, recalculation, session/CSRF/admin boundaries, strict YAML parsing, and runtime/OpenAPI route parity.
- Run `go test -count=1 ./...`, `go vet ./...`, migration documentation checks, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```sql
PERFORM mark_user_reputation_dirty(
  COALESCE(NEW.user_id, OLD.user_id),
  COALESCE(NEW.updated_at, OLD.updated_at, now())
);
RETURN COALESCE(NEW, OLD);
```

#### Correct

```sql
IF TG_OP = 'INSERT' THEN
  PERFORM mark_user_reputation_dirty(NEW.user_id, NEW.updated_at);
ELSIF TG_OP = 'DELETE' THEN
  PERFORM mark_user_reputation_dirty(OLD.user_id, OLD.updated_at);
ELSE
  PERFORM mark_user_reputation_dirty(OLD.user_id, NEW.updated_at);
  IF NEW.user_id IS DISTINCT FROM OLD.user_id THEN
    PERFORM mark_user_reputation_dirty(NEW.user_id, NEW.updated_at);
  END IF;
END IF;
RETURN NULL;
```

## Scenario: Resource-Level Source-Author Verification

### 1. Scope / Trigger

- Trigger: backend, PostgreSQL, OpenAPI, carpool, API-market, profile, or frontend work that records or displays whether a linux.do source topic belongs to the resource owner.
- This contract owns the verification fact. A non-empty source URL proves only that a source topic was supplied; it never proves authorship.

### 2. Signatures

```text
GET /api/v1/admin/source-author-verifications/{resourceType}/{resourceId}
PUT /api/v1/admin/source-author-verifications/{resourceType}/{resourceId}
  If-Match: "0" | "<current version>"

reputation.SourceAuthorRepository.GetSourceAuthorVerificationAudit(
  ctx, resourceType, resourceID, now
)
reputation.SourceAuthorRepository.UpdateSourceAuthorVerification(
  ctx, input, now
)

PostgreSQL:
  source_author_verifications
  source_author_verification_events
```

`resourceType` is `carpool|api_service`. Resource status is
`not_submitted|pending|verified|mismatch|expired`. Seller aggregate state is
`no_sources|pending|partial|verified|mismatch`; buyer aggregate state is always
`not_applicable`.

### 3. Contracts

- Each resource has at most one versioned verification row. Every administrator decision appends an immutable event in the same transaction.
- The stored decision snapshots the resource source URL and the owner's current `linux_do_user_id`. A later URL or binding change makes the effective status `pending` until an administrator reviews the new facts.
- A `verified` decision requires the observed author ID to equal the owner's current binding. A `mismatch` decision requires a different observed author ID and a non-empty failure reason.
- A verified row becomes effectively `expired` when `expires_at <= now`. Its expiration is also a reputation `next_recalculation_at` boundary.
- Carpool and API DTOs return `sourceAuthorVerification`. They must not infer it from `sourceUrl`.
- Seller aggregation includes only currently tradable resources with a non-empty source URL. `mismatch` has highest priority, all verified resources produce `verified`, a verified subset produces `partial`, remaining submitted/unsubmitted resources produce `pending`, and zero resources produce `no_sources`.
- A seller mismatch contributes a `caution` reputation fact. Source-verification, source-URL, owner, and linux.do-binding changes invalidate affected seller snapshots.
- The first administrator write uses `If-Match: "0"`. Existing rows use their positive ETag version. The GET response returns the current effective decision, the complete event list, and the matching ETag.
- Audit reads order events by `created_at DESC, version DESC, id DESC`. Timestamps can collide inside fast or test transactions, so `created_at` alone is not a deterministic latest-decision order.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Missing session | `401` Problem Details |
| Non-admin actor | `403 PERMISSION_DENIED` before repository access |
| PUT without valid CSRF | `403` Problem Details before mutation |
| Unknown resource type or invalid UUID | `422 VALIDATION_FAILED` |
| Resource does not exist | `404 OBJECT_NOT_FOUND` |
| Resource has no source URL | `422 VALIDATION_FAILED` on `sourceUrl` |
| First write without `If-Match: "0"` or stale positive version | `412 VERSION_CONFLICT`; missing header is `428 PRECONDITION_REQUIRED` |
| `verified`/`mismatch` without observed ID or method | `422 VALIDATION_FAILED` |
| `verified` with a different author ID | `422 VALIDATION_FAILED` |
| `mismatch` with the same author ID or no reason | `422 VALIDATION_FAILED` |
| Verified expiry is not in the future | `422 VALIDATION_FAILED` |
| Attempt to update/delete an audit event | PostgreSQL rejects the mutation |
| Two audit events share `created_at` | Return the higher version first; use `id DESC` as the final stable tie-breaker |

### 5. Good/Base/Bad Cases

- Good: an administrator verifies an API service against the owner's current linux.do binding; the public DTO shows `verified`, and expiry is scheduled for reputation recalculation.
- Good: the owner changes the source topic or linux.do binding; reads immediately return `pending` without presenting the old decision as current.
- Good: create and update events share one timestamp; the audit response still returns the update before the create event.
- Base: a tradable resource has a source URL but no decision row; its resource status is `not_submitted`, and its seller aggregate remains `pending`.
- Bad: use `sourceUrl != ""` as a verified flag, keep a verified badge after URL drift or expiry, apply seller source-author state to a buyer snapshot, or treat timestamp-only ordering as deterministic.

### 6. Tests Required

- Unit tests cover all resource and aggregate states, aggregate priority, buyer `not_applicable`, decision validation, and seller mismatch caution.
- Handler tests cover session/admin/CSRF boundaries, first version `0`, stale versions, ETags, strict JSON, resource-type validation, and OpenAPI route parity.
- Adapter tests prove carpool/API real-backend mappings preserve the backend status and never derive verification from a URL.
- Apply migrations 1 through the current expected version to a new isolated PostgreSQL database.
- PostgreSQL integration covers create/update, URL and binding drift, expiry, aggregate counts, next recalculation, stale version rejection, seller caution, append-only events, and deterministic ordering when event timestamps match.
- Run `go test -count=1 ./...`, `go vet ./...`, full frontend Vitest, Vue typecheck, real-backend production build, strict OpenAPI YAML parsing, route parity, migration-document validation, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
summary.Status = map[bool]string{true: SourceVerificationVerified}[sourceURL != ""]
```

#### Correct

```go
summary := reputation.NormalizeSourceAuthorResourceSummary(
    listing.SourceAuthorVerification,
)
```

The verification repository, not URL presence, owns authorship truth.

#### Wrong

```sql
SELECT *
FROM source_author_verification_events
WHERE verification_id = $1
ORDER BY created_at DESC;
```

#### Correct

```sql
SELECT *
FROM source_author_verification_events
WHERE verification_id = $1
ORDER BY created_at DESC, version DESC, id DESC;
```
