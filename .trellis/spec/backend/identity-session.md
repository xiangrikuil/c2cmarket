# Identity, Session, Marketplace Avatar, And User-Facing Time Contract

Date: 2026-07-17
Author: Codex

## Scenario: Linux.do Identity Presentation And Logout Consistency

### 1. Scope / Trigger

- Trigger: changes to linux.do OAuth userinfo decoding, OAuth identity persistence, profile/avatar projection, API-market merchant identity rendering, transaction-email time copy, or logout behavior.
- The contract spans the linux.do provider boundary, Go auth/profile/API-market modules, PostgreSQL identity tables, public API-service DTOs, frontend session cache, TanStack Query cache, and Vue account/market shells.

### 2. Signatures

```text
linux.do GET /api/user response:
  id: string | integer
  username: string
  name: string
  avatar_template: string
  trust_level: integer

GET  /api/v1/auth/session
POST /api/v1/auth/logout
GET  /api/v1/me/profile
POST /api/v1/me/email-verification/start
GET  /api/v1/api-services
GET  /api/v1/api-services/{id}
```

```text
PublicAPIService merchant identity projection:
  merchantIdentityMode: public_profile | store_alias
  merchantDisplayName: string
  merchantProfileSlug: string
  merchantAvatarUrl?: string
```

```go
func normalizeLinuxDoAvatarURL(value string) string
func formatEmailTime(value time.Time) string
```

```ts
type UserProfile = { avatarUrl: string | null }
function logoutBackendSession(): Promise<void>
```

### 3. Contracts

- The provider adapter accepts linux.do `avatar_template` in addition to generic `avatar_url` and `picture`. It replaces the documented `{size}` placeholder with `288` before creating `OAuthProfile`.
- The normalized URL flows through `OAuthProfile.AvatarURL` and `OAuthProfile.LinuxDoAvatarURL`, persists in `users.avatar_url` / `linux_do_bindings.avatar_url`, and is projected as `UserProfile.avatarUrl` by `/api/v1/me/profile`.
- Product UI uses the final `UserProfile.avatarUrl` projection. The account shell renders an image when it is non-empty and falls back to the display-name initial only when it is empty.
- API-market public responses expose the final `merchantAvatarUrl` projection; frontend adapters must not discard it, and all market/detail merchant badges use the shared `ApiMerchantAvatar` component.
- `public_profile` projects the owner's current public display name, username, and selected user avatar (`custom_avatar_url` or linux.do avatar). `store_alias` projects only `merchant_profiles.display_name`, `slug`, and `avatar_url`; it must not fall back to the owner's personal/linux.do avatar because that can deanonymize a store alias.
- Public merchant-profile DTOs expose `avatarUrl` from `merchant_profiles.avatar_url`. Empty avatar values remain empty and render the identity initial; the API must not invent a remote image URL.
- Database and JSON API timestamps keep their existing `time.Time` / RFC3339 semantics. Only transaction-email HTML/text converts business times to fixed UTC+8 and formats them as `YYYY-MM-DD HH:mm:ss（北京时间）`.
- Account-shell logout calls `POST /api/v1/auth/logout` with the cached CSRF token. Success requires backend session revocation and cookie clearing, frontend session/CSRF cache clearing, TanStack Query cache clearing, and navigation replacement to `/login`.
- OAuth codes, provider tokens, raw userinfo payloads, session cookies, and email verification codes must not be logged or persisted outside their existing credential boundaries.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| `avatar_template` contains `{size}` | Store and return the URL with `288` substituted. |
| All provider avatar fields are empty | Keep avatar empty; UI shows the display-name initial. |
| API service uses `public_profile` | Return the owner's selected profile avatar and public username; market/detail UI renders the image. |
| API service uses `store_alias` with a store avatar | Return only the store avatar/name/slug; do not expose the owner's user ID or personal avatar. |
| Store avatar is empty | Keep `merchantAvatarUrl` absent and render the store-name initial. |
| Email time input is UTC | Email copy displays the equivalent UTC+8 wall time with `（北京时间）`, never a trailing `Z`. |
| Logout returns `204` | Clear session/CSRF and all user-derived query data, then replace the route with `/login`. |
| Logout fails before session revocation | Keep the current UI state and show the error; do not claim logout succeeded. |
| A later authenticated user signs in | All profile/business queries reload from the backend instead of reusing the prior user's cache. |

### 5. Good/Base/Bad Cases

- Good: linux.do returns `https://linux.do/user_avatar/linux.do/orbit/{size}/42_2.png`; the profile and account shell use `.../288/42_2.png`.
- Good: a `public_profile` API service returns the same current profile avatar; a `store_alias` service returns its distinct store avatar.
- Base: a provider profile has no avatar; login succeeds and the shell shows the user's initial.
- Base: a store alias has no configured avatar; cards show the store initial without leaking the owner's linux.do image.
- Bad: AppShell uses a mock route jump for logout, or keeps `my-profile` cached after the backend cookie is cleared.
- Bad: the backend stores avatars correctly but omits `merchantAvatarUrl` from `PublicAPIService`, or each card reimplements an initials-only badge.

### 6. Tests Required

- Provider-boundary test with numeric `id` plus `avatar_template`; assert both OAuth avatar fields contain the normalized URL.
- Email sender test with a UTC input; assert HTML/text contain the UTC+8 timestamp and do not contain the UTC RFC3339 value.
- Backend logout route test; assert `204`, session revocation behavior, and the clear-cookie attributes.
- Frontend session-client test; prime the session cache, logout, then assert the next session read performs a network request and uses the prior CSRF token for logout.
- API-market router tests for both identity modes; assert public response name/slug/avatar fields and assert owner/contact identifiers stay absent.
- PostgreSQL projection test must preserve the identity-mode branches, including selected user avatar resolution and the store-avatar-only rule.
- Frontend adapter test must assert `merchantAvatarUrl` survives mapping; type-check/build must cover list, fixed-package, other-API, and detail consumers of `ApiMerchantAvatar`.
- Run `cd backend && go test ./...`, `cd frontend && pnpm test`, Vue type-check, real-mode production build, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
avatarURL := info.AvatarURL
expiresLabel := expiresAt.UTC().Format(time.RFC3339)
merchantAvatarURL := ownerProfile.AvatarURL // even when identity mode is store_alias
```

```ts
router.push('/login')
```

#### Correct

```go
avatarURL := normalizeLinuxDoAvatarURL(firstNonEmpty(info.AvatarURL, info.Picture, info.AvatarTemplate))
expiresLabel := formatEmailTime(expiresAt)
if service.MerchantIdentityMode == "store_alias" {
    service.MerchantAvatarURL = merchantProfile.AvatarURL
} else {
    service.MerchantAvatarURL = ownerProfile.AvatarURL
}
```

```ts
await logoutBackendSession()
queryClient.clear()
await router.replace('/login')
```
