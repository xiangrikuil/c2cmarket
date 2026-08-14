# Authentication Sessions

Date: 2026-08-12
Author: Codex

## Scenario: Turnstile Gates For Password Login And Student Signup

### 1. Scope / Trigger

- Trigger: adding or changing native password login, student-email registration start, their frontend forms, or their deployment configuration.
- Owners: `internal/platform/turnstile`, `internal/server/auth_handler.go`, `internal/app/app.go`, the OpenAPI auth request schemas, `TurnstileWidget.vue`, `LoginPage.vue`, Compose, Wrangler, and the Pages CSP.
- Goal: reject automated abuse at the HTTP boundary before password verification, email delivery, or account creation without treating frontend widget state as authorization.

### 2. Signatures

```http
POST /api/v1/auth/password/login
{"username":"...","password":"...","turnstileToken":"..."}

POST /api/v1/auth/email-registration/start
{"email":"...","turnstileToken":"..."}
```

```go
Verify(ctx context.Context, input turnstile.Verification) error

type Verification struct {
    Token    string
    Action   string
    RemoteIP string
}
```

### 3. Contracts

- Password login requires action `password_login`; email-registration start requires action `student_signup`. OAuth, development personas, and registration confirmation are not implicitly covered.
- The server submits `secret`, `response`, and the middleware-derived client IP to Cloudflare's canonical Siteverify endpoint. It accepts only `success=true`, the exact endpoint action, and a normalized exact hostname in `TURNSTILE_HOSTNAMES`.
- `TURNSTILE_SECRET` and `TURNSTILE_HOSTNAMES` are required in production. Production accepts `c2cmarket.shop`; staging accepts `staging.c2cmarket.shop`. The browser receives only `NUXT_PUBLIC_TURNSTILE_SITE_KEY`.
- The verifier uses a bounded response, a fixed timeout, no redirects, no environment proxy, and sanitized failures. Tokens are at most 2048 bytes and must never be logged or returned.
- The password form submits only after receiving a token and resets the widget after every request attempt. Expiry, timeout, widget error, and unmount clear the token.
- Pages CSP adds only `https://challenges.cloudflare.com` to `script-src`, `connect-src`, and `frame-src` for this integration.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing, blank, expired, duplicate, or oversized token | `403 TURNSTILE_VERIFICATION_FAILED`; do not call the auth/registration service |
| Provider returns `success=false`, non-2xx, malformed JSON, trailing JSON, or an oversized response | Same sanitized `403`; do not expose provider detail |
| Returned action differs from the endpoint action | Same sanitized `403` |
| Returned hostname is outside the exact environment allowlist | Same sanitized `403` |
| Verifier is absent because runtime configuration is incomplete | Fail closed with the same `403`; production configuration rejects startup earlier |
| Verification succeeds | Continue into the existing password-login or registration-start behavior |

### 5. Good / Base / Bad Cases

- Good: the browser obtains a `password_login` token, login succeeds or fails normally, and the widget resets in both cases.
- Base: student-registration start verifies `student_signup`, then checks the persistent global switch and exact enabled institution domain before sending a code. The migration-created switch remains disabled until an administrator explicitly enables it.
- Bad: trust a non-empty browser token without server-side Siteverify.
- Bad: accept `success=true` while ignoring action or hostname, allowing a token minted for another flow or host to cross the boundary.

### 6. Tests Required

- Verifier unit tests assert the canonical form fields, success path, action/hostname mismatch, provider rejection, transport error, non-2xx response, malformed/trailing JSON, and request/response size bounds.
- Handler tests inject a fake verifier and assert rejection happens before the application service while valid verification preserves existing downstream responses.
- Config tests assert local hostname defaults plus production secret/hostname requirements and invalid hostname rejection.
- Frontend tests assert official explicit rendering, one-time-token clearing/reset, login request propagation, production site-key guards, Compose/Wrangler wiring, and exact CSP origin additions.
- The hosted acceptance check uses one fresh browser token successfully and then proves replay of that same token fails.

### 7. Wrong vs Correct

#### Wrong

```go
if req.TurnstileToken != "" {
    return app.LoginWithPassword(ctx, req.Username, req.Password)
}
```

This treats attacker-controlled frontend state as proof and does not bind the token to the endpoint or hostname.

#### Correct

Call the injected verifier before the existing service, require `success=true` plus exact action and hostname, map every verifier failure to the stable sanitized problem, and reset the browser widget after the attempt.

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

## Scenario: Student Registration, Deterministic Capabilities, And Linux.do Linking

### 1. Scope / Trigger

- Trigger: changing student-email registration, institution-domain administration, usernames, session/profile capability projection, seller/probe authorization, or authenticated linux.do linking.
- Owners: `internal/module/auth`, `internal/store/postgres/auth*.go`, server capability guards, migration `000091`, OpenAPI auth/profile schemas, and capability-aware frontend navigation/query guards.
- Goal: create one durable buyer-only account per verified institution email, derive authority from current identity facts, and allow an explicit in-place linux.do link without account merging.

### 2. Signatures

```http
GET  /api/v1/auth/email-registration/config
POST /api/v1/auth/email-registration/start
POST /api/v1/auth/email-registration/confirm
POST /api/v1/auth/password/reauthenticate
GET  /api/v1/auth/oauth/start?purpose=link_linuxdo

GET   /api/v1/admin/student-registration
PATCH /api/v1/admin/student-registration
GET   /api/v1/admin/student-institution-domains
POST  /api/v1/admin/student-institution-domains
PATCH /api/v1/admin/student-institution-domains/{id}
```

```go
ProjectCapabilities(user auth.User) []string
HasCapability(user auth.User, capability string) bool
RequireCapability(user auth.User, capability string) *domain.AppError

VerificationCodeHash(
    pepper []byte,
    purpose, subject, normalizedEmail, code string,
) string
```

```text
Canonical capabilities:
  api_order.create
  carpool.apply
  carpool.publish
  api_service.publish
  api_quota.publish
  api_probe.manage
  admin.access
```

### 3. Contracts

- Migration `000091` creates the global registration switch disabled. Start and confirm both re-read it; no deployment or frontend flag implicitly enables registration.
- Institution eligibility is an exact, lowercase ASCII domain match against an enabled immutable row. There is no `.edu` suffix, subdomain, wildcard, or mutable-domain shortcut.
- Confirmation accepts a caller-selected username matching `^[a-z0-9_-]{3,24}$`; public flows reject rather than repair case, whitespace, reserved words, or conflicts.
- A six-digit code expires after 15 minutes, permits at most five failed attempts, invalidates prior active challenges on resend, and is stored as a purpose/subject/email-bound HMAC. Confirmation atomically creates the user, password, immutable `student_email_claim`, attribution, session, and safe identity event.
- `student_email_claims.normalized_email` is permanent and unique. Changing `users.email`, disabling an account, or linking linux.do never releases the claimed institution email.
- Capability authority is `ProjectCapabilities` over freshly loaded durable `StudentClaim`, `LinuxDoBinding`, and `IsAdmin` facts. `User.Capabilities` is a response projection only and must never authorize a request.
- A student claim without linux.do gets only `api_order.create`. A bound linux.do identity gets the six non-admin business capabilities. `admin.access` is derived independently from the existing administrator grant.
- Buyer after-sales, disputes, reviews, contacts with buyer/dispute scope, and eligible model tests remain governed by participant/order state rather than a new global capability.
- Every seller/probe HTTP boundary rejects a missing capability before idempotency acquisition, persistence, or outbound work; reusable service methods repeat the durable-fact guard.
- Probe navigation is visible whenever `api_probe.manage` is present, including with zero owned probes or services. Resource counts are never authorization or menu authority.
- Linking linux.do requires a password reauthentication recorded on the current session within ten minutes and a one-time OAuth state bound to that session/user/purpose. A foreign identity returns `OAUTH_IDENTITY_CONFLICT`; no user merge or identity transfer occurs.
- Successful linking keeps the same user, immutable student claim, password login, orders, and history, while atomically rotating the current session and CSRF token. V1 has no unlink operation.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Registration switch disabled at start or confirm | `403 EMAIL_REGISTRATION_DISABLED` |
| Domain is not an exact enabled institution row | `422 STUDENT_EMAIL_NOT_ELIGIBLE` |
| Institution email already has a durable claim | `409 STUDENT_EMAIL_CLAIMED` |
| Username has invalid syntax | `422 USERNAME_INVALID` |
| Username is reserved or occupied | `409 USERNAME_UNAVAILABLE`; valid code remains retryable with another username |
| Code is wrong, expired, exhausted, consumed, superseded, or wrong-purpose | `422 VERIFICATION_CODE_INVALID` |
| Business capability is absent | `403 CAPABILITY_REQUIRED` before side effects |
| Link start lacks recent password reauthentication | `403 RECENT_REAUTHENTICATION_REQUIRED` |
| linux.do identity belongs to another user | `409 OAUTH_IDENTITY_CONFLICT`; neither account changes |
| Domain/admin setting version is stale or `If-Match` is missing | `412 VERSION_CONFLICT` or `428 PRECONDITION_REQUIRED` |

### 5. Good / Base / Bad Cases

- Good: an administrator explicitly enables one exact institution domain, a student registers once, logs in by username or claimed email, and sees buyer functions without carpool, merchant, or probe actions.
- Good: the student reauthenticates, links an unused linux.do identity, keeps the same user/order history, receives seller/probe capabilities on the next read, and the old current-session token is revoked.
- Base: a linux.do seller owns no resources but still sees the probe page and first-create state.
- Bad: free an institution email by editing `users.email`, then register a second account with the same claimed email.
- Bad: trust a stale session `capabilities` slice, a frontend route flag, or an owned-resource count as authorization.
- Bad: auto-merge two users because their display email, username, or claimed real person appears to match.

### 6. Tests Required

- Auth unit tests assert switch/domain checks at start and confirm, exact username rules, HMAC purpose isolation, resend invalidation, attempt exhaustion, single use, permanent claim ownership, and atomic confirmation.
- Capability contract tests assert exactly seven OpenAPI/backend/frontend values, deterministic sorted projection for student/linux.do/admin combinations, and that a forged `User.Capabilities` slice never grants authority.
- Handler/service tests assert missing capabilities fail before idempotency, storage, and outbound probes while valid buyer after-sales/model-test paths remain available.
- PostgreSQL integration tests assert concurrent username/email claims, session hydration after identity/admin changes, raw-cookie logout revocation, recent-auth/state single use, conflict rollback, and session rotation.
- Frontend tests assert capability-based menus/routes/queries, zero-resource probe visibility, explicit anonymous/student/linux.do/admin mock states, and logout cache clearing without real-to-mock fallback.
- Release gates include OpenAPI generation/drift checks, full Go/Vitest/typecheck, production frontend build, migration `1 -> latest`, and `git diff --check`; the registration switch stays disabled after those tests.

### 7. Wrong vs Correct

#### Wrong

```go
func HasCapability(user User, required string) bool {
    return slices.Contains(user.Capabilities, required)
}
```

This trusts a transport/cache projection that may be stale or forged after identity or administrator facts change.

#### Correct

```go
func HasCapability(user User, required string) bool {
    return slices.Contains(ProjectCapabilities(user), required)
}
```

Load current durable identity facts, project the fixed vocabulary, reject before side effects, and expose the resulting slice only for clients to render the same boundary.

The replacement session is authoritative before active user state is refetched, and navigation sees the new profile.

## Scenario: Student-Claim Password Reset

### 1. Scope / Trigger

- Trigger: changing public password reset, student-email identity lookup, password credentials, verification challenges, session revocation, or the login/reset frontend request lifecycle.
- Owners: `internal/module/auth`, `internal/store/postgres/auth_password_reset.go`, password-reset handlers and email delivery, migration `000098`, OpenAPI generated types, and `/password-reset` frontend state.
- Goal: let an eligible student-claim owner replace a password without disclosing account existence, changing governance state, or creating a session.

### 2. Signatures

```http
POST /api/v1/auth/password-reset/start
{
  "email": "student@example.edu",
  "turnstileToken": "one-time-token"
}
-> 202 { "accepted": true }

POST /api/v1/auth/password-reset/confirm
{
  "email": "student@example.edu",
  "code": "123456",
  "newPassword": "Strong-password-1!"
}
-> 204 No Content
```

```sql
CREATE UNIQUE INDEX ux_email_verification_codes_active_password_reset
ON email_verification_codes(user_id, email)
WHERE purpose = 'password_reset' AND consumed_at IS NULL;
```

```go
VerificationCodeHash(
    pepper []byte,
    purpose, subject, normalizedEmail, code string,
) string
```

### 3. Contracts

- Canonicalize and validate the email with the shared strict email contract before rate-limit targeting or repository lookup. Identity resolution uses only `student_email_claims.normalized_email`; mutable profile email, username, linux.do profile data, and administrator identity are not reset authorities.
- A permanent student claim remains eligible after linux.do binding or institution disablement. Active, suspended, and banned users may reset; archived or security-locked users do not receive or confirm a usable challenge.
- Start verifies Turnstile action `password_reset`, applies separate IP/actor/normalized-email limits, and always returns the same `202 {"accepted":true}` for syntactically valid email input. Eligibility and delivery outcome never change that public shape.
- Codes are six digits, expire after 15 minutes, allow five failed attempts, and use an HMAC bound to `password_reset + userID + normalizedEmail + code`. Starting again consumes every active reset challenge for the email before conditionally creating the replacement.
- Confirmation atomically rechecks claim ownership and account eligibility, upserts the Argon2id password credential, consumes the challenge, revokes all live normal and restricted-business sessions, increments only `users.version`, and writes one secret-free `user.password_reset_completed` domain event. It creates no cookie or session and does not change governance state.
- The frontend carries only normalized safe `returnTo` values between `/login` and `/password-reset`. Each form has a synchronous pending-action guard plus a request generation; stale responses cannot mutate the current email, step, error, navigation, or Turnstile state.
- A submitted Turnstile token belongs to that request generation. The completion path may clear/reset the widget only while the captured generation is still current; an abandoned request must not erase a fresh token obtained after an email or step change.
- Backend field errors render only when they map to a field owned by the current form. Unknown field names must produce a visible panel error instead of being stored under an unused reactive key.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Malformed email, code, password, JSON, or unknown JSON field | Stable strict validation problem; no challenge or credential mutation |
| Turnstile missing, invalid, reused, or wrong action | Explicit Turnstile error; no identity lookup side effect |
| Unknown, pure-linux.do, admin-only, archived, or security-locked identity at start | Same public `202 {"accepted":true}`; no usable challenge or eligibility signal |
| Wrong, expired, exhausted, consumed, superseded, wrong-purpose, or newly ineligible challenge | `422 VERIFICATION_CODE_INVALID` on `code` |
| Institution domain disabled after the immutable claim exists | Reset remains eligible when account status and security lock allow it |
| Successful confirmation | `204`, password replaced, challenge consumed, normal/restricted sessions revoked, one safe event, no new session |
| Request becomes stale before completion | Ignore its response and cleanup; preserve current fields and fresh Turnstile token |
| Backend returns an unowned field name | Show a generic panel error; never hide the failure |

### 5. Good / Base / Bad Cases

- Good: a student later linked to linux.do resets through the immutable claimed email, all old login sessions are revoked, and the next login uses the existing audience rules.
- Good: a banned student resets the password but remains banned and receives only restricted-business access after a later successful login.
- Base: an unknown syntactically valid email receives the same accepted response and no email/challenge.
- Bad: resolve a reset subject through `users.email`, require the institution domain to remain enabled, or reveal that delivery was skipped.
- Bad: clear the Turnstile widget in an unconditional `finally` after the user changed the email and completed a fresh challenge.

### 6. Tests Required

- Auth unit tests assert strict email normalization, password policy, HMAC purpose/user/email isolation, generic start responses, and five-attempt exhaustion.
- PostgreSQL integration tests assert replacement challenge uniqueness, concurrent confirm succeeds once, active/suspended/banned and linked-student eligibility, pure OAuth/admin exclusion, disabled-domain recovery, archived/security-lock rechecks, and atomic credential/challenge/session/event/version mutation.
- Handler/OpenAPI tests assert strict JSON, exact `202`/`204` responses, no confirmation cookie, stable field errors, route parity, and separate Turnstile/rate-limit actions.
- Maintenance tests retain and delete reset rows through the common verification-code lifecycle without a purpose-specific leak.
- Frontend tests assert safe `returnTo`, inline owned-field errors, visible unknown-field fallback, synchronous re-entry guards, request-generation isolation, stale-request Turnstile preservation, independent password visibility, and terminal completion without session initialization.
- Browser acceptance covers `/login` and `/password-reset` on desktop and `390x844`, first-invalid focus, responsive institution directory, no horizontal overflow, and no console errors.

### 7. Wrong vs Correct

#### Wrong

```ts
try {
  await startPasswordReset(input)
} finally {
  turnstileToken.value = ''
  turnstileWidget.value?.reset()
}
```

An older request can finish after the user changes the email and erase the fresh token for the new request generation.

#### Correct

```ts
const generation = ++requestGeneration
try {
  await startPasswordReset(input)
} finally {
  if (generation === requestGeneration) {
    turnstileToken.value = ''
    turnstileWidget.value?.reset()
    pendingAction.value = null
  }
}
```

The repository must apply the same generation-independent safety on the server: lock the normalized reset subject, consume prior active challenges, and recheck identity eligibility inside the confirmation transaction.

## Scenario: Account-Governance Session Audience Isolation

Date: 2026-08-13
Author: Codex

### 1. Scope / Trigger

- Trigger: changing login routing, OAuth callbacks, session middleware, CSRF rotation, account-governance actions, or routes available to restricted users.
- Goal: keep `normal`, `restricted_business`, and `account_appeal` credentials independent while binding restricted access to the exact current governance action.

### 2. Signatures

```go
StartRestrictedBusinessOAuth(ctx context.Context) (string, *domain.AppError)
CompleteRestrictedBusinessOAuth(ctx context.Context, state string, profile OAuthProfile) (AuthenticationResult, *domain.AppError)
GetRestrictedBusinessSession(ctx context.Context, sessionID string) (User, RestrictedBusinessSession, *domain.AppError)
```

```text
normal cookie/state:              c2c_session / c2c_oauth_state
restricted cookie/state:          c2c_restricted_business_session / c2c_restricted_business_oauth_state
account-appeal cookie/state:       c2c_account_appeal_session / c2c_account_appeal_oauth_state
route selection header:            X-Session-Audience: restricted_business
```

### 3. Contracts

- The backend authenticates identity first, then reads the current account projection before creating any session: active and unlocked creates `normal`; suspended/banned and unlocked creates `restricted_business`; archived or security-locked creates no business session. Never create a normal session and downgrade it.
- Restricted and appeal OAuth use separate one-time state tables and purpose-bound cookies. A callback must match exactly one cookie name, signed purpose, and stored state; it never falls back to another purpose.
- Restricted/appeal OAuth resolves only an existing `(provider, provider_subject)`. It must not call registration/upsert paths or mutate identity, profile, attribution, referral, notification, or normal-session facts.
- A restricted session stores the governance action ID, governance version, and restriction effective time. Every read and CSRF rotation atomically rechecks suspended/banned status, no security lock, effective current action, and exact version.
- Route middleware is normal-only by default. A shared business route must opt in and build an explicit `BusinessActor`; the audience header only selects which server credential to validate and never grants authority.
- Status/action/version or security-lock changes invalidate old restricted sessions. Restoration revokes them and never upgrades them into normal sessions.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Restricted state used by appeal callback, or the reverse | `403 CSRF_TOKEN_INVALID`; original matching state remains usable |
| Unknown OAuth identity requests restricted/appeal access | Generic ineligible/restricted response; no user, identity, or session rows |
| Restricted session action/version no longer matches | `401 SESSION_EXPIRED`; CSRF rotation performs no update |
| Security lock is present | No business session is issued or accepted |
| Route does not explicitly allow restricted audience | Reject; never fall back to normal or merge both audiences |

### 5. Good / Base / Bad Cases

- Good: a suspended existing linux.do user completes the restricted purpose and receives only a 24-hour restricted cookie bound to the current action/version.
- Base: normal and restricted cookies coexist; a normal-only route validates only the normal cookie.
- Bad: call ordinary OAuth login and then replace its cookie based on account status.
- Bad: update CSRF first and check governance version in a later query.

### 6. Tests Required

- Unit/handler tests assert password and OAuth audience routing, exact cookie/purpose matching, no fallback, one-time state, unknown-identity no-registration, dedicated logout/CSRF, and explicit route audience declarations.
- PostgreSQL tests assert existing-identity-only completion, state replay rejection, cross-purpose isolation, action/version invalidation, atomic CSRF rejection, and lifecycle cleanup counters.
- Frontend tests assert restricted login does not populate normal session cache and redirects only from the server-returned audience.

### 7. Wrong vs Correct

#### Wrong

```go
result := ordinaryOAuthLogin(profile)
if result.User.Status != AccountStatusActive { replaceCookieWithRestricted(result) }
```

#### Correct

```go
// Purpose-specific state is consumed and current governance is checked in the
// same transaction that creates only the selected session audience.
result, appErr := authService.CompleteRestrictedBusinessOAuth(ctx, state, profile)
```
