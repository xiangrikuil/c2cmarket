# State Management

> How state is managed in this project.

Date: 2026-06-21
Executor: Codex

---

## Overview

The current frontend is a Vue 3 mock application. Durable domain state is owned by `frontend/src/lib/api.ts`, backed by seeded records from `frontend/src/data/mock.ts` and frontend-only `sessionStorage` stores. Page components should read and write through query hooks and mock API facade functions rather than mutating seed arrays or `sessionStorage` directly.

Pinia is reserved for app/session UI state. TanStack Query owns async server-like reads, mutation invalidation, and derived cache updates.

---

## State Categories

- Local component state: form fields, dialogs, selected tabs, local validation errors.
- URL state: route params and shareable filters such as `category`, `plan`, and search query `q`.
- Session mock domain state: records created or updated through mock API functions, stored under `c2cmarket.*.v1` keys.
- Server-like cache state: TanStack Query results derived from mock API facade functions.

---

## Mock Store Contract

Session-backed mock stores that contain records with stable `id` fields must merge current seed records into stored records on read:

```ts
const storedIds = new Set(stored.map(item => item.id))
return [
  ...stored,
  ...clone(seed.filter(item => !storedIds.has(item.id))),
]
```

Stored records win when the same ID exists, so local user actions remain visible. Seed records that do not exist in the current session are appended, so new mock examples introduced by code changes are not hidden by an older browser session.

Use this only for array stores whose items all have string `id` fields. Do not apply this contract to primitive stores such as notification read IDs or favorites unless the items have a domain record ID and seed data should be preserved.

In real backend mode, notification read state belongs to PostgreSQL `notifications.read_at`. The frontend may keep `notificationReadStore` only for mock mode; real mode `markNotificationRead()` and `markAllNotificationsRead()` must call the backend and invalidate notification query keys.

Feedback unread state belongs to feedback tickets, not to the notification list alone.

- Avatar-menu feedback red dots must read `getFeedbackUnreadCount()` through TanStack Query.
- Admin handling results make a ticket unread when `latestAdminUpdateAt` is newer than `submitterReadAt`.
- Opening a feedback detail or clicking the user confirmation action must call `markFeedbackRead()` and invalidate `feedback`, `feedback-unread-count`, `notifications`, and admin feedback/admin-section query families.
- Marking a feedback notification read from the notification center must also mark the matching feedback ticket read so the notification center and avatar-menu red dot cannot disagree.
- Mock mode may derive feedback notifications from the `feedbackTicketStore`, but the canonical unread flag still comes from ticket timestamps.

In real backend mode, global search state belongs to the backend `GET /api/v1/search` response. The frontend may keep mock aggregation in `api.ts` only for mock mode; real mode `searchMarket()` must call `searchBackend.ts` and must not mix backend results with sessionStorage/mock stores.

In real backend mode, auth state belongs to backend cookies plus `GET /api/v1/auth/session`. The frontend may cache only the returned CSRF token in `backendClient.ts` for subsequent mutations. It must not store OAuth provider access tokens, refresh tokens, callback codes, passwords, cookies, or linux.do raw provider payloads in Pinia, sessionStorage, localStorage, or route query state.

In real backend mode, product catalog state belongs to `GET /api/v1/product-categories`, `GET /api/v1/product-plans`, and admin `/api/v1/admin/product-plans`. Admin create/update/activate/deactivate mutations must invalidate admin plan queries and user-facing active catalog caches. If a backend adapter keeps a small in-memory product-plan cache for publish forms, expose a cache-clear helper and call it from the admin product catalog mutation success path.

## Scenario: Account Recovery Gate After OAuth Registration

### 1. Scope / Trigger

- Trigger: frontend work touching post-login routing, `AppShell.vue`, `MyCenterPage.vue`, `/my/account`, login return targets, verified email state, or password state.
- The first public registration/login path is linux.do OAuth. OAuth-created accounts have no default password, so the frontend must force linux.do-bound users to complete recoverable login settings before authenticated workspace or transaction actions. Public marketplace discovery remains browseable. Unbound development or bootstrap accounts cannot configure a backup password and must not be blocked by that inapplicable requirement.

### 2. Signatures

```ts
type AccountRecoveryProfile = Pick<UserProfile, 'emailVerified' | 'passwordConfigured'> & {
  linuxDoBinding: Pick<UserProfile['linuxDoBinding'], 'bound'>
}

const ACCOUNT_RECOVERY_PATH = '/my/account'

function isAccountRecoveryComplete(profile: AccountRecoveryProfile): boolean
function accountRecoveryRequirements(profile: AccountRecoveryProfile): AccountRecoveryRequirement[]
function isAccountRecoveryAllowedPath(path: string): boolean
function shouldRedirectToAccountRecovery(path: string, authAccess: unknown): boolean
function sanitizeAccountRecoveryReturnTo(value: unknown): string | null
```

### 3. Contracts

- The source of truth is `GET /api/v1/me/profile` mapped to `UserProfile.emailVerified`, `UserProfile.passwordConfigured`, and `UserProfile.linuxDoBinding.bound`.
- Do not store an additional "onboarding complete" flag in Pinia, localStorage, sessionStorage, or route meta.
- Incomplete logged-in accounts are redirected only from routes whose `meta.auth` is `user` or `admin`. Public market lists and details remain browseable so account recovery does not block discovery.
- The path allowlist prevents loops and preserves setup/explanation routes such as `/my/account`, login/mock, announcement details, and public profiles even when they carry authenticated shell metadata.
- Redirects may preserve an internal `returnTo`, but `returnTo` must be same-origin path-only and must not point back to an allowed/setup page.
- `/my/account` requires a verified email for every account. It requires and renders the backup-password step only when `linuxDoBinding.bound=true`; unbound accounts must not see a password action, password step, or recovery redirect caused only by `passwordConfigured=false`.
- The gate is frontend-enforced. If backend API blocking is required later, create a separate backend policy task instead of hiding that decision in frontend code.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Incomplete account opens public `/carpools`, `/api-market/:id`, or `/official-prices/:id` | Keep the public route visible; do not redirect. |
| Incomplete account opens authenticated `/carpools/new`, `/my/api-orders`, or `/admin` | Redirect to `/my/account` with an internal `returnTo`. |
| Email is verified and the account is unbound | No password step or redirect caused by `passwordConfigured=false`. |
| Email is verified and a linux.do-bound account has a password | No redirect; original route remains usable. |
| Incomplete account opens `/my/account` | No redirect loop; recovery tasks render. |
| Incomplete account opens `/u/:username` or `/announcements/:slug` | No redirect. |
| `returnTo` is external, protocol-relative, blank, or points to setup/allowed path | Drop it and do not render a continue action. |

### 5. Good/Base/Bad Cases

- Good: `AppShell.vue` passes `route.path` and `route.meta.auth` to the shared redirect helper, keeps `/api-market/:id` public, and redirects incomplete accounts from `/api-market/new` to `/my/account`.
- Base: login page still uses linux.do OAuth and password-login recovery copy; it does not become a public password registration page.
- Bad: each page independently checks `profile.emailVerified`, or the shell redirects every route without consulting `route.meta.auth`.

### 6. Tests Required

- Unit tests for completion, outstanding requirements, allowed paths, auth-meta redirect decisions, and return target sanitization.
- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Production build: real-mode `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Browser smoke when available:
  - incomplete account opens an authenticated publish/transaction route and reaches `/my/account`;
  - public market list and detail routes are not redirected;
  - completing email plus password setup allows continuing to the original route.

### 7. Wrong vs Correct

#### Wrong

```ts
if (!profile.emailVerified) router.push('/my/account')
```

#### Correct

```ts
if (!isAccountRecoveryComplete(profile) && shouldRedirectToAccountRecovery(route.path, route.meta.auth)) {
  router.replace({ path: ACCOUNT_RECOVERY_PATH, query: { returnTo: route.fullPath } })
}
```

## Scenario: Development API Mode And Account Recovery Persistence

### 1. Scope / Trigger

- Trigger: changes to Nuxt development commands, dotenv loading, runtime API
  mode, the local frontend/backend ports, or account recovery persistence.
- Account recovery state is an identity-domain record, not a page preference.
  Real-mode development must use the same backend and PostgreSQL ownership as
  staging and production.

### 2. Signatures

```ts
type ApiMode = 'real' | 'mock'

function requireApiMode(value: unknown): ApiMode
function setBackendRuntimeConfig(config: {
  apiMode?: string
  apiBaseUrl?: string
}): void
function shouldUseRealBackend(): boolean
```

```text
pnpm --dir frontend dev       -> real, http://127.0.0.1:5173
pnpm --dir frontend dev:mock  -> mock, http://127.0.0.1:5173

NUXT_PUBLIC_API_MODE=real|mock
NUXT_PUBLIC_SITE_URL=http://127.0.0.1:5173
NUXT_DEV_API_PROXY_TARGET=http://127.0.0.1:8080
```

### 3. Contracts

| Level | Change | Appropriate use | Limitation |
| --- | --- | --- | --- |
| Minimum immediate repair | Start Nuxt with `NUXT_PUBLIC_API_MODE=real`, or explicitly load the tracked development dotenv file | Unblock the current local session while backend and PostgreSQL are already running | Depends on the exact start command and does not prevent a later silent fallback |
| Minimum repository repair | Make the default `dev` script load the real-mode development dotenv file and document the command | Small, reviewable correction for the confirmed startup bug | Still needs a contract test to keep the command and runtime mode aligned |
| Long-term repair | Default development to validated `real` mode, provide a separate explicit Mock command, fail fast on missing/invalid mode, and test persistence across a profile refetch/page refresh | Normal ongoing development | Slightly more configuration and test work, but removes ambiguous runtime behavior |

- The long-term repair is the default when the backend contract and database
  persistence already exist.
- The default `dev` command must explicitly load `frontend/.env.development`.
  Do not assume Nuxt loads `.env.development` without `--dotenv`.
- Browser API calls use the same-origin `/api` path. Nuxt proxies it to the
  backend on `127.0.0.1:8080`; a public API base URL is not required locally.
- `real` and `mock` are the only valid modes. Missing or invalid mode fails
  during config loading and must not select Mock.
- Mock is available only through `dev:mock`. Its state is demo data and is not
  a persistence acceptance environment.
- Do not add `localStorage`, `sessionStorage`, or Pinia persistence for
  `email`, `emailVerified`, `emailVerifiedAt`, or `passwordConfigured` in real
  mode. Refresh must read those fields from `GET /api/v1/me/profile`.
- Backend unavailability in real mode remains a visible request failure.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| `NUXT_PUBLIC_API_MODE` is missing or unknown | Nuxt config and runtime config initialization fail with an explicit mode error |
| Default `dev` starts | Runtime payload contains `apiMode:"real"` and listens on `5173` |
| `dev:mock` starts | Runtime payload contains `apiMode:"mock"` and does not claim database persistence |
| Backend on `8080` is unavailable in real mode | API request fails visibly; no Mock record is returned |
| Email/password setup completes in real mode | PostgreSQL-backed profile returns `emailVerified=true` and `passwordConfigured=true` after refetch and frontend restart |

### 5. Good / Base / Bad Cases

- Good: `pnpm --dir frontend dev` loads the tracked development dotenv file,
  proxies `/api` to `8080`, and a refreshed account page reads completed state
  from the backend profile.
- Base: a developer intentionally runs `dev:mock` for a standalone UI demo and
  accepts that a reload may restore Mock seed state.
- Bad: an empty mode silently executes `api.ts` Mock mutations, or the frontend
  stores recovery fields in browser storage to hide that no database write
  occurred.

### 6. Tests Required

- Unit-test `requireApiMode()` with `real`, `mock`, blank, missing, and unknown
  values.
- Assert the default and Mock package commands load the expected mode and use
  port `5173`.
- Adapter regression: set password, confirm email, discard the frontend query
  cache, fetch `GET /api/v1/me/profile` again, and verify both completion
  fields.
- Runtime smoke: read the Nuxt payload for both start commands and assert
  `apiMode`; in real mode, proxy `/readyz` and confirm database readiness.
- Database smoke: complete account recovery through a linux.do-bound fake OAuth
  user, restart the frontend, and read the completed profile again.
- Run full Vitest, Nuxt typecheck, real-mode production build, relevant backend
  tests, source-package test, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```ts
const apiMode = process.env.NUXT_PUBLIC_API_MODE ?? ''
const useMock = apiMode !== 'real'
```

#### Correct

```ts
const apiMode = requireApiMode(process.env.NUXT_PUBLIC_API_MODE)
const useRealBackend = apiMode === 'real'
```

## Scenario: API Service Account Payment Settings Share One Editor And Snapshot Into Publish Payload

### 1. Scope / Trigger

- Trigger: frontend work touching API service payment settings, `ApiPaymentSettingsEditor.vue`, `ApiPaymentSettingsDialog.vue`, `ApiServicePublishPage.vue`, quota-rush publishing, My Center contact/workspace settings, or `submitApiService()` payload construction.
- The platform is a matching surface, not a payment processor. Account-level settings may describe off-platform confirmation instructions only.

### 2. Signatures

```ts
type ApiPaymentMethod = 'wechat' | 'alipay'

type ApiPaymentAccountSettings = {
  paymentWindowMinutes: number
  paymentOptions: Array<{
    paymentMethod: ApiPaymentMethod
    enabled: boolean
    paymentInstructions: string
    paymentQrCodeDataUrl: string | null
  }>
  updatedAt: string
}

getApiPaymentAccountSettings(): Promise<ApiPaymentAccountSettings>
updateApiPaymentAccountSettings(payload: Omit<ApiPaymentAccountSettings, 'updatedAt'>): Promise<ApiPaymentAccountSettings>

GET /api/v1/me/api-payment-settings
PUT /api/v1/me/api-payment-settings

type ApiPaymentSettingsEditorEmits = {
  cancel: []
  'dirty-change': [dirty: boolean]
  saved: [settings: ApiPaymentAccountSettings]
}

useApiPaymentAccountSettingsQuery()
useUpdateApiPaymentAccountSettingsMutation()
```

Publish payload fields remain service-level:

```ts
submitApiService({
  paymentWindowMinutes: number,
  paymentOptions: ApiPaymentAccountSettings['paymentOptions'],
  ...
})
```

### 3. Contracts

- My Center and API publish surfaces must compose the same `ApiPaymentSettingsEditor`; do not keep a second page-local payment form.
- A publish summary emits an `edit` command. The owning publish page renders one `ApiPaymentSettingsDialog` and must not navigate to `/my/contacts` for this action.
- Every dialog opening starts from a deep clone of the latest saved query data. Draft edits remain component-local and must not mutate the summary, query cache, or publish snapshot before save succeeds.
- The buyer payment confirmation window is fixed at 10 minutes; do not restore a 3-15 minute editor.
- WeChat Pay and Alipay settings are complete when a QR-code data URL is present. Their text instructions are optional operational notes.
- WeChat Pay and Alipay are the only supported account payment methods. Normalization must discard legacy or unknown methods such as `usdt`.
- The editor uses one `RadioGroup`: choosing WeChat Pay disables Alipay and choosing Alipay disables WeChat Pay. Inactive QR-code and instruction data remain in the draft and saved account record for later switching.
- Completeness requires exactly one enabled, complete method. Normalization of legacy Mock data with multiple enabled methods keeps only the first supported enabled method deterministically.
- Do not add real-name identity fields to API payment settings.
- API service and quota-rush publish pages render one compact row for the active method and open the shared editor in a dialog; they must not render cards for every supported method or duplicate the full editor inside the form section.
- In real mode the facade must call the authenticated backend endpoints. It must not read, write, or fall back to `localStorage` after a backend failure. Mock mode may keep the existing local facade store.
- Mutation success must write the returned settings into `apiPaymentAccountSettingsQueryKey()`. The existing publish-page watcher then clones query data into `form.paymentWindowMinutes` and `form.paymentOptions`; do not add a second snapshot-update path.
- Successful dialog save closes the dialog and shows success feedback. Failed save keeps the dialog and draft open.
- Closing a dirty dialog through Cancel, close button, overlay, or Escape requires discard confirmation. Continuing keeps the draft; discarding leaves saved settings and the publish form unchanged.
- Entering quota-rush new-service mode with a successfully loaded but incomplete account setting opens the shared dialog once. Dismissing it preserves the service and quota drafts; the next continue attempt opens it again.
- QR removal changes only the draft after a dedicated confirmation and takes effect only after saving.
- My Center must feed the shared editor's dirty state into its existing page-level unsaved-changes guard.
- Publishing copies the current account defaults into `paymentWindowMinutes` and `paymentOptions` so every new service stores a publish-time snapshot. Order creation then copies that service snapshot into the order.
- Updating account settings later must not silently change already-published services; changing a service must not rewrite existing orders.
- Public API service list/detail responses must not expose raw payment instructions or QR-code material. A purchase-intent detail may show the frozen snapshot to participants only.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| `paymentWindowMinutes !== 10` | Shared editor save blocks; publish page remains blocked. |
| No enabled payment method | Shared editor shows a missing-settings reason; publish CTA says to configure account payment settings. |
| Both methods are enabled in a draft | Completeness fails and save requires one selected method. |
| Enabled WeChat Pay / Alipay lacks a QR code | Shared editor save blocks and publish remains incomplete. |
| Legacy or unknown payment method is loaded | Normalization drops it; only WeChat Pay and Alipay remain, with at most one enabled. |
| Instructions include API keys, tokens, passwords, cookies, sessions, payment codes, bank-card numbers, or panel credentials | Save/publish validation rejects the content with visible boundary copy. |
| Dirty dialog close is requested | Show discard confirmation; continuing preserves the draft and discarding preserves saved/query state. |
| Account update succeeds | Query cache updates, the existing watcher refreshes the publish snapshot, the dialog closes, and success feedback appears. |
| Account update fails | Query cache and publish snapshot remain unchanged; the dialog stays open with its draft. |
| Real-mode account read fails | Show the request error; do not return Mock or browser-local settings as success. |
| Quota new-service continue runs without a complete setting | Keep every publish field, open the shared dialog, and remain on the same step and route. |
| Account settings are complete | Publish page copies settings into the hidden service snapshot fields and preview shows method labels plus confirmation window. |

### 5. Good/Base/Bad Cases

- Good: a merchant opens the publish summary dialog, selects Alipay, saves its QR code, immediately sees one `支付宝 · 固定 10 分钟确认` row, and submit still includes service-level `paymentOptions`.
- Base: no account settings exist; quota new-service mode opens an isolated inline dialog without changing the route, step, service form, or quota form.
- Bad: both methods remain enabled, the summary renders one card per method, real mode reads `localStorage`, the summary links to `/my/contacts`, or a service stores a live reference to mutable account settings.

### 6. Tests Required

- Unit tests must cover normalization to WeChat Pay/Alipay only, legacy dual-enabled normalization to one method, exactly-one completeness, QR data-URL validation, fixed 10-minute normalization, and cloned draft isolation.
- Backend-adapter tests must assert GET/PUT paths, session/CSRF behavior, response normalization, and no real-mode Mock fallback.
- Component/source regressions must prove My Center and publish reuse the shared editor, the editor uses radio semantics, publish owns one dialog, the summary renders only the enabled method, and the publish summary has no `RouterLink` or `/my/contacts`.
- Dialog regressions must cover fresh sessions, dirty close, continue editing, discard, successful/failed save lifecycle, QR upload validation, and confirmed QR removal.
- Query tests must assert mutation success updates `apiPaymentAccountSettingsQueryKey()` and the existing publish watcher copies that data into the service snapshot.
- Quota publish regressions must assert missing settings open the dialog on new-service entry and again on continue, while the service/quota form objects remain intact.
- `pnpm --dir frontend test`.
- `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Real-mode build: `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Source scan product-boundary copy around the touched publish/My Center files for payment custody, credentials, API keys, tokens, cookies, sessions, payment codes, and escrow wording.
- Browser smoke at `1440x900` and `390x844` must verify free, fixed-package, and limited modes open the dialog without route changes; dirty close/discard works; save updates the summary; mobile content scrolls with a reachable save action and no horizontal overflow.
- Browser or curl smoke must verify `/api-market/new` direct deep link renders the application and does not get swallowed by the Nuxt development `/api/` proxy.

### 7. Wrong vs Correct

#### Wrong

```vue
<RouterLink to="/my/contacts">配置 API 收款设置</RouterLink>
<PaymentSettingsSection :form="form" />
```

#### Correct

```vue
<AccountPaymentSummarySection
  :form="form"
  :settings="accountPaymentSettingsValue"
  @edit="paymentSettingsDialogOpen = true"
/>
<ApiPaymentSettingsDialog
  v-model:open="paymentSettingsDialogOpen"
  :settings="accountPaymentSettingsValue"
/>
```

```ts
watch(accountPaymentSettingsValue, settings => {
  form.paymentWindowMinutes = settings.paymentWindowMinutes
  form.paymentOptions = settings.paymentOptions.map(option => ({ ...option }))
}, { immediate: true })
```

---

## When to Use Global State

Use global/session mock state only when multiple pages must observe the same record after a mutation. Examples:

- API orders shown in API detail, buyer list, merchant list, notifications, and admin tracking views. Purchase intents remain internal adapter state and must not become a parallel user-facing queue.
- Carpool applications shown in carpool detail, my rides, owner applications, notifications, and admin views.
- Published carpool, API service, and price records that must appear in both user and admin pages.
- Feedback tickets shown in the avatar menu, feedback history, notification center, and admin feedback queue.
- Backend session and permission state shown in the login/account shell should be refreshed from `getCurrentBackendSession()` instead of mirrored as a mutable role selector in global state.
- Product catalog plans shown in low-price submit and carpool publish flows after admin plan mutations.

Keep transient UI choices local unless they must be shareable through the URL.

---

## Server State

All route-level domain reads should go through TanStack Query hooks in `frontend/src/queries/useMarketQueries.ts`. Mutation success must invalidate every affected query family rather than relying on a single local page update.

If a mutation needs immediate UX feedback, it may use `queryClient.setQueriesData` for the directly affected list, but the canonical mock store in `lib/api.ts` must still be updated first.

---

## Common Mistakes

- Letting an old `sessionStorage` array fully replace `mock.ts` seed records. This hides newly added demo rows such as new product examples and causes screenshot reviews to disagree with source code.
- Updating only a page-local `submittedId` after a publish action instead of writing the mock store and invalidating related queries.
- Invalidating only the admin product-plan list after a catalog mutation while leaving user-facing active plan dropdown caches stale.
- Duplicating status labels in page code instead of using facade helpers from `lib/api.ts`.
- Adding component-level fallback arrays that make an empty store look like valid data.
- Keeping a frontend role switcher or mock auth store active in real backend mode. Real mode permissions must come from `session.user.permissions`.
- Using notification unread count as the source of truth for feedback red dots. Feedback red dots must come from feedback unread count so admin result handling, detail reads, and notification-center reads stay consistent.

## Scenario: API Service Publish Page Composes Auto Approval And Publish

### 1. Scope / Trigger

- Trigger: frontend work touching `ApiServicePublishPage.vue`, `submitApiService()`, or `backendSubmitAPIService()`.
- The publish page is a new-service one-shot publish workflow, not a draft editor. It must not expose draft save until a complete draft resume/edit route exists.

### 2. Signatures

```ts
submitApiService({ status: 'reviewing', ...form }): Promise<ApiService>
backendSubmitAPIService(payload): Promise<ApiService>
```

Real backend calls used by the adapter:

```text
POST /api/v1/owner/api-services
POST /api/v1/owner/api-services/{id}/submit-review
POST /api/v1/owner/api-services/{id}/publish
```

### 3. Contracts

- `status: 'reviewing'` remains the frontend facade's publish intent for compatibility with carpool-style form payloads.
- In real backend mode, publish-page submission must create the service, run linux.do early auto approval through `submit-review`, then immediately call `publish` with the returned version.
- In mock mode, publish-page submission must return `state: 'online'`, `online: true`, and no review/admin warning. Mock mode represents a fully orderable development listing; real backend public visibility still depends on order settings.
- Public visibility is still controlled by the backend orderable predicate; publishing a service is not the same as making it publicly orderable.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| linux.do-bound owner submits complete publish form | Adapter returns a published service state, not `reviewing`. |
| owner is not linux.do-bound | Backend `submit-review` fails visibly; the UI must not convert it into mock success. |
| service lacks order settings/payment options in real backend mode | Service can be published but remains hidden from public orderable reads until configured. |
| mock publish page submission succeeds | The service appears online immediately and owner center must not show a second `上线` action. |
| raw draft service exists from a legacy path | Owner center may show it, but publish page must not create new dead-end drafts. |

### 5. Good/Base/Bad Cases

- Good: publish page primary CTA is `发布 API 服务`; real backend adapter performs create -> submit-review -> publish; mock facade returns an online service.
- Base: legacy owner center can still call `publishApiService(id)` for approved/offline records.
- Bad: publish page saves `draft/offline`, then shows an `上线` action that backend rejects because the service is not approved.

### 6. Tests Required

- `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Real-mode build: `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Source scan on `ApiServicePublishPage.vue` for removed copy: `提交审核`, `保存草稿`, `等待管理员审核`, `仍需手动上线`.
- Backend full tests when the real adapter sequence depends on existing owner action contracts: `cd backend && go test ./...`.

### 7. Wrong vs Correct

#### Wrong

```ts
if (payload.status === 'reviewing') {
  response = await backendOwnerAPIServiceAction(response.id, 'submit-review', response.version)
}
```

#### Correct

```ts
if (payload.status === 'reviewing') {
  response = await backendOwnerAPIServiceAction(response.id, 'submit-review', response.version)
  response = await backendOwnerAPIServiceAction(response.id, 'publish', response.version)
}
```

## Scenario: API Service Public Detail Requires Orderable State

### 1. Scope / Trigger

- Trigger: frontend work touching API service public lists, public detail links, owner/admin API service rows, `mapBackendAPIService()`, or `getApiServiceById()`.
- Backend `publicationStatus=online` means the owner has published the service. It does not by itself mean public `GET /api/v1/api-services/{id}` can return the service.

### 2. Signatures

```ts
type ApiService = {
  online: boolean
  publiclyOrderable: boolean
}

function isApiServicePubliclyOrderable(service: Pick<ApiService, 'online' | 'publiclyOrderable'>): boolean
function getApiServicePublicDetailUrl(service: Pick<ApiService, 'id' | 'online' | 'publiclyOrderable'>): string | null
```

Real backend response field:

```ts
type BackendAPIService = {
  publicationStatus?: string
  isOrderable?: boolean
}
```

### 3. Contracts

- `online` represents owner publication state.
- `publiclyOrderable` represents public-market readability and purchase-intent availability.
- In real backend mode, `publiclyOrderable` must be mapped from backend `isOrderable`.
- Public API service market lists and search results must include only `publiclyOrderable` services.
- Owner/admin rows may show published-but-not-orderable services, but must not link them to `/api-market/:id`.
- Direct public detail route 404 must remain visible as an unavailable/not-public UI state; do not silently fall back to mock or owner/admin data.
- Mock mode may set `publiclyOrderable = online` for published demo services, but normalization must fill this field for older `sessionStorage` records.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| `publicationStatus=online`, `isOrderable=false` | Owner/admin row shows the service but public detail link is disabled or absent. |
| `publicationStatus=online`, `isOrderable=true` | Public list/detail link and purchase-intent panel are available. |
| Direct `/api-market/:id` for backend 404 | Detail page shows `API 服务暂未公开` or equivalent unavailable state. |
| Real backend public list response contains a non-orderable row | Frontend adapter filters it out before market/search rendering. |
| Old mock session record lacks `publiclyOrderable` | Normalization derives it from `online`. |

### 5. Good/Base/Bad Cases

- Good: `getApiServicePublicDetailUrl(service)` returns `null` for an online service that still lacks order settings.
- Base: API publish flow can return an online service while the owner still needs to configure accepting-orders/payment options.
- Bad: owner center, admin drawer, favorites, search, or notification code builds ``/api-market/${service.id}`` for a service that is not `publiclyOrderable`.

### 6. Tests Required

- `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Real-mode build: `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Backend route tests or full suite to preserve the existing public 404-before-order-settings contract: `cd backend && go test ./...`.
- Source scan for public detail links around API services when changing owner/admin/search/favorite surfaces.

### 7. Wrong vs Correct

#### Wrong

```vue
<RouterLink :to="`/api-market/${item.id}`">
  <Button>查看</Button>
</RouterLink>
```

#### Correct

```vue
<RouterLink v-if="getApiServicePublicDetailUrl(item)" :to="getApiServicePublicDetailUrl(item)!">
  <Button>查看</Button>
</RouterLink>
<Button v-else disabled>待配置接单</Button>
```

## Scenario: Realtime Invalidation And Authoritative Navigation Badges

### 1. Scope / Trigger

- Trigger: frontend work touching `AppShell.vue`, notification read actions, API-order/carpool actions, administrator queues, navigation badges, or the authenticated realtime connection.
- The UI must update while open without making a bell click or route navigation act as a refresh mechanism.

### 2. Signatures

```ts
type NavigationBadgeSummary = {
  generatedAt: string
  notificationUnread: number
  importantAnnouncementUnread: number
  feedbackUnread: number
  buyer: { carpoolActions: number; apiOrderActions: number }
  merchant: { carpoolActions: number; apiOrderActions: number }
  admin: null | {
    total: number
    officialPrices: number
    carpools: number
    apiServices: number
    feedbackTickets: number
    reports: number
  }
}

useNavigationBadges(enabled)
useRealtimeSync(enabled)
```

Named SSE events `ready` and `invalidate` carry `{ schemaVersion: 1, topics: ['all-live'] }`.

### 3. Contracts

- `AppShell.vue` mounts one `useEventSource` connection only when real-backend mode and an authenticated profile are present. Mock mode must not construct `EventSource`. Fatal `CLOSED` streams retry indefinitely with a bounded delay and reopen immediately when the browser returns online.
- SSE events are invalidations only. They invalidate navigation badges, notifications, announcements, feedback, API orders/intents, carpool applications/details/contacts, and administrator query families; components never increment counts from an event.
- `useNavigationBadges` refetches every 15 seconds only while visible and also refetches on mount, focus, and reconnect. When SSE is not open, the realtime composable performs the same broad 15-second reconciliation and reconciles immediately on visibility/network recovery.
- AppShell reads buyer, merchant, feedback, notification, announcement, and administrator counts from `NavigationBadgeSummary`; it must not keep full order/application lists mounted only to count them.
- Bell badge is `notificationUnread` only. Important-announcement unread count belongs on the platform-announcement entry. Opening the bell only opens its dropdown; `查看全部通知` is a separate link.
- Mark-notification-read/read-all, announcement receipt, feedback, API-order, and carpool workflow mutations invalidate `navigation-badges` in addition to their normal query families. Direct page-level facade mutations follow the same rule, so the current actor never waits for fallback polling. Badge counts are never adjusted heuristically.
- The strict envelope decoder may reject invalid payloads in tests, but the EventSource serializer boundary converts malformed/unsupported server events to `null`; bad data must not become an uncaught browser exception or mutate cache state.
- Real backend failures remain visible; do not return mock badge data from `backendNavigationBadges()` on failure. Mock summary is derived from current mock stores and the same actionable-state semantics.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| No authenticated profile | Badge query disabled; no SSE connection |
| Mock mode | Deterministic local summary; no SSE |
| SSE open/reopen | Immediately invalidate all-live query families |
| SSE closed/error | Visible-page 15-second reconciliation remains active |
| Page hidden | Stop badge/background polling; reconcile when visible again |
| Unknown/malformed realtime envelope | Do not mutate cached counts; REST polling remains authoritative |
| EventSource enters terminal `CLOSED` | Reconnect after 3 seconds indefinitely; an `online` event may reopen immediately |
| `admin=null` | Do not render administrator badge values |
| Count is zero / above 99 | Hide zero; display `99+` above 99 |

### 5. Good/Base/Bad Cases

- Good: seller confirms payment in another browser; buyer receives `invalidate`, badge/order detail refetch, and sees the new state without touching the bell.
- Base: SSE message is missed; the next focus, reconnect, or 15-second reconciliation reaches the same REST state.
- Bad: admin carpool-application badge reuses the current user's owner queue count.
- Bad: bell click routes immediately to the notification page or exists primarily to trigger a fetch.

### 6. Tests Required

- Envelope parser rejects unsupported version/topic and accepts `all-live`.
- All-live invalidation list covers summary, notification, order, carpool, feedback, announcement, and admin prefixes.
- Source/integration tests assert no hard-coded admin counts, undefined admin queues stay unbadged, bell has a separate full-center link, and AppShell no longer mounts full lists for counts.
- Mutation tests assert notification/order/feedback/announcement success invalidates `navigation-badges`.
- Full Vitest, Nuxt typecheck, and real-mode Nuxt production build.

### 7. Wrong vs Correct

#### Wrong

```ts
const count = orders.value.filter(order => order.status === 'open').length
eventSource.onmessage = () => count++
```

#### Correct

```ts
const badges = useNavigationBadges(isAuthenticated)
useRealtimeSync(isAuthenticated) // invalidates; REST recomputes
```

## Scenario: API Order Payment And Cancellation UI State

### 1. Scope / Trigger

- Trigger: frontend work touching `ApiPurchaseOrderDetailPage.vue`, API-order payment instructions, buyer cancellation, or merchant processing timers.

### 2. Contracts

- `paymentExpiresAt` is the authoritative buyer payment deadline. The page may update its display clock once per second, but it must not persist another buyer deadline.
- After `payment_submitted`, the first-release merchant handling hint is derived as `paymentSubmittedAt + 10 minutes`. This derived time controls urgency and support copy only; it must not mutate the backend order or auto-cancel a potentially paid order.
- Payment instructions use a centered Dialog. Marking payment requires a separate confirmation Dialog and must close the payment details before opening the confirmation focus trap.
- Buyer cancellation is available only for `pending_payment`. The right-side cancellation Dialog requires a selected reason plus an explicit unpaid confirmation, then calls `cancelApiOrder(id, reason, version)`.
- Cancellation reasons must include a readable responsibility label (`商家原因` or `个人原因`) in the persisted text. Do not block immediate unpaid cancellation on seller confirmation.
- API-order mutations must update the actor detail cache and invalidate buyer, merchant, notification, navigation-badge, admin, and intent query families.
- Contact issue micro-actions may be hidden on the API order page when a single customer-support entry is present, but `OrderContactCard` keeps them enabled by default for other consumers.

### 3. Validation Matrix

| Condition | Expected behavior |
| --- | --- |
| Pending payment with active deadline | Show countdown, payment details action, and cancellation action. |
| Pending payment without QR material for a QR method | Show the missing-material state and disable payment confirmation. |
| Buyer confirms payment | Close payment UI, hide cancellation, switch to merchant handling countdown. |
| Merchant handling hint expires | Keep order state unchanged; show overdue copy and customer support. |
| Buyer cancellation without reason or unpaid confirmation | Keep destructive submit disabled. |
| Cancelled with `payment_timeout` | Render localized system-timeout copy, not the raw code. |

### 4. Tests Required

- Unit tests for countdown boundaries, merchant deadline derivation, cancellation reason formatting, and system reason labels.
- Mock facade test for pending-only cancellation and reason persistence.
- `pnpm --dir frontend exec vitest run`.
- `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Real-mode `pnpm --dir frontend build` with the required Nuxt runtime API variables.

## Scenario: Decimal API Orders As The UI Source Of Truth

### 1. Scope / Trigger

- Trigger: API service purchase previews, API order lists/detail, amount sorting/totals, legacy intent routes, or administrator API transaction views.

### 2. Signatures

```ts
ApiService.cnyPerUsdAllowance?: string
ApiService.availableUsdAllowance?: string
ApiService.maxUsdAllowancePerOrder?: string
ApiOrder.amountDecimal?: string
ApiOrder.requestedUsdAllowanceDecimal?: string
```

### 3. Contracts

- UI/domain adapters preserve backend decimal strings. `decimal.js` helpers own division, multiplication, comparison, addition, normalization, and display formatting.
- Purchase preview and submit derive allowance from the same CNY amount/rate strings. Display may trim trailing zeros; submitted and snapshot forms retain required scale.
- Buyer, merchant, and admin screens use “API 订单” as the primary object. Purchase intents remain internal adapter/API state and historical `/api-intents/{id}` resolves to the linked order route.
- The five visible responsibility steps are `创建订单 → 买家付款 → 商户确认收款 → 商户交付 → 买家验收`.
- Admin order rows may state that delivery was submitted but must never map or serialize the raw `deliveryCredential`.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| `10.00 / 0.8000` | Preview/list/detail show `$12.50`, submit uses `12.500000` |
| Sort or total decimal amounts | Use `compareDecimal` / `addDecimal`, never subtraction or numeric reduce |
| Historical intent has linked buyer order | Replace route with `/my/api-orders/{orderId}` |
| Historical intent has linked merchant order | Replace route with `/merchant/api-orders/{orderId}` |
| No linked order is visible | Return to the order list without rendering an intent detail as an order |

### 5. Good/Base/Bad Cases

- Good: lists render `¥10.00` and `12.50 美元额度` from immutable order strings.
- Base: a fixed-package order has no USD allowance and renders the package snapshot path.
- Bad: `Math.round(amount * creditPerCny)`, `b.amount - a.amount`, or `reduce((sum, row) => sum + row.amount)` on money/quota fields.

### 6. Tests Required

- Decimal helper regression for `10 / 0.8 = 12.500000`, formatting, comparison, and addition.
- Admin adapter regression proving a provided raw credential does not appear in the row JSON.
- Full Vitest suite, Nuxt typecheck, and real-mode Nuxt production build.

### 7. Wrong vs Correct

#### Wrong

```ts
const allowance = Math.round(amount * creditPerCny)
rows.sort((a, b) => b.amount - a.amount)
```

#### Correct

```ts
const allowance = divideDecimal(amount, cnyPerUsdAllowance, 6)
rows.sort((a, b) => compareDecimal(b.amountDecimal!, a.amountDecimal!))
```

## Scenario: Carpool Eligibility Projection

### 1. Scope / Trigger
- Trigger: carpool list or detail CTA, seat labels, application mutations, or backend eligibility adapter changes.

### 2. Signatures
```ts
type CarpoolApplicationEligibility = { code; canApply; reason; resolutionAction }
getCarpoolApplicationEligibility(id): Promise<CarpoolApplicationEligibility>
```

### 3. Contracts
- Real mode consumes the authenticated backend projection; mock mode uses `evaluateCarpoolApplicationEligibility` with the same codes and priority.
- The detail page has one application CTA. Its status badge, disabled state, reason, and seat label all derive from the same projection.
- Loading/error states are non-optimistic and cannot briefly show an enabled application action.

### 4. Validation & Error Matrix
| State | UI |
| --- | --- |
| `eligible` | One enabled `申请上车` action |
| blocked code | Disabled `当前不可申请` plus exactly one reason |
| query loading/error | Disabled neutral status; never optimistic apply |

### 5. Good/Base/Bad Cases
- Good: credential risk changes the badge, seat label, button, and reason together.
- Base: eligible response shows one primary CTA.
- Bad: a second computed property reconstructs risk/ownership/seat priority from raw listing fields.

### 6. Tests Required
- All eight codes, combined priority, and single-dialog-entry source regression.
- Full Vitest suite, typecheck, and real API production build.

### 7. Wrong vs Correct
#### Wrong
```ts
const disabled = paused || risky || hasApplication || seats === 0
```
#### Correct
```ts
const disabled = !eligibility.canApply
const reason = eligibility.reason
```

## Scenario: API Market Infinite Cursor Queries

All real-backend business lists follow the same ownership rule: the browser may retain opaque cursors for previous/next navigation, but it must not slice, filter, or sort a fetched array to manufacture pages. Every visible search, filter, and sort value belongs in the query key and backend request. Array pagination helpers are Mock-only and real mode must fail visibly when a server pagination adapter is missing.

### 1. Scope / Trigger

- Trigger: changes to API market service/offer lists, cursor adapters, market
  filters or tabs, SSR prefetch, or the infinite-scroll sentinel.

### 2. Signatures

```ts
type CursorPage<T> = { items: T[]; nextCursor?: string }

useInfiniteApiServices(filters, enabled, scope)
useInfiniteApiQuotaOffers(filters, enabled)
flattenUniqueCursorPages(pages)
```

### 3. Contracts

- Infinite queries request 20 rows per page, pass `nextCursor` back unchanged,
  and stop when it is absent. Query keys contain every server filter plus the
  market-view scope when two tabs share one endpoint.
- Hidden market views keep their query disabled. Filter, slot, or view changes
  create a new cursor chain from the first page rather than reusing stale data.
- API package/free tabs send the billing mode to the backend. Once a package
  model and duration are selected, both values are server filters; limited
  quota search and system-slot exclusion are also server filters. Components
  may defensively project returned DTOs, but must not use current-page array
  filtering as the source of visible matching results.
- Flattened pages are deduplicated by business ID; a later copy replaces the
  stale record without changing the first-seen card position.
- Each visible product source owns one sentinel. Intersection loads the next
  page only when it exists and no load/error is active. A next-page error keeps
  prior cards visible and exposes retry; the final page shows a terminal state.
- SSR prefetches only the first page of the currently visible top-level market
  view. Dependent sale-slot offers still await the slot list, select the same
  slot as the client, and then prefetch that slot's first page.
- Existing facade reads that promise a complete array may collect every page,
  but must fail on a repeated cursor.

### 4. Validation & Error Matrix

| Condition | Required UI behavior |
| --- | --- |
| First page has no cursor | Render rows/empty state and `已加载全部`; no extra request |
| Sentinel enters the 400px preload margin | Fetch one next page |
| Next page fails | Preserve loaded cards and show `重试加载` |
| Search/filter/view/slot changes | New query key and first-page cursor; matching is applied before backend pagination |
| Adjacent pages repeat an ID | Render one card using the later record |
| Hidden tab | No background page requests |
| SSR route uses `view=free` | Prefetch service page only, not limited offers |

### 5. Good / Base / Bad Cases

- Good: scrolling loads later cards, a transient failure leaves existing cards
  usable, and retry continues from the same cursor.
- Base: one short page naturally ends without a second request.
- Bad: `useQuery` discards `nextCursor`, both tabs prefetch on every SSR request,
  or a filter change appends new results to the old cursor chain.

### 6. Tests Required

- Cursor adapter serialization and null/blank cursor normalization.
- Mock paging, page-boundary deduplication, and repeated-cursor rejection.
- Query-key, enabled-state, visible-view SSR prefetch, and four-sentinel source
  regressions.
- Full Vitest, Nuxt typecheck, real-mode production build, and browser checks
  for incremental loading, retry, tab isolation, and horizontal overflow.

### 7. Wrong vs Correct

#### Wrong

```ts
const query = useQuery({ queryFn: () => getApiServices(filters.value) })
prefetchQueriesOnServer(quotaQuery, servicesQuery)
```

#### Correct

```ts
const query = useInfiniteQuery({
  queryKey: computed(() => ['api-services', 'infinite', view.value, filters.value]),
  queryFn: ({ pageParam }) => getApiServicesPage(filters.value, { limit: 20, cursor: pageParam || undefined }),
  getNextPageParam: page => page.nextCursor,
})

const visibleQuery = view.value === 'limited' ? quotaQuery : servicesQuery
prefetchQueriesOnServer(visibleQuery)
```

## Scenario: 通知与平台公告查询页签隔离

### 1. Scope / Trigger

- 触发条件：修改 `/my/notifications` 的 URL 页签、通知分类、公告列表或 `AppShell` 中指向同一路径的导航项。
- 目标：业务通知与平台公告可以共享路由页面，但不能共享活动状态或混排内容。

### 2. Signatures

```ts
type NotificationTab = 'todo' | 'transactions' | 'system' | 'announcements'

const announcementCenterTo = '/my/notifications?tab=announcements'
```

### 3. Contracts

- `tab=announcements` 必须直接映射到 `announcements`，不得降级成 `system`。
- `todo`、`transactions`、`system` 只渲染站内业务通知；`announcements` 只渲染公告查询结果。
- “系统通知”计数不包含公告未读数；公告未读数只属于“平台公告”。
- 待办、交易和系统页签数字必须复用渲染列表的同一个分类函数，不能用另一组业务类型汇总近似计算。
- 同一路径存在普通通知和公告两个侧栏入口时，活动态必须同时判断 `route.path` 与 `route.query.tab`。
- 公告入口属于认证用户的正常导航项，不得再用侧栏底部卡片重复表达同一目的地。
- 切换页签继续使用 URL 查询状态，使刷新、返回和深链接保持一致。

### 4. Validation & Error Matrix

| URL / 操作 | 必须结果 |
| --- | --- |
| `/my/notifications` | 选中“通知”和默认待办页签 |
| `?tab=system` | 只显示系统通知，公告行数为 0 |
| `?tab=announcements` | 选中“平台公告”，只显示公告列表 |
| 点击“平台公告”页签 | URL 更新为 `tab=announcements`，标题和侧栏活动态同步 |
| 未知 `tab` | 规范化回默认待办，不静默合并到其他业务页签 |
| 任一通知页签 | 页签数字等于该页签实际可见行数 |

### 5. Good / Base / Bad Cases

- Good：公告页显示“平台公告”标题、公告导航选中，系统通知不会出现在公告列表上方。
- Base：通知与公告继续复用现有路由、查询 hooks 和 shadcn-vue `Tabs`。
- Bad：为了复用模板把 `announcements` 映射成 `system`，再在系统通知卡片后追加公告卡片。

### 6. Tests Required

- 源契约测试断言四项 `NotificationTab`、独立查询映射、独立模板分支和查询感知的侧栏匹配。
- 浏览器分别打开 `tab=system` 与 `tab=announcements`，断言标题、选中页签、侧栏活动态和两类行数互斥。
- 运行全量 Vitest、Nuxt typecheck、real-mode production build，并在 390px 与 1440px 检查横向溢出。

### 7. Wrong vs Correct

#### Wrong

```ts
if (route.query.tab === 'system' || route.query.tab === 'announcements') return 'system'
```

#### Correct

```ts
if (route.query.tab === 'system') return 'system'
if (route.query.tab === 'announcements') return 'announcements'
```

## Scenario: 公告详情阅读层级与 Smoke 数据边界

### 1. Scope / Trigger

- 触发条件：修改用户公告详情布局、公告 Markdown 渲染、真实后端浏览器验证或 `announcement-smoke.mjs`。
- 目标：用户详情只呈现公告本身，自动化测试数据不得借助前端过滤隐藏，也不得在验证后继续出现在用户公告列表。

### 2. Signatures

```text
GET /api/v1/announcements
GET /api/v1/announcements/{slug}
POST /api/v1/admin/announcements/{id}/offline
```

```vue
<Card class="announcement-reference-article mx-auto max-w-4xl">
  <article><!-- metadata + Markdown + optional CTA --></article>
</Card>
```

### 3. Contracts

- 用户公告详情采用一个受限宽度的单栏文章容器；分类、标题、摘要、发布时间、条件更新时间、正文和 CTA 构成连续阅读顺序。
- 页面不得增加通用“阅读说明”侧栏，也不得在正文旁重复提供与底部返回按钮相同的公告列表动作。
- `AnnouncementDetailContent` 必须忠实渲染后端返回的 `contentMarkdown`。生产前端不得根据 `Smoke 公告`、`发布后编辑` 或其他测试关键词删除标题、摘要或正文。
- 真实后端 smoke 记录在验证期间可以短暂发布，但验证结束后原记录必须下线，复制记录必须保持草稿或下线。
- 浏览器手动发布 smoke 副本完成验证后必须恢复为下线；若本地记录已跨过 `expireAt` 而管理 API 不再允许下线，使用精确限定标题/ID 的本地数据修复，并保留记录而不是删除审计历史。
- 验证结束后，用户公告列表中标题以 `Smoke 公告` 开头的记录数必须为 0。

### 4. Validation & Error Matrix

| 条件 | 必须结果 |
| --- | --- |
| 正常公告详情 | 单栏文章，无右侧说明卡 |
| 公告正文包含“发布后编辑” | 按正文正常显示，不做前端关键词过滤 |
| smoke 原记录验证完成 | 状态为 `offline`，用户接口不可见 |
| smoke 复制记录未参与发布验证 | 状态保持 `draft`，用户接口不可见 |
| 手动发布 smoke 副本后结束验证 | 恢复为 `offline`，用户接口不可见 |
| 公告不存在或不可见 | 保留明确空状态和一个返回公告列表动作 |

### 5. Good / Base / Bad Cases

- Good：详情页只保留一篇文章和底部返回动作，真实用户列表没有 smoke 记录。
- Base：Markdown 内容、CTA、自动已读和条件更新时间继续复用现有组件与查询契约。
- Bad：因为测试正文难看就在 Vue 模板中判断标题前缀，或在详情右侧加入与正文无关的固定阅读说明。

### 6. Tests Required

- 源契约测试断言 `announcement-reference-article` 的宽度约束，并拒绝 `announcement-reference-aside`、“阅读说明”和“查看全部公告”。
- 全量 Vitest、Nuxt typecheck、real-mode production build。
- 真实后端验证后请求 `GET /api/v1/announcements`，断言 `title.startsWith('Smoke 公告')` 的结果数为 0。
- 浏览器在 390px 与 1440px 打开正常公告详情，断言无横向溢出、无空白侧栏、CTA 和返回动作可达。

### 7. Wrong vs Correct

#### Wrong

```ts
const visible = announcements.filter(item => !item.title.startsWith('Smoke 公告'))
```

#### Correct

```ts
const visible = announcements
// Smoke data is restored to draft/offline at the backend lifecycle boundary.
```

## Scenario: 共享缓存首页中的用户级公告状态

日期：2026-08-10

执行者：Codex

### 1. Scope / Trigger

- 触发条件：在首页或其他带共享 SSR/SWR 缓存的公开页面中读取公告、已读、关闭、收藏、资格等用户级状态。
- 目标：公开页面继续使用匿名 SSR 数据，同时确保用户级公告候选和关闭记录只在客户端确认登录后读取，不进入共享响应缓存。

### 2. Signatures

```ts
useMyProfileQuery(enabled: Ref<boolean> | boolean): UseQueryReturnType<UserProfile, Error>
useActiveHomeAnnouncement(enabled: Ref<boolean> | boolean): UseQueryReturnType<Announcement | null, Error>
prefetchQueriesOnServer(...queries: QueryObserverResult[]): Promise<void>
```

```text
GET  /api/v1/announcements/home
POST /api/v1/me/announcements/{announcementId}/dismiss
```

### 3. Contracts

- `/` 的服务端预取集合只能包含匿名用户共享的市场和目录查询；不得加入首页公告、公告回执或其他用户级查询。
- 首页在客户端通过 `useMyProfileQuery(import.meta.client)` 确认登录态，并把 `Boolean(myProfile)` 作为 `useActiveHomeAnnouncement` 的 `enabled` 条件。
- 匿名访问不得请求需要登录的首页公告接口，也不得把预期的未登录状态渲染成公告加载错误。
- 已登录用户的公告查询必须保留加载占位、独立失败重试、详情跳转和关闭反馈；公告失败不得阻塞公开市场内容。
- 公告渲染条件必须同时检查当前登录态和查询数据，避免退出登录后短暂显示旧缓存。退出登录或会话失效继续通过全局 `queryClient.clear()` 清除用户级缓存。
- 关闭成功后失效首页公告查询，由后端或 Mock 候选排序返回下一条未关闭公告；关闭失败必须保留当前公告并显示错误反馈。

### 4. Validation & Error Matrix

| 条件 | 必须结果 |
| --- | --- |
| 首页 SSR 或匿名首屏 | 只预取公开查询，不调用 `/api/v1/announcements/home` |
| 客户端确认已登录 | 启用首页公告查询并展示加载占位 |
| 没有有效候选 | 不渲染空公告容器，不影响市场概览 |
| 公告查询失败 | 显示局部错误与重试，市场内容保持可用 |
| `isDismissible=false` | 不渲染关闭按钮 |
| 关闭成功 | 失效查询并显示下一候选或隐藏公告条 |
| 关闭失败 | 公告保持可见并显示错误提示 |
| 退出登录或会话失效 | 清空查询缓存并立即隐藏用户级公告 |

### 5. Good / Base / Bad Cases

- Good：已登录用户在首页看到当前有效公告，关闭后只影响自己的候选结果，其他首页数据保持稳定。
- Base：匿名用户直接看到公开市场首页，没有公告请求、401 提示或多余占位。
- Bad：把 `homeAnnouncementQuery` 传给 `prefetchQueriesOnServer`，导致共享首页 HTML 或水合缓存携带某个用户的公告与关闭状态。

### 6. Tests Required

- 源契约测试断言首页公告查询由 `Boolean(myProfile)` 启停，并拒绝把它加入服务端预取集合。
- 组件契约测试覆盖加载、失败重试、公告展示、允许关闭判断、关闭 mutation 和有效详情链接语义。
- 浏览器分别验证匿名首页与已登录首页；已登录场景覆盖不可关闭、可关闭、候选切换和全部关闭后的隐藏状态。
- 运行全量 Vitest、Nuxt typecheck、real-mode build，并检查首页无横向溢出。

### 7. Wrong vs Correct

#### Wrong

```ts
const homeAnnouncementQuery = useActiveHomeAnnouncement()
prefetchQueriesOnServer(homeMarketQuery, homeAnnouncementQuery)
```

#### Correct

```ts
const { data: myProfile } = useMyProfileQuery(import.meta.client)
const homeAnnouncementEnabled = computed(() => Boolean(myProfile.value))
const homeAnnouncementQuery = useActiveHomeAnnouncement(homeAnnouncementEnabled)

prefetchQueriesOnServer(homeMarketQuery, productCategoriesQuery)
```
