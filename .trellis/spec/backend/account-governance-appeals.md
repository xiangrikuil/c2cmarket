# Restricted-Account Governance Appeals

Date: 2026-08-03
Author: Codex

## Scenario: Dedicated OAuth-Proven Account-Appeal Capability

### 1. Scope / Trigger

- Trigger: changes to restricted-account OAuth, `account_appeal_sessions`,
  account-governance appeals, account-status mutations, the standalone
  `/account-appeal` page, or Migration 78.
- Owners: `internal/module/auth`, `internal/module/report`,
  `internal/store/postgres`, `internal/server`, the OpenAPI document and
  generated frontend client types.
- Goal: let an existing suspended or banned linux.do identity submit one
  durable appeal without relaxing normal session checks or restoring any
  ordinary application access.

### 2. Signatures

```text
GET  /api/v1/auth/account-appeal/start
GET  /api/v1/auth/oauth/callback
GET  /api/v1/account-appeal/session
POST /api/v1/account-appeal/appeals

OAuth state purpose: account_appeal
Frontend callback:   /account-appeal?accountAppealOutcome=verified|ineligible
Cookie:              c2c_account_appeal; Path=/api/v1/account-appeal
CSRF header:         X-Account-Appeal-CSRF
Lifetime:            created_at + 15 minutes, fixed and never renewed
```

```go
StartAccountAppealSession(ctx, profile) (auth.User, auth.AccountAppealSession, *domain.AppError)
GetAccountAppealSession(ctx, sessionID) (auth.User, auth.AccountAppealSession, *domain.AppError)
GetAccountAppealSessionWithCSRF(ctx, sessionID, csrfToken) (auth.User, auth.AccountAppealSession, *domain.AppError)
CreateAccountGovernanceAppealWithIdempotency(ctx, appellantUserID, routeKey, key, requestHash, input, buildCompletion) (idempotency.Completion, *domain.AppError)
```

```sql
account_appeal_sessions(
  id uuid primary key,
  user_id uuid not null references users(id) on delete cascade,
  session_token_hash text not null unique,
  csrf_token_hash text not null,
  created_at timestamptz not null,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  check (expires_at = created_at + interval '15 minutes')
)
```

### 3. Contracts

- The OAuth callback branches on the signed state purpose. The normal/default
  purpose keeps the ordinary login flow. `account_appeal` may only resolve an
  existing `(provider, provider_subject)` identity and must not create or
  update users, identities, linux.do bindings, profiles, login/activity facts,
  attribution, referrals, coupons, or `auth_sessions`.
- Only existing linux.do identities whose current account status is
  `suspended` or `banned` are eligible. Unknown, active, and archived identities
  share the generic `ACCOUNT_APPEAL_INELIGIBLE` browser outcome.
- The opaque session and CSRF values are hashed before PostgreSQL storage. The
  cookie is HttpOnly, SameSite=Lax, production-Secure, fixed-expiry, and scoped
  to `/api/v1/account-appeal`; normal authentication middleware never accepts
  it. Session reads rotate only the dedicated CSRF token and return
  `Cache-Control: no-store` without extending `expires_at`.
- Production CORS preflights must allow `X-Account-Appeal-CSRF` together with
  `Content-Type` and `Idempotency-Key`; the public frontend and API use separate
  origins.
- `POST /api/v1/account-appeal/appeals` accepts only `{statement}` plus
  `Idempotency-Key`. The server derives appellant, title, target type, and
  target ID. Statements use the shared contact/credential-safe validation.
- Creation acquires the canonical user advisory lock, re-reads the user in the
  same PostgreSQL transaction, and writes an `appeals` row with
  `target_type='account_governance'`, `target_id=appellant_user_id::text`, and
  null report/dispute sources. At most one submitted row may exist per user.
- Account-status mutations acquire the same advisory lock. Approving or
  rejecting an appeal changes only appeal/audit records; it never changes
  `users.account_status`.
- The standalone frontend keeps the dedicated CSRF token in memory. It must
  not populate the ordinary session cache, call `/me` routes, enter the `/my`
  shell, or persist appeal capability state in `localStorage`.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Unknown, active, archived, or non-linux.do identity | `403 ACCOUNT_APPEAL_INELIGIBLE`; generic callback outcome |
| Missing/expired/revoked dedicated cookie | `401 SESSION_EXPIRED` or `SESSION_REVOKED`; no ordinary session fallback |
| Missing/wrong dedicated CSRF header | `403 CSRF_TOKEN_INVALID` |
| Statement shorter than 4 or longer than 1000 characters | `422 VALIDATION_FAILED` |
| Statement contains a complete contact value | `422 CONTACT_CONTENT_DETECTED` |
| Statement contains credential material | `422 SECRET_CONTENT_DETECTED` |
| Missing or over-128-character `Idempotency-Key` | `400 VALIDATION_FAILED` |
| Same idempotency key and request replayed | Replay the original `201` body and `ETag` |
| Different request reuses the key | `409 IDEMPOTENCY_KEY_REUSED` |
| Another submitted account-governance appeal exists | `409 INVALID_STATE_TRANSITION` |
| Status becomes active before transactional recheck | `403 ACCOUNT_APPEAL_INELIGIBLE`; insert rolls back |

### 5. Good / Base / Bad Cases

- Good: a banned user's linux.do subject resolves the existing user, receives
  a 15-minute dedicated cookie, submits once, and receives the self-safe eight
  field appeal projection.
- Base: two session reads rotate CSRF while returning the same fixed
  `expiresAt`; retrying the same create request returns the same result.
- Good: an administrator restores an account while submission is waiting; the
  shared lock serializes the operations and the submit recheck rejects stale
  eligibility.
- Bad: route code calls `LoginWithOAuthProfile` for the appeal purpose, creating
  an ordinary session or refreshing profile/login facts.
- Bad: appeal approval invokes the account-status service and silently restores
  application access.

### 6. Tests Required

- Auth unit/PostgreSQL tests assert existing-identity-only resolution, no
  normal auth/registration/referral side effects, hashed token storage, one
  live session replacement, status eligibility, fixed expiry, CSRF rotation,
  expiry/revocation rejection, and bounded lifecycle cleanup.
- Server tests assert purpose-tagged state, generic ineligible redirects,
  cookie name/path/production Secure attributes, no normal cookie, ordinary
  route isolation, `Cache-Control: no-store`, dedicated CSRF enforcement, and
  the exact self-safe `201` response fields plus `ETag`.
- Middleware tests assert the production account-appeal preflight allows the
  dedicated CSRF header from the configured frontend origin.
- Report/PostgreSQL tests assert statement safety, null legacy sources,
  canonical target ID, submitted uniqueness, idempotent replay, one event,
  shared advisory-lock serialization, and approval/rejection without account
  status mutation.
- Contract/frontend gates assert OpenAPI route parity and generated snapshot,
  standalone route/login link, in-memory CSRF, no normal session cache or
  `/me` access, full Vitest, real-mode typecheck, and production build.
- Migration gates assert empty upgrade, `78 -> 75 -> 78`, `65 -> 78`, rollback
  refusal after durable account-governance appeals, docs version 78, and
  Compose exposure safety.

### 7. Wrong vs Correct

#### Wrong

```go
// This creates or mutates ordinary login state for the restricted user.
user, session, appErr := authService.LoginWithOAuthProfile(ctx, profile)

// This couples appeal approval to automatic access restoration.
updateUserStatus(user.ID, auth.AccountStatusActive)
```

#### Correct

```go
// Resolve an existing identity and issue only the dedicated capability.
user, appealSession, appErr := authService.StartAccountAppealSession(ctx, profile)

// Serialize and recheck status inside the appeal transaction; administrator
// appeal actions update the appeal only.
completion, appErr := reportService.CreateAccountGovernanceAppealWithIdempotency(
    ctx, user.ID, routeKey, key, requestHash, input, buildCompletion,
)
```
