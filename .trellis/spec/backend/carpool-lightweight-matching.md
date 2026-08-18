# Carpool Lightweight Matching

Date: 2026-08-17
Author: Codex

## 1. Scope / Trigger

- Trigger: any change to carpool listing publication or editing, application acceptance, condition confirmation, membership lifecycle, contact access, recruitment visibility, reviews, reputation, OpenAPI, or migration 111 and later.
- This contract replaces the pre-launch reservation, dual join confirmation, dual completion confirmation, and carpool review workflow.
- API-order completion, reviews, and reputation remain independent and must not be changed by carpool work.

## 2. Signatures

```text
POST /api/v1/carpools
PATCH /api/v1/carpools/{listingId}
POST /api/v1/carpools/{listingId}/submit-review
POST /api/v1/carpools/{listingId}/applications

GET  /api/v1/me/carpools/{listingId}
POST /api/v1/me/carpools/{listingId}/stop-recruiting
POST /api/v1/me/carpools/{listingId}/resume-recruiting
POST /api/v1/me/carpool-applications/{applicationId}/confirm-conditions
POST /api/v1/me/carpool-applications/{applicationId}/cancel
POST /api/v1/me/carpool-memberships/{membershipId}/leave

POST /api/v1/owner/carpool-applications/{applicationId}/accept
POST /api/v1/owner/carpool-applications/{applicationId}/reject
POST /api/v1/owner/carpool-memberships/{membershipId}/remove
PATCH /api/v1/owner/carpool-memberships/{membershipId}/note

listing recruitment status: draft | active | stopped
listing governance status:  clear | removed
application status:         pending_owner | joined | rejected | cancelled_by_buyer
membership status:          active | left | removed

Create/Patch spend fields:
  dailySpendLimitUsd: decimal string | null
  weeklySpendLimitUsd: decimal string | null
  followsOfficialQuotaReset: boolean

backend/migrations/000111_carpool_lightweight_matching.{up,down}.sql
backend/migrations/000112_carpool_optional_spend_limits_account_login.{up,down}.sql
backend/migrations/000113_carpool_membership_owner_note.{up,down}.sql
database.ExpectedMigrationVersion = 113
```

Migration 111 owns `carpool_listing_condition_versions`, `carpool_application_condition_acceptances`, nullable `contact_sessions.ends_at`, listing `conditions_version`, `governance_status`, `recruitment_stop_reason`, `offline_occupied_seats`, and the application/membership condition snapshots. Its down migration restores the pre-launch schema only when no carpool listings, applications, memberships, condition versions, or open-ended contact sessions exist. Deleted workflow data remains backup-only and must never be fabricated by a down migration.

## 3. Contracts

- Owner acceptance locks the application and listing, rechecks current conditions and capacity, changes the application to `joined`, creates one `active` membership, freezes the accepted conditions, opens a membership-duration contact session, and completes events, notifications, and idempotency in one transaction.
- A membership contact session has `ends_at = NULL` while active. Leaving or removal revokes it and records an end time; the historical application pointer remains.
- Capacity is `buyer_seat_capacity - offline_occupied_seats - active membership seats`. Accepting the last seat changes recruitment to `stopped` with reason `full` in the same transaction.
- Public occupied-seat presentation is `clamp(offline_occupied_seats + active_buyer_members, 0, buyer_seat_capacity)`. Public list/detail views show that combined count without exposing the offline source and continue to use `available_seats` for application availability. Owner operational views may still distinguish platform members from offline occupied seats.
- Leaving or removal releases capacity but never resumes recruitment. Only the owner `resume-recruiting` action may make a stopped listing active again, and only when governance is clear and capacity exists.
- Owner recruitment intent and administrator governance are independent. Administrator restore changes only governance status and never overwrites `draft|active|stopped`.
- Published listings may be edited, but their product plan is immutable. Critical condition changes append a new normalized condition version. Pending buyers must confirm the current version before acceptance; existing active members retain their joined snapshot.
- Daily and weekly values mean per-member maximum spend in USD. Each field is independently nullable, where `null` means unlimited; zero and negative supplied amounts remain invalid. The publish UI and API expose no currency selector and no independent extra-quota description. `followsOfficialQuotaReset` remains required and versioned.
- VPS region and mainland direct-connect support are optional declarations and preserve missing values as `null`. Distribution supports `sub2api`, `account_login`, and `other`. Account-login listings normalize `provides_admin_account` to false and never collect credentials; clients omit the administrator-account signal for that mode.
- Carpool applications and memberships never create review eligibility, public completion counts, ratings, or reputation facts. Reports, disputes, audits, governance disposition, and account restrictions still accept `carpool_membership` where their own contracts require it.
- `PATCH /api/v1/owner/carpool-memberships/{membershipId}/note` updates `carpool_memberships.owner_note` for an active or historical membership. The request requires `note` plus `If-Match` and `Idempotency-Key`; whitespace is trimmed, an empty string clears the note, and values longer than 500 Unicode characters are rejected.
- Owner membership responses may include `ownerNote`; buyer membership responses and all public/listing/application projections must omit it. Removal reason is optional and an empty reason is stored as an empty string.
- Removed routes must stay absent: carpool `confirm-join`, carpool `confirm-complete`, `withdraw-acceptance`, and carpool review creation/edit compatibility routes. API-order `confirm-complete` remains valid.

## 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing or stale `If-Match` on a versioned action | `428 PRECONDITION_REQUIRED` or `412 VERSION_CONFLICT` |
| Missing `Idempotency-Key` on an idempotent mutation | validation Problem Details |
| Owner accepts a non-`pending_owner` application | `409 INVALID_STATE_TRANSITION` |
| Application has not accepted the current condition version | `409 INVALID_STATE_TRANSITION` |
| No remaining seat at accept time | `409 SEAT_UNAVAILABLE` |
| Buyer applies while recruitment is stopped, governance is removed, or capacity is zero | public not-found behavior |
| Published listing changes `productPlanId` | `409 INVALID_STATE_TRANSITION` |
| Total seats are below offline occupied plus active membership seats | `422 VALIDATION_FAILED` |
| A supplied daily/weekly maximum spend is not a positive decimal | `422 VALIDATION_FAILED` on the matching spend field |
| `followsOfficialQuotaReset` is missing | `422 VALIDATION_FAILED` |
| Resume has no capacity or governance is removed | `409 INVALID_STATE_TRANSITION` |
| Leave/remove targets a non-active membership | `409 MEMBERSHIP_NOT_ACTIVE` |
| Owner note is longer than 500 Unicode characters | `422 VALIDATION_FAILED` on `note` |
| Owner note update has a stale membership version | `412 VERSION_CONFLICT` |
| Owner note update targets another owner's membership | `404 OBJECT_NOT_FOUND` |
| Carpool source is submitted to the review service | `422 VALIDATION_FAILED` or database constraint failure |

## 5. Good / Base / Bad Cases

- Good: a buyer applies against condition version 2, the owner accepts once, and the response is `joined` with one active membership and a contact session; no second confirmation exists.
- Good: a listing becomes full and stops. The member leaves, contact access closes, the listing remains stopped, and the owner explicitly resumes it.
- Good: the owner changes price from version 2 to version 3. The pending buyer sees a condition diff, confirms version 3, and can then be accepted; an existing member still reads version 2.
- Base: the owner edits only a non-critical typo. The ordinary listing version changes without incrementing `conditions_version` or requiring buyer reconfirmation.
- Bad: infer contact access from a non-empty `contact_session_id`, automatically reopen after leave, overwrite the member snapshot from the current listing, or project a carpool membership into reviews/reputation.
- Good: the owner saves a trimmed note, reloads the owner member list, and sees it on both active and historical membership rows; the buyer response has no `ownerNote` key.
- Bad: put the note on the application or public listing, return it from `/me/carpool-memberships`, or require a removal reason when the owner only needs to release the seat.

## 6. Tests Required

- Unit and router tests must cover direct acceptance, idempotent replay, stale versions, current-condition confirmation, removed-route absence, leave/remove, and stop/resume authorization.
- PostgreSQL tests must cover concurrent acceptance of the last seat, atomic membership/contact/event/idempotency effects, full auto-stop, no auto-resume, immutable product plan, condition version history, and frozen application/member snapshots.
- Review and reputation regressions must prove carpool sources are excluded while API-order review and public reputation behavior remains unchanged.
- OpenAPI/generated types must contain nullable spend/network values, account-login distribution, the condition/governance contracts, and omit removed routes and fields.
- Frontend tests must cover the narrowed statuses, direct activation, condition diff confirmation, active-only contact disclosure, recruitment controls, independent unlimited spend modes, account-login presentation, combined occupied-seat presentation, USD labels, and absence of carpool reviews.
- Smoke must run against PostgreSQL and prove publish -> apply -> accept -> contacts -> full stop -> leave -> still stopped -> explicit resume.
- Required commands: `go test ./... -count=1`, `go vet ./...`, frontend Vitest/typecheck/build, OpenAPI type/route checks, migration documentation check, and `git diff --check`.

## 7. Wrong vs Correct

### Wrong

```text
pending_owner -> accepted_reserved -> two confirm-join calls -> active
active -> two confirm-complete calls -> completed -> review eligibility
member leaves -> listing automatically becomes active
```

### Correct

```text
pending_owner --owner accept transaction--> joined + active membership + open contact session
active membership --> left | removed, with contact revocation
full listing --> stopped; released capacity stays stopped until owner resume
carpool membership --> no review or public reputation projection
```

## Scenario: Owner Membership Notes and Optional Removal Reason

### 1. Scope / Trigger

- Trigger: changes to owner member management, membership response projection, migration 113, or the owner membership note endpoint.
- The feature supports private operational notes and direct member removal in the existing carpool lifecycle. It does not add manual member creation or a new membership state.

### 2. Signatures

```text
PATCH /api/v1/owner/carpool-memberships/{membershipId}/note
  If-Match: "<membership-version>"
  Idempotency-Key: <opaque-key>
  { "note": "string, maximum 500 Unicode characters" }

POST /api/v1/owner/carpool-memberships/{membershipId}/remove
  { "reason": "optional string" }
```

Database:

```sql
carpool_memberships.owner_note text NOT NULL DEFAULT ''
```

### 3. Contracts

- Only the owner of the membership may read or write `ownerNote` through owner-scoped APIs.
- Notes are bound to the membership, survive `left` and `removed` transitions, and can be updated or cleared after the membership becomes historical.
- Note writes trim leading/trailing whitespace, increment the membership version, and complete idempotency in the same PostgreSQL transaction as the note update.
- Buyer-scoped membership responses and public listing/application responses omit the field entirely.
- An owner removal request may omit `reason`; removal still releases active seats, revokes contact access, and preserves the historical snapshot.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing `note` property | `422 VALIDATION_FAILED` |
| Empty `note` string | Clear the note and return `200` |
| Note longer than 500 Unicode characters | `422 VALIDATION_FAILED` |
| Missing `If-Match` or `Idempotency-Key` | `428 PRECONDITION_REQUIRED` or validation Problem Details |
| Stale membership version | `412 VERSION_CONFLICT` |
| Membership belongs to another owner | `404 OBJECT_NOT_FOUND` |
| Removal request omits `reason` | Continue with removal and store an empty reason |

### 5. Good / Base / Bad Cases

- Good: an owner saves `"  已确认联系方式  "`, reloads the owner page, and sees `已确认联系方式`; the buyer cannot see the field.
- Base: an owner clears a historical member's note with `{"note":""}` and receives the next membership version.
- Bad: expose the note in a buyer response, overwrite a newer note without `If-Match`, or reject a valid removal solely because `reason` is empty.

### 6. Tests Required

- Router test: owner can create, clear, and read a note; buyer membership output omits `ownerNote`; empty removal reason succeeds.
- Service/store test: note length validation, owner ownership check, version conflict, trim/clear behavior, and idempotent replay.
- Migration/OpenAPI checks: migration 113 is documented, the generated membership type includes the optional field, and the new route has parity.
- Frontend tests: real adapter sends `If-Match` and `Idempotency-Key`; mock notes survive module reload; the management page exposes active/history tabs and optional removal copy.

### 7. Wrong vs Correct

#### Wrong

```text
Store ownerNote on the application and return it from every membership endpoint.
Require a non-empty reason before allowing owner removal.
```

#### Correct

```text
Store ownerNote on carpool_memberships, project it only in owner responses,
and allow POST .../remove with an omitted reason while preserving lifecycle rules.
```
