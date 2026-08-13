# Administrator Registration Invite Codes Contract

Date: 2026-08-13

Executor: Codex

## Scenario: Issue and consume administrator-managed single-use registration codes

### 1. Scope / Trigger

- When registration requires a code, each new account must submit a code issued by an administrator. The affiliate field `aff_code` remains separate and optional.
- Administrators and Root may manage codes. Only Root may change the existing registration-code-required option.
- Password, OAuth, and WeChat account creation use the same validation and consumption rules. Existing OAuth and WeChat identities log in without a code.
- One code creates one account and then becomes permanently used. Used records cannot be edited, deleted, or cleaned up.

### 2. Signatures

```text
GET    /api/registration-invite-code/?p=&page_size=
GET    /api/registration-invite-code/search?p=&page_size=&keyword=&status=
GET    /api/registration-invite-code/batches
GET    /api/registration-invite-code/batches/{batch_id}/codes
GET    /api/registration-invite-code/{id}
POST   /api/registration-invite-code/
PUT    /api/registration-invite-code/
DELETE /api/registration-invite-code/batch
DELETE /api/registration-invite-code/invalid
DELETE /api/registration-invite-code/{id}
```

```text
registration_invite_codes
  id, creator_user_id, batch_id, name, key, status,
  created_time, expired_time, used_user_id, used_time, deleted_at

ConsumeRegistrationInviteCodeWithTx(tx, key, required, userId) error

GET /batches -> [{ batch_id, name, created_time, count }]
GET /batches/{batch_id}/codes -> [complete_code]
DELETE /batch body -> { "ids": [positive_unique_id] }
```

Statuses are fixed as `1=enabled`, `2=disabled`, and `3=used`. `expired_time=0` means no expiration; every other value is a Unix timestamp in seconds.
`batch_id` is a lowercase 32-character immutable indexed identifier. New rows created in one request share one UUID-derived ID. Each legacy row receives its own stable `legacy%026d` ID because the original generation boundary cannot be reconstructed safely.

### 3. Contracts

- Batch creation accepts a trimmed `name` of 1-64 Unicode code points, `count` from 1-100, and `expired_time` equal to 0 or strictly later than the current second.
- Update accepts `id > 0` and only enabled/disabled status. Only an unused record's name, status, and expiration may change.
- List responses contain `items`, `total`, `page`, and `page_size`. Rows project creator, used user, and timestamps. Search accepts an ID, batch-name prefix, or code prefix.
- `status=expired` matches enabled rows with `expired_time <= now`; `status=1` excludes expired rows. Backend and frontend both use `expired_time <= now` at the boundary.
- Single delete accepts only unused disabled or expired rows. Cleanup deletes only unused disabled or expired rows.
- Batch summaries are ordered newest first and return at most 100 batches. A summary uses the latest row's current name only as a display label; editable `name` is never batch identity, so duplicate names remain separate.
- Batch export accepts one valid `batch_id`, returns all complete codes in ascending row-ID order, and includes enabled, disabled, expired, and used rows. It is independent of list pagination and is capped at 100 codes.
- Batch delete accepts 1-100 unique positive IDs. It locks and loads the full set in one transaction, requires every ID to exist, rejects the whole request if any row is used or still-valid enabled, and deletes only after every row passes. A rejected request deletes zero rows.
- New-account creation and code consumption run in one database transaction. Consumption locks the row through `lockForUpdate`, then performs an `id + enabled` conditional update and requires `RowsAffected == 1`.
- Successful consumption writes `status=used`, `used_user_id`, and `used_time`. Any failure in account creation, identity binding, or code consumption rolls back the whole transaction.
- Password registration sends `registration_code`; affiliate attribution continues to use `aff_code`. The browser sends `registration_code` only while the required option is enabled.
- OAuth state and WeChat new-account flows may carry `registration_code`. Existing identities return before code validation. A new identity missing a required code returns a stable code and redirects to sign-up.
- Management mutations use the existing administrator audit pipeline, but audit metadata must not contain a complete registration code.
- Batch export audit metadata contains only `batch_id` and count. Batch deletion audit metadata contains IDs and count, never code values.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Code is required but empty | `REGISTRATION_CODE_REQUIRED` |
| Code does not exist | `REGISTRATION_CODE_INVALID` |
| Code is disabled | `REGISTRATION_CODE_DISABLED` |
| `expired_time <= now` | `REGISTRATION_CODE_EXPIRED` |
| Code is used or loses a concurrent consume race | `REGISTRATION_CODE_USED` |
| Edit or delete targets a used row | Reject and retain the audit record |
| Delete targets a currently valid enabled row | Reject until disabled or expired |
| Batch export ID is malformed, missing, or exceeds 100 rows | Reject; return no code list |
| Batch delete body has 0 or more than 100 IDs, duplicates, non-positive IDs, or missing rows | Reject and delete zero rows |
| Batch delete contains any used or still-valid enabled row | Reject the entire transaction and delete zero rows |
| Management caller is not Admin/Root | Reject through `AdminAuth` |
| Non-Root caller changes the required-code option | Reject through the existing Root boundary |

### 5. Good / Base / Bad Cases

- Good: two concurrent registrations use one code; exactly one account commits and the other request receives `REGISTRATION_CODE_USED`.
- Good: an administrator creates 100 codes and each code registers no more than one account; used rows keep creator, consumer, and timestamp facts.
- Good: two generation requests reuse the same batch name; the summaries have different `batch_id` values and each export returns only its own complete codes.
- Good: a batch spans more than one list page or contains mixed statuses; export still returns every row once in creation order.
- Base: when the required option is disabled, password registration does not send a hidden stale registration code, while affiliate attribution remains available.
- Base: batch delete receives only disabled and expired unused rows; all selected rows are deleted in one transaction.
- Base: an existing OAuth or WeChat identity continues to log in after the required option is enabled.
- Bad: reuse a user's affiliate code as the administrator registration code, or let `?aff=` populate `registration_code`.
- Bad: commit the user transaction and consume the code in a second transaction, which can leave a new account without a consumed code.
- Bad: physically delete used rows or include them in cleanup.
- Bad: group historical rows by editable batch name, merge same-name generation requests, loop over single-delete endpoints, or partially delete an invalid selection.

### 6. Tests Required

- Model unit tests cover create/search, shared new-batch IDs, deterministic legacy IDs, duplicate-name batch isolation, complete ordered batch export, expiration boundary, atomic batch-delete restrictions, cleanup scope, and used-row immutability.
- Controller tests cover generation and delete bounds, malformed/missing batch IDs, Admin authorization, stable error codes, pagination response, export/delete responses, and management audit without complete codes.
- Registration tests cover password, OAuth, and WeChat new/existing identity branches and prove `registration_code` is independent from `aff_code`.
- PostgreSQL concurrency tests prove only one transaction consumes a code and only one account remains associated with it.
- Frontend tests cover option-controlled fields and payload, affiliate-query isolation, OAuth/WeChat missing-code redirect, selection reset, newline-only TXT output, and stable-error translation.
- Browser checks cover sign-up and management pages at desktop and 390x844 mobile viewports; verify desktop horizontal table scrolling, mobile cards, current-page selection, duplicate-name batch labels, batch export counts, atomic delete rejection, and no console errors. Used rows expose no edit/delete actions.
- Before release run `go test ./... -count=1`, focused race tests, `go vet ./...`, frontend typecheck/tests/production build, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
create and commit the user
look up either aff_code or registration_code
mark the code used in a second transaction

group rows by editable name
export only the current page
delete each selected row with a separate request
```

#### Correct

```text
begin the user-creation transaction
create the user and identity binding
read the administrator code under a row lock using registration_code
recheck enabled, unexpired, and unused state
conditionally update it to used and require RowsAffected == 1
commit the same transaction

identify each generated batch by immutable batch_id
load the full batch by batch_id and export complete codes only
lock and validate the complete ID set, then delete once in the same transaction
```
