# Nuxt SSR and Cloudflare Worker Contract

Date: 2026-07-18
Author: Codex

## Scenario: Hybrid rendering on Cloudflare Workers

### 1. Scope / Trigger

Apply this contract when changing Nuxt routing, SSR query prefetch, SEO metadata,
sitemap generation, runtime API environment variables, or Cloudflare Worker
build/deployment configuration.

### 2. Signatures

- Build: `pnpm --dir frontend build`
- Type check: `pnpm --dir frontend typecheck`
- Worker validation: `pnpm --dir frontend exec wrangler deploy --dry-run --config ../wrangler.jsonc`
- Nitro preset: `cloudflare_module`
- Worker entry: `frontend/.output/server/index.mjs`
- Worker assets: `frontend/.output/public`

### 3. Contracts

- Public SSR runtime keys: `NUXT_PUBLIC_SITE_URL`,
  `NUXT_PUBLIC_API_BASE_URL`, and `NUXT_PUBLIC_API_MODE=real`.
- Server-only API origin: `NUXT_API_BASE_URL`.
- Production builds require complete public/server API configuration and must reject any mode other than explicit `real`.
- Public market pages may prefetch anonymous queries only. Session, favorite,
  eligibility, notification, owner, merchant, and admin queries remain client-only.
- Public market detail absence returns HTTP 404 and `noindex`; non-404 upstream
  errors remain 5xx instead of rendering an empty successful page.
- Dynamic sitemap collection follows opaque `nextCursor` values without parsing
  them, emits `lastmod` when a valid public timestamp exists, excludes CSR/noindex
  routes, and fails visibly when an upstream list request fails.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Public entity exists | SSR HTML, canonical metadata, HTTP 200 |
| Public entity is absent / API 404 | Existing empty state, HTTP 404, `noindex` |
| Public API network error or 5xx | Worker response remains 5xx |
| Private/search/user route | CSR shell plus `X-Robots-Tag: noindex, nofollow` |
| Production hostname | Public robots rules and canonical sitemap URL |
| Staging, preview, or `workers.dev` | `robots.txt` returns `Disallow: /` |
| Sitemap cursor repeats or exceeds 100 pages | Sitemap source fails visibly |
| Any required Nuxt API variable is missing in production | Build fails |

### 5. Good / Base / Bad Cases

- Good: a market detail is present in initial HTML, carries entity metadata, and
  its URL appears in sitemap with `lastmod`.
- Base: production currently has no public entities, so sitemap contains only the
  five canonical discovery routes.
- Bad: an API outage is caught and converted to an empty HTTP 200 page or an empty
  sitemap, causing crawlers to treat a failure as valid content.

### 6. Tests Required

- Vitest: route indexability, route-rule partition, hydration wiring, sitemap
  opaque-cursor pagination, repeated-cursor rejection, lastmod normalization, and
  both Wrangler config entry/assets/runtime vars.
- HTTP Worker smoke: public SSR body, private/user noindex header, missing public
  detail 404, sitemap XML, production robots, and staging robots.
- Build gates: Nuxt typecheck, real-API Nuxt build, production and staging Wrangler
  dry-runs, OpenAPI route guard, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```ts
export default defineNuxtConfig({
  nitro: { preset: 'cloudflare' },
})
```

With Nitro 2.13 this resolves to the legacy `cloudflare-worker` output. Wrangler
Static Assets can pass dry-run yet local workerd then fails on `node:buffer`.

#### Correct

```ts
export default defineNuxtConfig({
  nitro: { preset: 'cloudflare_module' },
})
```

This produces an ES module Worker entry compatible with Wrangler's `main` plus
`assets.directory` deployment model and the `nodejs_compat` flag.

## Scenario: Query-safe Nuxt development first page

### 1. Scope / Trigger

Apply this contract when changing public route rules, root layouts, homepage
queries, development Vite configuration, or dependencies imported by every
SSR request.

### 2. Signatures

- Development route rules: public cached routes resolve to `{ cache: false }`.
- Production route rules: `/`, `/official-prices`, and
  `/official-prices/**` use 300-second SWR; `/carpools`, `/carpools/**`,
  `/api-market`, and `/api-market/**` use 120-second SWR.
- Development warmup hook:
  `'vite:serverCreated'(server, { isServer })` transforms each configured page
  `?macro=true` module with `{ ssr: true }` before server readiness.
- Homepage query entry: `useHomeMarket()` from
  `src/queries/useHomeMarketQuery.ts`.
- Shell query entry: profile, notification, carpool, and API-service owner reads
  come from `src/queries/useAppShellQueries.ts`.

### 3. Contracts

- Do not enable Nitro response cache/SWR for public routes while
  `NODE_ENV=development`. Query-bearing root URLs otherwise collide with the
  filesystem payload cache path and can fail with `EISDIR`.
- Production keeps the existing public SSR cache durations and SWR behavior.
- Default real-mode homepage, `AppShell`, and `AdminShell` entry points must not
  statically import `src/lib/api.ts` or `src/queries/useMarketQueries.ts`.
- Explicit Mock mode may dynamically import the compatibility facade so its
  session-owned in-memory data remains canonical.
- First-page icon imports use `lucide-vue-next/dist/esm/icons/*.js`; importing
  the package root from a static SSR entry transforms the package-wide export
  graph on the first development request.
- Development may keep a short, real-mode-only homepage data cache to replace
  burst protection lost with response caching disabled. It must not alter Mock
  ownership or production request behavior.
- Route-metadata pre-transformation runs only for the server Vite instance in
  development. It must not run in production builds.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Fresh real-mode development `GET /` | HTTP 200 and TTFB no more than 5 seconds on the baseline machine |
| Next four `GET /` requests | Each completes in less than 100 milliseconds at the HTTP boundary |
| `GET /?probe=1` or another query-bearing root URL | HTTP 200; no payload-cache `EISDIR` error |
| Default real-mode first-page source graph | No static runtime import of the monolithic facade/query entry |
| Explicit Mock mode | Dynamically loads facade-owned data without changing its store ownership |
| Production build | Generated Nitro route rules retain 300/120-second SWR |

### 5. Good / Base / Bad Cases

- Good: page metadata is transformed during development startup, the cold
  homepage returns within five seconds, and repeated/query requests remain
  below the warm threshold with HTTP 200.
- Base: production still generates the original cached public SSR rules and
  private routes remain CSR/noindex.
- Bad: adding a static `@/lib/api`, `useMarketQueries`, or package-root lucide
  import to the homepage or global shell restores multi-second first-request
  transforms.

### 6. Tests Required

- Source regression: focused homepage/shell imports, dynamic Mock boundary,
  per-icon first-page imports, development cache partition, production route
  durations, dependency pre-bundle list, and route-meta warmup.
- Clean-copy HTTP benchmark: exclude `.nuxt` and `.output`, start an unused
  local port, record one cold and four warm `/` requests, then smoke at least
  two query-bearing root URLs.
- Full frontend Vitest, Nuxt typecheck, configured real-mode production build,
  generated Nitro route-rule inspection, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```ts
export default defineNuxtConfig({
  routeRules: {
    '/': { cache: { maxAge: 300, swr: true } },
  },
})
```

This applies filesystem-backed response/payload caching to development query
URLs and can make `/?...` resolve the payload cache directory as a file.

#### Correct

```ts
const publicRouteRules = process.env.NODE_ENV === 'development'
  ? { '/': { cache: false } }
  : { '/': { cache: { maxAge: 300, swr: true } } }

export default defineNuxtConfig({ routeRules: publicRouteRules })
```

Development remains query-safe while production retains SSR SWR.

## Scenario: Hydration-stable primitives and dependent public queries

### 1. Scope / Trigger

Apply this contract when changing `App.vue`, Reka-backed shadcn-vue primitives,
or an SSR page where one public query determines the key or enabled state of a
second query.

### 2. Signatures

```ts
<ConfigProvider :use-id="useRekaId">...</ConfigProvider>

useApiQuotaOffers(
  filters: Ref<ApiQuotaOfferFilters> | ApiQuotaOfferFilters,
  enabled: Ref<boolean> | boolean,
)

onServerPrefetch(async () => {
  await parentQuery.suspense()
  selectFrom(parentQuery.data.value)
  if (selection.value) await childQuery.suspense()
})
```

### 3. Contracts

- `App.vue` wraps loading UI, standalone pages, application shells, and the
  global toaster in one Reka `ConfigProvider`.
- The deterministic ID counter lives inside `<script setup>`, which makes it
  app-instance scoped on both SSR and the client. A module-global counter is
  forbidden because request order would affect rendered IDs.
- A dependent child query stays disabled until its authoritative parent
  selection exists. During SSR, await the parent, assign the same default
  selection used by the client, then await the child only when the same full
  enable guard is true, including any active-view condition.
- Server HTML and the client's first render must use the same selected key,
  loading branch, and data branch. Do not rely on a watcher scheduled after
  server prefetch to establish hydration-critical state.
- Non-404 public query failures remain visible SSR failures; hydration fixes
  must not replace an upstream error with an empty successful page.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Fresh direct load with Reka Tabs/Popover | No ID, node, text, class, or children hydration mismatch |
| Parent data has an active/default item | Server and client select the same item before rendering child data |
| Parent data has no selectable item | Child query remains disabled and the documented empty state renders |
| Parent or child public query returns non-404 failure | SSR request remains an error response |
| Client-side navigation | Existing cached-query and interaction behavior remains unchanged |

### 5. Good / Base / Bad Cases

- Good: API market SSR awaits sale slots, selects the active slot, then awaits
  offers for that exact slot; the client hydrates the same card grid.
- Base: no slot is available, so no empty-key offer query runs and the page
  renders its explicit empty state.
- Bad: the server renders a skeleton with no selected slot while the hydrated
  client immediately renders a dated heading and offer cards.

### 6. Tests Required

- Source regression for the root `ConfigProvider`, app-scoped ID source, child
  query enable guard, and sequential `onServerPrefetch` order.
- Full Vitest and Nuxt typecheck.
- Real-API production build.
- Fresh direct browser loads for `/`, `/api-market`, `/carpools`,
  `/official-prices`, and `/login`; assert zero console warning/error entries,
  the default theme on first paint, and no horizontal overflow.

### 7. Wrong vs Correct

#### Wrong

```ts
const childQuery = useApiQuotaOffers(computed(() => ({ slotKey: selected.value })))
prefetchQueriesOnServer(parentQuery, childQuery)
```

Both queries start together, so the child can run with an empty key and the
server can render a different branch from the hydrated client.

#### Correct

```ts
const childQuery = useApiQuotaOffers(
  computed(() => ({ slotKey: selected.value })),
  computed(() => Boolean(selected.value)),
)

onServerPrefetch(async () => {
  await parentQuery.suspense()
  selectFrom(parentQuery.data.value)
  if (selected.value) await childQuery.suspense()
})
```
