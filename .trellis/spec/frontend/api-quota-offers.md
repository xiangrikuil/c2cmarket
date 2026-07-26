# Limited API Quota Offer Frontend Contract

Date: 2026-07-19
Updated: 2026-07-25
Author: Codex

## Scenario: Quota Offer Market, Purchase, And Owner Management

### 1. Scope / Trigger

- Trigger: changes to `ApiMarketPage.vue`, `components/api-quota`, quota order detail, API quota facade/adapters, query keys, mock behavior, or responsive quota UI.
- The market default is the fixed `限时额度包` view. The legacy `自由额度` view remains a peer tab and keeps its free-amount purchase flow.

### 2. Signatures

```text
PublicApiQuotaOffer
ApiQuotaOrderSnapshot
ApiQuotaOfferFilters
CreateApiQuotaBatchPayload
CreateApiQuotaOfferPayload
CreateApiQuotaRoundPayload
CreateApiQuotaOrderPayload

getApiQuotaOffers/getApiQuotaOfferById/createApiQuotaOrder
getOwnerApiQuotaBatches/getOwnerApiQuotaOffers/getOwnerApiQuotaRounds
createApiQuotaBatch/createApiQuotaOffer/createApiQuotaRound
importApiQuotaCredentials/getApiQuotaCredentialSummary
```

URL contract:

```text
/api-market                 -> default limited quota tab
/api-market?view=free       -> legacy free-amount tab
/my/api-services?intent=quota -> choose an existing service specifically for quota publishing
/my/account?returnTo=/my/api-services?intent=quota -> complete required account setup without losing the quota workflow
/api-market/new?after=quota -> create the prerequisite service and continue to its quota manager
/my/api-services/{id}       -> owner quota manager under the existing service
/my/api-orders/{id}         -> frozen quota order detail
```

### 3. Contracts

- `ApiMarketPage` owns two peer tabs: `limited` and `free`. Distribution system, `1.00x`, and orderability are filters inside the limited view, not primary navigation.
- The market header and limited-offer empty state expose `发布限时额度包` and route to `/my/api-services?intent=quota`. Do not discard the intent, route directly to the generic API service publish form, or create a second quota publish domain.
- The account-recovery guard preserves the complete `returnTo=/my/api-services?intent=quota` target. `MyCenterPage` names the blocked action and exposes `继续发布限时额度包` after setup instead of a generic profile-page continuation.
- `MyApiServicesPage` is the seller selection step: quota intent changes the heading to `选择 API 服务`; existing rows expose `选择并发布额度包` and deep-link to `/my/api-services/{id}#quota-offers`. When no service exists, the empty state explains the prerequisite and links to `/api-market/new?after=quota`.
- `ApiServicePublishPage` reads `after=quota`. Successful prerequisite creation routes to `/my/api-services/{newServiceId}#quota-offers`; ordinary service publishing still returns to `/my/api-services`.
- `ApiServicePublishPage` exposes peer `自由额度` and `限时额度包` tabs. `after=quota` selects `限时额度包`; ordinary publishing selects `自由额度`. Switching the tab changes the current publish workflow and post-submit destination without changing the backend service payload contract.
- The limited workflow must say that the current page creates the prerequisite base service. Fixed USD allowance, total CNY, offer multiplier, inventory, continuous/scheduled sale mode, absolute expiry, cutoff, and delivery inventory are configured in the next `#quota-offers` step; the preview must not invent those values early.
- The publish stepper reflects the real workflow. Limited publishing uses `选择销售方式 → 配置基础服务 → 设置额度包 → 确认发布`; its primary action is `保存基础服务，下一步设置额度包`. Ordinary publishing uses `发布自由额度服务`.
- The publish stepper uses the shared shadcn-vue `StepperSeparator`, but the page must provide an explicit non-zero horizontal height because the shared separator does not size itself. Completed segments use the primary color and incomplete segments use the border color.
- API quota publish pages use one progressive interaction contract: explicit current/completed step state, exactly one expanded step, real summaries for completed steps, compact pending rows, and revisitable completed steps. Continue validates only the current step; final publication validates the complete form and returns focus to the first error-owning step without rebuilding form state.
- Desktop publish pages render one sticky buyer-preview instance beside the active step flow. Below `1241px`, the desktop preview is not rendered and the same preview content opens through the shared Dialog surface; do not maintain simultaneous desktop/mobile preview trees or a second preview DTO.
- The publish page first viewport uses a compact marketplace hierarchy: page title and compact stepper remain full width; immediately below the stepper the page enters its form/preview grid. The left column begins with two icon-led selling-mode choices and one buyer-flow strip before the form fields, while the buyer preview begins at the same vertical position in the right column. Do not place separate boundary, completeness, or explanatory cards before this grid; completeness stays as one compact line beside the current step, and boundary copy stays in the flow/preview surfaces.
- The selling-mode choices preserve the approved green `自由额度` and orange `限时额度包` contrast. Form sections use small semantic Lucide icons, and the buyer preview uses icon-led comparison rows. Icons reinforce field scanning but do not replace labels.
- The primary publish/continue action remains visible in a concise sticky bottom bar on desktop and mobile. At `1440x900`, `.api-publish-layout` should begin no lower than `460px`; at `390x844`, it should begin no lower than `680px`. The sticky action must remain fully inside the viewport without covering its own validation reason.
- The limited buyer flow is `选择额度包 → 创建订单 → 站外付款 → 卖家确认收款 → 获取交付凭证`, with `平台记录订单，不代收款`. Do not use `自动发货`, `平台担保`, `资金安全`, `安全可靠`, or `获取 API Key`.
- Mobile publish pages keep identity configuration in normal document flow. Only the concise primary action bar may stick to the bottom; a multi-field configuration panel must not cover the viewport.
- A limited card prioritizes fixed USD, total CNY, derived `¥ / $1`, multiplier, absolute expiry/cutoff, remaining time, stock, seller identity, distribution system, TTFT, concurrency, confirmation time, and delivery ETA/mode.
- Fixed-session, other limited-offer, and free-amount service cards share `quota-offer-card` and one visual hierarchy. Limited offers infer the product category with `getProductCategory(\`${item.serviceTitle} ${item.name}\`)`; free services use provider-first `getApiServiceProductCategory(service)`. Icons and low-opacity watermarks come from the matching shared product-icon helpers. Do not add a duplicate backend category field or a page-local brand map.
- Category themes are product identity only: GPT purple, Claude coral, Gemini blue, Cursor cyan, Perplexity green, and Other neutral gray. The theme may affect the fine border, icon well, category badge, total CNY emphasis, very light card surface, and watermark. Availability, sold-out, ended, or cancelled badges keep semantic status variants, and the primary purchase button keeps the normal brand-blue `Button` treatment.
- The free-amount public query contains only orderable services, so its cards must not repeat a static `可创建订单` badge. Non-orderable services remain visible only in owner/admin workspaces with their authoritative reason.
- Free-amount cards show `¥ / $1`, available USD, minimum purchase, multiplier, TTFT, concurrency, payment window, seller, CNY order range, gateway, expiry, and reputation. Their full-width action navigates to the existing service detail so the buyer can choose an amount; it must not call the fixed-offer order mutation. The repeated merchant-declaration disclaimer belongs in the free-market alert, not every card.
- Fixed-session and other limited-offer grids keep three columns at the desktop `xl` breakpoint. The free-amount grid is centered, capped at `1640px`, and uses `repeat(auto-fit, minmax(min(100%, 330px), 1fr))`. This keeps at most four equal flexible columns, distributes spare row width into the cards instead of a blank right edge or oversized gaps, fits three columns at `1440x900`, and uses one fluid mobile track capped at `375px`.
- A normal free-amount card is `342px` high and must satisfy `scrollHeight <= clientHeight`; active reputation uses the compact one-line summary. A caution or restricted reputation card uses `min-height: 342px` and may grow so its warning and public badges are never clipped.
- Limited cards keep four performance metrics in one row at `sm` and above and `2x2` on mobile. Compact free cards keep all four metrics in one row, including at `390px`, while transaction details remain `2x2`. Purchase buttons stay full width and `40px` high.
- Countdown presentation derives from server timestamps through `apiQuotaOfferUi.ts`; expired, cutoff, not-started, sold-out, and credential-shortage states must not be inferred from button text.
- Purchase confirmation is fixed-value. Buyers cannot submit or edit price, USD allowance, multiplier, cutoff, or expiry. Scheduled orders include the current `saleRoundId`; continuous orders omit it.
- New offer forms default `modelMultiplier` to `1.0000` but keep the input editable for every distribution system, including Sub2API.
- Order detail renders the frozen `quotaSnapshot`; it must not recalculate historical price, multiplier, timing, TTFT, or delivery information from the current offer/service.
- `api.ts` is the mock/real facade. Real mode uses `apiMarketBackend.ts` and `backendClient.ts`; a real failure must not fall back to mock success data.
- Query invalidation includes public quota lists/details, owner batches/offers/rounds/credential summaries, and affected API order queries.
- Mock CSV import stores only counts and generated mock delivery state. Raw API keys/passwords must never enter session storage, list records, notifications, or summaries.
- `ApiQuotaOwnerManager` renders one owner workspace with four peer tabs: `额度批次`, `销售规格`, `放量计划`, and `交付凭据`. The batch query selects one batch; offer/round queries remain keyed by that selected batch, and credential summary remains keyed by the selected pre-imported offer. Tab switching must not eagerly fetch child resources for every batch.
- The owner batch table may show declared total and unallocated USD from the batch response. It must not rename unallocated allowance to sold/remaining or synthesize either value. A locked system-slot batch keeps pause/archive actions visible but disabled with the authoritative lock reason so the user can understand why the action is unavailable.
- Mobile pages must keep root `scrollWidth <= clientWidth`. Wide owner tables may scroll inside their own explicit `overflow-x-auto` container.

### 4. Validation & Error Matrix

| Problem code / condition | UI behavior |
| --- | --- |
| `API_QUOTA_NOT_STARTED` | Show not-started state and next-round time |
| `API_QUOTA_ROUND_ENDED` | Disable the ended round and refresh offer data |
| `API_QUOTA_SOLD_OUT` | Show sold-out state; never decrement client stock optimistically below zero |
| `API_QUOTA_BUYER_ROUND_LIMIT` | Explain the same-round cross-offer limit |
| `API_QUOTA_CREDENTIAL_UNAVAILABLE` | Show delivery inventory unavailable |
| `API_QUOTA_BATCH_EXPIRED` | Show cutoff/expired state using frozen server timestamps |
| Invalid CSV template or duplicate | Show sanitized error without echoing raw CSV fields |
| Missing real backend | Render the query error state; do not return mock rows |
| Category cannot be inferred | Use `other` and the neutral gray theme; never guess a status or hide the offer |
| Category identity conflicts with sale status | Keep the category theme on decorative surfaces and render the authoritative semantic status badge independently |
| Account setup incomplete | Redirect with the full quota `returnTo`, explain why setup is required, then resume the same workflow |
| Seller has no API service | Show the prerequisite-service explanation and one `发布 API 服务并继续` action with `after=quota` |
| `/api-market/new?after=quota` | Select limited mode, hide free-amount price/inventory inputs, and show the base-service continuation action |
| `/api-market/new` | Select free mode, show the free-amount price/inventory inputs, and preserve normal list navigation |

### 5. Good / Base / Bad Cases

- Good: the first market render selects `限时额度包`, while switching to `自由额度` updates the URL and preserves the legacy amount input flow.
- Good: a seller selects `发布限时额度包`, chooses an existing API service, and lands at that service's `#quota-offers` management section.
- Good: an incomplete account finishes email/password setup, clicks `继续发布限时额度包`, and returns to the `选择 API 服务` step.
- Good: a Sub2API seller enters `1.2500`; the owner table, public card, purchase dialog, and order snapshot all display `1.25x`.
- Good: a Claude offer uses the coral identity border/icon/price while an active sale still uses the shared orderable status badge and blue purchase button.
- Good: a GPT free-amount service uses the same purple identity surfaces, shows `¥0.80 / $1` and available USD, then opens the service detail for amount selection.
- Base: a continuous `$50 / ¥5` offer confirms ten-minute payment and no round ID.
- Base: an unrecognized provider renders as `其他` with a neutral card and all real quota fields intact.
- Base: a seller with no API service sees why a service is required and continues to the existing service publish form.
- Base: an ordinary API service publish without `after=quota` still returns to the normal service list.
- Bad: the purchase dialog contains editable amount or multiplier fields.
- Bad: the public market only says `发布 API 服务`, leaving sellers to infer where quota offers are created.
- Bad: the account guard redirects to `/my/account` with `returnTo=/my/api-services`, because that loses the quota-specific selection and continuation copy.
- Bad: `paymentWindowMinutes` is displayed as API response time.
- Bad: category colors replace sold-out/cancelled semantics, or the purchase button becomes purple/red/green per category.
- Bad: the free-amount card calls `createApiQuotaOrder` directly or invents a fixed USD package instead of opening the amount-selection detail.
- Bad: CSV raw contents are serialized to session storage for later mock delivery.

### 6. Tests Required

- Vitest: default tab/query, `intent=quota` entry, account-recovery `returnTo` preservation, `after=quota` post-publish navigation, quota-section anchor, countdown boundaries, fixed amount, five/ten-minute windows, Problem Details mapping, cross-offer round limit, cancellation release, CSV non-persistence, auto-delivery, editable non-one Sub2API multiplier, real adapter fields, and realtime invalidation.
- Publish-page regression tests also cover mode defaults, corrected labels, mode-specific primary actions, limited preview fields, buyer-flow copy, and forbidden wording absence.
- Progressive publish tests cover active/completed/pending state, visitable-step rules, first-error step mapping, completed summaries, sticky primary action, and the shared responsive preview boundary.
- Market-card source regressions assert both limited variants and the free-amount variant use `quota-offer-card`, `data-category`, shared product-category inference, the six static CSS selectors, full-width brand buttons, the centered `1640px` free grid with flexible `330px` minimum tracks, `342px` normal card height, compact active reputation, and no unsupported `自动交付` / `安全可靠` / `平台担保` wording.
- Free-amount market cards omit source-author verification entirely. API service detail hides `not_submitted`, `pending`, and `expired`, and displays the badge only for `verified` or `mismatch`; the backend field remains unchanged.
- Type/build: `pnpm --dir frontend typecheck` and `VITE_API_MODE=real pnpm --dir frontend build`.
- Browser: `1920x1080`, `1440x900`, and `390x844` for free-market cards; assert first-row counts of four, three, and one respectively, normal card dimensions of approximately `398x342`, `383x342`, and `347x342`, `scrollHeight <= clientHeight`, four metric columns, `40px` purchase buttons, no page-level horizontal overflow, and no console warnings/errors. At `1920x1080`, the centered grid is `1640px` wide and must not create a fifth column. The broader `1440x900` and `390x844` suite still covers the compact publish first viewport, limited cards, purchase dialog, owner batch/offer/round/CSV sections, and quota order detail.

### 7. Wrong vs Correct

#### Wrong

```ts
const modelMultiplier = distributionSystem === 'sub2api' ? '1.0000' : form.modelMultiplier
const responseTime = service.paymentWindowMinutes
```

#### Correct

```ts
const modelMultiplier = form.modelMultiplier // defaults to 1.0000, remains editable
const responseTime = offer.declaredTtftBand // merchant-declared, unverified
```

```vue
<!-- Wrong: this hides the quota workflow behind generic service publishing. -->
<RouterLink to="/api-market/new">发布 API 服务</RouterLink>

<!-- Correct: select the existing service, then reuse its quota manager. -->
<RouterLink to="/my/api-services?intent=quota">发布限时额度包</RouterLink>
<RouterLink :to="`/my/api-services/${service.id}#quota-offers`">管理额度包</RouterLink>
```

```vue
<!-- Wrong: identity color replaces business state and the primary action. -->
<Card class="border-purple-500">
  <Badge class="bg-purple-500">售罄</Badge>
  <Button class="bg-purple-600">立即抢购</Button>
</Card>

<!-- Correct: category is decorative; status and action keep shared semantics. -->
<Card class="quota-offer-card" :data-category="quotaOfferCategory(item)">
  <Badge variant="outline" class="quota-offer-category">{{ getProductCategoryLabel(quotaOfferCategory(item)) }}</Badge>
  <Badge :variant="offerStatusVariant(item)">{{ item.orderabilityReason }}</Badge>
  <Button class="h-10 w-full">立即抢购</Button>
</Card>
```

Limited offers preserve their own frozen business contract; legacy free-amount service defaults do not override them.

## Scenario: Rush Countdown, Simplified Publication, And Copy Drafts

### 1. Scope / Trigger

- Trigger: changes to `ApiMarketPage.vue`, `ApiQuotaRushPublishPage.vue`, `ApiQuotaOwnerManager.vue`, system-slot queries, direct rush purchase, or copy-again behavior.
- The rush experience is the first section of the existing limited-quota market. Do not add a separate primary market or client-side inventory state.

### 2. Signatures

```text
ApiQuotaSystemSaleSlot
ApiQuotaSystemSaleSlotList
CreateApiQuotaRushOfferPayload
ApiQuotaRushOfferPublication

getApiQuotaSaleSlots()
getApiQuotaOffers({ slotKey })
createApiQuotaRushOffer(payload)
createApiQuotaOrder({ offerId, saleRoundId })

/api-market/quota/new
```

### 3. Contracts

- Render the `09:00`, `13:00`, and `20:00` sessions from the server response. Do not build Beijing dates or infer the seven-day range in the browser.
- Select the active session first, otherwise the next non-ended session. After all sessions for the first returned date end, preview the next date and derive its heading from the selected slot.
- Compute a display-only clock offset from `serverNow`. When the selected slot reaches start or end, refetch both slots and slot-filtered offers before changing purchase authority.
- Only `isOrderable=true` enables `立即抢购 ¥xx`. One click creates the existing quota order with the current round ID and navigates to payment; pending state keeps the button dimensions stable and prevents duplicate submission.
- The three-step seller wizard selects or creates an eligible API service, configures one offer, then selects a `registration_open` slot and an absolute expiry. It uses the existing API service field components and the atomic rush publication endpoint.
- The three-step seller wizard follows the shared progressive contract without merging its business state into the free-amount form: step one owns service selection/creation, step two owns the package and CSV draft, and step three owns slot/expiry plus final atomic publication. Back/edit navigation changes only the current step and must retain `rush`, slot, expiry, and selected-file state.
- When the first open slot arrives asynchronously, default expiry must be initialized from that selected slot immediately. Expiry shortcuts still produce an absolute Beijing input and must satisfy slot end plus one hour.
- Manual delivery is the default and hides CSV controls. Pre-imported delivery expands one template/file input and requires the parsed row count to cover `copies`.
- `复制再发` may copy only service ID, name, USD allowance, CNY price, multiplier, delivery mode, and delivery ETA. It must not copy copies, slot key, expiry, or CSV.
- The advanced owner manager remains available. For a locked system-slot batch, keep owner pause/archive actions disabled with the lock reason and let the backend remain authoritative if state changes after render.

### 4. Validation & Error Matrix

| Condition | UI behavior |
| --- | --- |
| Slot query is loading or fails | Show stable loading/error state; do not synthesize slots |
| No `registration_open` slot | Disable publication and explain that no session accepts registration |
| Expiry is earlier than slot end plus one hour | Disable publication and show the absolute minimum |
| Pre-imported CSV rows are fewer than copies | Disable publication; keep user input for correction |
| Countdown reaches a boundary | Refetch authoritative slot and offer data once for that boundary |
| Purchase is pending | Disable the fixed-size offer button and ignore repeat clicks |
| Purchase returns a quota Problem Details code | Show the mapped message and refresh session data |
| Copied query contains unsupported timing/inventory fields | Ignore them and keep fresh defaults |

### 5. Good / Base / Bad Cases

- Good: the market opens on an active `13:00` session, shows a tabular `HH:MM:SS` countdown, and directly creates one order after a single click.
- Good: copying a scheduled `$100` offer preserves pricing and delivery settings, while copies remain `10` and slot/expiry are freshly selected.
- Base: a seller with no eligible service creates the prerequisite service inside step one and resumes the same wizard.
- Bad: the browser enables purchase merely because its local countdown reached zero.
- Bad: a copy URL carries old `copies`, `slotKey`, `expiresAt`, or raw CSV data.

### 6. Tests Required

- Vitest: default/active/tomorrow session selection, dynamic heading, server clock offset, boundary refetch, one-click purchase, pending deduplication, Problem Details mapping, wizard prerequisites, conditional CSV, immediate default expiry, and copy-field allowlist.
- Type/build: `pnpm --dir frontend exec vue-tsc -b --pretty false` and `VITE_API_MODE=real pnpm --dir frontend exec vite build`.
- Browser: `1440x900` and `390x844`; assert three tabs, fixed countdown/button dimensions, enabled valid publication, conditional CSV, no application console errors, and no page-level horizontal overflow.

### 7. Wrong vs Correct

#### Wrong

```ts
if (countdown.value === '00:00:00') offer.isOrderable = true
const copied = { ...oldOffer, copies: oldCopies, slotKey: oldSlot, expiresAt: oldExpiry }
```

#### Correct

```ts
if (boundaryReached.value) {
  await Promise.all([slotQuery.refetch(), rushQuery.refetch()])
}

const copied = {
  name: offer.name,
  usdAllowance: offer.usdAllowance,
  priceCny: offer.priceCny,
  modelMultiplier: offer.modelMultiplier,
  deliveryMode: offer.deliveryMode,
  deliveryEtaMinutes: offer.deliveryEtaMinutes,
}
```

The server owns orderability and timing; copy drafts carry stable commercial settings only.
