# Referral Rewards And Promotion Benefits Frontend Contract

Date: 2026-08-02
Author: Codex

## Scenario: Invitation benefits, administrator controls, and reward placement

### 1. Scope / Trigger

- Trigger: frontend work touching invitation capture, `/my/promotion-benefits`, `/admin/growth-promotions`, promotion-coupon queries/mutations, reward placement, poster generation, or promotion-benefit analytics.
- This UI presents API-service promotion benefits. It never describes coupons as cash, transferable value, reputation, fixed ranking, guaranteed traffic, or platform endorsement.

### 2. Signatures

```text
canonicalReferralCode(value)
captureReferralCode(value, storage?, now?)
getReferralCapture(storage?, now?)
clearReferralCapture(storage?)

usePromotionRewardPublicConfig()
useMyReferralSummary(enabled?)
useMyPromotionCoupons(query)
useApplyPromotionCouponMutation()
useAdminPromotionRewardCampaign()
useAdminReferrals(query)
useAdminPromotionCoupons(query)
useUpdatePromotionRewardCampaignMutation()
useGrantAdminPromotionCouponMutation()
useRevokeAdminReferralMutation()
useRevokeAdminPromotionCouponMutation()

promotionsForBillingMode(promotions, fixedPackage, matchesCurrentFilters?)
placePromotions(naturalRows, promotions, resolvePromotedRow, serviceId)
```

### 3. Contracts

#### Capture And OAuth

- The browser canonicalizes `?invite=` into the same eight-character alphabet as the backend and stores the first valid code in `localStorage` for seven days. A later link cannot overwrite an unexpired first-touch code.
- Malformed/expired storage is cleared. Storage absence or exceptions are silent; invitation attribution must not block rendering or OAuth.
- Login sends the valid captured code as `inviteCode` to the OAuth start endpoint. OAuth completion clears the capture after a registered or logged-in result.
- Invite codes never become UTM/referrer attribution and are never sent to Umami.

#### User Benefits Surface

- `/my/promotion-benefits` is authenticated and linked from the user shell. The page reads public config, referral summary, paginated coupon wallet, and the user's real API services through backend adapters; real mode has no mock-success fallback.
- Disabled, loading, failure, empty, pending, available, used, expired, and revoked states are explicit. Coupon status and timestamps come from the server; the browser does not derive or persist a second lifecycle.
- Invite actions provide code copy, absolute link copy, and downloadable Canvas poster with QR code. Success feedback appears only after the underlying clipboard, QR, Canvas, or download operation succeeds.
- Coupon apply uses a dialog and selects only the caller's services currently returned as publicly orderable. Submission remains a server-authorized operation and invalidates promotion-reward plus public-promotion queries.
- Rules state that promotion is rotational, does not guarantee rank/exposure, cannot be transferred or exchanged for cash, and does not change reputation or bypass review.

#### Administrator Surface

- `/admin/growth-promotions` is administrator-only and linked beside the existing growth dashboard. Tabs separate configuration, referral records, and coupons without nesting cards.
- Campaign form mirrors server ranges, converts Beijing `datetime-local` values to ISO, requires a reason and confirmation, and submits the current version through `If-Match` plus an idempotency key.
- Referral and coupon tables use server pagination/filter/search, preserve previous page data during fetch, and expose administrator-only IDs/risk flags. Mobile tables keep a stable minimum width inside their scroll container.
- Grant and revoke are explicit confirmation dialogs. Revoke copy states that active reward display ends immediately and no coupon is restored. Successful mutations invalidate the complete promotion-reward query family and public promotion query.

#### API Market Placement And Disclosure

- Public promotion DTOs use `kind: operator | reward`. For the active billing-mode/filter grid, select at most one of each kind; never inject promotions into the limited-quota view.
- Operator promotion remains first. Reward promotion is inserted after up to three natural rows, must differ from the operator service, and is not inserted adjacent when there are no separating natural rows.
- Remove injected service IDs from natural rows without changing the remaining order. If a promoted service cannot resolve for the active grid/filter, omit it and leave natural rows unchanged.
- Both kinds reuse existing market cards, permanent `推广` disclosure, impression timer, and click tracking. Reward placement does not earn recommendation badges or change service facts.

#### Analytics And Privacy

- `promotion_benefit_action` accepts only `copy_code`, `copy_link`, `poster_download`, or `coupon_apply`, plus the normalized source route.
- API promotion impression/click keeps only placement, position, provider category, billing mode, target type, and normalized route. IDs may be used in local rendering/impression de-duplication but are never emitted.
- Never send invite/referral/coupon/promotion/user/service IDs, invite codes, display names, titles, emails, contacts, or raw query strings to analytics.

### 4. Validation & Error Matrix

| Condition | UI behavior |
| --- | --- |
| Invalid or expired invite value | Ignore/clear it; continue login normally |
| Referral program disabled | Hide the active invitation workflow and show the disabled state without discarding coupons/history |
| Referral/coupon read fails | Use `ErrorState` with retry; never substitute mock success data |
| Coupon has no eligible service | Keep apply unavailable and direct the user to their API-service workflow |
| Apply receives overlap, expiry, feature-disabled, or validation failure | Preserve current form state, show the backend problem detail, and refetch authoritative data after settlement |
| Campaign/referral/coupon version conflict | Show mutation error and refetch through invalidation; never overwrite silently |
| Clipboard, poster, or download fails | Show failure feedback and emit no success analytics event |
| Short natural result list cannot separate reward from operator | Omit reward; do not render adjacent promoted cards |
| Admin table is wider than 390px viewport | Scroll within the table container; no document-level horizontal overflow or overlap |

### 5. Good / Base / Bad Cases

- Good: an invited seller opens the benefits page, sees a pending server-owned coupon, later applies it to an eligible service, and sees the used interval after query invalidation.
- Good: an operator card remains first, three natural rows retain order, then one reward card appears with `推广`.
- Base: no active campaign or no coupons renders a useful state without fabricated zero-value business data.
- Bad: embed the invite code in analytics, generate a poster from an unvalidated query value, or overwrite the first captured inviter.
- Bad: place operator and reward cards next to each other in an otherwise empty grid.
- Bad: label the reward as recommendation, certification, guaranteed exposure, commission, or reputation benefit.

### 6. Tests Required

- Run frontend tests, type-check, OpenAPI drift check, and a production real-backend build. Keep focused coverage for canonicalization/expiry/first-touch capture, OAuth transport/clear, adapter boundaries, analytics allowlisting, all coupon states, campaign/admin mutation contracts, reward placement/filter/deduplication/separation, promotion disclosure, and responsive table widths.
- Browser acceptance covers desktop and 390px mobile for both new pages, clipboard success, poster generation, coupon/grant/revoke dialogs, all administrator tabs, and document-level overflow. API-market browser QA must use a backend that returns the generated `kind` contract.

### 7. Wrong vs Correct

#### Wrong

```ts
trackAnalytics('promotion_benefit_action', {
  action: 'coupon_apply',
  inviteCode,
  couponId,
  serviceId,
})
```

#### Correct

```ts
trackAnalytics('promotion_benefit_action', {
  action: 'coupon_apply',
  source_route: '/my/promotion-benefits',
})
```
