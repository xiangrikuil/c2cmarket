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

## Scenario: Frontend Route Access And Session Invalidation

### 1. Scope / Trigger

- Trigger: adding or changing account, publish, order, merchant, or admin routes; changing frontend session caching; or handling authenticated API failures.
- The shared route table owns access classification. Page components may enforce business prerequisites, but must not be the first login boundary.

### 2. Signatures

```ts
type AuthAccess = 'user' | 'admin'

type ProtectedRouteMeta = {
  auth: AuthAccess
}

function normalizeReturnTo(value: unknown, fallback?: string): string
function loginRoute(returnTo: unknown): RouteLocationRaw
function ensureBackendSession(
  username?: string,
  admin?: boolean,
  options?: { notifySessionInvalidation?: boolean },
): Promise<BackendSession>
function subscribeToBackendSessionInvalidation(
  handler: (error: BackendProblemError) => void,
): () => void
```

### 3. Contracts

- Protected user routes declare `meta.auth = 'user'`; every `/admin/**` record, including redirect records, declares `meta.auth = 'admin'`. Public market, announcement-detail, and public-profile routes have no auth metadata.
- The client global route middleware checks the backend session before mounting a protected page. Missing, expired, or revoked sessions replace navigation with `/login?returnTo=<to.fullPath>`; authenticated non-admin users attempting admin routes are replaced to `/`.
- `AppShell` derives authenticated navigation from `useMyProfileQuery`; it must not maintain a second mutable login flag. Before profile resolution and for anonymous visitors, desktop and mobile navigation expose only public browse groups.
- Anonymous shells hide transaction, account, merchant, admin, notification, and private announcement-center entries. A publish affordance may remain only when it is explicitly labeled as login-required and links to login with the intended publish route as `returnTo`.
- Profile-dependent queries such as notifications, navigation badges, owned objects, and realtime synchronization stay disabled until an authenticated profile exists. Loading must not flash private menus or anonymous login controls.
- `normalizeReturnTo` accepts only same-origin paths beginning with one `/`, preserves path/query/hash, and rejects absolute, protocol-relative, blank, and backslash-normalized external targets.
- `SESSION_EXPIRED` and `SESSION_REVOKED` clear the session/CSRF cache. They notify the global redirect subscriber only when a valid session had previously been cached. An anonymous public-page profile probe must remain anonymous and must not redirect.
- `CSRF_TOKEN_INVALID` clears stale session/CSRF data and follows the existing refresh-and-retry path; it never emits a login redirect.
- Route middleware session probes pass `notifySessionInvalidation: false` because the middleware itself owns the target `to.fullPath`. Other authenticated API failures use the subscriber, clear TanStack Query state, and coalesce concurrent redirects.
- Network and server failures retain their original errors. They must not be rewritten as `SESSION_EXPIRED`.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Anonymous user opens a protected user/admin route | Replace to login with the complete original path, query, and hash. |
| Profile query is unresolved | Show public browse navigation only; do not flash private menus, login, notification, or publish controls. |
| Profile query confirms an anonymous visitor | Keep public browse navigation; show login plus explicitly login-required publish actions; keep private queries disabled. |
| Profile query returns an authenticated user | Restore transaction, publish, account, notification, and eligible merchant/admin navigation. |
| Authenticated non-admin opens `/admin/**` | Replace to `/`; never mount admin content. |
| Anonymous user opens a public route and optional profile query returns `SESSION_EXPIRED` | Stay on the public route; no global invalidation notification. |
| Cached authenticated session receives `SESSION_EXPIRED` or `SESSION_REVOKED` | Clear session and QueryClient data; issue one coalesced login redirect. |
| Mutation receives `CSRF_TOKEN_INVALID` | Refresh session/CSRF and retry once; do not navigate. |
| Session request fails with a network or 5xx error | Preserve the failure for the Nuxt error boundary; do not claim the user is logged out. |
| `returnTo` is absolute, protocol-relative, blank, or normalizes to another origin | Use `/`. |

### 5. Good/Base/Bad Cases

- Good: `/api-market/quota/new?serviceId=id#payment` redirects to login and returns to the exact target after authentication.
- Good: an anonymous visitor can browse `/api-market`, `/announcements/:slug`, and `/u/:username` even though the shell probes optional profile data.
- Good: the anonymous shell shows `登录后发布`, whose menu targets `/login?returnTo=/carpools/new` and `/login?returnTo=/api-market/new`, while private sidebar groups and the notification bell remain absent.
- Base: an already authenticated normal user opens `/admin/users` and returns to `/`.
- Bad: the shell renders all private links and relies on the route guard only after the anonymous visitor clicks them.
- Bad: every `401` from an optional public-page query emits a login redirect.
- Bad: a CSRF retry redirects to login, or a network failure is converted to “请先登录”.

### 6. Tests Required

- Unit-test internal return targets, path/query/hash preservation, and external/protocol-relative/backslash-normalized rejection.
- Route-contract test every `/admin/**` record plus all `/my/**`, `/merchant/**`, publish, and legacy order routes; assert public routes remain unclassified.
- Shell regression test anonymous/loading/authenticated group composition, private control visibility, profile-gated queries, and login-required publish targets. Desktop and mobile must consume the same navigation group source.
- Session-client tests must distinguish cached-session expiry, anonymous session probes, CSRF invalidation, unsubscribe behavior, and preserved network errors.
- Coordinator test concurrent invalidations and assert exactly one redirect.
- Browser-test anonymous direct navigation to protected routes and anonymous access to public market, announcement, and profile routes.
- Run full frontend Vitest, Nuxt type-check, real-mode production build, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```ts
if (error.status === 401) {
  router.replace('/login')
}
```

#### Correct

```ts
const hadCachedSession = cachedSession !== null
if (isSessionCacheInvalidationError(error)) clearBackendSessionCache()
if (hadCachedSession && isLoginRequiredError(error)) notifySessionInvalidated(error)
```

```ts
const isAuthenticated = computed(() => Boolean(myProfile.value))
if (!isAuthenticated.value) return [browseGroup]

const groups = [browseGroup, transactionGroup, publishGroup, accountGroup]
```

## Scenario: Real Avatar Reset And Linux.do Profile Actions

### 1. Scope / Trigger

- Trigger: changing profile avatar shortcuts, linux.do profile links, or user-facing identity synchronization actions.
- The contract spans the profile API, the frontend facade/query cache, and public/account profile pages.

### 2. Signatures

```text
GET   /api/v1/me/profile
PATCH /api/v1/me/profile
```

```ts
function deleteCustomAvatar(): Promise<UserProfile>
function useLinuxDoAvatar(): Promise<UserProfile>
function linuxDoProfileSummaryUrl(username: string): string
```

### 3. Contracts

- In real API mode, both avatar shortcuts first read the current profile and then reuse `PATCH /api/v1/me/profile`; the patch preserves all editable profile and privacy fields while setting `avatarMode="linuxdo"` and clearing the custom avatar URL.
- A successful mutation replaces `['my-profile']` with the server response. A failed request must not mutate the query cache or any Mock store.
- Mock-only profile mutation remains confined to explicitly selected Mock mode.
- Public profile pages show the shared `https://linux.do/u/{username}/summary` link only when a linux.do username exists.
- OAuth login/binding remains the identity synchronization boundary. Do not add a button that reports a successful linux.do sync without a backend request.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Real profile GET or PATCH fails | Surface the error; keep the previous cache value. |
| Profile has a custom avatar | Reset through the real PATCH and return the server projection. |
| linux.do username is blank | Omit the external profile link. |
| Account page displays `lastSyncedAt` | Render it as read-only provider state; do not expose a fake refresh action. |

### 5. Good / Base / Bad Cases

- Good: resetting a custom avatar sends GET plus PATCH and the account shell immediately renders the returned linux.do avatar.
- Base: a user has no linux.do username, so the public page renders no provider link.
- Bad: a real-mode shortcut mutates `mockProfileStore`, writes optimistic success into the query cache, or shows a sync-success toast without network activity.

### 6. Tests Required

- Frontend adapter tests for real GET/PATCH payload preservation, success mapping, and failed-request cache isolation.
- Mock-mode regression for the retained local shortcut behavior.
- Source/component assertions for the shared summary URL helper, conditional link visibility, and removal of fake synchronization controls.
- Full Vitest, typecheck, real-mode build, and authenticated browser checks.

### 7. Wrong vs Correct

#### Wrong

```ts
profile.avatarUrl = null
queryClient.setQueryData(['my-profile'], profile)
toast.success('linux.do information synchronized')
```

#### Correct

```ts
const current = await backendMyProfile()
const updated = await backendUpdateMyProfile({
  ...editableProfilePayload(current),
  avatarMode: 'linuxdo',
  avatarUrl: null,
})
queryClient.setQueryData(['my-profile'], updated)
```
