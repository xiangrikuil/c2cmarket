# Authentication Sessions

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

## Scenario: Fixed Development Personas

### 1. Scope / Trigger

- Trigger: adding or changing fixed local identities, development login helpers, the persona switcher, or session bootstrap scripts.
- Owners: `internal/module/devpersona`, `internal/module/auth`, `internal/server`, `scripts/dev-personas.mjs`, and the frontend session/query cache.
- Goal: make buyer, seller, and administrator flows reproducible locally without turning development authentication into a production account-mutation API.

### 2. Signatures

```http
POST /api/v1/auth/dev-persona-session
Content-Type: application/json

{"persona":"buyer|seller|admin"}
```

```go
PrepareDevPersonaSession(ctx context.Context, persona string) (devpersona.Result, *domain.AppError)
SetDevAdminPermission(ctx context.Context, userID string, isAdmin bool, now time.Time) *domain.AppError
```

```bash
node scripts/dev-personas.mjs [all|buyer|seller|admin] \
  [--base-url http://127.0.0.1:8080] [--output-dir output/dev-sessions]
```

### 3. Contracts

- Register the route only when `EnableDevAuth=true`; production configuration must continue to reject development authentication.
- Accept only the exact lowercase enum. The response is a normal cookie-backed session with `persona`, `user`, `csrfToken`, and `expiresAt`.
- Fixed usernames are `dev-buyer`, `dev-seller`, and `dev-admin`. The OAuth provider subject is deterministic, and the returned username must equal the fixed username before permissions are changed.
- Buyer and seller sessions explicitly remove `admin`; the administrator session explicitly grants it. Never rely on a previous persona session's permission state.
- Repeated preparation may repair missing readiness data but must preserve usable edited contacts, passwords, merchant profiles, and payment settings. Do not seed listings, services, orders, disputes, reviews, or history.
- The browser switcher is development-build plus real-API-mode only. Replacing a session increments the client session generation so an older in-flight session read cannot restore the old CSRF token. Refetch active user queries before navigating to `/my`, because route guards consume the profile cache.
- The CLI writes cookies and CSRF tokens only to a real, non-symlink directory at mode `0700` and non-symlink files at mode `0600`; stdout contains persona names, usernames, and paths only.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Development auth disabled | Route is not registered; request resolves as `404` |
| Unknown, mixed-case, or whitespace-padded persona | `422 VALIDATION_FAILED` on `persona` |
| Unknown JSON field or malformed JSON | Strict JSON problem response; do not prepare a persona |
| Fixed username occupied by another OAuth identity | `409 VALIDATION_FAILED`; do not grant or revoke permissions on the occupied user |
| Buyer or seller previously has `admin` | Remove `admin` before issuing the new session |
| Existing seller has one usable enabled payment option | Preserve it; do not rewrite payment settings |
| Old `/auth/session` read finishes after switching | Return/retain the replacement session; never overwrite its CSRF token |
| Script output directory or target file is a symlink | Fail closed without writing credentials |

### 5. Good / Base / Bad Cases

- Good: switch admin -> buyer, land on `/my`, and observe that the management navigation is gone immediately.
- Good: run the seller bootstrap twice after editing the seller's WeChat contact and keep the edited value.
- Base: run the script with no persona and receive three protected session files without raw credentials in terminal output.
- Bad: implement personas by calling the arbitrary `/auth/dev-session` endpoint and leave a buyer with a previous administrator grant.
- Bad: navigate before refreshing `my-profile`; the account-recovery or admin guard can redirect using the previous user's cached profile.

### 6. Tests Required

- Auth unit tests assert display-name preservation, username-collision isolation, and exact administrator grant/revoke behavior.
- Persona service tests assert strict parsing, identity idempotency, recovery readiness, both buyer/seller contact types, usable seller merchant/payment state, and preservation of edited usable data.
- Router/OpenAPI tests assert the route exists only in the development route set and is marked `x-dev-only`.
- PostgreSQL integration asserts cookie/session issuance, stable identity, verified email, password, contacts, seller readiness, and admin demotion/promotion across repeated calls.
- Frontend tests assert replacement of cached session/CSRF state, protection from older in-flight session reads, absence of a mock fallback, query refresh ordering, and shared user/admin shell integration.
- Browser verification covers buyer -> seller -> admin -> buyer at desktop and mobile widths, including the admin navigation boundary.

### 7. Wrong vs Correct

#### Wrong

```ts
const session = await createDevPersonaSession(persona)
await router.replace('/my')
await queryClient.resetQueries({ type: 'active' })
```

The route guard can read the previous user's active `my-profile` query and redirect before the reset completes.

#### Correct

```ts
const session = await createDevPersonaSession(persona)
await queryClient.cancelQueries()
await queryClient.resetQueries({ type: 'active' })
await router.replace('/my')
```

The replacement session is authoritative before active user state is refetched, and navigation sees the new profile.
