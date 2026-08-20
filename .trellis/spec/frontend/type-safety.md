# Type Safety

> Type safety patterns in this project.

Date: 2026-06-20
Executor: Codex

---

## Scenario: API Merchant Identity Display

### 1. Scope / Trigger

- Trigger: API service seller identity can be shown as either a public profile or a store alias.
- Public UI must not infer or display the linux.do username for `store_alias` services.
- The contract is frontend-local today: mock data lives in `frontend/src/data/mock.ts`, and the API facade/helpers live in `frontend/src/lib/api.ts`.

### 2. Signatures

```ts
export type ApiMerchantIdentityMode = 'public_profile' | 'store_alias'

export type ApiService = {
  merchant: string
  merchantUsername: string
  merchantIdentityMode: ApiMerchantIdentityMode
  merchantDisplayName: string
}

export type ApiPurchaseIntentSnapshot = {
  merchant: string
  merchantUsername: string
  merchantIdentityMode: ApiMerchantIdentityMode
  merchantDisplayName: string
}
```

Use the facade helpers instead of direct field access in pages/components:

```ts
getApiMerchantDisplayName(source)
canOpenApiMerchantProfile(source)
getApiMerchantProfileUrl(source)
getApiMerchantAvatarText(source)
getApiMerchantVisibilityLabel(source)
```

### 3. Contracts

- `merchantIdentityMode = 'public_profile'`: public pages may show the seller profile link and may include the username in search/profile surfaces.
- `merchantIdentityMode = 'store_alias'`: public pages show `merchantDisplayName` only and must not link to `/u/:merchantUsername`.
- `merchantDisplayName`: required for `store_alias`; must be the name shown in market cards, service detail, order detail, order lists, event timelines, and admin rows.
- Order snapshots copy `merchantIdentityMode`, `merchantDisplayName`, and `merchantUsername` at purchase-intent creation so historical orders do not drift when a service changes later.
- Admin-only surfaces may show `merchantDisplayName -> merchantUsername` mapping for moderation. Ordinary public and buyer-facing pages must not.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| `store_alias` with empty display name | Block publish submit and show a field error |
| Display name length outside 2-20 characters | Block publish submit and show a field error |
| Display name contains contact-like text, link, or linux.do username shape | Block publish submit and show a field error |
| Display name contains misleading guarantee/official wording | Block publish submit and show a field error |
| Public UI receives `store_alias` | Render display name and a visibility label; return `null` for profile URL |
| Admin UI receives `store_alias` | Render display name plus real username mapping |

### 5. Good/Base/Bad Cases

- Good: `store_alias`, `merchantDisplayName: '小葵 API'`, market card shows `小葵 API` and no `/u/orbit` link.
- Base: `public_profile`, `merchantDisplayName` matching the public merchant name, profile links continue to work.
- Bad: A page renders `service.merchantUsername`, `service.merchant`, or ``/u/${service.merchantUsername}`` directly for a store-alias seller.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Production build: real-mode `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Source scans must verify no misleading official/guarantee wording is introduced.
- Source scans must verify public page/component code uses helper functions for API merchant identity instead of direct profile links.

### 7. Wrong vs Correct

#### Wrong

```vue
<RouterLink :to="`/u/${service.merchantUsername}`">
  {{ service.merchant }}
</RouterLink>
```

#### Correct

```vue
<RouterLink v-if="getApiMerchantProfileUrl(service)" :to="getApiMerchantProfileUrl(service)!">
  {{ getApiMerchantDisplayName(service) }}
</RouterLink>
<span v-else>{{ getApiMerchantDisplayName(service) }}</span>
```

---

## Scenario: Public Review and Search Aggregation

### 1. Scope / Trigger

- Trigger: public profile reviews or search results are mapped from backend responses into frontend display records.
- Real public review lists must reflect only backend-published transaction reviews and must never mix them with static or locally derived rows.
- Store-alias API merchant identity must not leak through dynamic review aggregation or search result subtitles.

### 2. Signatures

```ts
export type SearchResult = {
  type: '官方价格' | '车源' | 'API 服务' | '用户' | '商户'
  title: string
  subtitle: string
  badge: string
  to: string
}

function publicReviewsForProfile(username: string): PublicReviewRecord[]
function backendPublicUserReviews(username: string): Promise<PublicReviewRecord[]>
function backendSearchMarket(query: string): Promise<SearchResult[]>
```

### 3. Contracts

- In real backend mode, `GET /api/v1/users/{username}/reviews` is the only public-review source. The adapter maps returned published reviews and must not merge `publicReviewRecords`, carpool application state, purchase intents, or API-order stores.
- A missing `rating` is not a zero-star review. Received sealed rows exist only in the authenticated review center and never enter `PublicReviewRecord`.
- Static `publicReviewRecords` and local source-derived rows are mock-mode data only. They must remain isolated from real public-profile requests.
- Public review DTOs preserve backend `verified`, rating, tags, note, date, and service type. Transaction type and buyer/seller role fields may be added to the frontend record only through an explicit shared type update; consumers must not infer them from titles.
- Store-alias API merchants must not gain a public user link through a review or search mapping. Backend public-review identity policy remains authoritative.
- Mock `searchMarket()` must include `publicMerchantProfiles` as `type: '商户'` results in addition to user profiles.
- Real `searchMarket()` must call `searchBackend.ts` and map backend rows to the same `SearchResult` union without silently falling back to mock data.
- Store-alias API service search may return an `API 服务` result with the public merchant display name, but must not return a separate `商户` result or `/u/:merchantUsername` link that reveals the hidden owner.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Backend returns a published verified review | Public profile renders the mapped review |
| Backend omits a sealed/removed/excluded review | Frontend does not reconstruct it |
| Real public-review request fails | Visible error; no static/local fallback |
| Review-center row has `rating=null` | Preserve `null`; do not map to zero |
| Search keyword matches merchant username/display name/identity | Search returns a `商户` result |
| Search keyword matches store alias display name through API service | Search may return the API service, but must not expose hidden merchant username or a public user link |

### 5. Good/Base/Bad Cases

- Good: a published API-order seller-to-buyer review returned by the backend appears on the buyer's public profile with the backend rating and verified flag.
- Base: mock mode continues to render its explicit seed reviews without affecting real mode.
- Bad: concatenate backend reviews with `publicReviewRecords`, derive a sealed review from a completed order, or create `/u/orbit` from a store-alias service owner.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Production build: real-mode `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Adapter test: real public-review reads call the backend and do not fall back to mock/local rows.
- Source scan for product-boundary wording.
- Source scan for store-alias leakage patterns such as direct `/u/${service.merchantUsername}` in public pages/components.
- Browser or SPA route smoke for `/search?q=<merchant>` and a public profile with backend reviews when browser tooling is available.

### 7. Wrong vs Correct

#### Wrong

```ts
return [...await backendPublicUserReviews(username), ...publicReviewRecords]
```

#### Correct

```ts
if (shouldUseRealBackend()) {
  return backendPublicUserReviews(username)
}
return publicReviewsForProfile(username)
```

---

## Scenario: API Purchase Intent Boundary Language

### 1. Scope / Trigger

- Trigger: API service public/detail pages create purchase-intent records before an API order exists.
- UI copy must not imply that C2CMarket processes payment, stores API keys during the intent step, stores panel accounts during the intent step, or automatically delivers credentials.

### 2. Signatures

```ts
export type ApiDeliveryMode = 'api_key_endpoint' | 'sub2api_panel_account'

function getApiDeliveryModeLabel(mode: ApiDeliveryMode): string
function getApiDeliveryModeDescription(mode: ApiDeliveryMode): string
function createApiPurchaseIntent(payload: CreateApiPurchaseIntentPayload): Promise<ApiPurchaseIntent>
```

### 3. Contracts

- Public and buyer-facing UI calls the record a `购买意向`, `API 意向`, or `意向记录`.
- Money labels use `意向金额`; supporting copy must say final amount and payment are confirmed off-platform by both parties.
- CTA copy uses `提交购买意向并查看商户联系方式` or shorter `提交意向`.
- Access mode labels are non-sensitive descriptions:
  - `API 请求地址接入说明`
  - `Sub2API 面板接入说明`
- Successful API intent creation may immediately show the frozen merchant contact to that buyer; the owner may view the frozen buyer-selected contact from the owner detail.
- API intent pages must not show a countdown, contact-window expiry, or owner-accept-before-contact step. Those concepts belong to carpool application contact sessions only.
- Purchase-intent and public API-service pages must never show, request, paste, upload, store, or automatically deliver API keys, endpoint secrets, panel passwords, tokens, sessions, recovery codes, or account credentials. The only frontend exception is the API order delivery credential flow described below, after buyer payment submission and seller payment confirmation.
- Carpool detail copy must distinguish `成员席位 / 官方邀请 / 无需共享密码方案` from shared password, token, or session credential transfer.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| API market card action | Label is `提交意向` or purchase-intent wording |
| API detail submit panel | Shows immediate merchant-contact disclosure after submit and explicit no-credential warning |
| API amount labels | Use `意向金额`, not payable/payment wording |
| API access-mode labels | Do not contain `API Key` or `面板账号` as a thing delivered by the platform |
| Store-alias merchant profile | Real user profile does not show `API 商户` badge unless a public-profile API service exists |
| Carpool detail | Shows seat/rules model and forbids shared passwords/tokens/sessions |

### 5. Good/Base/Bad Cases

- Good: `提交购买意向并查看商户联系方式`.
- Good: `提交后将立即展示商户选择的联系方式，同时商户可以查看你选择的联系方式。`
- Good: `不得在平台填写、粘贴或上传 API Key、密码、token、session 或面板登录凭据。`
- Base: internal type names may remain `ApiPurchaseIntent` and existing route paths may remain `/my/api-orders` during mock frontend work.
- Bad: `本次应付`, `确认购买并联系商户`, `购买后自动展示接入信息`, `Sub2API 面板账号`, or `API 请求地址 + API Key` on user-facing API pages.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Production build: real-mode `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Product-boundary source scan for:
  - `购买后自动展示`
  - `购买后提供面板`
  - `本次应付`
  - `确认购买`
  - `API 请求地址 + API Key`
  - `Sub2API 面板账号`
  - `共享订阅`
  - `真实成交`
  - `官方低价`
  - `官方最低价`
  - `当前最低`
- Screenshot review for API market/detail, carpool detail, search, notifications, and public profile after copy changes.

### 7. Wrong vs Correct

#### Wrong

```vue
<Button>确认购买并联系商户</Button>
<dt>本次应付</dt>
<option>API 请求地址 + API Key</option>
```

#### Correct

```vue
<Button>提交购买意向并查看商户联系方式</Button>
<dt>意向金额</dt>
<option>API 请求地址接入说明</option>
```

---

## Scenario: API Order Payment And Delivery Credential Flow

### 1. Scope / Trigger

- Trigger: frontend work touching `/my/api-orders`, `/merchant/api-orders`, API order backend adapters, TanStack Query hooks, or API order detail/action pages.
- Product flow: submit purchase intent -> create API order -> show frozen merchant/contact/payment materials -> buyer marks paid -> seller either confirms off-platform receipt or reports `未到账`/`金额不符`/`备注不符` -> buyer supplements and resubmits when needed -> seller confirms receipt -> seller submits one structured delivery credential -> buyer/seller can view it long term in order detail.
- Boundary: this is not platform payment, escrow, API verification, API proxying, automatic delivery, chat, file upload, refund, or a credential history/editor.

### 2. Signatures

```ts
export type ApiOrderDeliveryKind = 'api_key_endpoint' | 'login_account'

export type ApiOrderDeliveryCredential = {
  deliveryKind: ApiOrderDeliveryKind
  apiBaseUrl?: string
  apiKey?: string
  panelLoginUrl?: string
  username?: string
  password?: string
  instructions?: string
  submittedAt: string
}

export type ApiOrderPaymentInstructions = {
  orderId: string
  paymentMethod: 'wechat' | 'alipay'
  paymentInstructions: string
  paymentQrCodeDataUrl: string | null
  paymentExpiresAt: string
}

export type APIIntentPricingSnapshotProjection = {
  models: string[]
  multiplier: string
  defaultMultiplier: number
  usageVisibility: ApiUsageVisibility
  usageVisibilitySnapshotMissing: boolean
  merchantNote: string
  merchantSupportNote: string
  issue?: 'missing' | 'invalid'
}

projectAPIIntentPricingSnapshot(value: string): APIIntentPricingSnapshotProjection

isApiOrderReceiptConfirmed(status: ApiOrderStatus): boolean
```

### 3. Contracts

- API service detail may still create a purchase intent first, but once a payment method is selected it must create an API order and navigate to the order detail, not keep driving fulfillment from `ApiPurchaseIntent.status`.
- Buyer order detail must display the frozen merchant display name, merchant contact snapshot, selected WeChat/Alipay payment method, private payment instructions, and QR-code snapshot from the explicit payment-instructions read endpoint.
- Buyer action copy is `我已付款`; seller action copy is `确认已收款` followed by `确认已交付`.
- Seller delivery form appears only for owner view when `status === 'paid_confirmed'` and no `deliveryCredential` exists.
- Delivery form supports only `api_key_endpoint` and `login_account`. It must not expose a generic chat/message/file upload field, and it must not allow editing after submit.
- Buyer/seller order detail may render `deliveryCredential` with copy buttons and long-term visibility. Lists, public API service pages, notifications, reports, admin summaries, and search rows must not render raw API keys or passwords.
- UI wording should say `交付凭证`, `买家专属`, and `提交后不可修改`; do not claim platform revocation support, and avoid `自动发货`, `平台担保`, `平台验真`, and `主账号密码`.
- Real backend mode must call API order endpoints through `apiMarketBackend.ts` and must not catch failures to return mock orders.
- Intent creation freezes `pricingSnapshot.models[].modelKey`, each model's `merchantMultiplier`, `usageVisibility`, `merchantNote`, and seller-authored `merchantSupportNote`. Order creation copies that JSON unchanged. Service/package API DTOs separately use `modelKeySnapshot`; neither contract uses the removed `modelNameSnapshot`.
- `mapBackendAPIOrder` must project `order.pricingSnapshot`, not a current service response, `serviceTitleSnapshot`, or the separately fetched intent projection. The order record is the authority after creation even when the intent usually contains the same bytes.
- Order detail renders every non-empty frozen model name in snapshot order with duplicates removed. A missing snapshot shows explicit historical-missing copy; malformed JSON shows an explicit unavailable state. Neither path may substitute the service title or current service models.
- Order information labels seller-authored frozen content as `商户售后说明`. The fixed platform copy is a separate `平台交易边界` field and must not be written into or presented as seller input.
- Buyer and merchant detail routes share this snapshot projection and page component. Their five-step workflow uses the existing shadcn-vue Stepper with an explicit non-zero separator height; single-choice dialogs use the official `RadioGroupItem` plus `Label` composition.
- Buyer navigation, list titles, detail return actions, and buyer task summaries use `API 购买订单`; merchant equivalents use `API 销售订单`. Neutral administration/review domain copy may continue to use `API 订单`.
- Merchant operating metrics apply search, time-range, and service filters before aggregation. The active status tab filters only table rows and must not change the metric values.
- `已确认收款金额` includes only `paid_confirmed`, `delivery_submitted`, and `completed`, because each follows explicit merchant receipt confirmation. It excludes `pending_payment`, `payment_submitted`, `payment_issue`, and `cancelled`.
- Cancelled merchant orders contribute only to the `已取消订单` count. The UI must not present a cancelled amount or include cancelled orders in revenue-like totals.
- A merchant order table with six columns keeps an explicit minimum table width inside the shared `SoftTable` overflow container. Mobile root width must remain bounded while the table scrolls horizontally.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Payment instructions include `paymentQrCodeDataUrl` | Buyer detail renders the QR image inside the payment card. |
| Buyer clicks `我已付款` | Mutation calls `POST /api/v1/me/api-orders/{id}/submit-payment`, invalidates buyer/merchant order queries, and shows waiting-for-seller state. |
| Seller confirms receipt | Mutation calls `POST /api/v1/owner/api-orders/{id}/confirm-payment`, then the seller can open the delivery form. |
| Seller submits `api_key_endpoint` | Payload includes `deliveryKind`, `apiBaseUrl`, `apiKey`, and optional `instructions`; the detail response shows the credential. |
| Seller submits `login_account` | Payload includes `deliveryKind`, `panelLoginUrl`, `username`, `password`, and optional `instructions`; the detail response shows the credential. |
| Order list receives a delivered order | It may show status and submitted time, but must not render raw `apiKey` or `password`. |
| `pricingSnapshot` is empty on a historical order | Render historical-missing model/usage/seller-support copy; do not query mutable service values as a fallback. |
| `pricingSnapshot` is malformed JSON | Render snapshot-unavailable model/multiplier/seller-support copy; do not display `serviceTitleSnapshot` as a model. |
| `pricingSnapshot.models` contains several valid models | Render all unique `modelKey` values and show one multiplier or `按模型分别计算` as appropriate. |
| Merchant opens the cancelled status tab | Table rows show only cancelled orders; operating metrics retain the same search/time/service population. |
| Merchant cancelled an unpaid order | `已取消订单` increments, while `已确认收款金额` remains unchanged. |
| Merchant confirms receipt, delivers, or completes | The order amount is counted exactly once in `已确认收款金额`. |

### 5. Good/Base/Bad Cases

- Good: buyer detail in `pending_payment` calls `readApiOrderPaymentInstructions()` and renders the frozen WeChat QR code plus merchant contact snapshot.
- Good: seller detail in `paid_confirmed` submits `{ deliveryKind: 'api_key_endpoint', apiBaseUrl, apiKey, instructions }`, receives `deliveryCredential`, and the form becomes read-only.
- Good: an order frozen with `gpt-4.1`, `gpt-4.1-mini`, and `gpt-4o` shows those exact canonical keys even after the merchant edits the service title or enabled models.
- Base: a delivered order list row shows `已交付` and `deliverySubmittedAt`, but no raw `apiKey` or `password` text.
- Base: an older order without seller-support fields says `历史订单未冻结商户售后说明` beside the separate fixed platform boundary.
- Good: one cancelled `¥10.00` order shows `已取消订单 1` and `已确认收款金额 ¥0.00`, including while the cancelled tab is active.
- Bad: a page derives API order fulfillment from `ApiPurchaseIntent.status`, or renders a generic `deliveryNote` textarea that can be edited after delivery.
- Bad: a list, notification, search result, report row, or admin summary renders `order.deliveryCredential.apiKey` or `order.deliveryCredential.password`.
- Bad: `models: [intent.serviceTitleSnapshot]`, reading the current service to repair history, or labeling fixed platform copy as seller after-sales input.
- Bad: labeling both role-specific routes `我的 API 订单`, aggregating merchant metrics from status-tab-filtered rows, or counting cancelled/submitted-payment amounts as confirmed receipts.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Real-mode build: `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Source scan for forbidden product wording outside the spec allowlist.
- Adapter/review checks must verify `paymentQrCodeDataUrl` is mapped both in order-settings submit payloads and payment-instructions responses.
- Backend intent tests must assert the JSON snapshot freezes model names/multipliers, usage visibility, merchant note, and merchant support note.
- Frontend projection tests must cover one/many models, duplicate removal, different multipliers, historical missing fields, malformed JSON, and the rule that order mapping reads `order.pricingSnapshot`.
- Order-detail source/browser checks must cover explicit Stepper separator dimensions and state color, `RadioGroupItem + Label` wiring, seller/platform field separation, and mobile page overflow.
- Receipt-status unit tests must assert all seven API order statuses, including the three included and four excluded states.
- Workspace source tests must assert buyer/seller labels, base-filter metric aggregation, cancelled count, confirmed-receipt wording, and the merchant table minimum width.
- Browser checks must verify stable metric values across status tabs and confirm `documentElement.scrollWidth <= clientWidth` while the mobile table container remains horizontally scrollable.

### 7. Wrong vs Correct

#### Wrong

```vue
<textarea v-model="deliveryNote" />
<div v-for="order in orders">{{ order.deliveryCredential?.apiKey }}</div>
```

This treats delivery as an editable generic note and leaks raw credentials from a list view.

#### Correct

```vue
<ApiOrderDeliveryForm
  v-if="isMerchantView && order.status === 'paid_confirmed' && !order.deliveryCredential"
  @submit="submitApiOrderDeliveryCredential(order.id, payload, order.version)"
/>
<span>{{ getApiOrderStatusLabel(order.status) }}</span>
```

The detail-only form submits a typed credential once; list rows render status helpers, not secret fields.

#### Wrong: Reconstruct Frozen Order Fields

```ts
const models = [intent.serviceTitleSnapshot]
const warranty = '最终金额和售后由双方站外确认'
```

#### Correct: Project The Order Snapshot And Keep Responsibility Explicit

```ts
const pricing = projectAPIIntentPricingSnapshot(order.pricingSnapshot ?? '')
const models = pricing.models
const merchantSupportNote = pricing.merchantSupportNote
const platformTradeBoundary = '售后由双方站外确认；平台不代收、不托管、不担保、不代赔。'
```

#### Wrong: Aggregate Merchant Amounts From Visible Rows

```ts
const orderAmountTotal = filteredRows.value.reduce(addOrderAmount, '0.00')
```

#### Correct: Aggregate Confirmed Receipts From The Base Population

```ts
const confirmedReceiptAmount = baseFilteredRows.value
  .filter(order => isApiOrderReceiptConfirmed(order.status))
  .reduce(addOrderAmount, '0.00')
```

---

## Scenario: API Merchant Contacts And Frozen Refund Evidence

### 1. Scope / Trigger

- Trigger: frontend work touching API-service or quota publication, personal contact settings, purchase-intent contact disclosure, API-order dispute entry, or refund-policy evidence.
- The UI helps participants contact each other and inspect frozen evidence. It does not provide in-platform chat, payment, refund execution, API verification, or compensation.

### 2. Signatures

```ts
type APIServiceRequest = {
  ownerContactMethodId: string
}

type BackendAPIPurchaseIntent = {
  merchantContact?: ContactDisclosure | null
  merchantContacts?: ContactDisclosure[]
}

type ApiOrder = {
  afterSalesExpiresAt?: string
  canOpenDispute?: boolean
  disputeEligibilityReason?: string
}

type OpenApiOrderDisputeInput = {
  issueOccurredAt?: string | null
}
```

### 3. Contracts

- Publish pages list only eligible transaction contacts: enabled WeChat or enabled verified email with a current version. Selection uses radio controls and requires exactly one explicit contact ID.
- `linuxdo`, unverified email, disabled contacts, and unsupported contact types never enter the selector. An empty state links to `/my/contacts`; publication must never silently create or choose a placeholder contact.
- Real and Mock submit paths send the selected `ownerContactMethodId`. The backend remains authoritative for ownership, enabled state, type, email verification, and immutable-version validation.
- Authorized intent/order detail renders every frozen `merchantContacts[]` item in snapshot order. It falls back to legacy `merchantContact` only when the array is absent, never to the mutable current service or profile.
- Buyer and merchant detail trust `canOpenDispute` and `afterSalesExpiresAt`. A completed eligible order shows the reporting-grace explanation and requires `issueOccurredAt`; the input maximum is the earlier of browser-now and the frozen service validity end.
- Refund evidence labels `api-merchant-refund-v1` as `API 商户退款规则 v1`, states `下单时已锁定`, and opens the frozen merchant commitment, applicability, exclusions, and platform boundary. Unknown historical versions show their literal value without borrowing current policy copy.
- Copy must say that 24 hours is a reporting window only and the issue must have occurred during service validity. It must not claim extended validity, automatic refunds, platform custody, verification, advance payment, or compensation.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| No eligible merchant contact | Disable/block publication and link to `/my/contacts` |
| More than one eligible contact | Require the user to choose one; do not preselect a preferred type |
| Contact becomes unavailable before real submit | Show backend error; do not create a placeholder or report success |
| Intent contains `merchantContacts[]` | Render all frozen items in returned order |
| Array absent but legacy contact exists | Render the single frozen legacy contact |
| Completed order is eligible | Show dispute action, reporting deadline, occurrence input, and reporting-only copy |
| Completed order is ineligible | Hide the dispute action and do not recompute eligibility from browser time |
| Known v1 refund rule | Show readable name plus frozen applicability, exclusions, commitment, and platform boundary |
| Unknown historical rule version | Show literal version and only evidence actually frozen on the order |

### 5. Good / Base / Bad Cases

- Good: a merchant explicitly selects verified email, the buyer later sees that frozen value even after the merchant edits the account contact, and the seller sees the same evidence.
- Good: an order completed inside the grace period asks when the outage occurred and clearly says that API validity has not been extended.
- Base: a historical intent with only `merchantContact` still renders one contact without fabricating an array or reading a current service.
- Bad: silently create or auto-select a contact, accept linux.do as a transaction contact, or call the current contact API to repair an old order.
- Bad: display only `api-merchant-refund-v1`, omit exclusions/platform boundary, or describe `补报截止` as a guaranteed refund deadline.

### 6. Tests Required

- Component tests cover eligible contact filtering, explicit radio selection, WeChat/verified-email choices, empty-state settings link, and no placeholder creation.
- Adapter tests cover new arrays plus legacy fallbacks, all participant after-sales fields, administrator contact omission, and frozen snapshot mapping in normal and limited-quota paths.
- Order-detail tests cover completed eligible/ineligible states, earlier-of-now-and-validity occurrence maximum, occurrence payload serialization, grace-period copy, rule display name, lock label, applicability, exclusions, and platform boundary.
- Mock/real contract tests prove equivalent contact validation, frozen refund evidence, after-sales eligibility fields, and completed-order occurrence validation.
- Run full Vitest, OpenAPI generated-type check, Nuxt typecheck/build, and desktop/mobile browser checks for both publish flows and buyer/merchant order detail.

### 7. Wrong vs Correct

#### Wrong

```ts
const ownerContactMethodId = contacts.value.find(item => item.enabled)?.id
const canOpenDispute = order.status !== 'completed'
```

#### Correct

```ts
const ownerContactMethodId = selectedTransactionContactId.value
const canOpenDispute = order.canOpenDispute
```

Selection is explicit, historical disclosure is frozen, and eligibility comes from the server's order projection.

---

## Scenario: Capability-Driven Student, Seller, And Administrator Navigation

### 1. Scope / Trigger

- Trigger: changing authenticated navigation, route metadata, query enablement, student/linux.do mock identities, merchant workspaces, probe entry points, or administrator routes.
- The UI shell must not make ordinary users feel that full admin tooling is part of their normal workspace.
- Student-email and linux.do users are different identity facts with different deterministic capabilities; neither is a frontend-selectable role.
- Sidebar, route, mutation, and owner-query visibility derive from the canonical capability array returned by the profile API, never from owned-resource or pending-count heuristics.
- Admin moderation rows must provide enough context for local mock review before backend integration.

### 2. Signatures

```ts
type UserProfile = {
  permissions: Array<'admin'>
  capabilities: Capability[]
}

type Capability =
  | 'api_order.create'
  | 'carpool.apply'
  | 'carpool.publish'
  | 'api_service.publish'
  | 'api_quota.publish'
  | 'api_probe.manage'
  | 'admin.access'

type WorkspaceNavKey =
  | 'personal-center'
  | 'account-settings'
  | 'reputation-rights'
  | 'support-center'
  | 'message-center'

export type AdminRow = {
  id: string
  primary: string
  secondary: string
  owner: string
  status: string
  risk: string
  targetType?: string
  detailItems?: Array<{ label: string, value: string }>
  targetTo?: string | null
}

export function usePersistentSidebar(storageKey: string): {
  sidebarCollapsed: Ref<boolean>
}

export function initialSidebarCollapsed(
  storageValue: string | null,
  viewportWidth: number,
): boolean

export function useUnsavedChangesGuard(
  dirty: Ref<boolean>,
  message?: string,
): void
```

### 3. Contracts

- `App.vue` selects exactly one layout: standalone routes render directly, `/admin/**` uses `AdminShell`, and all other authenticated pages use `AppShell`.
- `AppShell` always shows public browse, authenticated buyer transactions, notifications, and account settings. It shows carpool apply/publish, API service/quota publishing, merchant workspaces, and probes only for their exact capabilities.
- Aggregated workspace entries use typed `meta.workspaceNavKey` values rather than duplicated path arrays. `/my` maps exactly to `personal-center`; `/my/profile`, `/my/contacts`, `/my/account`, and `/my/privacy` map to `account-settings`; `/my/notifications` maps to `message-center`; `/my/reputation` and `/my/promotion-benefits` map to `reputation-rights`; report/appeal and feedback list/detail routes map to `support-center`. Unrelated `/my/**` routes must not inherit these active states.
- Every sidebar item has a stable `key`; active state compares that key after resolving the best route match. Query tabs and detail routes therefore activate exactly one aggregated entry even when several items share a route prefix.
- The account group is fixed at `个人中心 / 账户设置 / 信誉与权益 / 支持中心`. Message, reputation/promotion, and report/feedback variants live in page-level tabs and must not reappear as separate sidebar or avatar-menu entries.
- The four account-setting routes retain stable deep links and share one persistent tab shell. Because they reuse the same page component, unsaved-change protection must register both `onBeforeRouteLeave` and `onBeforeRouteUpdate`, plus `beforeunload` for browser refresh and tab close.
- Profile, contact/payment, and privacy forms compute dirty state from independent saved snapshots. A successful save refreshes only its own snapshot; a failed save remains dirty; query refetches do not overwrite a dirty draft. Profile writes send the saved privacy projection, while privacy writes send the saved profile projection.
- A student profile has only `api_order.create`; it can buy quota/API service offers and use order-scoped after-sales, dispute, review, buyer contacts, and eligible model testing, but it must not trigger carpool/seller/probe owner queries.
- A linux.do-bound profile has all six non-admin business capabilities even with zero owned resources. In particular, `api_probe.manage` always exposes `/my/api-probe-connections` and its first-create empty state.
- `admin.access` controls one `进入管理台` entry plus `/admin/**`; legacy `permissions: ['admin']` remains a displayed identity fact but is not a second route authority.
- Route records use typed `meta.capability`; global middleware loads the current profile, rejects missing capability to `/forbidden`, and cannot authorize from stale mock/resource state.
- TanStack owner/merchant/probe queries use `enabled: hasCapability(...)`. A hidden component with an active query is still a capability leak and is not acceptable.
- Real backend failures remain visible and never fall back to mock authorization/data. Mock mode has explicit anonymous, student, linux.do, and administrator sessions and uses the same capability helper.
- `AdminShell` owns the grouped administration directory, global search, pending total, administrator identity, and a clear return-to-user-side action.
- Both shells persist desktop collapse state independently. With no stored preference, widths below 1024 pixels default to collapsed.
- Mobile navigation is a modal drawer with dialog semantics, a close action, Escape support, and enough width/scrolling to avoid obscuring navigation content.
- The sidebar must not expose a manual `用户 / 管理员` or `用户 / 商户 / 管理` switch.
- Navigating directly to `/merchant...` must remain in the normal `user` role because merchant workspaces belong to the same user permission class.
- Admin negative actions (`take_down`, `restore`, `restrict`, `warn`, `suspend`, `ban`) require a reason and explicit second confirmation.
- Restore actions are enabled only for restorable statuses; take-down actions are enabled only for currently active/verified/online-like statuses.
- Official price admin maintenance rows must include record context:
  - source,
  - historical price context,
  - exchange-rate timestamp,
  - duplicate offer check,
  - region restriction note,
  - operation log summary.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Student has only `api_order.create` | Buyer/order/account links remain; carpool, publish, merchant, payment, and probe links/queries stay absent |
| User opens any of the four account-setting deep links | The same four tabs render, exactly one tab is current, and the sidebar activates only `account-settings` |
| User opens `/my` or an unrelated `/my/**` route | Only the exact matching workspace entry activates; account settings and personal center do not light together |
| User opens any message query tab | Preserve the query, select the matching one of four page tabs, and activate only `message-center` |
| Promotion program is disabled on `/my/promotion-benefits` | Replace to `/my/reputation?notice=promotion-disabled`, show one notice, remove the query, and keep `reputation-rights` active |
| User opens report/appeal or feedback list/detail | Preserve the detail location, select the matching support tab, and activate only `support-center` |
| Dirty settings form switches between routes backed by the same page component | `onBeforeRouteUpdate` prompts before navigation; cancel keeps the route and draft, confirm discards and continues |
| Server data refetches while a settings draft is dirty | Preserve the local draft and saved snapshot until the user saves or explicitly leaves |
| Linux.do seller has zero owned resources | All applicable publish/merchant/probe entries remain visible, including probe first-create |
| Route has `meta.capability` absent from profile | Redirect to `/forbidden`; do not start its owner query |
| Profile has `admin.access` | `AppShell` shows one management-console entry; `/admin/**` switches to `AdminShell` |
| Profile lacks `admin.access` even if local state says admin | Administration route and API query remain blocked |
| No stored collapse preference and viewport is below 1024px | Desktop shell initializes collapsed |
| Mobile drawer is open and user presses Escape | Drawer closes and page content remains unobscured |
| User opens `/merchant/api-orders` | Sidebar still shows personal plus merchant workspace groups |
| Negative admin action without reason | Block action and show warning |
| Negative admin action without second confirmation | Block action and show warning |
| Restore on a non-restorable row | Button disabled and action rejected |
| Official price record selected | Detail panel shows record/source/context fields listed above |

### 5. Good/Base/Bad Cases

- Good: a student sees API buying and order after-sales but no carpool, publish, payment settings, merchant order, or probe request.
- Good: profile and privacy drafts can both exist, and saving either section does not silently persist or reset the other draft.
- Good: a dirty contact form prompts from account tabs, sidebar links, avatar-menu links, browser Back, refresh, and tab close.
- Good: a first-time linux.do seller sees publish actions and the zero-resource probe onboarding without manufacturing a resource first.
- Good: a profile with `admin.access` sees one `进入管理台` entry in `AppShell`, then the complete grouped directory inside `AdminShell`.
- Good: admin official-price panel shows `来源`, `历史价格`, `汇率时间`, `重复 offer`, `地区限制`, and `操作记录`.
- Base: direct `/admin/price-leads` redirects to `/admin/official-prices` for compatibility.
- Bad: ordinary user sidebar always lists `用户管理`, `低价线索审核`, and `举报纠纷`.
- Bad: administration pages render inside the ordinary user shell.
- Bad: hiding publish actions until the account already owns a listing.
- Bad: use listing/service/order counts, pending badges, or `permissions` as a replacement for the matching business capability.
- Bad: hide a link but let its query execute, or catch a real `403` and substitute mock owner data.
- Bad: ordinary user sidebar hides merchant workspace links behind a separate `商户` role switch.
- Bad: sidebar has a manual `用户 / 管理员` role toggle.
- Bad: maintain a hard-coded list of account-setting paths in `AppShell` or use `/my` prefix matching for personal-center activation.
- Bad: render separate sidebar entries for platform announcements, promotion benefits, reports, or feedback, or use `to` as a `v-for` key when distinct navigation concepts may share a destination.
- Bad: register only `onBeforeRouteLeave` for several route records that reuse the same Vue page component; tab navigation can bypass the guard through an in-place route update.
- Bad: submit the current privacy draft from the profile save action, or overwrite a dirty form when TanStack Query refreshes its server projection.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Production build: real-mode `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Product-boundary scan for official-price and API-intent wording drift.
- Browser/DOM smoke:
  - sidebar has no manual role switch,
  - student has no seller/carpool/probe links or owner requests,
  - zero-resource linux.do profile still exposes probe/publish workspaces,
  - profile-driven `admin.access` exposes exactly one management-console entry,
  - `/admin/**` renders the independent admin layout,
  - route metadata and query `enabled` predicates use the same typed capability helper,
  - `/merchant/...` keeps the user permission sidebar,
  - persisted collapse preference overrides the viewport default,
  - mobile drawers expose dialog semantics and close with Escape,
  - price-lead detail includes evidence/context fields,
  - negative action controls show reason and second confirmation.
- Account-settings browser smoke at `1440x900`, `1024x768`, `390x844`, and `360x800`:
  - all four deep links show one active shared tab and one `account-settings` sidebar item,
  - `/my` remains outside the settings shell and activates only `personal-center`,
  - the current mobile tab scrolls into view without horizontal page overflow,
  - dirty profile/contact/privacy drafts prompt through tabs, sidebar, avatar menu, Back, and refresh,
  - capability profiles without `api_service.publish` retain contact fields but do not render API payment settings.
- Workspace aggregation smoke at the same four viewports:
  - all four message query tabs remain directly addressable and activate only `message-center`,
  - reputation and disabled-promotion redirect activate only `reputation-rights`, with the promotion notice consumed once,
  - report/appeal and feedback list/detail routes activate only `support-center`,
  - page-level tab strips have no page overflow or clipped control text.

### 7. Wrong vs Correct

#### Wrong

```ts
const navGroups = [
  userLinks,
  adminLinks, // complete admin directory mixed into the user shell
]
```

#### Correct

```ts
const userShellGroups = [browseGroup, buyerTransactionGroup, accountGroup]
if (hasCapability(profile, CAPABILITY.apiServicePublish)) userShellGroups.push(merchantGroup)
if (hasCapability(profile, CAPABILITY.apiProbeManage)) userShellGroups.push(probeEntry)
if (hasCapability(profile, CAPABILITY.adminAccess)) userShellGroups.push(managementConsoleEntry)

const layout = route.meta.standalone
  ? null
  : route.path.startsWith('/admin')
    ? AdminShell
    : AppShell
```

#### Wrong

```ts
const accountSettingsPaths = ['/my/profile', '/my/contacts', '/my/account']
onBeforeRouteLeave(confirmNavigation)
```

#### Correct

```ts
const accountSettingsMeta = { auth: 'user', workspaceNavKey: 'account-settings' } as const

onBeforeRouteLeave(confirmNavigation)
onBeforeRouteUpdate(confirmNavigation)
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
```

## Scenario: API Probe Connections And Model Tester State

### 1. Scope / Trigger

- Trigger: frontend types, adapters, queries, routes, forms, or tests for seller probe connections,
  service binding, public health, order credential import, or `/tools/api-model-tester`.
- Backend/OpenAPI fields remain the authority. Handwritten UI types must mirror generated DTO
  names and must not restore deleted service-level probe fields.

### 2. Signatures

```ts
type ApiModelTesterCredentialSource =
  | { kind: 'manual'; baseUrl: string; apiKey: string; acknowledgeInsecureHttp: boolean }
  | { kind: 'order'; orderId: string; acknowledgeInsecureHttp: boolean }

type ApiModelTesterRowState = {
  state: 'pending' | 'completed' | 'cancelled' | 'failed'
  result?: ApiModelTesterModelResult
  message?: string
}

type ApiProbeConnectionPreflight = {
  errorCode: string | null
  availableModels: string[]
  probeModel: string | null
  probeProtocol: 'openai_responses_v1' | 'openai_chat_completions_v1' | null
  probeEnvironment: 'us-west-v1'
  dailyBaseCostUpperBoundUsd: string | null
  priceUnavailable: boolean
  preflightToken: string | null
}
```

```text
/my/api-probe-connections
/tools/api-model-tester
/admin/api-health
```

### 3. Contracts

- `/my/api-probe-connections` owns reusable seller connection management. Connection forms send
  name, Base URL, optional write-only credential, exact probe model, enabled state, and HTTP
  acknowledgement. Measurement-changing writes first use the create/existing preflight endpoint,
  then send its one-time `preflightToken`; save must not repeat the paid real-model verification.
- Preflight model options are only the current `/models` IDs. The form defaults to exact
  `gpt-5.6-luna` when the backend selected it, shows the fixed Responses/Chat result and the 1.00x
  daily estimate, and invalidates its token when Base URL, credential, or model changes.
- The connection table and public market card reuse the same 24-hour health component. The compact
  surface shows 24 fixed-size hourly cells, stability, average TTFT, coverage, and runner warnings;
  the tooltip owns protocol, environment, P50/P95, retry/failure, and cost detail.
- Re-enabling a disabled connection is measurement-changing: the quick switch first performs an
  existing-credential preflight, then updates with its token. Disabling remains a direct update.
- Create, update, verify, and delete send a fresh `Idempotency-Key`. Completed replay returns the
  stored response; frontend retry must reuse the same key for the same submitted mutation and must
  not start another preflight/verify call.
- API service create/edit sends `probeConnectionId`; it never duplicates Base URL or key in the
  service form. The selector shows only the current seller's connections and explains unavailable
  binding state without exposing credentials.
- Public market types contain only `healthSummary`. Owner connection ID/name/readiness fields must
  not leak into public DTO adapters or cards.
- `/admin/api-health` reads exact model/protocol/environment calibration facts, allows X/Y preview,
  and enables publication only when the backend reports at least seven complete UTC days and five
  independent connections. The page never derives or auto-publishes thresholds from percentiles.
- Model tester `ApiModelTesterCredentialSource` is a strict manual/order union. Both variants carry
  `acknowledgeInsecureHttp`; manual carries Base URL and Key, order carries only order ID.
- The HTTP warning is conditional on the current manual or selected-order Base URL. It starts
  unchecked, resets when the target changes, and blocks discovery until checked. The acknowledgement
  travels with discover and every subsequent model-test request but is not persisted.
- Discovery results, selected models, per-row protocol results, cancellation controllers, and the
  manual key are component-memory state. Source changes and unmount cancel work and clear all
  transient data. Never use route query, local/session storage, analytics, or a global store for keys.
- The UI renders every unique `/models` ID exactly as returned. It does not title-case or map IDs to
  catalog display names. Single, selected, and all-model tests operate only on the current discovery
  list; at most three model requests run concurrently and each row has stable Responses/Chat columns.
- Order import uses only backend-authorized order-source items. The frontend never receives or
  reconstructs the delivered key. Opening from an order detail may preselect an order ID only when
  that ID exists in the authorized source list.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Manual Base URL or Key missing | Disable/get-model action and retain no test state |
| Current target is HTTP and acknowledgement is false | Show warning and block discover/test |
| Probe preflight did not return a usable one-time token | Keep connection save disabled |
| Probe Base URL, credential, or model changes after preflight | Invalidate token and require preflight again |
| Disabled probe connection is switched on | Preflight stored credential, then update with returned token |
| Health runner is disabled or stale | Show the backend `runner_disabled`/`stale` explanation, not silent grey |
| Fewer than three current-version health samples | Show sample accumulation and grey cells |
| Calibration readiness is false | Disable rule publication while retaining observed percentiles |
| Source Base URL/order changes | Cancel active work, clear discovery/results, reset HTTP acknowledgement |
| `/models` returns zero IDs | Show an empty discovered state; do not invent catalog models |
| One protocol fails while the other succeeds | Render two independent cells for the same model |
| User cancels an all-model run | Abort active requests and mark unfinished rows cancelled |
| User leaves the page | Abort requests and clear the manual Key from memory |

### 5. Good / Base / Bad Cases

- Good: seller selects one existing verified connection on several publish forms; no form asks for
  the endpoint or key again and public cards expose only the shared health result.
- Good: seller preflights `gpt-5.6-luna`, receives a one-time token, saves once, and the connection
  plus every bound service display the same 24-hour health strip without duplicate model calls.
- Base: the runner is disabled. The enabled connection displays a clear operational warning while
  retaining its configuration and old immutable samples.
- Good: buyer selects an HTTP order, checks the warning, discovers `gpt-4.1-mini`, then runs either
  one row or all rows with separate Responses and Chat results.
- Base: an HTTPS source keeps acknowledgement false and performs the same discovery/test workflow
  without showing an HTTP warning.
- Base: a source change during an all-model run aborts active requests and returns the page to an
  empty discovery state.
- Bad: storing the manual key, allowing free-form probe/test model IDs, title-casing model IDs,
  saving without a matching preflight token, dynamically ranking health colors, or showing seller
  connection IDs/Base URLs on public service cards.

### 6. Tests Required

- Adapter request-body tests for both credential-source variants, trimming, HTTP acknowledgement,
  cancellation signal, CSRF-backed mutation, and absence of keys in URLs.
- Probe adapter/component tests for preflight token transport, token invalidation, default Luna,
  re-enable preflight, exact model IDs, 24 fixed cells, coverage, runner warnings, Chat fallback,
  cost unknown state, and no public connection internals.
- Calibration tests for dimension queries, invalid X/Y, not-ready publication, preview, published
  immutable rule history, and generated OpenAPI type parity.
- Component/source tests for conditional HTTP warning, target-change reset, authorized order
  preselection, discovery-only model selection, three-worker concurrency, cancellation, and unmount
  cleanup.
- Projection scans that reject `modelNameSnapshot`, public probe connection internals, challenge
  copy, local/session storage, and tester analytics.
- Run OpenAPI generation/check, full Vitest, Nuxt typecheck, and production build.

### 7. Wrong vs Correct

```ts
// Wrong: persistent secret and a model name rewritten for display.
localStorage.setItem('testerKey', apiKey)
const visibleModel = titleCase(modelId)

// Correct: component-memory secret and provider-returned ID.
const manual = reactive({ baseUrl: '', apiKey: '' })
const visibleModel = modelId
```

```ts
// Wrong: order import reconstructs credentials in the browser.
const source = { kind: 'manual', baseUrl: order.baseUrl, apiKey: order.apiKey }

// Correct: submit the authorized order reference only.
const source = { kind: 'order', orderId: order.orderId, acknowledgeInsecureHttp }
```
