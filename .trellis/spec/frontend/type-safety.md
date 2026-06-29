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
- Production build: `pnpm --dir frontend exec vite build`.
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
- Production build: `pnpm --dir frontend exec vite build`.
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

- Trigger: API service pages create frontend-local purchase-intent records, not platform orders, in-platform payments, or credential delivery flows.
- UI copy must not imply that C2CMarket processes payment, stores API keys, stores panel accounts, or automatically delivers credentials.

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
- The platform must never show, request, paste, upload, store, or automatically deliver API keys, endpoint secrets, panel passwords, tokens, sessions, recovery codes, or account credentials.
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
- Production build: `pnpm --dir frontend exec vite build`.
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

## Scenario: Backend-Driven Navigation Permissions And Moderation Context

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
```

### 3. Contracts

- `AppShell` always shows browse, publish, personal workspace, and merchant workspace links for normal users.
- Merchant workspace links are normal user-permission links, not a separate account role.
- Admin navigation is appended only when `getMyProfile()` / `useMyProfileQuery()` returns `permissions` containing `admin`.
- The sidebar must not expose a manual `用户 / 管理员` or `用户 / 商户 / 管理` switch.
- Navigating directly to `/merchant...` must remain in the normal `user` role because merchant workspaces belong to the same user permission class.
- Admin negative actions (`take_down`, `restore`, `restrict`, `warn`, `suspend`, `ban`) require a reason and explicit second confirmation.
- Restore actions are enabled only for restorable statuses; take-down actions are enabled only for currently active/verified/online-like statuses.
- Official price/lead admin rows must include review context:
  - evidence preview,
  - source,
  - historical price context,
  - exchange-rate timestamp,
  - duplicate lead check,
  - region restriction note,
  - submitter history,
  - operation log summary.

### 4. Validation & Error Matrix

| Condition | Expected behavior |
| --- | --- |
| Profile has no `admin` permission | User and merchant workspace navigation links are visible; admin navigation links are hidden |
| Profile has `admin` permission | Admin navigation appears as an additional `管理` module below normal user/merchant modules |
| User opens `/merchant/api-orders` | Sidebar still shows personal plus merchant workspace groups |
| Negative admin action without reason | Block action and show warning |
| Negative admin action without second confirmation | Block action and show warning |
| Restore on a non-restorable row | Button disabled and action rejected |
| Official price lead selected | Detail panel shows evidence/context fields listed above |

### 5. Good/Base/Bad Cases

- Good: profile with `permissions: ['admin']` shows personal workspace, merchant workspace, and an appended management module.
- Good: profile without `admin` permission shows personal workspace and merchant workspace only.
- Good: admin price-lead panel shows `证据预览`, `历史价格`, `汇率时间`, `重复线索`, `地区限制`, `提交者历史`, and `操作记录`.
- Base: direct `/admin/price-leads` remains reachable in frontend mock for review.
- Bad: ordinary user sidebar always lists `用户管理`, `低价线索审核`, and `举报纠纷`.
- Bad: ordinary user sidebar hides merchant workspace links behind a separate `商户` role switch.
- Bad: sidebar has a manual `用户 / 管理员` role toggle.

### 6. Tests Required

- Type check: `pnpm --dir frontend exec vue-tsc -b --pretty false`.
- Production build: `pnpm --dir frontend exec vite build`.
- Product-boundary scan for official-price and API-intent wording drift.
- Browser/DOM smoke:
  - sidebar has no manual role switch,
  - profile-driven admin permission appends the management module,
  - user sidebar shows both personal workspace and merchant workspace links,
  - `/merchant/...` keeps the user permission sidebar,
  - price-lead detail includes evidence/context fields,
  - negative action controls show reason and second confirmation.

### 7. Wrong vs Correct

#### Wrong

```ts
const navGroups = [
  userLinks,
  adminLinks, // unconditional
]
```

#### Correct

```ts
return myProfile.value?.permissions.includes('admin')
  ? [browseGroup, publishGroup, userGroup, merchantGroup, adminGroup]
  : [browseGroup, publishGroup, userGroup, merchantGroup]
```
