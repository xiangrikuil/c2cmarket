# API Service Promotion Frontend Contract

Date: 2026-08-02
Author: Codex

## Scenario: Public disclosure and administrator scheduling

### 1. Scope / Trigger

- Trigger: frontend work touching category-grid promotion injection, administrator scheduling, promotion analytics, or promotion DTO adapters.
- Buyers see a disclosed promoted card inside the matching market grid. Merchants have no V1 promotion request, purchase, status, or reporting surface.

### 2. Signatures

```text
useApiPromotions()
useAdminApiPromotions()
useAdminApiPromotionAvailability(apiServiceId, startsAt, endsAt, enabled)
useCreateApiPromotionMutation()
useStopApiPromotionMutation()
createPromotionImpressionTracker(options)
apiPromotionAvailabilityBlockReasons(availability)
```

### 3. Contracts

#### Public Market Surface

- Do not render a standalone promotion section. Inject at most one daily-ordered matching promotion as the first card in the free-quota or fixed-package grid; the limited-quota view never consumes API-service promotions.
- Fixed-package promotion selection must also satisfy the active exact-model and duration filters. If the first campaign does not match, continue through the server's daily order until one does. A matching promotion may be supplied only by the promotion response and need not be present in the natural API-service page.
- Remove the promoted service from all subsequent natural rows without reordering the remaining rows. Preserve their natural rank and recommendation score; promotion does not earn the natural `综合推荐` badge.
- Promoted and ordinary variants reuse `ApiFreeServiceCard` or `ApiPackageCard`, the grid width, the same field hierarchy, and an approximately 410px stable height. Only the promoted variant renders the permanent text label `推广` and emphasized border. Do not render a separate full-width disclaimer row; ordinary-card accessibility output must contain no promotion label.
- Both variants show the real account pool when present, merchant-declared maximum concurrency, refund commitment choice, and authority-derived merchant badges. Historical missing pool/concurrency values use explicit missing labels and are never inferred.
- If no campaign matches the active grid and filters, render the natural rows exactly as returned and reserve no placeholder.
- The promotion query runs only in the browser, is immediately stale, and refetches on mount so the SSR page cache cannot preserve expired promotion state.
- Public adapters consume generated `PublicApiService` contracts directly. Do not use double assertions or owner/admin DTOs to bypass the public privacy boundary.

#### Administrator Scheduling

- The create dialog queries `/api/v1/admin/api-service-promotions/availability` for the selected service and proposed start/end range before enabling submission.
- It displays `configurable`, `displayable`, hard-block reasons, warnings, suppression reasons, peak occupancy, capacity, remaining capacity, and same-service overlap.
- Hard eligibility blocks, same-service overlap, and zero remaining capacity disable creation. `displayable=false` alone does not: the UI explains that the schedule can be created and will appear after public orderability recovers.
- The server always rechecks eligibility and capacity during create; preflight data is advisory and may change.
- Create and stop reasons are required and limited to 500 characters. Successful mutations invalidate both public promotion and administrator promotion queries.

#### Analytics And Environment

- An impression requires at least 50% intersection, a visible document, and one continuous second. Leaving the threshold, hiding the page, unmounting, or removing the campaign cancels the timer.
- Each campaign emits at most one impression per API market page lifecycle. The campaign ID may be used only in the in-memory de-duplication set and is never sent to analytics.
- Mouse, touch, and keyboard activation of the card's real internal link emits a best-effort click before normal navigation. Clicking non-interactive card background must not emit a click event. Analytics failure cannot block display or navigation.
- Promotion events contain only placement, display position, provider category, billing mode, target type, and normalized source route. They exclude promotion/service/user IDs, merchant names, amounts, raw titles, and query strings.
- Production uses Umami Cloud for `c2cmarket.shop` without `data-host-url`. Staging remains disabled and must not contain or reuse the production Website ID.

#### Product Boundary

- Do not add promotion navigation, status, application, purchase, or reporting surfaces to merchant pages or APIs.
- Promotions do not change real price, stock, quota, merchant identity, reputation, badges, service detail, or natural result order.

### 4. Validation & Error Matrix

| Condition | UI behavior |
| --- | --- |
| No matching public promotion | Keep the active natural grid unchanged; render no empty placeholder |
| First fixed-package campaign misses active filters | Continue to the next daily-ordered fixed-package campaign |
| Matching promoted service is absent from the natural page | Inject the service/package carried by the promotion DTO, then keep natural order |
| Availability is loading | Show progress and disable create |
| Availability fails | Show a retry action and disable create |
| Hard block, same-service overlap, or remaining capacity 0 | Show all reasons and disable create |
| Warning only | Show operator-review warning; keep create available |
| Display suppressed but configurable | Explain deferred display; keep create available |
| Umami is absent, blocked, or throws | Continue display and internal navigation |
| Page becomes hidden before 1000 ms | Cancel the timer and require a new full visible interval |

### 5. Good / Base / Bad Cases

- Good: the create dialog shows peak occupancy, remaining capacity, overlap, eligibility, warnings, and suppression before enabling submission.
- Good: a temporarily sold-out but configurable service can be scheduled with an explicit deferred-display explanation.
- Good: a promoted package matching the current model/duration appears first, while the same service is removed from later natural results and the remaining ranks stay unchanged.
- Base: an empty public response leaves the natural API market in its normal position.
- Bad: allow create when availability is missing or failed and use a generic `409` as the only explanation.
- Bad: hide the `推广` label because the current operator placement is free.
- Bad: send promotion or service IDs to Umami and treat the result as billing-grade attribution.

### 6. Tests Required

- Run frontend tests, type-check, OpenAPI drift check, and a production-mode build after changing this surface. Keep focused tests for category/filter matching, out-of-page injection, de-duplication, promotion-label presence, long-disclaimer absence, real-link click activation, availability blocking rules, analytics sanitization, visibility timing, cancellation, and page-lifecycle de-duplication.

### 7. Wrong vs Correct

#### Wrong

```ts
const canCreate = Boolean(serviceId && startsAt && endsAt)
```

#### Correct

```ts
const canCreate = validLocalInput
  && availabilityQuery.isSuccess.value
  && apiPromotionAvailabilityBlockReasons(availability.value).length === 0
  && reason.trim().length > 0
```
