# Reputation Presentation

Date: 2026-07-24
Executor: Codex
Updated: 2026-08-15

## Scenario: Unknown Reputation Values In Real Backend Mode

### 1. Scope / Trigger

- Trigger: frontend adapters, types, public profiles, market cards, application queues, or detail panels that display reputation, completion, review, cancellation, or dispute facts.
- Mock mode may keep explicit demonstration facts. Real backend mode must render only facts returned by the backend.

### 2. Signatures

```ts
type ReputationFact = number | null

type CounterpartyStats = {
  trustLevel: number | null
  completed30d: number | null // compatibility name; UI labels it "近期完成"
  buyerResponsibleCancellations: number | null
  ownerResponsibleCancellations: number | null
  unresolvedDisputes: number | null
}
```

Public profile reputation fields follow the same nullable contract. A future unified reputation summary must preserve explicit availability/confidence instead of converting missing data to zero.

### 3. Contracts

- Real adapters map unavailable reputation facts to `null`; they must not inject fixed trust levels, completion counts, review counts, or dispute counts.
- UI renders `null` as `暂无数据`. It must not render `0`, a low-trust color, a low tier, or a warning based only on missing data.
- Numeric zero is rendered as zero only when the backend response explicitly contains zero.
- Compatibility fields named `completed30d` may remain until a separate breaking-contract migration. User-facing copy uses neutral `近期完成` and must not claim a 30-day window when the backend aggregates 90 days.
- Public profile privacy flags may hide a fact, but hiding and zero are different states.
- Real backend failures remain visible through the query error path. They must not fall back to mock reputation data.
- Trust/risk presentation is informational and must not claim platform guarantee, official endorsement, or transaction safety.
- Compact market-card summaries collapse the active + insufficient-tier + low-confidence combination to `状态正常 · 交易样本较少`; higher-evidence tiers and all caution/restricted states keep their authoritative tier, confidence, and warning presentation.
- API order detail keeps the transaction flow primary. It renders the counterparty identity with only the authoritative completed-order count and role completion rate; full tier, confidence, cancellation, dispute, rating, and restriction details remain on the public profile or reputation surfaces.
- API order detail renders participant names and avatars from the frozen order participant facts. It must not load the current API service to reconstruct identity or profile navigation.
- API order contact cards render only the frozen order contact snapshots. A later public-profile link requires an explicit immutable participant ID plus privacy mode in the order contract; store aliases remain non-linkable and must not expose the underlying username.

### 4. Validation & Error Matrix

| Input/state | UI result |
| --- | --- |
| `null` reputation fact | `暂无数据` or omitted optional section |
| Backend-provided `0` | Render `0` without warning styling |
| Positive count | Render the returned number with the neutral fact label |
| Missing role completion rate | Render `暂无数据`; do not derive or invent a percentage in the UI |
| Active state + insufficient tier + low confidence in a compact summary | Render `状态正常 · 交易样本较少` instead of three repetitive labels |
| Current service identity differs from the order snapshot | Keep the frozen order participant name and contact facts; do not replace them |
| Real adapter request fails | Visible error state; no mock/fixed replacement |
| Store-alias merchant | Preserve existing identity privacy while rendering public reputation |

### 5. Good/Base/Bad Cases

- Good: a new seller has `trustLevel=null`; the badge says `信任等级暂无数据`.
- Base: the backend proves `unresolvedDisputes=0`; the profile renders zero.
- Bad: map missing fields to `trustLevel: 2`, `completed30d: 0`, or show a red low-trust badge for `null`.
- Bad: label a 90-day backend aggregate as `近 30 天完成`.

### 6. Tests Required

- Adapter tests must assert unavailable real-backend facts map to `null`.
- Component/source tests must assert `暂无数据` behavior and neutral `近期完成` wording.
- Scan real adapters for fixed trust/completion/review/dispute literals.
- Run the complete Vitest suite, Nuxt typecheck, and a production build with `NUXT_PUBLIC_API_MODE=real`, `NUXT_PUBLIC_API_BASE_URL`, and `NUXT_API_BASE_URL`.

### 7. Wrong vs Correct

#### Wrong

```ts
return {
  trustLevel: response.trustLevel ?? 1,
  completed30d: response.completed30d ?? 0,
}
```

#### Correct

```ts
return {
  trustLevel: response.trustLevel ?? null,
  completed30d: response.completed30d ?? null,
}
```

```vue
<span>{{ stats.completed30d === null ? '暂无数据' : stats.completed30d }}</span>
```

## Scenario: Bidirectional Sealed Review Presentation

### 1. Scope / Trigger

- Trigger: review-center, carpool membership detail, API order detail, public profile, adapter, query, or mutation work that displays or submits transaction reviews.
- Real backend mode is authoritative for eligibility, preset tags, direction, visibility, deadline, and editability.

### 2. Signatures

```ts
type ReviewCenterRow = {
  transactionType: 'carpool_membership' | 'api_order'
  direction: 'pending' | 'sent' | 'received'
  reviewerRole: 'buyer' | 'seller'
  revieweeRole: 'buyer' | 'seller'
  status: 'reviewable' | 'expired' | 'sealed' | 'published' | 'removed'
  visibility: 'none' | 'sealed' | 'published' | 'removed'
  allowedTags: Array<{ code: string; label: string; polarity: 'positive' | 'negative' | 'neutral' }>
  canCreate: boolean
  canEdit: boolean
  commercialOutcome: 'normal_fulfillment' | 'full_refund' | 'partial_refund' | 'continued_fulfillment' | ''
  reviewPaused: boolean
  rating: number | null
  tags: string[]
  note: string | null
}

type SubmitReviewPayload = {
  transactionType: ReviewCenterRow['transactionType']
  transactionId: string
  operation: 'create' | 'edit'
  rating: number
  tags: string[]
  note: string
}
```

### 3. Contracts

- `GET /api/v1/me/reviews` supplies each row's scenario-filtered `allowedTags`. The frontend submits tag codes and renders labels; it must not offer free-form tags or duplicate scenario rules in real mode.
- Tabs/filters distinguish pending, sent, and received rows without deriving visibility from a missing rating.
- Before publication, the UI must not tell users whether the counterparty submitted. The API omits received sealed rows and the frontend must not derive a signal from other row differences.
- A sent sealed row may show the author's own content and edit action only when `canEdit=true`. Create uses generic `POST`; edit uses generic `PUT`.
- Published rows show content and cannot expose an edit action. Removed rows do not render hidden content or removal internals.
- API-order rows render the backend `commercialOutcome` and `reviewPaused` facts. `full_refund` and `partial_refund` remain reviewable outcomes; the frontend must not infer eligibility from order status or rename them to `full_refund_confirmed` / `partial_refund_confirmed`.
- While `reviewPaused=true`, pending and sent-sealed rows show a neutral paused state and expose neither create nor edit actions. The UI does not run a local deadline or imply that a sealed counterparty review exists. Published/frozen rows remain read-only.
- New reviews start with `rating=null`. Five visible stars use radio semantics, mouse/keyboard input, and show a number in read-only mode. Submit requires a rating and either one tag or a non-empty note.
- A shared `ReviewDialog` owns create/edit/read-only states and keeps form state after a failed mutation. Detail pages drive it with `?review=open`; the review center includes transaction type, transaction ID, and direction so refresh/back and duplicate transaction rows resolve exactly.
- On mobile, the Dialog stays within the dynamic viewport and scrolls internally. Closing deletes only review-owned query keys and preserves unrelated query parameters.
- Public profiles render only backend-returned verified reviews. They must not reconstruct received reviews from local orders or combine real results with mock review arrays.
- Public review presentation computes raw average, count, and 1-5 distribution from the same visible list, shows each review's stars plus number, and labels evidence `来自平台内已完成交易`. Public reputation summaries must not present weighted/Bayesian rating as a second ordinary average.
- Real backend failures remain visible. The adapter must not catch them and return mock review rows or locally invented preset tags.

### 4. Validation & Error Matrix

| Backend/UI state | Required presentation |
| --- | --- |
| `direction=pending`, `canCreate=true` | Create-review action |
| `direction=sent`, `visibility=sealed`, `canEdit=true` | Own content plus edit action |
| `reviewPaused=true` | Paused state; no create/edit action and no counterparty-submission signal |
| `commercialOutcome=full_refund|partial_refund` after closure | Render the returned outcome and backend deadline; do not hide the review action solely because order status is not completed |
| Counterparty submitted before publication | No received row, field, or differentiated message |
| `visibility=published` | Read-only rating, tags, note, and publication time |
| `status=expired` with no review | No submit action |
| `visibility=removed` | Removed state without content |
| `rating=null` | Five unselected stars and disabled submit |
| Rating selected, tags and trimmed note empty | Disabled submit and backend `422` if bypassed |
| Mutation fails | Keep rating, tags, and note in the open Dialog |
| Real request failure | Error state and retry; no mock fallback |

### 5. Good/Base/Bad Cases

- Good: an API seller opens `?review=open`, chooses four stars and the `付款及时` label, and submits `quick_payment` without leaving the detail page.
- Good: a buyer sees the same conditional publication rule before submission regardless of whether the seller already submitted.
- Good: after a confirmed refund, the page renders the server-provided commercial result and new deadline; during an active dispute it shows paused without exposing sealed metadata.
- Base: the author edits a sealed review, then loses the edit action immediately after both reviews publish.
- Bad: default to five stars, use a score dropdown, show `counterpartySubmitted`, select a review row only by transaction ID when multiple directions exist, or show mock success after a real backend error.

### 6. Tests Required

- Adapter tests must assert structured `allowedTags`, exact commercial-outcome enum values, and `reviewPaused` survive mapping while `counterpartySubmitted` remains absent.
- Mutation tests must assert create uses `POST`, edit uses `PUT`, and both send CSRF plus idempotency through `backendClient`.
- Component tests must cover unselected stars, click/arrow/Home/End behavior, ARIA, tag-or-note validation, failed-mutation state retention, sent sealed edit, published, expired, and removed states.
- Route tests must cover `?review=open`, exact direction selection, refresh/back behavior, and preserving unrelated query parameters.
- Public-profile tests must cover raw average/count/distribution, per-review stars, and completed-transaction wording.
- Run full Vitest, Vue typecheck, real-mode Vite build, and browser checks at `1440x900` plus a mobile width.

### 7. Wrong vs Correct

#### Wrong

```ts
const dialogOpen = ref(false)
const tags = globalPresetTags
```

#### Correct

```ts
const dialogOpen = computed(() => route.query.review === 'open')
const tags = row.allowedTags
const rating = ref<number | null>(row.canEdit ? row.rating : null)
const canMutate = computed(() => !row.reviewPaused && (row.canCreate || row.canEdit))
```

## Scenario: Source-Author Verification Presentation

### 1. Scope / Trigger

- Trigger: carpool/API adapters, market cards, detail pages, owner lists, publish previews, or ranking code that displays or compares source-author verification.
- The backend resource summary is authoritative. A source URL and an authorship decision are separate facts.

### 2. Signatures

```ts
type SourceAuthorVerificationStatus =
  | 'not_submitted'
  | 'pending'
  | 'verified'
  | 'mismatch'
  | 'expired'

type SourceAuthorResourceSummary = {
  status: SourceAuthorVerificationStatus
  verifiedAt?: string
  expiresAt?: string
}

isSourceAuthorVerified(summary): boolean
sourceAuthorVerificationLabel(summary): string
sourceAuthorVerificationRank(summary): number
```

Carpool listings and API services expose the summary as
`sourceAuthorVerification`.

### 3. Contracts

- Only `status === 'verified'` may render `原帖作者已验证` or a verified visual treatment on a surface that still supports source-author verification.
- Subscription carpool publish, market, detail, application, and owner-management surfaces omit every source-author state. API market cards also omit source-author verification, while API service detail renders only `verified` and `mismatch`; `not_submitted`, `pending`, and `expired` stay hidden there.
- The carpool display removal does not remove the historical backend field, admin audit controls, API-service source-author presentation, or shared normalization helpers.
- Real adapters copy the backend summary. They must not derive status from `sourceUrl`, source-version fields, product metadata, or mock defaults.
- Opening the source topic depends only on a real source URL. A verified summary does not create a link, and a link does not create a verified badge.
- Carpool recommendation, sorting, tradability, and availability helpers must not compare source-author status. The shared status rank remains only for compatible source-author features outside the carpool decision path.
- Mock rows declare their source-author status explicitly so tests and previews do not teach production code to infer it.

### 4. Validation & Error Matrix

| Input/state | Required presentation |
| --- | --- |
| Missing summary or `not_submitted` | Carpool and API market/detail hide it |
| `pending` | Carpool and API market/detail hide it |
| `verified` | Carpool and API market hide it; API detail may show `原帖作者已验证` |
| `mismatch` | Carpool and API market hide it; API detail may show `原帖作者不一致` |
| `expired` | Carpool and API market/detail hide it |
| Missing source URL | No source-topic link, regardless of verification status |
| Real adapter request fails | Visible request error; no inferred or mock verification |

### 5. Good/Base/Bad Cases

- Good: an API service with `pending` status does not add a low-value source-author row to the market card or detail purchase panel.
- Good: an API service with `mismatch` status still surfaces the destructive warning on detail.
- Good: a historical carpool response may still carry an expired decision, but no carpool surface renders or ranks by it.
- Base: an older or absent optional summary still normalizes to `not_submitted`; API display policy may hide it.
- Bad: use `Boolean(sourceUrl)`, `sourceUrl ? 'verified' : 'not_submitted'`, or a fixed mock status in a real adapter.

### 6. Tests Required

- Helper tests cover every label, verified-only truth, missing-summary normalization, and stable status ranking.
- Carpool and API adapter tests pass a non-empty source URL with `pending`/`mismatch` and assert the backend status survives unchanged.
- Source/component tests cover all five shared labels, prove only `verified` receives verified styling, assert the API market omission/detail allowlist, and prove carpool surfaces omit the feature.
- Carpool ranking and tradability tests must pass for a listing with no source URL and `not_submitted` verification.
- Run full Vitest, Nuxt typecheck, and a production build with `NUXT_PUBLIC_API_MODE=real`, `NUXT_PUBLIC_API_BASE_URL`, and `NUXT_API_BASE_URL`.
- Scan production frontend code for URL-to-verification boolean inference.

### 7. Wrong vs Correct

#### Wrong

```ts
sourceAuthorVerification: {
  status: service.sourceUrl ? 'verified' : 'not_submitted',
}
```

#### Correct

```ts
sourceAuthorVerification: service.sourceAuthorVerification
```

The URL controls source navigation; the resource summary controls authorship presentation.
