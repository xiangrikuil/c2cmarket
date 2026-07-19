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

- Trigger: review center submissions are stored on source records, and public profile/search surfaces derive display rows from those source records.
- Public profile review lists must reflect newly submitted reviews without copying static arrays.
- Store-alias API merchant identity must not leak through dynamic review aggregation or search result subtitles.

### 2. Signatures

```ts
export type SearchResult = {
  type: '官方价格' | '车源' | '求车' | 'API 服务' | '用户' | '商户'
  title: string
  subtitle: string
  badge: string
  to: string
}

function publicReviewsForProfile(username: string): PublicReviewRecord[]
function buildPublicReviewFromCarpoolApplication(application: CarpoolApplication): PublicReviewRecord | null
function buildPublicReviewFromApiIntent(intent: ApiPurchaseIntent): PublicReviewRecord | null
```

### 3. Contracts

- Static `publicReviewRecords` are seed data only; public profile APIs must also derive reviews from completed `carpoolApplicationStore` and `apiPurchaseIntentStore`.
- Carpool application reviews can be public when the application is `completed` and has `buyerReview`; the reviewed username is `ownerUsername`.
- API purchase intent reviews can be public only when the intent is `completed`, has `review`, and `snapshot.merchantIdentityMode === 'public_profile'`.
- API purchase intent reviews for `store_alias` merchants must return `null`; public profile aggregation must not reveal the backing `merchantUsername`.
- Mock `searchMarket()` must include `publicMerchantProfiles` as `type: '商户'` results in addition to user profiles.
- Real `searchMarket()` must call `searchBackend.ts` and map backend rows to the same `SearchResult` union without silently falling back to mock data.
- Store-alias API service search may return an `API 服务` result with the public merchant display name, but must not return a separate `商户` result or `/u/:merchantUsername` link that reveals the hidden owner.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Completed carpool application with buyer review | Public profile for the owner includes a verified review row |
| Completed API intent with review and public profile merchant | Public profile for that merchant includes a verified review row |
| Completed API intent with review and store alias merchant | No public review row is generated |
| Search keyword matches merchant username/display name/identity | Search returns a `商户` result |
| Search keyword matches store alias display name through API service | Search may return the API service, but must not expose hidden merchant username or a public user link |

### 5. Good/Base/Bad Cases

- Good: `qingning` API order review appears on `/u/qingning` because the merchant identity mode is `public_profile`.
- Base: seed review rows continue to render alongside derived rows.
- Bad: a `store_alias` order for `小葵 API` creates a public review row under `/u/orbit`.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Production build: real-mode `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Source scan for product-boundary wording.
- Source scan for store-alias leakage patterns such as direct `/u/${service.merchantUsername}` in public pages/components.
- Browser or SPA route smoke for `/search?q=<merchant>` and a public profile with derived reviews when browser tooling is available.

### 7. Wrong vs Correct

#### Wrong

```ts
if (intent.status === 'completed' && intent.review) {
  return { username: intent.snapshot.merchantUsername, ... }
}
```

#### Correct

```ts
if (intent.status !== 'completed' || !intent.review) return null
if (intent.snapshot.merchantIdentityMode === 'store_alias') return null
return { username: intent.snapshot.merchantUsername, ... }
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

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Payment instructions include `paymentQrCodeDataUrl` | Buyer detail renders the QR image inside the payment card. |
| Buyer clicks `我已付款` | Mutation calls `POST /api/v1/me/api-orders/{id}/submit-payment`, invalidates buyer/merchant order queries, and shows waiting-for-seller state. |
| Seller confirms receipt | Mutation calls `POST /api/v1/owner/api-orders/{id}/confirm-payment`, then the seller can open the delivery form. |
| Seller submits `api_key_endpoint` | Payload includes `deliveryKind`, `apiBaseUrl`, `apiKey`, and optional `instructions`; the detail response shows the credential. |
| Seller submits `login_account` | Payload includes `deliveryKind`, `panelLoginUrl`, `username`, `password`, and optional `instructions`; the detail response shows the credential. |
| Order list receives a delivered order | It may show status and submitted time, but must not render raw `apiKey` or `password`. |

### 5. Good/Base/Bad Cases

- Good: buyer detail in `pending_payment` calls `readApiOrderPaymentInstructions()` and renders the frozen WeChat QR code plus merchant contact snapshot.
- Good: seller detail in `paid_confirmed` submits `{ deliveryKind: 'api_key_endpoint', apiBaseUrl, apiKey, instructions }`, receives `deliveryCredential`, and the form becomes read-only.
- Base: a delivered order list row shows `已交付` and `deliverySubmittedAt`, but no raw `apiKey` or `password` text.
- Bad: a page derives API order fulfillment from `ApiPurchaseIntent.status`, or renders a generic `deliveryNote` textarea that can be edited after delivery.
- Bad: a list, notification, search result, report row, or admin summary renders `order.deliveryCredential.apiKey` or `order.deliveryCredential.password`.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Real-mode build: `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Source scan for forbidden product wording outside the spec allowlist.
- Adapter/review checks must verify `paymentQrCodeDataUrl` is mapped both in order-settings submit payloads and payment-instructions responses.

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

---

## Scenario: Permission-Driven User/Admin Shells And Progressive Navigation

### 1. Scope / Trigger

- Trigger: frontend mock exposes user, merchant-workspace, and admin routes before real auth and permissions exist.
- The UI shell must not make ordinary users feel that full admin tooling is part of their normal workspace.
- User and merchant are the same account permission class: a normal user can be a buyer, carpool owner, and API service merchant at the same time.
- Sidebar visibility must be derived from the current user profile returned by the API facade, not a manual role switch in the shell.
- Admin moderation rows must provide enough context for local mock review before backend integration.

### 2. Signatures

```ts
type UserProfile = {
  permissions: Array<'admin'>
}

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
```

### 3. Contracts

- `App.vue` selects exactly one layout: standalone routes render directly, `/admin/**` uses `AdminShell`, and all other authenticated pages use `AppShell`.
- `AppShell` always shows browse, transaction, publish, and account entry points for normal users.
- Merchant workspace links are normal user-permission links, not a separate account role.
- Owner/merchant management links are progressive: show the management group only when the account owns a carpool/API service or has a real owner/merchant pending count. Publish entry points remain visible without ownership records.
- When `getMyProfile()` / `useMyProfileQuery()` returns `permissions` containing `admin`, `AppShell` exposes one `进入管理台` link only; it must not append the administration directory.
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
| Profile has no `admin` permission | User routes remain in `AppShell`; the management-console entry is hidden |
| Profile has `admin` permission | `AppShell` shows one management-console entry; `/admin/**` switches to `AdminShell` |
| Profile owns no listing and has no owner pending work | Management group stays hidden; publish links remain visible |
| Profile owns a carpool or API service | Relevant owner/merchant links appear in the management group |
| No stored collapse preference and viewport is below 1024px | Desktop shell initializes collapsed |
| Mobile drawer is open and user presses Escape | Drawer closes and page content remains unobscured |
| User opens `/merchant/api-orders` | Sidebar still shows personal plus merchant workspace groups |
| Negative admin action without reason | Block action and show warning |
| Negative admin action without second confirmation | Block action and show warning |
| Restore on a non-restorable row | Button disabled and action rejected |
| Official price record selected | Detail panel shows record/source/context fields listed above |

### 5. Good/Base/Bad Cases

- Good: profile with `permissions: ['admin']` sees one `进入管理台` entry in `AppShell`, then the complete grouped directory inside `AdminShell`.
- Good: a first-time profile sees publish actions but no permanently empty management group.
- Good: a carpool owner or API merchant sees only the relevant progressive management links.
- Good: admin official-price panel shows `来源`, `历史价格`, `汇率时间`, `重复 offer`, `地区限制`, and `操作记录`.
- Base: direct `/admin/price-leads` redirects to `/admin/official-prices` for compatibility.
- Bad: ordinary user sidebar always lists `用户管理`, `低价线索审核`, and `举报纠纷`.
- Bad: administration pages render inside the ordinary user shell.
- Bad: hiding publish actions until the account already owns a listing.
- Bad: ordinary user sidebar hides merchant workspace links behind a separate `商户` role switch.
- Bad: sidebar has a manual `用户 / 管理员` role toggle.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Production build: real-mode `pnpm --dir frontend build` with the required Nuxt runtime API variables.
- Product-boundary scan for official-price and API-intent wording drift.
- Browser/DOM smoke:
  - sidebar has no manual role switch,
  - profile-driven admin permission exposes exactly one management-console entry,
  - `/admin/**` renders the independent admin layout,
  - publish actions remain visible while owner/merchant groups are progressive,
  - `/merchant/...` keeps the user permission sidebar,
  - persisted collapse preference overrides the viewport default,
  - mobile drawers expose dialog semantics and close with Escape,
  - price-lead detail includes evidence/context fields,
  - negative action controls show reason and second confirmation.

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
const userShellGroups = [browseGroup, transactionGroup, publishGroup, accountGroup]
if (hasOwnerObjectsOrPendingWork.value) userShellGroups.splice(3, 0, managementGroup)
if (myProfile.value?.permissions.includes('admin')) userShellGroups.push(managementConsoleEntry)

const layout = route.meta.standalone
  ? null
  : route.path.startsWith('/admin')
    ? AdminShell
    : AppShell
```
