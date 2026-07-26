# Reputation Presentation

Date: 2026-07-24
Executor: Codex

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

### 4. Validation & Error Matrix

| Input/state | UI result |
| --- | --- |
| `null` reputation fact | `暂无数据` or omitted optional section |
| Backend-provided `0` | Render `0` without warning styling |
| Positive count | Render the returned number with the neutral fact label |
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
- Run the complete Vitest suite, Vue typecheck, and `VITE_API_MODE=real` production build.

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
  canCreate: boolean
  canEdit: boolean
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

- `GET /api/v1/me/reviews` supplies the row list and the only selectable preset tags. The frontend must not offer free-form tag input.
- Tabs/filters distinguish pending, sent, and received rows without deriving visibility from a missing rating.
- A received sealed row shows that the counterparty submitted and explains when content will appear, but renders no rating, tags, or note.
- A sent sealed row may show the author's own content and edit action only when `canEdit=true`. Create uses generic `POST`; edit uses generic `PUT`.
- Published rows show content and cannot expose an edit action. Removed rows do not render hidden content or removal internals.
- Completed carpool and API-order detail pages navigate to the review center with `transactionType` and the backend transaction ID so the matching row can open.
- Public profiles render only backend-returned verified reviews. They must not reconstruct received reviews from local orders or combine real results with mock review arrays.
- Real backend failures remain visible. The adapter must not catch them and return mock review rows or locally invented preset tags.

### 4. Validation & Error Matrix

| Backend/UI state | Required presentation |
| --- | --- |
| `direction=pending`, `canCreate=true` | Create-review action |
| `direction=sent`, `visibility=sealed`, `canEdit=true` | Own content plus edit action |
| `direction=received`, `visibility=sealed` | Sealed placeholder; no content |
| `visibility=published` | Read-only rating, tags, note, and publication time |
| `status=expired` with no review | No submit action |
| `visibility=removed` | Removed state without content |
| Real request failure | Error state and retry; no mock fallback |

### 5. Good/Base/Bad Cases

- Good: an API seller completes an order, opens the review center from order detail, selects backend preset tags, and creates a seller-to-buyer review.
- Good: a buyer sees that the seller submitted but sees no stars or text until the buyer submits or the deadline passes.
- Base: the author edits a sealed review, then loses the edit action immediately after both reviews publish.
- Bad: use `rating ?? 0`, show arbitrary tag text input, post every mutation to the legacy carpool route, or show a mock success after a real backend error.

### 6. Tests Required

- Adapter tests must assert sealed nullable content and backend preset tags survive mapping.
- Mutation tests must assert create uses `POST`, edit uses `PUT`, and both send CSRF plus idempotency through `backendClient`.
- Source/component tests must cover pending, sent sealed, received sealed, published, expired, and removed actions.
- Run full Vitest, Vue typecheck, real-mode Vite build, and browser checks at `1440x900` plus a mobile width.

### 7. Wrong vs Correct

#### Wrong

```ts
const rating = row.rating ?? 0
const tags = row.tags.length ? row.tags : ['沟通顺畅']
```

#### Correct

```ts
const rating = row.rating ?? null
const presetTags = response.presetTags
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

- Only `status === 'verified'` may render `原帖作者已验证` or a verified visual treatment.
- Carpool surfaces continue to render every explicit state. API market cards omit source-author verification, while API service detail renders only `verified` and `mismatch`; `not_submitted`, `pending`, and `expired` stay hidden there.
- The display reduction does not remove the backend field, admin audit controls, source-author reputation evidence, shared labels, or ranking contracts.
- Real adapters copy the backend summary. They must not derive status from `sourceUrl`, source-version fields, product metadata, or mock defaults.
- Opening the source topic depends only on a real source URL. A verified summary does not create a link, and a link does not create a verified badge.
- Ranking or tradability helpers may compare the shared status rank, but must use the same normalization helper as display code.
- Mock rows declare their source-author status explicitly so tests and previews do not teach production code to infer it.

### 4. Validation & Error Matrix

| Input/state | Required presentation |
| --- | --- |
| Missing summary or `not_submitted` | Carpool shows `原帖作者未验证`; API market/detail hide it |
| `pending` | Carpool shows `原帖作者待核验`; API market/detail hide it |
| `verified` | Carpool and API detail show `原帖作者已验证`; API market hides it |
| `mismatch` | Carpool and API detail show `原帖作者不一致`; API market hides it |
| `expired` | Carpool shows `原帖作者验证已过期`; API market/detail hide it |
| Missing source URL | No source-topic link, regardless of verification status |
| Real adapter request fails | Visible request error; no inferred or mock verification |

### 5. Good/Base/Bad Cases

- Good: an API service with `pending` status does not add a low-value source-author row to the market card or detail purchase panel.
- Good: an API service with `mismatch` status still surfaces the destructive warning on detail.
- Good: an expired carpool decision remains visible as expired and does not retain the verified badge.
- Base: an older or absent optional summary still normalizes to `not_submitted`; API display policy may hide it.
- Bad: use `Boolean(sourceUrl)`, `sourceUrl ? 'verified' : 'not_submitted'`, or a fixed mock status in a real adapter.

### 6. Tests Required

- Helper tests cover every label, verified-only truth, missing-summary normalization, and stable status ranking.
- Carpool and API adapter tests pass a non-empty source URL with `pending`/`mismatch` and assert the backend status survives unchanged.
- Source/component tests cover all five shared labels, prove only `verified` receives verified styling, assert the API market omission/detail allowlist, and keep carpool rendering unchanged.
- Run full Vitest, Vue typecheck, and `VITE_API_MODE=real` production build.
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
