# Authentication Session Renewal

Date: 2026-07-21
Author: Codex

## Scenario: Throttled Sliding Session Renewal

### 1. Scope / Trigger

- Trigger: creating, validating, renewing, revoking, or documenting a `c2c_session` login session.
- Owners: `internal/module/auth`, `internal/store/postgres/auth.go`, `internal/server/middleware.go`, `internal/server/auth_handler.go`, and migration `000053`.
- Goal: keep actively used sessions convenient without writing expiry state on every request or allowing one login to remain valid beyond thirty days.

### 2. Signatures

```text
Initial idle expiry:       created_at + 7 days
Renewal write interval:    24 hours since renewed_at
Absolute expiry:           created_at + 30 days
Renewed idle expiry:       min(now + 7 days, absolute_expires_at)
```

```go
RenewSession(ctx context.Context, sessionID string) (auth.Session, bool, *domain.AppError)
```

```sql
UPDATE auth_sessions
SET renewed_at = $now,
    expires_at = LEAST($target_expires_at, absolute_expires_at),
    updated_at = $now,
    last_seen_at = $now
WHERE session_token_hash = $token_hash
  AND revoked_at IS NULL
  AND expires_at > $now
  AND absolute_expires_at > $now
  AND renewed_at <= $renew_before
RETURNING expires_at;
```

### 3. Contracts

- PostgreSQL `expires_at` is the final idle-validity authority. A browser cookie cannot revive an expired, revoked, or absolute-expired row.
- `renewed_at` throttles renewal writes. Authentication reads do not update `last_seen_at` on every request; the successful renewal update records both timestamps.
- Only a request whose cookie and optional CSRF token have already passed authentication may attempt renewal.
- Valid user activity includes normal authenticated reads and mutations. Exclude `OPTIONS`, health/readiness, static assets, login/session/logout routes, SSE events, navigation-badge polling, feedback/notification/announcement unread counters, and any dedicated renewal route.
- PostgreSQL renewal is one conditional `UPDATE ... RETURNING`. Only the concurrent request that receives a returned row emits `Set-Cookie`.
- The cookie uses the opaque session ID, `Path=/`, `HttpOnly`, `SameSite=Lax`, production-only `Secure`, and synchronized `Expires` plus `Max-Age`. Near the absolute limit, `Max-Age` must use the shorter actual remaining lifetime.
- `GET /api/v1/auth/session` returns the stored `expiresAt` and rotates CSRF state, but it does not count as user activity for renewal.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Valid authenticated request before 24 hours since `renewed_at` | Continue request; no renewal write and no `Set-Cookie` |
| Valid authenticated request at or after 24 hours | Atomically renew and send matching cookie only if the row was updated |
| Two requests race at the renewal boundary | Exactly one row update and at most one renewal cookie |
| `expires_at <= now()` | `401 SESSION_EXPIRED`; no renewal cookie |
| `absolute_expires_at <= now()` | `401 SESSION_EXPIRED`; no renewal cookie |
| `revoked_at IS NOT NULL` | `401 SESSION_REVOKED`; no renewal cookie |
| Renewal repository failure | Surface the storage error; do not silently continue with a guessed expiry |

### 5. Good / Base / Bad Cases

- Good: an authenticated profile read after twenty-four hours moves expiry to seven days from now and emits the same expiry in the cookie.
- Base: repeated authenticated requests during the next twenty-four hours read the same session without another renewal write.
- Good: a renewal on day twenty-nine stops at day thirty rather than extending to day thirty-six.
- Bad: navigation-badge polling keeps an otherwise idle user signed in indefinitely.
- Bad: application code updates expiry first and checks revocation afterward, allowing a revoked session to receive a fresh cookie.

### 6. Tests Required

- Auth service tests assert seven-day initial expiry, twenty-four-hour throttling, thirty-day capping, and no renewal after revoke or idle expiry.
- Handler tests assert an excluded session read emits no renewal cookie, a normal authenticated read emits one at the boundary, and an immediate second read emits none.
- Cookie tests assert `HttpOnly`, production `Secure`, `SameSite=Lax`, `Expires`, and `Max-Age=604800` for a full seven-day period.
- Migration tests assert the renewal columns, defaults, expiry-order constraint, and latest schema version.
- PostgreSQL integration should assert concurrent renewal returns exactly one updated row when `C2C_TEST_DATABASE_URL` is available.

### 7. Wrong vs Correct

#### Wrong

```sql
UPDATE auth_sessions
SET expires_at = now() + interval '7 days';
```

This renews every row, ignores revocation and absolute expiry, and races across concurrent requests.

#### Correct

Use the conditional single-row `UPDATE ... RETURNING` signature above, cap with `LEAST`, and send `Set-Cookie` only when the query returns the new expiry.
