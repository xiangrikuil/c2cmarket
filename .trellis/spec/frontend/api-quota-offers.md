# Limited API Quota Offer Frontend Contract

Date: 2026-07-19
Updated: 2026-08-20
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
/api-market/new           -> choose a publish mode
/api-market/new?mode=free -> publish free-amount credit
/api-market/new?mode=package -> publish fixed credit packages
/api-market/new?mode=limited -> create the prerequisite service for limited quota
/api-market/new?after=quota -> create the prerequisite service and continue to its quota manager
/api-market/quota/new?serviceId={id} -> publish a fixed-session quota offer for one exact service
/my/api-services/{id}       -> owner quota manager under the existing service
/my/api-orders/{id}         -> frozen quota order detail
```

### 3. Contracts

- `ApiMarketPage` owns two peer tabs: `limited` and `free`. Distribution system, `1.00x`, and orderability are filters inside the limited view, not primary navigation.
- The market header and limited-offer empty state expose `发布限时额度包` and route to `/my/api-services?intent=quota`. Do not discard the intent, route directly to the generic API service publish form, or create a second quota publish domain.
- Generic and rush publication pages query seller orders with `dispute=active` before the final write. They always state upfront that unresolved API-order disputes prevent publishing/restoring services and quota and prevent new orders; loading/error/active states disable the final publish action, while the backend remains authoritative.
- The account-recovery guard preserves the complete `returnTo=/my/api-services?intent=quota` target. `MyCenterPage` names the blocked action and exposes `继续发布限时额度包` after setup instead of a generic profile-page continuation.
- `MyApiServicesPage` is the seller selection step: quota intent changes the heading to `选择 API 服务`; existing rows expose `选择并发布额度包` and deep-link to `/my/api-services/{id}#quota-offers`. When no service exists, the empty state explains the prerequisite and links to `/api-market/new?after=quota`.
- `ApiServicePublishPage` reads `after=quota`. Successful prerequisite creation routes to `/my/api-services/{newServiceId}#quota-offers`; ordinary service publishing still returns to `/my/api-services`.
- Generic `ApiServicePublishPage` entry renders only three peer choices: `自由额度`, `固定额度包`, and `限时额度包`. It must not default a mode or render the editor/preview before the seller chooses.
- The publish-mode URL is authoritative: `mode=free`, `mode=package`, and `mode=limited` restore their matching editor. Invalid modes return to the chooser. Legacy `after=quota` maps to `limited`; contextual `发布限时额度包` actions still enter the existing limited workflow directly.
- Free mode maps to `metered_credit`; package mode maps to `fixed_package`; limited mode continues through the prerequisite base-service flow. The fixed-package editor is a peer mode surface and must not be nested under another billing-mode radio group.
- The limited workflow must say that the current page creates the prerequisite base service. Fixed USD allowance, total CNY, inventory, continuous/scheduled sale mode, absolute expiry, cutoff, and delivery inventory are configured in the next `#quota-offers` step. The offer multiplier inherits the base service default and is not configured again; the preview must not invent the remaining values early.
- The publish stepper reflects the real workflow and contains exactly three steps. Limited publishing treats its selected mode as complete, starts at `配置基础服务`, and uses `保存基础服务，下一步设置额度包`; the third step is completed on the dedicated quota page. Free and package publishing use three editable configuration steps and publish directly from `交易与服务` through `发布自由额度服务` or `发布固定额度包`.
- The publish stepper uses the shared shadcn-vue `StepperSeparator`, but the page must provide an explicit non-zero horizontal height because the shared separator does not size itself. Completed segments use the primary color and incomplete segments use the border color.
- API quota publish pages use one progressive interaction contract: explicit current/completed step state, exactly one expanded step, real summaries for completed steps, compact pending rows, and revisitable completed steps. Continue validates only the current step; final publication validates the complete form and returns focus to the first error-owning step without rebuilding form state.
- New API services default to `public_profile`. The identity section states that the community identity is public by default and exposes one explicit checkbox to opt into `store_alias`; the rush publisher uses the same default when it creates a prerequisite service.
- The generic publisher has no separate confirmation step or repeated seller checklist. Completed configuration steps remain revisitable, the adjacent buyer preview is the only review surface, and the sticky action bar shows completeness plus the final publish action on step three. Below the desktop-preview breakpoint, that final action bar also exposes the shared preview dialog without creating another preview tree. Final publication validates the complete form and returns focus to the first error-owning step.
- The shared free-quota marketplace card shows one or two model tags directly. For three or more models it shows the first two tags plus `+N`, while the subtitle states the distribution system and total supported-model count. The full model list remains on the service detail page. Publish-preview purchase controls remain visually primary but are non-interactive and visibly labeled `预览状态，不可操作`.
- `ApiQuotaRushPublishPage` begins with `选择要发布额度的 API 服务` and a compact `我的 API 服务` list. `新建 API 服务` is a secondary action and must not be a peer tab beside service selection.
- Contact selectors show the account-bound linux.do method at most once and may show manual WeChat/email channels separately. Buyer order, purchase-intent, carpool publish, and carpool-application adapters reuse the bound linux.do method returned by `/me/contact-methods`; they never POST a synthetic `@buyer` or `@owner` linux.do contact.
- Existing-service rows show title, orderability, model summary, and a stable short service ID. The list has a bounded height with internal scrolling, and the selected row is repeated in a compact `当前服务` summary.
- A `serviceId` query selects only that exact eligible service. If it is missing or no longer orderable, show the explicit unavailable state and never select a different service.
- New-service mode is a subordinate state with `返回选择服务`. After creation, return to the workflow with the new service selected. Copy and quota drafts survive every switch between the list, new-service mode, and payment dialog.
- New-service mode reads the account-level payment query. If it is complete, render only its active method summary; if it is incomplete, open the shared payment dialog in place. Dismissal preserves the publish draft, and the next continue attempt opens the dialog again.
- Desktop publish pages render one sticky buyer-preview instance beside the active step flow. Below `1241px`, the desktop preview is not rendered and the same preview content opens through the shared Dialog surface; do not maintain simultaneous desktop/mobile preview trees or a second preview DTO.
- The generic publish first viewport is a dedicated, centered chooser capped at `max-w-5xl`: three restrained cards on desktop and one column on mobile. It contains no stepper, form, preview, or sticky publication action. After selection, the page enters the existing compact stepper and form/preview grid.
- Mode choices preserve the approved green `自由额度`, blue `固定额度包`, and orange `限时额度包` contrast. Form sections use small semantic Lucide icons, and the buyer preview uses icon-led comparison rows. Icons reinforce field scanning but do not replace labels.
- Returning through `更换销售模式` removes `mode` and legacy `after` while preserving unrelated query keys. When the current form is dirty, the page confirms before returning because a same-route query change is not covered by the route-leave guard.
- The primary publish/continue action remains visible in a concise sticky bottom bar on desktop and mobile. At `1440x900`, `.api-publish-layout` should begin no lower than `460px`; at `390x844`, it should begin no lower than `680px`. The sticky action must remain fully inside the viewport without covering its own validation reason.
- The limited buyer flow is `选择额度包 → 创建订单 → 站外付款 → 卖家确认收款 → 获取交付凭证`, with `平台记录订单，不代收款`. Do not use `自动发货`, `平台担保`, `资金安全`, `安全可靠`, or `获取 API Key`.
- Mobile publish pages keep identity configuration in normal document flow. Only the concise primary action bar may stick to the bottom; a multi-field configuration panel must not cover the viewport.
- A limited card prioritizes fixed USD, total CNY, 5h/daily multiplier-adjusted USD limits, absolute expiry, platform health for one probe model, multiplier, stock, seller identity, distribution system, merchant-declared concurrency, and delivery ETA/mode.
- Fixed-session, other limited-offer, and free-amount service cards share `quota-offer-card` and one visual hierarchy. Limited offers infer the product category with `getProductCategory(\`${item.serviceTitle} ${item.name}\`)`; free services use provider-first `getApiServiceProductCategory(service)`. Icons and low-opacity watermarks come from the matching shared product-icon helpers. Do not add a duplicate backend category field or a page-local brand map.
- Category themes are product identity only: GPT purple, Claude coral, Gemini blue, Cursor cyan, Perplexity green, and Other neutral gray. The theme may affect the fine border, icon well, category badge, total CNY emphasis, very light card surface, and watermark. Availability, sold-out, ended, or cancelled badges keep semantic status variants, and the primary purchase button keeps the normal brand-blue `Button` treatment.
- The free-amount public query contains only orderable services, so its cards must not repeat a static `可创建订单` badge. Non-orderable services remain visible only in owner/admin workspaces with their authoritative reason.
- Free-amount cards show `¥ / $1`, available USD, minimum purchase, 5h/daily quota policy, expiry, multiplier, platform health, merchant-declared concurrency, payment window, seller, CNY order range, gateway, and reputation. Their full-width action navigates to the existing service detail so the buyer can choose an amount; it must not call the fixed-offer order mutation. Seller-declared TTFT and the old merchant-performance disclaimer are forbidden on current public cards and service details.
- Fixed-session and other limited-offer grids keep three columns at the desktop `xl` breakpoint. The free-amount grid is centered, capped at `1640px`, and uses `repeat(auto-fit, minmax(min(100%, 330px), 1fr))`. This keeps at most four equal flexible columns, distributes spare row width into the cards instead of a blank right edge or oversized gaps, fits three columns at `1440x900`, and uses one fluid mobile track capped at `375px`.
- A normal free-amount card is `360px` high and must satisfy `scrollHeight <= clientHeight`; active reputation uses the compact one-line summary. A caution or restricted reputation card uses `min-height: 360px` and may grow so its warning and public badges are never clipped.
- Limited cards keep four performance metrics in one row at `sm` and above and `2x2` on mobile. Compact free cards keep all four metrics in one row, including at `390px`, while transaction details remain `2x2`. Purchase buttons stay full width and `40px` high.
- Countdown presentation derives from server timestamps through `apiQuotaOfferUi.ts`; expired, cutoff, not-started, sold-out, and credential-shortage states must not be inferred from button text.
- Purchase confirmation is fixed-value. Buyers cannot submit or edit price, USD allowance, multiplier, cutoff, or expiry. Scheduled orders include the current `saleRoundId`; continuous orders omit it.
- New offer forms do not expose a `modelMultiplier` input. Models, fixed packages, and limited quota offers inherit the selected API service's positive default multiplier. The frontend still submits `modelMultiplier` because the backend persists it as an immutable offer/order snapshot.
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
| `/api-market/new` or invalid `mode` | Show only the three-mode chooser with no default selection |
| `/api-market/new?mode=free` | Show free-amount price/inventory inputs and preserve normal list navigation |
| `/api-market/new?mode=package` | Show one initialized fixed package and no nested billing-mode selector |
| `/api-market/new?mode=limited` | Show the base-service continuation workflow |

### 5. Good / Base / Bad Cases

- Good: the first market render selects `限时额度包`, while switching to `自由额度` updates the URL and preserves the legacy amount input flow.
- Good: a seller selects `发布限时额度包`, chooses an existing API service, and lands at that service's `#quota-offers` management section.
- Good: an incomplete account finishes email/password setup, clicks `继续发布限时额度包`, and returns to the `选择 API 服务` step.
- Good: a seller configures one API service multiplier; its model rows and newly created fixed or limited packages use that same value, while the owner table, public card, purchase dialog, and order snapshot display the frozen snapshot.
- Good: a Claude offer uses the coral identity border/icon/price while an active sale still uses the shared orderable status badge and blue purchase button.
- Good: a GPT free-amount service uses the same purple identity surfaces, shows `¥0.80 / $1` and available USD, then opens the service detail for amount selection.
- Base: a continuous `$50 / ¥5` offer confirms ten-minute payment and no round ID.
- Base: an unrecognized provider renders as `其他` with a neutral card and all real quota fields intact.
- Base: a seller with no API service sees why a service is required and continues to the existing service publish form.
- Base: an ordinary API service publish chooses free or fixed-package mode, refreshes the deep link without losing mode, and returns to the normal service list after publication.
- Bad: the purchase dialog contains editable amount or multiplier fields.
- Bad: the public market only says `发布 API 服务`, leaving sellers to infer where quota offers are created.
- Bad: the account guard redirects to `/my/account` with `returnTo=/my/api-services`, because that loses the quota-specific selection and continuation copy.
- Bad: `paymentWindowMinutes` is displayed as API response time.
- Bad: category colors replace sold-out/cancelled semantics, or the purchase button becomes purple/red/green per category.
- Bad: the free-amount card calls `createApiQuotaOrder` directly or invents a fixed USD package instead of opening the amount-selection detail.
- Bad: CSV raw contents are serialized to session storage for later mock delivery.

### 6. Tests Required

- Vitest: default tab/query, `intent=quota` entry, account-recovery `returnTo` preservation, `after=quota` post-publish navigation, quota-section anchor, countdown boundaries, fixed amount, five/ten-minute windows, Problem Details mapping, cross-offer round limit, cancellation release, CSV non-persistence, auto-delivery, service-default multiplier inheritance, real adapter fields, and realtime invalidation.
- Publish-page regression tests cover the null/default chooser, all three current query modes, legacy `after=quota`, invalid mode handling, corrected labels, removal of the nested billing choice, mode-specific primary actions, limited preview fields, buyer-flow copy, and forbidden wording absence.
- Progressive publish tests cover active/completed/pending state, visitable-step rules, first-error step mapping, completed summaries, sticky primary action, and the shared responsive preview boundary.
- Publish source regressions cover the `public_profile` defaults in both service-creation entry points, the explicit store-alias opt-in copy, the exact three-step workflow, direct publication from step three without a duplicate confirmation checklist, the shared marketplace-card preview, and the one/two/many model-tag compaction rules.
- Market-card source regressions assert both limited variants and the free-amount variant use `quota-offer-card`, `data-category`, shared product-category inference, the six static CSS selectors, full-width brand buttons, the centered `1640px` free grid with flexible `330px` minimum tracks, `342px` normal card height, compact active reputation, and no unsupported `自动交付` / `安全可靠` / `平台担保` wording.
- Free-amount market cards omit source-author verification entirely. API service detail hides `not_submitted`, `pending`, and `expired`, and displays the badge only for `verified` or `mismatch`; the backend field remains unchanged.
- Type/build: `pnpm --dir frontend typecheck`, then run `pnpm --dir frontend build` with `NUXT_PUBLIC_API_MODE=real`, `NUXT_PUBLIC_API_BASE_URL`, and `NUXT_API_BASE_URL`.
- Browser: `1920x1080`, `1440x900`, and `390x844` for free-market cards; assert first-row counts of four, three, and one respectively, normal card dimensions of approximately `398x360`, `383x360`, and `347x360`, `scrollHeight <= clientHeight`, four metric columns, `40px` purchase buttons, compact model tags, no page-level horizontal overflow, and no console warnings/errors. At `1920x1080`, the centered grid is `1640px` wide and must not create a fifth column. At `1440x900` and `390x844`, the generic publish chooser must show exactly three modes, no editor/preview, and no page-level horizontal overflow; all three mode deep links and legacy `after=quota` remain covered by the broader publish suite.

### 7. Wrong vs Correct

#### Wrong

```ts
const modelMultiplier = offerForm.modelMultiplier
const responseTime = service.paymentWindowMinutes
```

#### Correct

```ts
const modelMultiplier = normalizeDecimal(selectedService.defaultMultiplier, 4)
const responseTime = offer.healthSummary.medianTtftMs // platform-measured for probeModel
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

## Scenario: Free-Amount API Quota Tail Purchase

### 1. Scope / Trigger

- Trigger: changes to free-amount API-service pricing presentation, market cards, detail amount selection, purchase validation, or real purchase-intent request mapping.
- This scenario applies only to `metered_credit`; fixed packages and limited quota offers retain their existing fixed-price flows.

### 2. Signatures

```ts
isApiServiceTailOrder(service): boolean
requestedUsdAllowanceForApiServicePurchase(service, requestedCnyAmount): string
maximumPurchaseCnyForInventory(availableUsdAllowance, cnyPerUsdAllowance): number
backendCreateAPIPurchaseIntent(payload): Promise<ApiPurchaseIntent>
```

### 3. Contracts

- A metered service is a tail order when `maxBuy` is at least `0.01` and below `minimumPurchaseCny`. The public backend excludes lower-value inventory as sold out.
- Normal services display and accept `minimumPurchaseCny..maxBuy`. Tail cards and details display one fixed `尾单 ¥x.xx` amount with `一次买完`; they must never render an inverse range such as `¥10-¥9.99`.
- The detail page initializes a tail amount to `maxBuy`, disables amount editing, and shows the complete remaining USD allowance in the confirmation flow.
- Real purchase-intent mapping submits the complete `availableUsdAllowance`, normalized to six decimal places, for a tail. Normal metered purchases continue deriving allowance from CNY and rate; fixed packages submit no metered allowance.
- Client tail detection is presentation and request-shaping only. The server revalidates current inventory, amount, and allowance and remains authoritative after concurrent inventory changes.

### 4. Validation & Error Matrix

| Condition | UI behavior |
| --- | --- |
| `maxBuy >= minimumPurchaseCny` | Editable normal amount with the configured lower and current upper bounds |
| `0.01 <= maxBuy < minimumPurchaseCny` | Fixed tail amount, disabled input, complete allowance submission |
| Tail amount differs from `maxBuy` | Disable confirmation and show the fixed tail amount |
| Backend rejects a stale tail | Show the Problem Details message and require refreshed service data |
| Service is fixed-package or limited-offer | Do not apply tail detection or metered allowance mapping |

### 5. Good/Base/Bad Cases

- Good: a service with `minimumPurchaseCny=10`, `maxBuy=9.99`, and `availableUsdAllowance=12.499000` shows `尾单 ¥9.99 · 一次买完` and submits all `$12.499000`.
- Base: a service with `maxBuy=10.00` keeps the normal editable amount control.
- Bad: show `¥10-¥9.99`, let the buyer enter `¥5`, or derive the tail USD allowance by dividing the two-decimal CNY amount.

### 6. Tests Required

- Pricing helper tests cover normal/tail detection, tail display, and exact six-decimal tail allowance mapping.
- Detail/card source regressions assert the tail default, disabled input, fixed label, full allowance preview, and absence of inverse ranges.
- Run full Vitest, typecheck, and a real-mode production build.
- Browser-check normal and tail market/detail states at desktop and mobile widths with no overflow, overlap, console errors, or editable tail input.

### 7. Wrong vs Correct

#### Wrong

```ts
const requestedUsdAllowance = divideDecimal(requestedCnyAmount, rate, 6)
const range = `¥${service.minimumPurchaseCny}-¥${service.maxBuy}`
```

#### Correct

```ts
const requestedUsdAllowance = isApiServiceTailOrder(service)
  ? normalizeDecimalTrimmed(service.availableUsdAllowance, 6)
  : normalizeDecimalTrimmed(divideDecimal(requestedCnyAmount, rate, 6), 6)
const label = isApiServiceTailOrder(service)
  ? `尾单 ¥${service.maxBuy} · 一次买完`
  : `¥${service.minimumPurchaseCny}-¥${service.maxBuy}`
```

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

- Render the daily `20:00` sessions from the server response. Do not build Beijing dates or infer the seven-day range in the browser.
- Select the active session first, otherwise the next non-ended session. After all sessions for the first returned date end, preview the next date and derive its heading from the selected slot.
- Compute a display-only clock offset from `serverNow`. When the selected slot reaches start or end, refetch both slots and slot-filtered offers before changing purchase authority.
- Only `isOrderable=true` enables `立即抢购 ¥xx`. One click creates the existing quota order with the current round ID and navigates to payment; pending state keeps the button dimensions stable and prevents duplicate submission.
- The three-step seller wizard selects or creates an eligible API service, configures one offer, then selects a `registration_open` slot and an absolute expiry. It uses the existing API service field components and the atomic rush publication endpoint.
- The three-step seller wizard follows the shared progressive contract without merging its business state into the free-amount form: step one owns service selection/creation, step two owns the manual-delivery package draft, and step three owns slot/expiry plus final atomic publication. Back/edit navigation changes only the current step and must retain `rush`, slot, and expiry state.
- When the first open slot arrives asynchronously, default expiry must be initialized from that selected slot immediately. Expiry shortcuts still produce an absolute Beijing input and must satisfy slot end plus one hour.
- New rush publication is manual-delivery only and limits `copies` to `1..10`. Historical pre-imported offers remain readable, but the wizard must not expose CSV or credential fields.
- The owner manager shows fulfillment confirmation for unconfirmed system rounds. It must keep the action visible and let the server accept or reject the `[startsAt-30m, startsAt)` window; client time is display-only.
- `复制再发` may copy only service ID, name, USD allowance, CNY price, multiplier, delivery mode, and delivery ETA. It must not copy copies, slot key, expiry, or CSV.
- The advanced owner manager remains available. For a locked system-slot batch, keep owner pause/archive actions disabled with the lock reason and let the backend remain authoritative if state changes after render.

### 4. Validation & Error Matrix

| Condition | UI behavior |
| --- | --- |
| Slot query is loading or fails | Show stable loading/error state; do not synthesize slots |
| No `registration_open` slot | Disable publication and explain that no session accepts registration |
| Expiry is earlier than slot end plus one hour | Disable publication and show the absolute minimum |
| Copies are outside `1..10` | Disable publication and preserve the entered package fields |
| Unconfirmed system round | Show confirmation state/action; public purchase remains disabled until the server returns orderable |
| Countdown reaches a boundary | Refetch authoritative slot and offer data once for that boundary |
| Purchase is pending | Disable the fixed-size offer button and ignore repeat clicks |
| Purchase returns a quota Problem Details code | Show the mapped message and refresh session data |
| Copied query contains unsupported timing/inventory fields | Ignore them and keep fresh defaults |

### 5. Good / Base / Bad Cases

- Good: the market opens on an active `20:00` session, shows a tabular `HH:MM:SS` countdown, and directly creates one order after a single click.
- Good: copying a scheduled `$100` offer preserves pricing and delivery settings, while copies remain `10` and slot/expiry are freshly selected.
- Good: at `19:29` the seller can still see the confirmation action; a server rejection explains that the window has not opened instead of the UI silently hiding the workflow.
- Base: a seller with no eligible service creates the prerequisite service inside step one and resumes the same wizard.
- Bad: the browser enables purchase merely because its local countdown reached zero.
- Bad: a copy URL carries old `copies`, `slotKey`, `expiresAt`, or raw CSV data.

### 6. Tests Required

- Vitest: default/active/tomorrow 20:00 session selection, dynamic heading, server clock offset, boundary refetch, one-click purchase, pending deduplication, Problem Details mapping, wizard prerequisites, 10-copy validation, immediate default expiry, confirmation action/state, and copy-field allowlist.
- Type/build: `pnpm --dir frontend typecheck`, then run `pnpm --dir frontend build` with `NUXT_PUBLIC_API_MODE=real`, `NUXT_PUBLIC_API_BASE_URL`, and `NUXT_API_BASE_URL`.
- Browser: `1440x900` and `390x844`; assert the 20:00 session, fixed countdown/button dimensions, enabled valid manual publication, confirmation state, no application console errors, and no page-level horizontal overflow.

### 7. Wrong vs Correct

#### Wrong

```ts
if (countdown.value === '00:00:00') offer.isOrderable = true
if (deviceNow >= confirmationOpensAt) showConfirmButton = true
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
  deliveryMode: offer.deliveryMode,
  deliveryEtaMinutes: offer.deliveryEtaMinutes,
}
```

The server owns orderability and timing; copy drafts carry stable editable commercial settings only. Multiplier always comes from the currently selected service.

## Scenario: Role-Aware API Order Detail And Buyer Review

### 1. Scope / Trigger

- Trigger: changes to buyer, seller, or administrator API order lists/details; API order countdowns; completion/dispute mutations; or backend adapters for order projections.
- Buyer and seller reuse the participant detail page and the same order facts. Administrator detail uses a separate read-only management page.

### 2. Signatures

```ts
type ApiOrderCompletionSource = 'buyer_confirmed' | 'auto_completed' | 'seller_delivered' | 'remedy_confirmed'

type ApiPurchaseOrder = {
  merchantConfirmDueAt?: string
  merchantConfirmOverdue: boolean
  deliveryDueAt?: string
  deliveryOverdue: boolean
  latePaymentStatus?: 'reported' | 'not_received' | 'received_refund_pending'
  canReportLatePayment: boolean
  deliveryReviewExpiresAt?: string
  completionSource?: ApiOrderCompletionSource
}

getAdminApiOrder(id: string): Promise<AdminApiOrder>

/my/api-orders/:id
/merchant/api-orders/:id
/admin/api-orders/:id
```

### 3. Contracts

- Seller submission returns `completed/seller_delivered`. Buyer detail shows the completed delivery, `联系商家`, and `凭证存在问题`; seller detail/list shows that delivery and the order are complete with no reminder or pending action.
- There is no buyer confirmation mutation or review countdown. `凭证存在问题` reuses the existing dispute mutation with a structured reason and server-owned occurrence time/deadline validation.
- Historical `completed/buyer_confirmed` and `completed/auto_completed` values keep distinct truthful copy. Automatic completion must not render as buyer approval, a rating, or platform verification.
- The shared timeline ends at seller delivery/completion. `delivery_submitted` remains a legacy read-compatible status but must not create a buyer or seller pending action.
- Credential access remains available to participants after completion. Seller delivery remains immutable and the UI must not offer resubmission or editing.
- Admin detail shows buyer/seller IDs, frozen commercial data, event times, deadline, completion source, and a dispute link when present. It renders no raw credential, payment/contact value, or participant contact detail.
- Payment, merchant-confirmation, delivery, and review deadlines come from the order DTO. Device time may animate a countdown but never authorizes an action or synthesizes an overdue state.
- While `pending_payment`, buyer detail persistently states that an off-platform transfer does not update the order and that the buyer must click `我已完成付款` before the deadline.
- A cancelled buyer order shows `我已发生逾期付款` only when `canReportLatePayment=true`. Seller detail shows a resolution action only for `latePaymentStatus=reported`; both views state that reporting cannot restore the order, stock, or rush eligibility.
- New dispute controls omit `continue_fulfillment`. The shared read-only label remains so historical disputes are explainable.

### 4. Validation & Error Matrix

| Condition | UI behavior |
| --- | --- |
| Buyer reviews `completed/seller_delivered` | Show credential, merchant contact, and problem/dispute actions while eligible |
| Seller reviews `completed/seller_delivered` | Show order complete; no pending action or reminder |
| `disputeStatus=open` | Show dispute state without reverting the completed order fact |
| Removed confirmation route or client function | Must stay absent |
| `completionSource=buyer_confirmed` | Show buyer-confirmed completion copy |
| `completionSource=auto_completed` | Show review-window-ended copy without endorsement semantics |
| Dispute deadline missing | Show stable completed state without fabricating a browser deadline |
| `merchantConfirmOverdue=true` or `deliveryOverdue=true` | Show the matching server-projected overdue state and existing dispute entry; do not cancel or release inventory |
| Timed-out cancellation has `canReportLatePayment=true` | Show report dialog; optional note contains no credential or full account data |
| Seller sees `latePaymentStatus=reported` | Offer only `not_received` or `received_refund_pending` |
| Admin response contains no credential | Render order facts only; never infer or request participant credential fields |

### 5. Good / Base / Bad Cases

- Good: buyer and seller open the same delivered order; the buyer sees contact/problem actions and the seller sees a completed order with no task.
- Good: an order cancelled for payment timeout exposes one late-payment report action for 24 hours, then renders the recorded seller outcome without implying refund completion.
- Base: historical automatic completion remains readable by both participants, while completion copy says the old review window ended.
- Bad: seller detail says `等待买家确认`, the browser starts a fresh 24-hour timer, or admin detail renders a credential/contact section.

### 6. Tests Required

- Vitest asserts buyer/seller status copy, actions, pending-badge differences, all server deadline/overdue projections, persistent transfer warning, late-payment report/resolution, both completion-source labels, new-dispute option filtering, historical dispute labels, and admin credential/contact omission.
- Adapter tests assert real and mock projections preserve `deliveryReviewExpiresAt` and `completionSource` and never add credentials to admin rows/details.
- Run full Vitest, Nuxt typecheck, and real-mode production build.
- Browser-check buyer and seller at `1440x900` and `390x844`, administrator at `1440x900`, both buyer dialogs, no page-level horizontal overflow, and no relevant console errors.

### 7. Wrong vs Correct

#### Wrong

```ts
const deadline = addHours(order.deliverySubmittedAt, 24)
const canReportLatePayment = order.cancelReason === 'payment_timeout' && deviceNow < addHours(order.cancelledAt, 24)
const sellerNextStep = order.status === 'delivery_submitted' ? '等待买家确认' : ''
```

#### Correct

```ts
const deadline = order.deliveryReviewExpiresAt
const canReportLatePayment = order.canReportLatePayment
const sellerNextStep = order.status === 'delivery_submitted' ? '无需操作' : nextStepFor(order)
```
