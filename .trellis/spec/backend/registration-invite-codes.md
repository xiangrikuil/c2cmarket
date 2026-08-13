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
GET    /api/registration-invite-code/{id}
POST   /api/registration-invite-code/
PUT    /api/registration-invite-code/
DELETE /api/registration-invite-code/invalid
DELETE /api/registration-invite-code/{id}
```

```text
registration_invite_codes
  id, creator_user_id, name, key, status,
  created_time, expired_time, used_user_id, used_time, deleted_at

ConsumeRegistrationInviteCodeWithTx(tx, key, required, userId) error
```

Statuses are fixed as `1=enabled`, `2=disabled`, and `3=used`. `expired_time=0` means no expiration; every other value is a Unix timestamp in seconds.

### 3. Contracts

- Batch creation accepts a trimmed `name` of 1-64 Unicode code points, `count` from 1-100, and `expired_time` equal to 0 or strictly later than the current second.
- Update accepts `id > 0` and only enabled/disabled status. Only an unused record's name, status, and expiration may change.
- List responses contain `items`, `total`, `page`, and `page_size`. Rows project creator, used user, and timestamps. Search accepts an ID, batch-name prefix, or code prefix.
- `status=expired` matches enabled rows with `expired_time <= now`; `status=1` excludes expired rows. Backend and frontend both use `expired_time <= now` at the boundary.
- Single delete accepts only unused disabled or expired rows. Cleanup deletes only unused disabled or expired rows.
- New-account creation and code consumption run in one database transaction. Consumption locks the row through `lockForUpdate`, then performs an `id + enabled` conditional update and requires `RowsAffected == 1`.
- Successful consumption writes `status=used`, `used_user_id`, and `used_time`. Any failure in account creation, identity binding, or code consumption rolls back the whole transaction.
- Password registration sends `registration_code`; affiliate attribution continues to use `aff_code`. The browser sends `registration_code` only while the required option is enabled.
- OAuth state and WeChat new-account flows may carry `registration_code`. Existing identities return before code validation. A new identity missing a required code returns a stable code and redirects to sign-up.
- Management mutations use the existing administrator audit pipeline, but audit metadata must not contain a complete registration code.

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
| Management caller is not Admin/Root | Reject through `AdminAuth` |
| Non-Root caller changes the required-code option | Reject through the existing Root boundary |

### 5. Good / Base / Bad Cases

- Good: two concurrent registrations use one code; exactly one account commits and the other request receives `REGISTRATION_CODE_USED`.
- Good: an administrator creates 100 codes and each code registers no more than one account; used rows keep creator, consumer, and timestamp facts.
- Base: when the required option is disabled, password registration does not send a hidden stale registration code, while affiliate attribution remains available.
- Base: an existing OAuth or WeChat identity continues to log in after the required option is enabled.
- Bad: reuse a user's affiliate code as the administrator registration code, or let `?aff=` populate `registration_code`.
- Bad: commit the user transaction and consume the code in a second transaction, which can leave a new account without a consumed code.
- Bad: physically delete used rows or include them in cleanup.

### 6. Tests Required

- Model unit tests cover create/search, expiration boundary, disable, delete restrictions, cleanup scope, and used-row immutability.
- Controller tests cover batch bounds, Admin authorization, stable error codes, pagination response, and management audit.
- Registration tests cover password, OAuth, and WeChat new/existing identity branches and prove `registration_code` is independent from `aff_code`.
- PostgreSQL concurrency tests prove only one transaction consumes a code and only one account remains associated with it.
- Frontend tests cover option-controlled fields and payload, affiliate-query isolation, OAuth/WeChat missing-code redirect, and stable-error translation.
- Browser checks cover sign-up and management pages at desktop and 390x844 mobile viewports; used rows expose no edit/delete actions.
- Before release run `go test ./... -count=1`, focused race tests, `go vet ./...`, frontend typecheck/tests/production build, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```text
create and commit the user
look up either aff_code or registration_code
mark the code used in a second transaction
```

#### Correct

```text
begin the user-creation transaction
create the user and identity binding
read the administrator code under a row lock using registration_code
recheck enabled, unexpired, and unused state
conditionally update it to used and require RowsAffected == 1
commit the same transaction
```
