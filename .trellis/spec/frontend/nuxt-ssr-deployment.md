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
- Production builds must reject explicit mock mode.
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
| `VITE_ENABLE_MOCK=true` in production | Build fails |

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
