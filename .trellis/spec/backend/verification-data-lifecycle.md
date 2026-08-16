# Verification And Data Lifecycle

Date: 2026-07-26
Author: Codex

## Scenario: Bounded Verification, Idempotency, And PostgreSQL Maintenance

### 1. Scope / Trigger

- Trigger: changing bind-email verification, idempotency state/replay,
  PostgreSQL retention, maintenance scheduling, or the application shutdown
  lifecycle.
- Owners: `internal/module/profile`, `internal/module/idempotency`,
  `internal/maintenance`, `internal/store/postgres`, `internal/app`, and
  migration 63.

### 2. Signatures

```go
CreateEmailVerificationCode(ctx, input, codeHash, expiresAt, now)
ConfirmEmailVerificationCode(ctx, input, codeHash, now)
BeginIdempotency(ctx, entry)
CompleteIdempotency(ctx, entry, completion, completedAt)
CancelIdempotency(ctx, entry, failedAt)
RunDataLifecycle(ctx, now, batchSize, policy)
```

```text
verification digest = HMAC-SHA256(
  EMAIL_VERIFICATION_PEPPER,
  userID + ":" + normalizedEmail + ":" + code
)

processing expiry = created_at + 15 minutes
failed expiry     = failed_at + 1 hour
completed expiry  = completed_at + 7 days
cached body limit = 64 KiB
```

### 3. Contracts

- Production requires `EMAIL_VERIFICATION_PEPPER` with at least 32 bytes.
  Development/test use the explicit local-only default.
- One user has at most one unconsumed `bind_email` challenge. Creating a new
  challenge locks the user and consumes every prior active bind-email
  challenge in the same transaction.
- Confirmation locks the latest challenge by user and normalized email, not by
  the submitted digest. Wrong attempts increment atomically; attempt five
  consumes the challenge. Email update and challenge consumption commit
  together.
- An expired idempotency row may be reset for any request hash. A failed row
  may be retried immediately only with the same hash. A completed row replays
  until expiry.
- `request_hash + created_at` identifies the current idempotency execution
  generation. A superseded request must not complete, cancel, or perform a
  transactional mutation for a replacement generation.
- Completed responses over 64 KiB or marked `SkipBodyCache` retain status and
  resource identity but not the body. Resource routes rebuild the response;
  generic routes return `IDEMPOTENCY_RESULT_NOT_REPLAYABLE`.
- The PostgreSQL runner executes immediately and periodically, uses
  `pg_try_advisory_xact_lock`, and limits every data type with stable ordering
  plus `FOR UPDATE SKIP LOCKED`.
- Terminal retention uses the actual terminal timestamp: `revoked_at` instead
  of `expires_at` for revoked sessions, and `consumed_at` instead of
  `expires_at` for consumed verification challenges.
- Maintenance never deletes contact ciphertext/history, contact access logs,
  `admin_audit_logs`, moderation/dispute audit rows, or contact sessions.
  Ended open contact sessions are updated to `expired`.
- API order delivery credentials and their delivered pre-imported source rows
  are different from contact history: after the later of order completion,
  package/quota expiry, and delivery plus `API_DELIVERY_CREDENTIAL_RETENTION`,
  maintenance irreversibly nulls every credential payload and fingerprint.
  Open disputes and submitted appeals hold destruction. Retired unused
  pre-imported credentials use their retirement timestamp and are destroyed
  with reason `retired_unused`; available or reserved inventory is never
  selected.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Production pepper missing or shorter than 32 bytes | Configuration load fails |
| Verification missing, wrong, expired, or locked | `422 VALIDATION_FAILED` with the same challenge-state detail |
| Fifth wrong verification attempt | Attempt increments to five and challenge is consumed |
| Non-expired idempotency key with another hash | `409 IDEMPOTENCY_KEY_REUSED` |
| Non-expired processing request | `409 IDEMPOTENCY_IN_PROGRESS` |
| Old execution completes after key takeover | `409 IDEMPOTENCY_IN_PROGRESS`; replacement row is unchanged |
| Completed body cannot be cached or rebuilt | `409 IDEMPOTENCY_RESULT_NOT_REPLAYABLE`; mutation is not repeated |
| Maintenance advisory lock is already held | Successful skipped result with `LockAcquired=false` |
| One maintenance run fails | Log the code; the app stays available and the next interval retries |

### 5. Good / Base / Bad Cases

- Good: two concurrent correct confirmations produce exactly one successful
  email update and one consumed challenge.
- Base: a 20 KiB JSON response is cached and replayed for seven days.
- Good: a 100 KiB response stores no body but its completed row remains
  authoritative.
- Bad: an old request finishes after a key was reset and updates the
  replacement row using only `(user, route, key)`.
- Bad: cleanup deletes a challenge from its old `expires_at` even though it was
  consumed less than 24 hours ago.
- Bad: a maintenance query deletes all matching rows without a batch limit or
  advisory lock.

### 6. Tests Required

```bash
cd backend
go test -count=1 ./internal/module/profile ./internal/module/idempotency ./internal/maintenance
go test -race -count=1 ./internal/module/profile ./internal/module/idempotency ./internal/maintenance
go test -count=1 ./...
go vet ./...
test -z "$(gofmt -l .)"
```

- PostgreSQL integration must apply the complete migration chain through
  `database.ExpectedMigrationVersion` to a dedicated empty database and assert
  the migration 63 contracts: HMAC storage, five-attempt lockout, one
  concurrent confirmation, failed/completed expiry, body truncation,
  stale-generation rejection, advisory-lock skip, bounded batches,
  notification-before-event deletion, and preservation of
  contact/admin/moderation audit history.
- Migration checks must assert the current `database.ExpectedMigrationVersion`,
  plus the migration 63 failed-state constraints, one-active-challenge index,
  and lifecycle indexes. Do not freeze the repository-wide latest-version
  assertion at 63 when later migrations are added.

### 7. Wrong vs Correct

#### Wrong

```sql
DELETE FROM auth_sessions
WHERE revoked_at < $cutoff OR expires_at < $cutoff;
```

This can delete a recently revoked session because its older expiry also
matches.

#### Correct

```sql
DELETE FROM auth_sessions
WHERE (revoked_at IS NOT NULL AND revoked_at < $cutoff)
   OR (revoked_at IS NULL AND expires_at < $cutoff);
```

For idempotency completion, the same rule applies to execution ownership:

```sql
WHERE user_id = $user
  AND route_key = $route
  AND idempotency_key = $key
  AND request_hash = $hash
  AND created_at = $generation_started_at
  AND status = 'processing';
```

## Scenario: Independent Transaction Contact Email Verification

### 1. Scope / Trigger

- Trigger: changing email contact CRUD, contact verification routes, challenge
  persistence, account-email binding, or the profile contact editor.
- Owners: migration 110, `internal/module/contact`,
  `internal/store/postgres/contact.go`, contact HTTP handlers/OpenAPI, and the
  frontend profile adapters and Mock facade.

### 2. Signatures

```text
POST /api/v1/contact-methods/{id}/email-verification/start
POST /api/v1/contact-methods/{id}/email-verification/confirm
Idempotency-Key: required on confirm
```

```go
StartEmailVerification(ctx, userID, methodID)
ConfirmEmailVerificationWithIdempotency(
    ctx, userID, routeKey, key, requestHash, methodID, code, requestID,
    buildCompletion,
)
```

```text
contact digest = HMAC-SHA256(
  EMAIL_VERIFICATION_PEPPER,
  purpose="contact_email",
  subject=userID + ":" + contactMethodID + ":" + contactMethodVersionID,
  normalizedEmail,
  code
)
```

### 3. Contracts

- Account recovery email and transaction contact email are independent. A
  successful contact confirmation updates only `contact_methods.verified_at`;
  it never updates `users.email` or `users.email_verified_at`.
- A challenge binds the authenticated user, contact method, immutable current
  version, normalized email, purpose, and code. Migration 110 enforces this
  shape with composite ownership foreign keys and one active challenge per
  contact method.
- Updating only label, usage scopes, default, or enabled state preserves the
  current contact version and verification timestamp. Changing type or value
  creates a new version and clears verification, even if the value is later
  changed back.
- Confirmation locks the contact method before the active challenge. Start,
  confirm, and contact updates must keep the same parent-before-child lock
  order. A successful confirmation atomically consumes the challenge, marks
  the method verified, writes one safe audit event, and completes idempotency.
- Wrong attempts increment atomically; attempt five consumes the challenge.
  Expired, consumed, stale-version, and mismatched-email challenges share the
  stable invalid-code response.
- The proofless `/contact-methods/{id}/verify` route is forbidden. Frontend
  contact flows must never call `/me/email-verification/*`; explicitly using
  the verified account email only prefills a contact draft and still requires
  independent contact verification.
- PostgreSQL, in-memory service, frontend Mock, OpenAPI, and generated types
  must preserve the same version-bound lifecycle.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Contact is missing, disabled, or owned by another user | `404 OBJECT_NOT_FOUND` without ownership disclosure |
| Contact exists but is not email | `422 VALIDATION_FAILED` on `contactMethodId` |
| Code is not six digits | `422 VERIFICATION_CODE_INVALID` on `code` |
| Challenge is wrong, expired, consumed, exhausted, or version-stale | `422 VERIFICATION_CODE_INVALID` on `code` |
| Confirm omits `Idempotency-Key` | Existing mutation idempotency validation error |
| Completed confirm is replayed with the same key and hash | Stored completion; no new verification event |
| The same key is reused with another request hash | `409 IDEMPOTENCY_KEY_REUSED` |

### 5. Good / Base / Bad Cases

- Good: a user independently verifies `trade@example.com`, then changes the
  account recovery email; the contact value and verification remain intact.
- Base: a verified email contact changes only its label and buyer/dispute
  scopes; its current version and `verified_at` remain unchanged.
- Good: two correct concurrent confirmations produce one verified update, one
  audit event, and one completed mutation result.
- Bad: a contact value changes after start, then changes back, and an adapter
  accepts the old code because it compares only the email string.
- Bad: start locks method then challenge while confirm locks challenge then
  method, creating a PostgreSQL lock-order inversion.
- Bad: contact confirmation delegates to the account-email confirm mutation.

### 6. Tests Required

- Contact service tests cover success, malformed/wrong/expired/exhausted code,
  disabled/foreign/missing/non-email targets, metadata preservation, value/type
  invalidation, version drift, and concurrent confirmation.
- PostgreSQL integration asserts account/contact isolation, method-before-code
  lock order, atomic audit/idempotency effects, replay, and one concurrent
  winner.
- Migration gates apply `1 -> current`, exercise `109 -> 110 -> 109 -> 110`,
  and assert `dirty=false`, purpose shape, composite foreign keys, and the
  partial active-challenge index.
- Frontend real-adapter tests assert contact endpoints never call account-email
  endpoints. Mock tests must include change-away/change-back version drift and
  five-attempt lockout.
- OpenAPI route and generated-type drift checks must pass after removing the
  proofless endpoint.

### 7. Wrong vs Correct

#### Wrong

```text
confirm: lock email_verification_codes -> lock contact_methods
start:   lock contact_methods -> lock email_verification_codes
```

The opposing lock order can deadlock concurrent start and confirm requests.

#### Correct

```text
start:   lock contact_methods -> consume/insert challenge
confirm: lock contact_methods -> lock challenge -> verify -> audit -> complete
```

The method row is the parent lock for every contact mutation. Verification
must also bind the immutable `contact_method_version_id`; matching the current
email text alone is insufficient.

## Scenario: API Delivery Credential Physical Destruction

### 1. Scope / Trigger

- Trigger: changing API order or quota credentials, credential re-encryption,
  report/dispute/appeal creation for API orders, or data-lifecycle retention.
- Owners: migrations 76-77, `internal/store/postgres/maintenance.go`,
  `api_order.go`, `contact_reencrypt.go`, and `report.go`.

### 2. Signatures

```go
RunDataLifecycle(ctx, now, batchSize, policy)
GetAPIOrderForBuyer(ctx, buyerUserID, orderID)
GetAPIOrderForSeller(ctx, sellerUserID, orderID)
ReencryptContactCipherBatch(ctx, ContactReencryptOptions{Kind, DryRun})
```

```text
API_DELIVERY_CREDENTIAL_RETENTION = positive duration
lifecycle lock key = hashtextextended(
  "api_order_credential_lifecycle:" + order_id::uuid::text,
  0
)
```

### 3. Contracts

- `api_order_delivery_credentials` and `api_quota_credentials` have mutually
  exclusive live and destroyed shapes. A destroyed row keeps only audit-safe
  identity, kind, key-format metadata, timestamps, and reason; every URL,
  username, instruction, ciphertext, nonce, and quota fingerprint is `NULL`.
- The completed-order retention anchor is the greatest of completion,
  package expiry, quota expiry, and delivery submission. Open disputes and
  submitted appeals hold destruction. Retired unused quota credentials use
  `retired_at`; available and reserved rows are never eligible.
- Buyer and seller detail reads acquire the blocking lifecycle advisory lock
  and read/decrypt inside that same transaction. Destroyed rows return only
  the audit projection and never call the crypto codec.
- Maintenance first locks the order credential and order with `SKIP LOCKED`,
  then locks any source quota row with `SKIP LOCKED`, keeps only fully locked
  candidates, and finally uses `pg_try_advisory_xact_lock`. It destroys the
  quota source before the order copy, so a quota-backed pair cannot be partly
  destroyed.
- API order and quota re-encryption scans use `FOR UPDATE SKIP LOCKED` even in
  dry-run mode and exclude `destroyed_at IS NOT NULL`. A dry run may decrypt
  live rows for eligibility statistics, but it must not read an old MVCC
  snapshot while destruction owns the row.
- Report resolution returns PostgreSQL `o.id::text`, and moderation lifecycle
  locks cast input through `uuid::text`. Raw client UUID casing must never
  create a second lock key or a hold record that maintenance cannot match.
- Migration 76 down refuses to run after any destruction. When rollback is
  still reversible it restores the original quota fingerprint/payload/state
  constraints, including `api_quota_credentials_check2`, before making the
  fingerprint `NOT NULL` and dropping destruction columns.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Retention duration is missing, zero, or negative | Configuration validation fails |
| Detail read owns the lifecycle lock | Maintenance skips that order without blocking |
| Quota re-encryption owns the source row | Maintenance skips the whole credential pair without blocking |
| Open dispute or submitted appeal exists | Destruction count remains zero |
| Credential is already destroyed | Detail returns audit fields only; re-encryption does not select or decrypt it |
| Rollback is attempted after physical destruction | Migration 76 down raises an exception |

### 5. Good / Base / Bad Cases

- Good: the retention deadline passes, maintenance locks both rows, destroys
  the quota source and order copy atomically, and repeated runs return zero.
- Base: a completed manual-delivery order has no quota source; maintenance
  locks and destroys its order credential only.
- Good: an uppercase API-order UUID creates a report whose canonical target is
  the database lowercase UUID; its dispute and appeal block maintenance.
- Bad: maintenance locks the order row and then waits on a quota row held by
  re-encryption, causing the whole lifecycle transaction to time out.
- Bad: a detail read decrypts before taking the shared lock and returns a
  pre-destruction MVCC snapshot after maintenance commits.

### 6. Tests Required

- PostgreSQL migration gates must cover empty `1 -> current` and reversible
  `77 -> 75 -> 77`, assert the restored Version 75 quota state constraint, and
  prove rollback refusal after a destroyed row exists.
- Lifecycle integration must prove anchors, holds, retired inventory,
  batching, idempotency, quota/order atomicity, non-blocking row/advisory lock
  skips, uppercase UUID canonicalization, and both detail-read race directions.
- Re-encryption integration must prove destroyed rows are excluded and locked
  API order/quota rows are skipped in apply and dry-run paths.
- Store and HTTP tests must assert destroyed rows never increment decrypt
  counters and never return secret fields; frontend tests must assert the
  retention notice replaces secret reveal/copy controls.

### 7. Wrong vs Correct

#### Wrong

```sql
UPDATE api_quota_credentials
SET api_key_ciphertext = NULL
WHERE id IN (SELECT api_quota_credential_id FROM candidates);
```

This can block behind re-encryption and can destroy only one side of a
quota-backed credential pair.

#### Correct

```text
order credential + order FOR UPDATE SKIP LOCKED
  -> quota source FOR UPDATE SKIP LOCKED
  -> keep fully locked pairs
  -> pg_try_advisory_xact_lock(canonical order UUID)
  -> destroy quota source
  -> destroy order credential
```
