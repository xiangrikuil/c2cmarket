# Hook Guidelines

> How hooks are used in this project.

---

## Overview

Frontend server-like reads and mutations use TanStack Query from `@tanstack/vue-query`. Query hooks live in `frontend/src/queries`, while small reusable UI state composables live in `frontend/src/composables`.

The main hook file today is `frontend/src/queries/useMarketQueries.ts`; announcements use `frontend/src/queries/useAnnouncementQueries.ts`. Hooks should wrap facade functions from `frontend/src/lib` instead of duplicating request or mock-store logic in pages.

---

## Custom Hook Patterns

- Hooks exported from `src/queries` should be named `use<Domain>` or `use<Domain><Action>Mutation`.
- Query keys must be stable arrays. For reactive params, use `computed(() => [...])` and unwrap refs through the existing `valueOf` helper pattern.
- Mutation hooks should update the canonical API/facade first, then invalidate all query families that can show the changed record.
- Keep route navigation and toasts in the page when they are workflow-specific. Put only reusable invalidation and API mutation logic in the hook.
- Small UI-only composables, such as pagination, belong in `src/composables` and must not import business seed data.

Example:

```ts
export function useCarpoolApplication(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => ['carpool-application', valueOf(id)]),
    queryFn: () => getCarpoolApplicationById(valueOf(id)),
  })
}
```

---

## Data Fetching

- Use `useQuery` for route-level reads and `useMutation` for writes.
- API wrappers in `src/lib/api.ts` decide whether to use mock/session state or real backend adapters via `shouldUseRealBackend()`.
- Real backend adapters must use `backendClient.ts` so cookies, CSRF token, idempotency keys, `If-Match`, and Problem Details handling stay consistent.
- Real auth/session calls live in `src/lib/backendClient.ts`. `getCurrentBackendSession()`, `startOAuthLogin()`, and `logoutBackendSession()` are the only frontend helpers that should call `/api/v1/auth/session`, `/api/v1/auth/oauth/start`, and `/api/v1/auth/logout` directly.
- In real backend mode, `ensureBackendSession()` must not call `/api/v1/auth/dev-session`. Missing or mismatched sessions must surface as a visible login/permission error so admin and owner/buyer views cannot silently switch identity.
- Real notification center calls live in `src/lib/notificationBackend.ts`; `getNotifications()`, `markNotificationRead()`, and `markAllNotificationsRead()` must switch through the `api.ts` facade and must not catch real backend failures to return locally derived rows.
- Real global search calls live in `src/lib/searchBackend.ts`; `searchMarket()` must switch through the `api.ts` facade and must not catch real backend failures to return mock search rows.
- Do not call `fetch`, `sessionStorage`, or raw mock arrays directly from pages when an existing facade/hook can be extended.
- Use `refetchOnMount: 'always'` for workflow pages where another route likely changed the same state, such as my carpool applications or API intents.
- Use `placeholderData: previousData => previousData` only when keeping the previous result is an intentional UX behavior for filter changes.

---

## Naming Conventions

- Query hooks: `useOfficialPrices`, `useCarpools`, `usePublicUserProfileQuery`.
- Mutation hooks: `useCreateCarpoolApplicationMutation`, `useUpdateMyProfileMutation`, `useAnnouncementMutation`.
- Query key helper functions: `<domain>QueryKey`, for example `transactionTrendQueryKey` and `publicUserProfileQueryKey`.
- Non-query composables: `usePagination` and similar `use*` names under `src/composables`.
- Facade functions called by hooks do not use the `use` prefix; keep names action-based such as `getCarpools`, `submitOfficialPriceLead`, `publishApiService`.

---

## Common Mistakes

- Invalidating only the current detail query after a mutation even though lists, notifications, admin rows, and profile surfaces also show the changed record.
- Building backend request headers in individual hooks instead of using `backendClient.ts`.
- Returning mock data after a real backend request fails. Real mode failures must be visible to the caller/user.
- Calling `/auth/dev-session` from a real backend route/page to get a buyer, owner, or admin identity. Dev sessions are reserved for development/test smoke scripts and mock/local mode.
- Using plain values in query keys for refs without `computed`, causing stale reads after route param or filter changes.
- Putting page-only tab/form state into TanStack Query or Pinia when normal Vue local state is sufficient.
