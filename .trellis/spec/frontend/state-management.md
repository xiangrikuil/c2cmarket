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

---

## When to Use Global State

Use global/session mock state only when multiple pages must observe the same record after a mutation. Examples:

- API purchase intents shown in API detail, buyer list, merchant list, notifications, reviews, and admin views.
- Carpool applications shown in carpool detail, my rides, owner applications, notifications, and admin views.
- Published carpool/API service/price lead/demand records that must appear in both user and admin pages.
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
- Real-mode build: `VITE_API_MODE=real pnpm --dir frontend exec vite build`.
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
- Real-mode build: `VITE_API_MODE=real pnpm --dir frontend exec vite build`.
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
