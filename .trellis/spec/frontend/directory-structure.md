# Directory Structure

> How frontend code is organized in this project.

---

## Overview

C2CMarket frontend is a Nuxt 4 + Vue 3 + TypeScript application with hybrid SSR/CSR rendering. Route pages live under `src/pages`, reusable business UI lives under `src/components`, official shadcn-vue primitives live under `src/components/ui`, and data access is intentionally funneled through `src/lib` plus TanStack Query hooks in `src/queries`.

Do not let pages import raw seed arrays for primary business reads. Pages should call query hooks, and query hooks should call facade/API functions.

---

## Directory Layout

```
frontend/
├── src/
│   ├── App.vue
│   ├── router.ts
│   ├── pages/                 # route-level views
│   ├── components/
│   │   ├── ui/                # generated shadcn-vue primitives
│   │   ├── layout/            # app shell and navigation
│   │   ├── market/            # shared market display components
│   │   ├── announcements/     # announcement-specific UI
│   │   ├── api-service-detail/
│   │   ├── api-service-publish/
│   │   ├── carpool-publish/
│   │   ├── official-price-submit/
│   │   └── profile/
│   ├── queries/               # TanStack Query hooks and mutation wrappers
│   ├── lib/                   # API facades, backend adapters, utilities
│   ├── data/                  # seed mock data only
│   ├── types/                 # shared domain types
│   ├── stores/                # Pinia app/session state only
│   ├── composables/           # reusable Vue composables
│   ├── middleware/            # Nuxt route middleware
│   ├── plugins/               # Nuxt runtime integrations
│   ├── seo/                   # route metadata and indexability policy
│   └── theme/                 # theme-level styling helpers
├── server/                    # Nitro server routes, sitemap, and robots handlers
├── components.json            # shadcn-vue config
├── nuxt.config.ts
├── package.json
└── vitest.config.ts
```

---

## Module Organization

- Route files in `src/pages` should compose workflow UI and call hooks from `src/queries`; they should not contain reusable domain transformations when a `src/lib` facade or feature utility can own them.
- Business components belong in a domain folder under `src/components/<feature>/` when reused by one workflow family, or under `src/components/market/` when shared across market/list/detail pages.
- Keep generated shadcn-vue primitives under `src/components/ui/<primitive>/`. Product-specific components must compose those primitives instead of replacing them.
- Real backend integration belongs in focused adapters such as `src/lib/backendClient.ts`, `src/lib/apiMarketBackend.ts`, `src/lib/carpoolBackend.ts`, `src/lib/profileBackend.ts`, and `src/lib/announcementsApi.ts`.
- `src/lib/api.ts` remains the compatibility facade for mixed mock/real market flows. When a domain has a real backend adapter, route calls through the facade based on `shouldUseRealBackend()` rather than branching inside pages.
- Seed examples belong in `src/data/*.mock.ts` or `src/data/mock.ts`. Mutations must update facade-owned state or real backend APIs, not seed arrays.
- Shared product calculations belong in `src/lib/pricing.ts`, `src/lib/productCategories.ts`, or a domain utility file next to related components.

---

## Naming Conventions

- Vue route and business component files use PascalCase, for example `ApiServiceDetailPage.vue`, `AnnouncementListItem.vue`, and `CarpoolPublishPreview.vue`.
- Feature folders use kebab-case when they contain several related components, for example `api-service-publish` and `official-price-submit`.
- Query hook files use `use*Queries.ts`; individual hooks start with `use`, for example `useOfficialPrices` and `useAnnouncementList`.
- Backend adapter functions use `backend*` prefixes when they are not the public facade, for example `backendGetCarpools` and `backendUpsertMerchantProfile`.
- Domain types should be exported from `src/types/<domain>.ts` when shared across multiple modules; feature-local form types can live next to the feature under `components/<feature>/types.ts`.

---

## Examples

- `frontend/src/pages/ApiMarketPage.vue`: route page using query/facade data rather than raw seed arrays.
- `frontend/src/queries/useMarketQueries.ts`: central query hook file for market, profile, notification, review, and admin flows.
- `frontend/src/lib/backendClient.ts`: shared real-backend client for base URL, cookies, CSRF, idempotency keys, `If-Match`, and Problem Details.
- `frontend/src/lib/apiMarketBackend.ts`: real backend adapter that maps API service and purchase-intent DTOs into frontend domain records.
- `frontend/src/components/api-service-detail/*`: feature components for a complex detail page split by responsibility.
- `frontend/src/components/market/*`: reusable table, filter, chart, badge, and market-summary presentation components.
