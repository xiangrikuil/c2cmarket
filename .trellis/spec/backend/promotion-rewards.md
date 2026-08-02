# Referral Rewards And Promotion Benefits Contract

Date: 2026-08-02
Author: Codex

## Scenario: API-service referral rewards and promotion-coupon activation

### 1. Scope / Trigger

- `internal/module/promotionreward` owns the single `api_service_referral_v1` campaign, referral codes and relations, promotion-coupon lifecycle, qualification, administrator controls, reward activation, audit, and notifications.
- V1 targets API services only. It does not add cash rewards, withdrawals, multi-level referrals, transferable coupons, reputation changes, bidding, paid self-service promotion, or a polymorphic promotion target.
- Existing `apipromotion` administrator schedules retain their three-slot capacity and scheduling rules. Reward activations use an independent one-slot public projection and never consume administrator capacity.

### 2. Signatures

```text
GET   /api/v1/promotion-rewards/public-config
GET   /api/v1/me/referral
GET   /api/v1/me/promotion-coupons?page=&limit=&status=
POST  /api/v1/me/promotion-coupons/{id}/apply

GET   /api/v1/admin/promotion-reward-campaign
PATCH /api/v1/admin/promotion-reward-campaign
GET   /api/v1/admin/referrals?page=&limit=&status=&search=
POST  /api/v1/admin/referrals/{id}/revoke
GET   /api/v1/admin/promotion-coupons?page=&limit=&status=&source=&search=
POST  /api/v1/admin/promotion-coupons/grant
POST  /api/v1/admin/promotion-coupons/{id}/revoke
```

Coupon apply, campaign update, administrator grant, and both revoke commands require CSRF and `Idempotency-Key`. Versioned campaign/referral/coupon mutations also require `If-Match`; replay restores the original response and `ETag`.

### 3. Contracts

#### Campaign And Feature State

- Migration 73 creates `promotion_reward_campaigns`, `referral_codes`, `referral_relations`, and `promotion_coupons`; it seeds the only campaign disabled with 24-hour promotion, 30-day validity, 72-hour delay, and monthly inviter limit 10.
- The effective campaign requires `program_enabled`, `starts_at <= now`, and either no end or `now < ends_at`. Welcome and referral switches apply only within that effective range.
- Disabling the program stops new bindings, qualifications, rewards, and coupon application. It preserves all rows and does not truncate an activation that already started.
- Public config exposes only switches, dates, durations, limits, and public rules. User referral reads remove user IDs and risk flags. Administrator reads retain operational IDs and risk flags.

#### Referral Capture And Binding

- Codes are canonical uppercase eight-character strings from `23456789ABCDEFGHJKLMNPQRSTUVWXYZ`; ambiguous `0/O/1/I` characters are excluded.
- OAuth state carries a canonical optional code. Invalid, missing, inactive, self-referral, disabled-campaign, inactive-inviter, and duplicate-invitee cases are silent no-ops and must never block login or account creation.
- A relation is bound only while creating a new OAuth user, in the same transaction as identity creation. One invitee can have at most one relation and cannot change inviter later.
- A user referral read creates or returns that user's stable campaign code transactionally. User history is inviter-scoped and masks invitee identity according to the HTTP DTO contract.

#### First-Service Qualification And Rewards

- Qualification runs inside every API-service mutation that can make the service publicly orderable. It locks the service/owner and requires immutable `first_published_at`, the shared public-orderable predicate, an active owner, and both user creation and first publication within the campaign period.
- Uniqueness and row/advisory locks make concurrent qualifying mutations idempotent. A user receives at most one welcome coupon; each referral relation creates at most one invitee and one inviter source coupon.
- With welcome enabled, the first eligible service receives an immediately available coupon. If no administrator or reward activation overlaps, the same transaction auto-uses it for the configured duration; otherwise the coupon remains available.
- With referral enabled, the invitee and eligible inviter receive coupons available after `reward_delay_hours`, expiring `coupon_valid_days` later. The inviter limit uses Asia/Shanghai calendar-month bounds and an advisory lock. Reaching it suppresses only the inviter coupon and records `inviter_monthly_limit_reached`; the invitee coupon and rewarded relation remain.

#### Coupon Lifecycle And Activation

- Effective status is derived at read/write time: persisted `used` and `revoked` win; otherwise `expires_at <= now` is `expired`, `available_at > now` is `pending`, and the remainder is `available`.
- A coupon belongs to one user, cannot be transferred or exchanged, lasts 1-168 hours, and may be applied only to that user's currently publicly orderable API service while the campaign program is active.
- Apply locks the idempotency entry and coupon, then rechecks ownership, effective status, campaign state, service orderability/ownership, and half-open overlap against administrator and reward promotions. It writes activation facts, event, notification, and completed idempotency response atomically.
- Reward activation IDs are the only reward promotion IDs exposed publicly. Public reward selection rechecks current orderability, excludes a service already under an active administrator promotion, rotates deterministically by Asia/Shanghai hour, and returns at most one reward item.
- Revoking a used coupon preserves the used facts but sets status `revoked` and shortens a still-active interval to the revocation time. Revoking a relation revokes all linked non-revoked inviter/invitee coupons in the same transaction. No revocation restores a coupon.

#### Administration And Audit

- Campaign update limits are duration 1-168 hours, validity 1-365 days, delay 0-720 hours, inviter monthly limit 0-1000, non-empty rules up to 2000 Unicode code points, and reason 2-500 code points.
- Administrator grant requires an active target account, duration 1-168 hours, validity 1-365 days, and reason 2-500 code points. It creates an immediately available `admin_grant` coupon independent of a referral source.
- Referral and coupon filters are server paginated, default to page 1/limit 20, cap limit at 100, and restrict status/source enums. Administrator search is trimmed and limited to 100 Unicode code points.
- Campaign update, grant, relation revoke, and coupon revoke write administrator audit facts in the same transaction as the business change. Reward creation/use/revoke also writes de-duplicated domain events and user notifications.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing session or non-administrator access | `401 SESSION_EXPIRED` or `403 PERMISSION_DENIED` |
| Referral UI requested while program/referral is disabled | `404 FEATURE_DISABLED`; history remains stored |
| Invalid pagination, enum, identifier input, date/range, limit, rules, or reason | `422 VALIDATION_FAILED` |
| Coupon is not owned by the caller | `404 OBJECT_NOT_FOUND` without ownership disclosure |
| Coupon is pending, expired, used, or revoked | `409 INVALID_STATE_TRANSITION` |
| Campaign is inactive during coupon apply | `404 FEATURE_DISABLED` |
| Target service is not owned and publicly orderable | `422 VALIDATION_FAILED` on `apiServiceId` |
| Administrator/reward interval overlaps target activation | `409 INVALID_STATE_TRANSITION` |
| Version differs from `If-Match` | `412 VERSION_CONFLICT` |
| Versioned mutation omits `If-Match` | `428 PRECONDITION_REQUIRED` |
| Duplicate idempotency key has a different request hash | Existing idempotency conflict contract; no business mutation |

### 5. Good / Base / Bad Cases

- Good: two concurrent service transitions qualify one owner once, auto-use one welcome coupon, and create at most one coupon per referral side.
- Good: a service already under administrator promotion leaves the welcome coupon available instead of stacking or wasting it.
- Good: revoking an active reward coupon removes it from the next public read while preserving its historical activation facts and audit record.
- Base: an invalid invite code during OAuth behaves exactly like no invite code and account creation succeeds.
- Bad: grant a reward on registration alone, increase reputation, or treat a promotion as platform endorsement.
- Bad: calculate coupon state only when a scheduled job rewrites rows, allowing an expired persisted `available` coupon to be applied.
- Bad: append reward rows to the administrator schedule table or count them against the three administrator slots.

### 6. Tests Required

Run `go test ./...`, `go vet ./...`, OpenAPI route/type checks, migration documentation checks, and the PostgreSQL integration suite. PostgreSQL coverage must include migration 1 -> 73 and 73 -> 72 -> 73, referral binding concurrency, first-service qualification concurrency, inviter month boundaries, coupon apply overlap/replay, public suppression, and active revocation.

### 7. Wrong vs Correct

#### Wrong

```go
if coupon.StoredStatus == "available" {
    activate(coupon)
}
```

#### Correct

```text
lock coupon and campaign
derive effective status from available_at/expires_at and persisted terminal state
recheck program, ownership, shared public-orderable predicate, and both promotion sources
write activation + event + notification + idempotency completion in one transaction
```
