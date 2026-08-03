# Administrator User Directory

Date: 2026-08-01
Author: Codex

## Scenario: Server-Owned Directory And Account Governance

### 1. Scope / Trigger

- Trigger: changes to administrator account discovery, pagination, safe detail, account status, administrator permission, session revocation, or user-targeted governance audit records.
- Ownership spans PostgreSQL, the `auth` module, HTTP/OpenAPI, generated TypeScript, TanStack Query, and `AdminUsersPage.vue`.
- Account governance is separate from reputation governance. The account-detail audit list must not absorb every audit record whose target happens to be a user.

### 2. Signatures

```text
GET  /api/v1/admin/users?page&limit&search&status&role&linuxDo&sort
GET  /api/v1/admin/users/{id}
POST /api/v1/admin/users/{id}/status
POST /api/v1/admin/users/{id}/admin-permission
```

```go
ListAdminUsers(context.Context, auth.AdminUserDirectoryQuery) (auth.AdminUserDirectory, *domain.AppError)
AdminUserDetail(context.Context, string) (auth.AdminUserDetail, *domain.AppError)
UpdateAdminUserStatusWithIdempotency(context.Context, idempotency.Entry, auth.AdminUserStatusInput, time.Time, auth.AdminUserCompletionBuilder)
UpdateAdminUserPermissionWithIdempotency(context.Context, idempotency.Entry, auth.AdminUserPermissionInput, time.Time, auth.AdminUserCompletionBuilder)
```

```sql
CREATE INDEX ix_admin_audit_logs_user_target_recent
ON admin_audit_logs(target_id, created_at DESC, id DESC)
WHERE target_type = 'user';
```

### 3. Contracts

- List query defaults are page `1`, limit `20`, status/role/linux.do `all`, and sort `created_desc`.
- Limits are exactly `20`, `50`, or `100`. Search is trimmed and limited to 100 Unicode code points.
- Supported sorts are `created_desc`, `created_asc`, `active_desc`, `username_asc`, and `username_desc`. SQL order expressions must come from a whitelist, never request interpolation.
- PostgreSQL owns filtering, ordering, `LIMIT`/`OFFSET`, filtered `totalItems`, and global summary counts. The browser must not download the full directory, call `usePagination`, filter rows, or slice result rows.
- The URL owns `search`, `status`, `role`, `linuxDo`, `sort`, `page`, and `limit`. Filter changes reset page 1; a stale page is replaced with the last valid page.
- Safe detail may expose identity labels, account status/role/version/timestamps, linux.do binding summary, verified-email and password-configured booleans, provider names/times, active-session count/latest activity, and the two account-governance audit projections.
- Safe detail must not expose contact values, credential material, provider subjects, session IDs/hashes, IP/device data, raw reports, payment records, or unrelated audit payloads.
- Recent audit actions are restricted to `user.account_status_changed` and `user.admin_permission_changed`.
- Status transitions are `active -> suspended|banned|archived`, `suspended -> active|banned|archived`, `banned -> active|archived`, and `archived -> active`.
- Both mutations require session, administrator authority, CSRF, `Idempotency-Key`, `If-Match`, and a trimmed reason of 1-500 Unicode code points.
- JSON fields marked required in OpenAPI must be distinguishable from Go zero values. In particular, decode `isAdmin` as `*bool`, reject `nil`, then project to the service's explicit grant/revoke boolean.
- A successful mutation increments `users.version`. Leaving `active` revokes all live target sessions.
- User/permission changes, session revocation, domain event, safe administrator audit, target notification, response encoding, and idempotency completion commit in one transaction.
- Self-target changes are forbidden. A table-level transaction lock plus an active-administrator count prevents concurrent removal of the last active administrator.
- Real frontend mode surfaces backend failure. Deterministic mock behavior is allowed only when `NUXT_PUBLIC_API_MODE=mock` was explicitly selected.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Invalid page, limit, filter, sort, UUID, status, missing required boolean, or blank/oversized reason | `422 VALIDATION_FAILED` with a field error |
| Missing `If-Match` | `428 PRECONDITION_REQUIRED` |
| Stale version | `412 VERSION_CONFLICT`; refetch list and detail |
| Non-administrator caller or self-target mutation | `403 PERMISSION_DENIED` |
| Missing target user | `404 OBJECT_NOT_FOUND` |
| No-op/unsupported transition, grant to inactive user, or last-active-administrator violation | `409 INVALID_STATE_TRANSITION` |
| Reused idempotency key with the same request hash | Replay the stored response without repeating side effects |
| Reused idempotency key with a different request hash | `409 IDEMPOTENCY_KEY_REUSED` |

### 5. Good / Base / Bad Cases

- Good: page 2 requests `/admin/users?page=2&limit=20...`, receives at most 20 rows, and renders backend pagination and global summary values.
- Base: a filter has zero matches, so `items=[]`, `totalItems=0`, `totalPages=0`, while global summary values remain populated.
- Bad: `GET /admin/users` returns every account and `AdminUsersPage.vue` calls `Array.slice()` to emulate pages.
- Bad: account detail queries all `admin_audit_logs` rows with `target_type='user'`; this leaks unrelated governance domains into the account contract.

### 6. Tests Required

- Auth unit tests: defaults/validation, bounded page, filters/order, summary, stale page, transition matrix, self-target, stale version, inactive grant, last-active-administrator, session revocation, and idempotent replay.
- HTTP route tests: authentication/authority, strict JSON, explicit required `isAdmin`, query errors, CSRF, idempotency, `If-Match`, ETag, redaction, and response shape.
- PostgreSQL integration: row bounds, filtered/global counts, detail redaction and audit action whitelist, transaction lock behavior, session revocation, event/audit/notification writes, and idempotency completion/replay.
- Frontend tests: URL normalization/serialization, one backend request per list state, no real-to-mock fallback, mutation headers, stale-page correction, confirmation gating, and retained profile/reputation entry points.
- Gates: full Go tests/vet, full Vitest, Nuxt typecheck/build in real mode, OpenAPI route/type drift guards, migration documentation guard, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
type adminUserPermissionRequest struct {
    IsAdmin bool `json:"isAdmin"`
}
// Missing isAdmin silently becomes false and can be mistaken for revoke.
```

#### Correct

```go
type adminUserPermissionRequest struct {
    IsAdmin *bool `json:"isAdmin"`
}

if request.IsAdmin == nil {
    return validationError("isAdmin", "required")
}
input.Grant = *request.IsAdmin
```
