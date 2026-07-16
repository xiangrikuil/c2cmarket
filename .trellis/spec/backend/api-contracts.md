# API Contracts

Date: 2026-06-21
Author: Codex

## Scenario: Backend Contract Foundation And Current Real Business Slices

### 1. Scope / Trigger

- Trigger: backend work that adds or changes HTTP endpoints, request/response DTOs, sessions, CSRF, idempotency, contact windows, official price leads, carpool listings/applications, API services, API purchase intents, profiles, announcements, reports, disputes, appeals, OpenAPI, or PostgreSQL migrations.
- Current implementation: Go `net/http` handlers routed by `github.com/go-chi/chi/v5` under `backend/internal/server`, dependency composition under `backend/internal/app`, migration-phase business behavior under `backend/internal/module/core`, shared error codes under `backend/internal/domain`, OpenAPI under `docs/openapi/c2c-market-api-v1.yaml`.
- Runtime persistence can be in-memory only when the task explicitly says so. SQL migrations still define the database contract baseline.

### 2. Signatures

Implemented HTTP signatures:

```text
GET  /health
GET  /readyz
POST /api/v1/auth/dev-session
POST /api/v1/auth/password/login
POST /api/v1/auth/email-registration/start
POST /api/v1/auth/email-registration/confirm
GET  /api/v1/auth/oauth/start
GET  /api/v1/auth/oauth/callback
GET  /api/v1/auth/session
POST /api/v1/auth/logout
GET  /api/v1/me/profile
PATCH /api/v1/me/profile
GET  /api/v1/me/contact-methods
GET  /api/v1/me/merchant-profile
POST /api/v1/me/merchant-profile
PATCH /api/v1/me/merchant-profile
GET  /api/v1/users/{username}/public-profile
GET  /api/v1/merchant-profiles/{slug}
GET  /api/v1/announcements
GET  /api/v1/announcements/active
GET  /api/v1/announcements/home
GET  /api/v1/announcements/{slug}
GET  /api/v1/product-categories
GET  /api/v1/product-plans
GET  /api/v1/product-plans/{id}
GET  /api/v1/api-models
GET  /api/v1/api-models/{id}
GET  /api/v1/api-services
GET  /api/v1/api-services/{id}
POST /api/v1/api-services/{id}/purchase-intents
GET  /api/v1/official-prices
GET  /api/v1/official-prices/{id}
GET  /api/v1/carpools
POST /api/v1/carpools
GET  /api/v1/carpools/{id}
PATCH /api/v1/carpools/{id}
POST /api/v1/carpools/{id}/submit-review
POST /api/v1/carpools/{id}/applications
POST /api/v1/official-price-leads
GET  /api/v1/me/official-price-leads
GET  /api/v1/me/official-price-leads/{id}
GET  /api/v1/me/carpools
GET  /api/v1/me/carpool-applications
GET  /api/v1/me/carpool-applications/{id}
POST /api/v1/me/carpool-applications/{id}/cancel
POST /api/v1/me/carpool-applications/{id}/confirm-join
GET  /api/v1/me/carpool-memberships
POST /api/v1/me/carpool-memberships/{id}/confirm-complete
POST /api/v1/me/carpool-memberships/{id}/leave
GET  /api/v1/me/api-purchase-intents
GET  /api/v1/me/api-purchase-intents/{id}
POST /api/v1/me/api-purchase-intents/{id}/cancel
POST /api/v1/me/api-purchase-intents/{id}/orders
GET  /api/v1/me/api-orders
GET  /api/v1/me/api-orders/{id}
POST /api/v1/me/api-orders/{id}/payment-instructions
POST /api/v1/me/api-orders/{id}/submit-payment
POST /api/v1/me/api-orders/{id}/cancel
POST /api/v1/me/api-orders/{id}/confirm-complete
POST /api/v1/me/api-orders/{id}/dispute
GET  /api/v1/me/notifications
GET  /api/v1/me/notifications/unread-count
POST /api/v1/me/notifications/{id}/read
POST /api/v1/me/notifications/read-all
GET  /api/v1/me/navigation-badges
GET  /api/v1/me/events
GET  /api/v1/me/announcements/unread-count
GET  /api/v1/me/announcements/important-unread-count
POST /api/v1/me/announcements/{id}/seen
POST /api/v1/me/announcements/{id}/read
POST /api/v1/me/announcements/{id}/dismiss
GET  /api/v1/me/favorites
GET  /api/v1/me/favorites/{targetType}/{targetId}
PUT  /api/v1/me/favorites/{targetType}/{targetId}
DELETE /api/v1/me/favorites/{targetType}/{targetId}
GET  /api/v1/me/reviews
PUT  /api/v1/me/reviews/carpool-memberships/{membershipId}
GET  /api/v1/users/{username}/reviews
POST /api/v1/reports
GET  /api/v1/me/reports
POST /api/v1/me/appeals
GET  /api/v1/me/appeals
GET  /api/v1/users/{username}/disputes
GET  /api/v1/owner/carpool-applications
GET  /api/v1/owner/carpool-applications/{id}
POST /api/v1/owner/carpool-applications/{id}/accept
POST /api/v1/owner/carpool-applications/{id}/confirm-join
POST /api/v1/owner/carpool-applications/{id}/reject
POST /api/v1/owner/carpool-applications/{id}/withdraw-acceptance
GET  /api/v1/owner/carpool-memberships
POST /api/v1/owner/carpool-memberships/{id}/confirm-complete
POST /api/v1/owner/carpool-memberships/{id}/remove
GET  /api/v1/owner/api-services
POST /api/v1/owner/api-services
GET  /api/v1/owner/api-services/{id}
PATCH /api/v1/owner/api-services/{id}
POST /api/v1/owner/api-services/{id}/submit-review
POST /api/v1/owner/api-services/{id}/publish
POST /api/v1/owner/api-services/{id}/pause
POST /api/v1/owner/api-services/{id}/resume
POST /api/v1/owner/api-services/{id}/start-revision
PATCH /api/v1/owner/api-services/{id}/order-settings
GET  /api/v1/owner/api-purchase-intents
GET  /api/v1/owner/api-purchase-intents/{id}
POST /api/v1/owner/api-purchase-intents/{id}/mark-contacted
POST /api/v1/owner/api-purchase-intents/{id}/close
GET  /api/v1/owner/api-orders
GET  /api/v1/owner/api-orders/{id}
POST /api/v1/owner/api-orders/{id}/confirm-payment
POST /api/v1/owner/api-orders/{id}/submit-delivery
GET  /api/v1/admin/official-price-leads
GET  /api/v1/admin/official-price-leads/{id}
POST /api/v1/admin/official-price-leads/{id}/approve
POST /api/v1/admin/official-price-leads/{id}/reject
POST /api/v1/admin/official-price-leads/{id}/request-changes
GET  /api/v1/admin/official-price-records
POST /api/v1/admin/official-price-records
GET  /api/v1/admin/official-price-records/{id}
PUT  /api/v1/admin/official-price-records/{id}
POST /api/v1/admin/official-price-records/{id}/take-down
GET  /api/v1/admin/carpools
GET  /api/v1/admin/carpools/{id}
POST /api/v1/admin/carpools/{id}/approve
POST /api/v1/admin/carpools/{id}/reject
POST /api/v1/admin/carpools/{id}/request-changes
POST /api/v1/admin/carpools/{id}/pause
POST /api/v1/admin/carpools/{id}/restore
GET  /api/v1/admin/api-services
GET  /api/v1/admin/api-services/{id}
POST /api/v1/admin/api-services/{id}/approve
POST /api/v1/admin/api-services/{id}/request-changes
POST /api/v1/admin/api-services/{id}/reject
POST /api/v1/admin/api-services/{id}/suspend
POST /api/v1/admin/api-services/{id}/restore
POST /api/v1/admin/api-services/{id}/remove
GET  /api/v1/admin/api-purchase-intents
GET  /api/v1/admin/api-purchase-intents/{id}
GET  /api/v1/admin/announcements
POST /api/v1/admin/announcements
GET  /api/v1/admin/announcements/{id}
PATCH /api/v1/admin/announcements/{id}
POST /api/v1/admin/announcements/{id}/publish
POST /api/v1/admin/announcements/{id}/offline
POST /api/v1/admin/announcements/{id}/duplicate
GET  /api/v1/admin/announcement-audit-logs
GET  /api/v1/admin/reports
GET  /api/v1/admin/reports/{id}
POST /api/v1/admin/reports/{id}/triage
POST /api/v1/admin/reports/{id}/reject
POST /api/v1/admin/reports/{id}/open-dispute
GET  /api/v1/admin/disputes
GET  /api/v1/admin/disputes/{id}
POST /api/v1/admin/disputes/{id}/request-info
POST /api/v1/admin/disputes/{id}/resolve
POST /api/v1/admin/disputes/{id}/close
GET  /api/v1/admin/appeals
GET  /api/v1/admin/appeals/{id}
POST /api/v1/admin/appeals/{id}/approve
POST /api/v1/admin/appeals/{id}/reject
POST /api/v1/contact-methods
PATCH /api/v1/contact-methods/{id}
DELETE /api/v1/contact-methods/{id}
POST /api/v1/contact-methods/{id}/set-default
POST /api/v1/contact-methods/{id}/verify
POST /api/v1/dev/contact-sessions
GET  /api/v1/contact-sessions/{id}/contacts
```

Required headers:

```text
Cookie: c2c_session=<opaque session id>
X-CSRF-Token: <session CSRF token>              # all state-changing API requests except dev-session
Idempotency-Key: <opaque key>                   # create/action POST requests
If-Match: "<version>"                            # required for versioned admin actions
```

### 3. Contracts

- JSON API uses camelCase. Database schema uses snake_case.
- Public resource IDs in responses and path parameters use UUID strings, matching PostgreSQL `uuid` keys. Opaque auth/session tokens are not resource IDs and must not be treated as UUIDs.
- Problem responses use `application/problem+json` and include `code` plus `requestId`.
- Session auth is same-origin cookie auth. Production code must not accept request headers as user impersonation.
- `POST /api/v1/auth/dev-session` is a development entry only. It must be disabled outside development/test by `APP_ENV` / `ENABLE_DEV_AUTH` startup configuration.
- First-release public registration/login is linux.do OAuth only. Native username/password is a backup login path for accounts with `linuxDoBinding.bound=true`, plus the explicit first-admin bootstrap account. `POST /api/v1/auth/password` and `POST /api/v1/auth/password/login` must reject unbound non-admin users with `403 LINUX_DO_BINDING_REQUIRED` before creating or changing credentials. Password credentials must be stored only as salted hashes; plaintext passwords must never be stored in PostgreSQL, logs, OpenAPI examples, or frontend state.
- `POST /api/v1/auth/email-registration/start` and `POST /api/v1/auth/email-registration/confirm` are retained only as stable disabled compatibility endpoints. Both return `403 EMAIL_REGISTRATION_DISABLED` and must not send registration email, create challenges, create users, create sessions, or set session cookies. Login-bound `/me/email-verification/*` remains a profile/contact verification feature.
- OAuth login is another real session entry. `GET /api/v1/auth/oauth/start?returnTo=/path` sets an HttpOnly OAuth state cookie and returns `{authorizationUrl}`. `GET /api/v1/auth/oauth/callback?code=...&state=...` must compare query state with the state cookie, exchange the code for a provider profile, upsert `users`, `auth_identities`, `linux_do_bindings`, create an `auth_sessions` row, set `c2c_session`, clear the state cookie, and redirect to the normalized `FRONTEND_ORIGIN` plus the sanitized relative `returnTo`. Production `FRONTEND_ORIGIN` must be an absolute HTTPS origin without credentials, path, query, or fragment.
- OAuth provider mode can be `fake` only in development/test for smoke automation. Production must use `OAUTH_PROVIDER_MODE=oauth2` with `OAUTH_CLIENT_ID`, `OAUTH_CLIENT_SECRET`, `OAUTH_AUTHORIZE_URL`, `OAUTH_TOKEN_URL`, `OAUTH_USERINFO_URL`, and `OAUTH_REDIRECT_URL`.
- OAuth token responses are request-time credentials only. Do not persist provider access tokens, refresh tokens, userinfo raw payloads, session cookies, or OAuth codes in database rows, logs, OpenAPI examples, or frontend state.
- `GET /api/v1/auth/session` returns `user.permissions` and `user.linuxDoBinding`. Admin UI and backend admin routes must derive admin authority from the returned backend session/user permission source, not frontend-selected mock roles.
- `linuxDoBinding` means the account has a bound linux.do identity summary. It must not be worded as linux.do official certification, endorsement, or guarantee.
- `GET /readyz` is an unversioned operational endpoint. It returns process/database readiness, `schema_migrations` state, and the expected migration version when PostgreSQL is configured; business APIs must not depend on it for authorization or user-visible status.
- State-changing endpoints must call session and CSRF validation before decoding business actions.
- Create/action endpoints must reserve an idempotency entry before running the action and replay completed responses when method, route key, key, and request hash match.
- Multi-row state-changing actions with durable side effects, such as admin official price record create/update/take-down, legacy official price approval, carpool application acceptance, carpool join/completion, and API purchase-intent creation/actions, must write the completed idempotency response cache in the same PostgreSQL transaction as the business rows/events/audit/notifications. Do not leave a committed business side effect with a still-processing idempotency row.
- Versioned admin actions must require `If-Match`; missing preconditions return `428 PRECONDITION_REQUIRED`, stale versions return `412 VERSION_CONFLICT`. Do not accept a body-level `expectedVersion` in new endpoints.
- Public `POST /api/v1/official-price-leads` is a disabled compatibility endpoint. It must not create leads or records and returns `403 OFFICIAL_PRICE_USER_SUBMIT_DISABLED`.
- Admin official price record create/update computes normalized CNY price, fingerprint, and offer key server-side/admin-side. The PostgreSQL runtime writes the compatibility lead, price record, domain event, admin audit log, notification, and completed idempotency response cache in one transaction.
- Public official price read endpoints return active approved records only. They may expose public source URL, channel, normalized price, FX snapshot source, and offer key, but must not expose reviewed admin ID, fingerprint, duplicate detection internals, or audit fields.
- `GET/PATCH /api/v1/me/profile` owns editable user profile, privacy flags, display name, avatar mode, username, and public-profile toggles. Public profile routes must not expose contact values, contact method IDs, hidden owner mappings, or private owner user IDs.
- `GET/POST/PATCH /api/v1/me/merchant-profile` owns the current user's store alias profile. Self responses may include the owner ID; public merchant profile responses must not expose owner user ID or contact values.
- API services with `merchantIdentityMode=store_alias` must reference a merchant profile owned by the service owner. Public API service DTOs may expose `merchantDisplayName` and `merchantProfileSlug`, but not the backing owner user ID or contact method IDs.
- Public API service reads and API purchase-intent creation use the orderable service predicate, not only the public status triple. A public/orderable API service must be approved, online, clear, accepting orders, have `paymentWindowMinutes` between 3 and 15, and have at least one enabled payment option. Apply this same predicate to list, detail, search, favorite validation/listing, and purchase-intent creation.
- Product catalog read endpoints return active categories/plans and publish-policy fields from PostgreSQL. Frontend and backend must use `publishPolicy`, `accessMode`, `providerPolicyStatus`, `riskLevel`, `riskAckRequired`, and `policyVersion` instead of hard-coded Plus/Pro or Business branches.
- Carpool listing creation must resolve `productPlanId` from the product catalog. `publishPolicy=blocked` and `publishPolicy=info_only` cannot enter the listing/application flow. Plans with `riskAckRequired=true` require matching `riskNoticeCode` and `policyVersion` on both listing creation and application creation.
- Carpool listing creation creates `draft`; owners may edit only `draft` or `changes_requested` listings. The retained owner `submit-review` route is now the publish compatibility route: a linux.do-bound owner publishes directly to `active` after re-checking current `publishPolicy` and owner contact availability. Create/update requests must include structured `cycleTerm` fields for billing period, exit policy, and usage rules so applicants can review rules before applying. They must also include structured quota reference fields: `serviceMultiplier` as a positive decimal string, `averageQuotaPeriod` as `weekly` or `monthly`, and `averageQuotaUsd` as a positive decimal string. PostgreSQL stores these as `service_multiplier`, `average_quota_period`, and `average_quota_usd`; legacy rows may use migration defaults, but new API writes must pass service validation. Admin approve remains only for legacy `pending_review -> active`; request-changes remains only `pending_review -> changes_requested`; reject remains only `pending_review -> rejected`; pause is `active -> paused`; restore is `paused -> active`.
- Carpool listing requests use `buyerSeatCapacity` and `activeBuyerMembers`; both count buyer seats only and exclude the listing owner.
- Carpool public listing endpoints return `active` listings only. Owner/admin views may return non-public statuses.
- `/owner/*` carpool endpoints are a resource perspective for the current authenticated user as listing owner, not a separate merchant account role. Do not branch permissions on an independent merchant role for these routes.

## Scenario: Carpool Publish Region And Simplified Form Contract

### 1. Scope / Trigger

- Trigger: frontend publish-form, backend carpool listing DTO, OpenAPI, or PostgreSQL work touching carpool listing opening regions, payment method selection, billing-period presentation, or access-arrangement generation.
- Product scope: owners pick one off-platform payment method, publish listings with a persisted opening-region display value, and do not manually fill the backend access-arrangement text from a standalone publish-page section.
- Boundary: this contract does not add platform payments, escrow, credential custody, automatic delivery, or API proxy behavior.

### 2. Signatures

```text
POST  /api/v1/carpools
PATCH /api/v1/carpools/{id}
GET   /api/v1/carpools
GET   /api/v1/carpools/{id}
GET   /api/v1/me/carpools

Create/update JSON fields:
  regionCode: string       # required, max 64; custom regions use "other"
  regionName: string       # required, max 64; owner-facing display text
  cycleTerm.billingPeriod: "monthly"
  accessArrangement: string

PostgreSQL:
  carpool_listings.region_code text NOT NULL
  carpool_listings.region_name text NOT NULL
```

### 3. Contracts

- Frontend `SaveCarpoolDraftPayload.paymentMethodCodes` remains an array for facade compatibility, but publish UI, import normalization, validation, and submit mapping must keep exactly one payment method code.
- Frontend custom region state is `regionCode="other"` plus `customRegionName`; the real backend adapter sends `regionCode` and the final trimmed `regionName`.
- Backend create/update responses return `regionCode` and `regionName`. Public, owner, and admin listing reads must preserve these values without remapping custom regions to a fixed fallback.
- The publish page must not expose a writable billing-period control. The backend request still writes `cycleTerm.billingPeriod="monthly"` so applicants can review monthly-cycle rules.
- The publish page must not expose a standalone access-arrangement section. It derives `accessArrangement` from product `accessMode`; high-risk products still require the versioned risk acknowledgement before publish.
- Public copy may display access-arrangement summaries, but must not ask for or imply sharing account passwords, API keys, sessions, cookies, tokens, or other login state.

### 4. Validation & Error Matrix

| Condition | HTTP / UI result | Stable code / field |
| --- | --- | --- |
| Missing `regionCode` | 422 | `VALIDATION_FAILED`, `regionCode` |
| Missing `regionName` | 422 | `VALIDATION_FAILED`, `regionName` |
| `regionCode` or `regionName` longer than 64 runes | 422 | `VALIDATION_FAILED`, field-specific |
| Region/title/summary/access-arrangement contains credential-shaped text or NUL | 422 | `SECRET_CONTENT_DETECTED`, field-specific |
| Frontend custom region selected with empty `customRegionName` | Block submit | `region` field error |
| Frontend zero or multiple `paymentMethodCodes` | Block submit | `paymentMethodCodes` field error |
| High-risk product without current risk acknowledgement | 422 / block submit | `RISK_ACK_REQUIRED` or `accessArrangement` field error |

### 5. Good/Base/Bad Cases

- Good: owner selects `regionCode="other"` with `customRegionName="印度区"`; preview, linux.do post text, create payload, PostgreSQL row, public listing, owner listing, and application snapshots display `印度区`.
- Good: topic import detects multiple payment methods; the publish form keeps the first detected code and never presents a multi-select state.
- Base: owner selects a common region such as `jp`; frontend sends that code and display name, backend persists both, and reads return the same pair.
- Bad: custom region is empty, contains `token=...`, or UI submits two payment method codes.
- Bad: frontend removes `cycleTerm` or sends a non-monthly billing period because the readonly field was removed from the UI.

### 6. Tests Required

- Frontend tests must assert custom region display and exactly-one payment method behavior in publish helpers.
- Frontend type-check and real-backend build must cover the `SaveCarpoolDraftPayload` to backend request mapping.
- Backend router tests must assert region fields round-trip and credential-shaped region text is rejected.
- PostgreSQL integration tests must assert `region_code` and `region_name` survive publish/listing reads.
- OpenAPI must list `regionCode` and `regionName` as required create fields and listing response fields.

### 7. Wrong vs Correct

#### Wrong

```ts
payload.paymentMethodCodes = ["credit_card", "paypal"]
request.regionName = "其他"
request.cycleTerm = undefined
request.accessArrangement = form.freeTextAccessArrangement
```

This loses the owner's custom region, reintroduces multi-payment listings, and breaks the backend monthly-cycle/access-arrangement contract.

#### Correct

```ts
payload.paymentMethodCodes = [selectedPaymentCode]
request.regionCode = form.regionCode
request.regionName = finalRegionName
request.cycleTerm.billingPeriod = "monthly"
request.accessArrangement = defaultAccessArrangementNote(selectedProduct)
```

## Scenario: Carpool Cancel And Exit Lifecycle

### 1. Scope / Trigger

- Trigger: backend, OpenAPI, frontend adapter, or PostgreSQL work touching carpool application cancellation, owner acceptance withdrawal, membership leave/remove, or contact-window availability.
- Boundary: application-stage cancellation ends a request/reservation. Joined applications are terminal; post-join exit/remove must use the membership lifecycle.

### 2. Signatures

```text
POST /api/v1/me/carpool-applications/{id}/cancel
POST /api/v1/owner/carpool-applications/{id}/withdraw-acceptance
POST /api/v1/me/carpool-memberships/{id}/leave
POST /api/v1/owner/carpool-memberships/{id}/remove

Cancel/withdraw request body:
  { "reason": string }

Required headers:
  Cookie: c2c_session=<session>
  X-CSRF-Token: <session token>
  Idempotency-Key: <key>
  If-Match: "<application version>"
```

### 3. Contracts

- Buyer cancel returns a single `CarpoolApplication` response and supports:
  - `pending_owner -> cancelled_by_buyer`
  - `accepted_reserved -> cancelled_by_buyer`
- Owner withdraw acceptance returns a single `CarpoolApplication` response and supports:
  - `accepted_reserved -> cancelled_by_owner`
- `joined` applications cannot be cancelled through application endpoints. Buyer exit is `POST /api/v1/me/carpool-memberships/{id}/leave`; owner removal is `POST /api/v1/owner/carpool-memberships/{id}/remove`.
- `contact_session_id` is historical association, not access permission. Do not clear it when cancelling or withdrawing. Close the related contact session instead.
- Frontend real-backend actions must branch by status:
  - buyer `pending_owner` / `accepted_reserved` / projected `joined_pending_confirmation` calls application cancel;
  - buyer `active` / `pending_completion` calls membership leave;
  - owner `accepted_reserved` / projected `joined_pending_confirmation` calls withdraw acceptance;
  - owner `active` / `pending_completion` calls membership remove.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing session or invalid CSRF | `401` / `403` Problem Details |
| Missing `Idempotency-Key` | idempotency validation Problem Details |
| Missing `If-Match` | `428 PRECONDITION_REQUIRED` |
| Stale application version | `412 VERSION_CONFLICT` |
| Buyer cancels another user's application | `404 OBJECT_NOT_FOUND` |
| Owner withdraws another owner's application | `404 OBJECT_NOT_FOUND` |
| Buyer cancels `joined`, `rejected`, `expired`, or cancelled application | `409 INVALID_STATE_TRANSITION` |
| Owner withdraws `pending_owner` or any non-reserved state | `409 INVALID_STATE_TRANSITION`; use reject for pending applications |

### 5. Good/Base/Bad Cases

- Good: buyer cancels `accepted_reserved`, application stays linked to `contactSessionId`, status becomes `cancelled_by_buyer`, and contact read returns `CONTACT_WINDOW_EXPIRED`.
- Base: buyer cancels `pending_owner`, status becomes `cancelled_by_buyer`, no contact session is required.
- Bad: buyer tries to cancel a joined application; response is conflict and UI should guide them to exit membership.
- Bad: owner tries withdraw on `pending_owner`; response is conflict and UI should use reject.

### 6. Tests Required

- Router/API tests for buyer pending cancel, buyer reserved cancel, owner withdraw, joined cancel conflict, and owner reject/withdraw invalid transition.
- Contact-window regression tests must assert contact read fails after buyer cancel, owner withdraw, buyer leave, and owner remove.
- OpenAPI route parity tests must include the new runtime routes.
- Frontend type/build checks must cover real-backend action imports and application-detail button conditions.

### 7. Wrong vs Correct

#### Wrong

```text
Application accepted_reserved -> cancelled_by_buyer
carpool_applications.contact_session_id = NULL
contact_sessions.status remains open
```

This loses the historical association and can leave an accessible contact window.

#### Correct

```text
Application accepted_reserved -> cancelled_by_buyer
carpool_applications.contact_session_id unchanged
contact_sessions.status = revoked
contact_sessions.ends_at <= now()
```

The application history remains auditable while access permission is revoked.

## Scenario: Admin Product Plan Catalog CRUD

### 1. Scope / Trigger

- Trigger: backend, OpenAPI, frontend adapter, admin UI, or PostgreSQL work touching global `product_plans`, product catalog dropdowns, carpool publish policy, or official-price submit product/plan selection.
- Product contract: the admin catalog is the global option source for low-price lead submission, carpool publishing, and demand filters. User-entered custom plan text remains allowed at the submission boundary and is not automatically promoted into `product_plans`.

### 2. Signatures

```text
GET   /api/v1/product-plans?category=<code>
GET   /api/v1/product-plans/{id}

GET   /api/v1/admin/product-plans?category=<code>
GET   /api/v1/admin/product-plans/{id}
POST  /api/v1/admin/product-plans
PATCH /api/v1/admin/product-plans/{id}
POST  /api/v1/admin/product-plans/{id}/activate
POST  /api/v1/admin/product-plans/{id}/deactivate

ProductPlanRequest:
  categoryId, providerCode, slug, displayName, description
  publishPolicy, accessMode, providerPolicyStatus, riskLevel
  riskAckRequired, riskNoticeCode, policyNote
  active, allowCustomVariant, sortOrder
```

### 3. Contracts

- Public product-plan reads return only active plans whose category is active. Admin reads return active and inactive product plans.
- Admin writes require a backend admin session. State-changing admin endpoints require CSRF validation before business decoding.
- Admin create/update payloads are complete forms, not merge patches. JSON fields use camelCase and map to the existing snake_case `product_plans` columns.
- `slug` is globally unique across product plans and uses lowercase letters, numbers, and dashes. `providerCode` uses the same lowercase slug shape.
- Valid policy enums are:
  - `publishPolicy`: `allowed`, `info_only`, `blocked`
  - `accessMode`: `personal_account_cost_share`, `provider_member_invitation`, `owner_managed_access`, `other_off_platform`, `unsupported`
  - `providerPolicyStatus`: `known_restricted`, `possibly_restricted`, `unknown`
  - `riskLevel`: `normal`, `elevated`, `high`
- If `riskAckRequired=true`, `riskNoticeCode` is required and must reference a supported risk notice.
- Policy fields are `publishPolicy`, `accessMode`, `providerPolicyStatus`, `riskLevel`, `riskAckRequired`, `riskNoticeCode`, and `policyNote`.
- Only policy field changes increment `policyVersion` and append `product_plan_policy_history`. Display name, description, sort order, active state, and custom-variant toggles must not increment policy version.
- Activate/deactivate changes only `active` and `updated_at`; it never physically deletes rows and never writes policy history.
- Frontend mutations must invalidate both admin product-plan queries and user-facing active catalog caches so dropdowns refresh after admin changes.

### 4. Validation & Error Matrix

| Condition | HTTP | Code / Behavior |
| --- | ---: | --- |
| Non-admin calls admin list/detail/write | 403 | Admin authority comes from backend session/user permissions |
| Missing CSRF on create/update/activate/deactivate | 401/403 | Session/CSRF middleware rejects before mutation |
| Unknown request body field on create/update | 400 | Strict JSON decoding rejects it |
| Missing `categoryId`, invalid category, invalid enum, invalid slug/provider code | 422 | `VALIDATION_FAILED` field error |
| Duplicate `slug` on create/update | 409 | `VALIDATION_FAILED` field error on `slug` |
| Unknown plan ID on admin detail/update/toggle | 404 | Product plan not found |
| Public list/detail points at inactive plan | 404 or omitted | Public reads are active-only |

### 5. Good/Base/Bad Cases

- Good: admin creates an inactive plan, sees it in `GET /api/v1/admin/product-plans`, and public `GET /api/v1/product-plans` does not expose it until activation.
- Base: admin changes only `displayName` or `sortOrder`; `policyVersion` remains unchanged.
- Bad: admin deactivates a plan and existing historical records break because the row was deleted or public code hard-coded Plus/Pro behavior instead of resolving catalog policy.

### 6. Tests Required

- Backend route/service tests for create, policy update version increment, deactivate, admin inactive visibility, and public active-only visibility.
- PostgreSQL repository coverage or focused review for policy history insertion and non-policy updates avoiding policy history.
- OpenAPI YAML parse and route parity checks after adding or changing admin catalog routes.
- Frontend type/build checks after changing product catalog adapters, query hooks, pages, or route integration.
- Browser smoke for `/admin/product-plans` when the admin UI changes.

### 7. Wrong vs Correct

#### Wrong

```go
if req.Active == false {
    _, _ = db.Exec(ctx, "DELETE FROM product_plans WHERE id = $1", id)
}
```

This destroys historical references from carpool listings, low-price leads, and price records.

#### Correct

```go
UPDATE product_plans
SET active = false, updated_at = now()
WHERE id = $1
```

The catalog row remains durable, and public reads decide visibility through the active-only predicate.

## Scenario: Admin-Maintained Official Price Contract

### 1. Scope / Trigger

- Trigger: backend, OpenAPI, frontend adapter, smoke, or UI work touching official price public reads, admin official price maintenance, legacy official price lead routes, or "lowest price" wording.
- Product contract: official price intelligence means one admin-maintained monthly single-account official opening price. It is not carpool pricing, seat sharing, bulk purchase pricing, annual lock-in pricing, user-submitted lead collection, or an absolute all-market lowest-price guarantee.

### 2. Signatures

```text
POST /api/v1/official-price-leads
Cookie: c2c_session=<buyer session>
X-CSRF-Token: <session token>
Idempotency-Key: <opaque key>
Body: SubmitOfficialPriceLeadRequest
Response: 403 Problem Details code=OFFICIAL_PRICE_USER_SUBMIT_DISABLED

GET /api/v1/admin/official-price-records
Response: OfficialPriceRecordList including active, superseded, and taken_down records

POST /api/v1/admin/official-price-records
Cookie: c2c_session=<admin session>
X-CSRF-Token: <session token>
Idempotency-Key: <opaque key>
Body: AdminOfficialPriceRecordRequest
Response: OfficialPriceRecord

PUT /api/v1/admin/official-price-records/{id}
Cookie: c2c_session=<admin session>
X-CSRF-Token: <session token>
Idempotency-Key: <opaque key>
If-Match: "<record version>"
Body: AdminOfficialPriceRecordRequest
Response: replacement OfficialPriceRecord

POST /api/v1/admin/official-price-records/{id}/take-down
Cookie: c2c_session=<admin session>
X-CSRF-Token: <session token>
Idempotency-Key: <opaque key>
If-Match: "<record version>"
Body: { "reason": string }
Response: OfficialPriceRecord status="taken_down"

GET /api/v1/official-prices
Response: OfficialPriceRecordList

GET /api/v1/official-prices/{id}
Response: OfficialPriceRecord
```

### 3. Contracts

- Public submit is disabled. `SubmitOfficialPriceLeadRequest` is retained only so old clients receive a stable `OFFICIAL_PRICE_USER_SUBMIT_DISABLED` problem instead of silently creating a user lead.
- `AdminOfficialPriceRecordRequest` accepts admin-maintained single-account monthly official price fields:
  - `productText`, optional `productPlanId`, optional `planText`
  - `regionCode`, `channel`, `openingMethod`
  - `sourceUrl`
  - `observedAt`
  - `billingPeriod="monthly"`
  - `currency`, `originalAmount`, `taxIncluded`
  - `fxRateToCny`, `fxSource`, `fxObservedAt`
  - `validFrom`, `reason`
- Admin maintenance UI should source known product/plan candidates from `GET /api/v1/product-plans` instead of maintaining a separate hard-coded plan list.
- Admin-entered product/plan text remains allowed. When a catalog row is selected, the frontend sends both `productPlanId` and the visible `productText` / `planText`; custom text leaves `productPlanId` empty.
- Admin create/update must not accept `priceUnit`, `seatCount`, `quantity`, or `commitmentMonths`. Strict JSON decoding should reject them as unknown fields.
- The service still normalizes durable official price rows to the database baseline:
  - `price_unit='per_account'`
  - `seat_count=NULL`
  - `quantity=1`
  - `commitment_months=NULL`
- Because `official_price_records.lead_id` is currently required, admin create/update internally creates an admin-owned compatibility lead in the same transaction as the active record. That lead is an audit/source carrier, not a user submission workflow.
- Admin create for an offer supersedes any existing active record for the same offer key before writing the new active record.
- Admin update creates a replacement active record and marks the old public record `superseded`; it does not mutate the old active row in place.
- Admin take-down marks the active record `taken_down`. Public list/detail must not return taken-down or superseded records.
- `GET /api/v1/admin/official-price-records` may return active, superseded, and taken_down records for maintenance and audit.
- `GET /api/v1/official-prices` returns active records only. Legacy pending, changes-requested, and rejected leads are admin compatibility data only.
- Public record responses include `isLowestReference`. This is a backend-derived flag, not a frontend guess.
- Public list order is `normalized_monthly_cny ASC`, then stable tie-breakers.
- Lowest-reference grouping uses:
  - `productPlanId`
  - `regionCode`
  - `channel`
  - `openingMethod`
  - `billingPeriod`
  - `priceUnit`
  - `taxIncluded`
- Lowest-reference grouping explicitly ignores `commitmentMonths`, `seatCount`, and `quantity`.
- UI copy should use "官网公开价", "官网价格记录", "已验证参考低价", or "已验证低价记录". Avoid "官方最低价", "官方已验证最低", "全网最低", and other absolute guarantees.

### 4. Validation & Error Matrix

| Condition | HTTP | Code / Behavior |
| --- | ---: | --- |
| Public user posts `/api/v1/official-price-leads` | 403 | `OFFICIAL_PRICE_USER_SUBMIT_DISABLED`; no lead or record is created |
| Admin create/update body contains `priceUnit`, `seatCount`, `quantity`, or `commitmentMonths` | 400 | Strict JSON unknown-field rejection |
| Admin create/update body contains authority fields such as `fingerprint` or `offerKey` | 400 | Strict JSON unknown-field rejection |
| Admin create/update has custom product/plan text and empty `productPlanId` | 201/200 | Record is created with text fields and no catalog mapping |
| `billingPeriod` is not `monthly` | 422 | `PRICE_NORMALIZATION_REQUIRED` / validation field error |
| Missing `If-Match` on admin update/take-down | 428 | `PRECONDITION_REQUIRED` |
| Stale record version on admin update/take-down | 412 | `VERSION_CONFLICT` |
| Public list/detail contains superseded or taken_down record | Bug | Public reads must source only active records |
| Frontend receives missing `isLowestReference` from an older mock or fixture | N/A | Treat as `false`, never infer from `status === active` |

### 5. Good/Base/Bad Cases

- Good: admin creates a monthly single-account official price, public list returns the active record sorted by normalized monthly CNY and marks the group reference low via `isLowestReference`.
- Base: admin edits an active record; the response is a replacement active record, the old record becomes `superseded`, and public detail for the old record returns `404 OBJECT_NOT_FOUND`.
- Base: admin takes down an active record; admin list still shows it as `taken_down`, while public list/detail hide it.
- Bad: a public user submit request creates a lead, admin UI exposes "submit low-price lead", or frontend maps every `active` record to "lowest".

### 6. Tests Required

- Handler tests must assert public submit returns `403 OFFICIAL_PRICE_USER_SUBMIT_DISABLED`.
- Handler/service tests must assert admin create, update replacement, take-down, required `If-Match`, active-only public listing, and hidden public detail for superseded/taken-down records.
- Service tests must assert `isLowestReference` ignores `commitmentMonths`, `seatCount`, and `quantity`.
- Public API tests must assert active-only listing, price ascending order, and `isLowestReference` on list/detail responses.
- PostgreSQL integration tests must assert admin create/update/take-down writes record, compatibility lead, domain event, admin audit log, notification, and completed idempotency cache in one transaction.
- OpenAPI route parity and YAML parse tests must pass after changing official price DTOs.
- Frontend type-check must pass after adapter DTO changes.
- Smoke must cover disabled public submit plus admin create/update/take-down and search creation through admin records.

### 7. Wrong vs Correct

#### Wrong

```ts
await submitOfficialPriceLead(userPayload)
await approveOfficialPriceLead(lead.id)
```

This revives the user-submitted lead workflow and exposes a product path that no longer exists.

#### Correct

```ts
await createAdminOfficialPriceRecord(adminPayload)
```

Admin maintenance is the only current write path for official price records.

#### Wrong

```ts
isLowest: record.status === 'active'
```

This treats every public record as the lowest price and duplicates business logic in the frontend.

#### Correct

```ts
isLowest: record.isLowestReference === true
```

The backend owns the grouping rule and the frontend only renders the contract.

## Scenario: Carpool linux.do direct publish

### 1. Scope / Trigger

- Trigger: backend, OpenAPI, frontend, or smoke work touching carpool owner publication, public visibility, admin carpool actions, or copy around the retained `/api/v1/carpools/{id}/submit-review` endpoint.

### 2. Signatures

```text
POST /api/v1/carpools/{id}/submit-review
Cookie: c2c_session=<owner session>
X-CSRF-Token: <session token>
Idempotency-Key: <opaque key>
If-Match: "<listing version>"
Body: {}
```

Response is `CarpoolListing`. Successful owner publish returns `status="active"` and increments `version`.

### 3. Contracts

- The route name remains `submit-review` for compatibility, but user-facing copy must say publish, not submit for review.
- Current user must own the listing and have `user.linuxDoBinding.bound=true`.
- Listing status must be `draft` or `changes_requested`.
- The service must re-check the current product plan `publishPolicy`; only `allowed` can publish.
- The owner contact method must still belong to the owner and have a current usable version.
- Public carpool list/detail/application reads continue to accept only `active` listings.
- Admin `pause` hides an active listing from public reads and applications; admin `restore` makes a paused listing public again.
- Existing `pending_review` rows are legacy data and must remain actionable through admin approve/request-changes/reject.

### 4. Validation & Error Matrix

| Condition | HTTP | Code |
| --- | ---: | --- |
| Owner lacks linux.do binding | 422 | `VALIDATION_FAILED` with `field=linuxDoBinding` |
| Listing missing or not owned | 404 | `OBJECT_NOT_FOUND` |
| Stale `If-Match` | 412 | `VERSION_CONFLICT` |
| Missing `If-Match` | 428 | `PRECONDITION_REQUIRED` |
| Status is not `draft` / `changes_requested` | 409 | `INVALID_STATE_TRANSITION` |
| Product plan is `blocked` or `info_only` | 422 | `INVALID_STATE_TRANSITION` field error on `productPlanId` |
| Owner contact method unavailable | 422 | `CONTACT_METHOD_NOT_OWNED` |

### 5. Good/Base/Bad Cases

- Good: linux.do-bound owner publishes a draft listing and immediately receives `status=active`; public detail returns 200.
- Base: admin pauses an active listing; public detail and new application creation return 404 until admin restores it.
- Bad: a dev-session, self-registered, or GitHub-only user tries to publish and the listing remains non-public.

### 6. Tests Required

- Router test for linux.do-bound direct publish, public visibility, admin pause/restore, and paused application rejection.
- Router or PostgreSQL test for unbound owner publish returning 422 `VALIDATION_FAILED`.
- PostgreSQL integration coverage for legacy `pending_review` approve/request-changes/reject.
- Smoke scripts that publish carpools must use fake OAuth/linux.do sessions for owners and assert `status="active"` after the retained submit-review call.

### 7. Wrong vs Correct

#### Wrong

```text
owner submit-review -> pending_review -> admin approve -> active
```

#### Correct

```text
linux.do-bound owner submit-review compatibility route -> active
admin pause -> paused
admin restore -> active
legacy pending_review -> admin approve/request-changes/reject
```
- Carpool owner acceptance requires `If-Match`, `Idempotency-Key`, owner authorization, pending application status, available seats, buyer contact method ownership, and listing owner contact method ownership. Acceptance opens a 30-minute contact window, freezes contact method versions from the application/listing stored selections, writes event/notification, and reserves one buyer seat until `reservationExpiresAt`.
- Carpool join confirmation requires `If-Match`, `Idempotency-Key`, participant authorization, and an unexpired `joinConfirmationDeadline`. The first side confirmation keeps the application `accepted_reserved`; the second side confirmation changes it to `joined`, creates exactly one active `carpool_memberships` row, increments `activeBuyerMembers`, writes event/notification, and completes idempotency in the same PostgreSQL transaction.
- Carpool membership completion requires `If-Match`, `Idempotency-Key`, participant authorization, and active membership status. The first side confirmation keeps the membership `active`; the second side confirmation changes it to `completed`, sets `endedAt`, decrements `activeBuyerMembers`, writes event/notification, and completes idempotency in the same PostgreSQL transaction.
- Carpool buyer leave and owner remove require `If-Match`, `Idempotency-Key`, participant authorization, active membership status, and a non-empty reason. These actions move active membership to `left` or `removed`, set `endedAt`, decrement `activeBuyerMembers`, write event/notification, and do not imply platform payment, refund, compensation, or guarantee handling.
- Expired `accepted_reserved` reservations must not consume capacity and should read as `expired` even before a scheduler materializes the row.
- API model catalog endpoints return active model catalog rows and current price snapshots.
- API service creation and update store service root fields, access modes, supported model snapshots, and package rows. API service owner create/action POST endpoints require `Idempotency-Key`; update and state-changing owner/admin actions require `If-Match`.
- API service review state is `draft -> pending_review -> approved|changes_requested|rejected`; owner publication state is `offline -> online -> owner_paused -> online` plus `online|owner_paused -> offline/changes_requested` for revision; admin moderation is `clear -> admin_suspended -> clear` or `clear|admin_suspended -> removed`.
- Public API service reads return only services where `reviewStatus=approved`, `publicationStatus=online`, and `moderationStatus=clear`. Public DTOs must not expose owner contact method IDs, owner user IDs, review/admin internals, moderation reasons, or merchant internal notes.
- `distributionSystem=sub2api` fixes service model `merchantMultiplier` to `1.0000` in service validation and database constraints. Do not hard-code this only in frontend behavior.
- API service rows and DTOs must not store or return passwords, API keys, Sub2API keys, sessions, cookies, third-party tokens, panel owner credentials, payment proofs, or platform verification artifacts.
- API service orderability uses `acceptingOrders` as the owner-controlled willingness flag and `isOrderable` as the server-derived current predicate. First-release public API service list, detail, search, favorite validation/listing, and purchase-intent creation return only orderable services and support `paymentMethod=wechat|alipay`.
- API purchase intent creation is allowed only for public API services where `reviewStatus=approved`, `publicationStatus=online`, `moderationStatus=clear`, `acceptingOrders=true`, `paymentWindowMinutes` is between 3 and 15, and at least one payment option is enabled. An orderable online service is treated as the owner having pre-consented to receive compliant purchase intents and to disclose the service's selected merchant contact to the successful buyer.
- API purchase intent creation freezes the service version, buyer contact method version, owner contact method version, pricing snapshot, requested CNY amount, requested USD allowance or selected package snapshot in one PostgreSQL transaction. It writes event/notification side effects and completes idempotency metadata in that same transaction, but must not create or reference API-specific `contact_sessions`.
- API purchase intent amount fields are internal pre-order snapshots. They are not payable orders or platform-held credit; the linked API order owns fulfillment state and reserves service-level quota inventory.
- API purchase intent states are stored as `open`, `contacted`, `ordered`, `buyer_cancelled`, and `owner_closed`. Explicit transitions are buyer cancel `open|contacted -> buyer_cancelled`, owner mark contacted `open -> contacted`, owner close `open|contacted -> owner_closed`, and API order create `open|contacted -> ordered`.
- API purchase intent cancel and owner close require non-empty reasons; owner mark-contacted uses an empty JSON body and must not imply platform verification, payment, delivery, or fulfillment.
- API purchase intent successful create and buyer detail responses include frozen `merchantContact.value` and must set `Cache-Control: no-store`. Owner detail responses include frozen `buyerContact.value` and must also use `Cache-Control: no-store`. Buyer/owner lists and admin endpoints must not expose plaintext contact values.
- API purchase intent completed idempotency rows must store `resource_type='api_purchase_intent'` and `resource_id`, with `response_body_cache_allowed=false` for create responses that include `merchantContact.value`. Replay reconstructs the response from the frozen contact version instead of storing plaintext contact values in `idempotency_keys.response_body_json`.
- API orders are the participant/admin-facing business object; purchase intents remain internal tracking/audit records. A buyer can create at most one API order from a purchase intent across all statuses, including cancelled, payment-timeout-cancelled, and completed orders. A successful order creation atomically changes that intent to `ordered`, reserves service-level quota, releases the `open|contacted` active-intent uniqueness slot for the same buyer/service, and lets the buyer create a new intent when another purchase is needed. Duplicate or concurrent order creation, and cancel/close of an intent that already has an order, must return `409 API_PURCHASE_INTENT_HAS_ORDER`.
- API order creation accepts only `paymentMethod`. Amount, currency, service title, package/quote snapshot, buyer/seller IDs, payment window, expiry time, and private payment instructions are all server-frozen.
- API order states are `pending_payment -> payment_submitted -> paid_confirmed -> delivery_submitted -> completed`, with `payment_submitted -> payment_issue -> payment_submitted` for seller-reported `not_received`, `amount_mismatch`, or `remark_mismatch`, and `pending_payment -> cancelled` for buyer cancellation or payment timeout. A payment issue keeps quota reserved and waits for the buyer to supplement the non-sensitive payment summary. Disputes use `disputeStatus`, create or bind a `dispute_cases` row with `target_type='api_order'`, save `api_orders.dispute_case_id`, and must not overwrite the main fulfillment state.
- Buyer cancellation requires a non-empty user-facing reason and stores that reason in `cancelReason`; system timeout continues to store `payment_timeout`. A buyer can cancel only `pending_payment`, so cancellation is immediate and never waits for seller confirmation. Once payment is submitted, the cancel action must be rejected and the UI must route unresolved delays to support instead of auto-cancelling the order.
- API order responses that contain payment summaries, delivery notes, payment instructions, structured delivery credentials, or other sensitive order context must set `Cache-Control: private, no-store`. Order create responses must not include `paymentInstructions`; `POST /me/api-orders/{id}/payment-instructions` is the explicit audited read endpoint.
- API order delivery is a narrow product-boundary exception: after `paid_confirmed`, the seller may submit exactly one structured `deliveryCredential` for that order. Allowed shapes are `api_key_endpoint` (`apiBaseUrl`, `apiKey`, optional `instructions`) and `login_account` (`panelLoginUrl`, `username`, `password`, optional `instructions`). `deliveryNote` remains a generated non-sensitive summary such as `商户已提交 API Key 接入信息。` and must not store the raw credential. Detail/action responses for the buyer and seller may include the credential; list/admin/public responses must not.
- API order delivery credentials may contain only buyer-specific API keys or initial account passwords and are immutable after submission; the platform must not claim revocation support. They must reject cookies, sessions, OAuth/access/refresh tokens, recovery codes, MFA codes, provider master keys, owner/master account credentials, subscription links, proxy node links, encoded/nested subscription URLs, attachment payloads, and query-string secrets with `SECRET_CONTENT_DETECTED` or field-level `VALIDATION_FAILED`.
- User announcement routes return only user-visible announcements plus the current user's receipt state. `seen`, `read`, and `dismiss` write receipt timestamps and must not mutate announcement content.
- Announcement home-banner selection uses published, non-expired, home-channel announcements and receipt dismissal state. Dismissal hides only the banner for the current user; it must not archive or offline the announcement.
- Admin announcement routes own draft/create/update/publish/offline/duplicate/audit flows. Offlining requires a non-empty reason and writes an audit log. Duplicating creates a new draft rather than editing the source.
- Report creation accepts only target-scoped, sanitized user statements. It must reject full contact values, passwords, API keys, tokens, sessions, cookies, recovery codes, and other credential-looking content.
- Report target types are `contact_snapshot`, `public_user`, `carpool_membership`, `api_purchase_intent`, and `api_order`. `public_user` requires `reportedUsername`; other target types require a non-empty `targetId`.
- Report state is `submitted -> triaged|rejected|dispute_opened`. `open-dispute` creates a `dispute_cases` row and links it to the report.
- Dispute state is `open -> waiting_info|resolved|closed`; `resolve` and `close` must store public-safe summary/result fields when public output changes.
- Appeal state is `submitted -> approved|rejected`; appeal creation must reference a report or dispute.
- Admin report/dispute/appeal actions require session, CSRF, `Idempotency-Key`, and `If-Match`.
- `GET /api/v1/users/{username}/disputes` and public profile embedded disputes return only public-safe fields from `dispute_cases.public_summary/public_result`; they must not expose reporter IDs, admin IDs, raw report descriptions, appeal statements, contact values, internal notes, evidence, or admin reasons.
- Contact session reads return full selected contact values only to participants before the deadline and must set `Cache-Control: no-store`.
- Product boundary: do not add payment, escrow, wallet, platform guarantee, API proxying, generalized third-party credential custody, or automatic credential delivery behavior to this backend. The only credential-storage exception is the one-time API order `deliveryCredential` described above.

### 4. Validation & Error Matrix

| Condition | HTTP | Stable code |
| --- | ---: | --- |
| Missing/expired session | 401 | `SESSION_EXPIRED` |
| Invalid native username/password credentials | 401 | `INVALID_CREDENTIALS` |
| Development auth disabled | 404 | `OBJECT_NOT_FOUND` |
| Revoked session | 401 | `SESSION_REVOKED` |
| Missing or wrong CSRF token | 403 | `CSRF_TOKEN_INVALID` |
| Missing, expired, or mismatched OAuth state | 403 | `CSRF_TOKEN_INVALID` |
| OAuth callback without code | 422 | `VALIDATION_FAILED` |
| OAuth provider token/userinfo failure | 502 | `INTERNAL_ERROR` |
| Non-admin admin action | 403 | `PERMISSION_DENIED` |
| Missing idempotency key | 400 | `VALIDATION_FAILED` |
| Same idempotency key, different request body | 409 | `IDEMPOTENCY_KEY_REUSED` |
| Same idempotency key still processing | 409 | `IDEMPOTENCY_IN_PROGRESS` |
| Invalid review state transition | 409 | `INVALID_STATE_TRANSITION` |
| Duplicate ongoing carpool application | 409 | `ACTIVE_APPLICATION_EXISTS` |
| User is already an active member of the carpool listing | 409 | `ACTIVE_MEMBERSHIP_EXISTS` |
| No available carpool seat on application or owner accept | 409 | `SEAT_UNAVAILABLE` |
| Join confirmation deadline expired | 409 | `JOIN_CONFIRMATION_EXPIRED` |
| Membership action attempted on non-active membership | 409 | `MEMBERSHIP_NOT_ACTIVE` |
| Missing `If-Match` for versioned admin action | 428 | `PRECONDITION_REQUIRED` |
| Version mismatch | 412 | `VERSION_CONFLICT` |
| Validation failure | 422 | `VALIDATION_FAILED` |
| Missing or stale carpool risk acknowledgement | 422 | `RISK_ACK_REQUIRED` |
| Credential-looking evidence URL | 422 | `SECRET_CONTENT_DETECTED` |
| Credential-looking report or appeal content | 422 | `SECRET_CONTENT_DETECTED` |
| Public profile not found | 404 | `OBJECT_NOT_FOUND` |
| Merchant profile slug unavailable | 409 | `VALIDATION_FAILED` |
| Announcement slug/id not found or not visible | 404 | `OBJECT_NOT_FOUND` |
| Announcement offline without reason | 422 | `VALIDATION_FAILED` |
| Report/dispute/appeal not found | 404 | `OBJECT_NOT_FOUND` |
| Report/dispute/appeal invalid state action | 409 | `INVALID_STATE_TRANSITION` |
| Contact window expired | 409 | `CONTACT_WINDOW_EXPIRED` |
| API service not currently orderable for order creation | 409 | `INVALID_STATE_TRANSITION` |
| Same API purchase intent already has any order | 409 | `API_PURCHASE_INTENT_HAS_ORDER` |
| Unsupported API order payment method | 422 | `VALIDATION_FAILED` |
| API order action in wrong state | 409 | `INVALID_STATE_TRANSITION` |
| Credential-looking API order payment/reason text | 422 | `SECRET_CONTENT_DETECTED` |
| Forbidden API order delivery credential content such as cookies, sessions, OAuth tokens, recovery codes, subscriptions, proxy URLs, or owner/master credentials | 422 | `SECRET_CONTENT_DETECTED` or `VALIDATION_FAILED` |
| Second API order delivery submission for the same order | 409 | `INVALID_STATE_TRANSITION` |

### 5. Good/Base/Bad Cases

- Good: submit a lead with raw observed price fields, then approve it with `fxSnapshot`; response includes server-computed normalized monthly CNY and a price record.
- Base: replay the exact same idempotent approval with the same `Idempotency-Key`; response body and record ID are stable.
- Bad: submit `fxRate` in the public lead body; strict decoding returns `400 VALIDATION_FAILED`.
- Bad: submit an evidence URL containing `access_token` or `password`; validation returns `422 SECRET_CONTENT_DETECTED`.
- Bad: request contact values after `endsAt`; response returns a Problem Details body and never includes contact values.
- Good: create a high-risk carpool listing with current risk acknowledgement, approve it, apply with current risk acknowledgement, then owner-accept it; response includes an `accepted_reserved` application with a contact session ID.
- Bad: create or apply to a high-risk carpool without matching risk acknowledgement; returns `422 RISK_ACK_REQUIRED`.
- Good: buyer and owner both confirm join before the deadline; response includes a `joined` application and buyer/owner membership lists include the active membership.
- Good: buyer and owner both confirm membership completion; response includes a `completed` membership with `endedAt`, and the listing active buyer-member cache is decremented.
- Good: buyer leaves or owner removes an active membership with a reason; response status is `left` or `removed`, with no payment/refund platform semantics.
- Bad: owner accepts a second pending application after the last seat has already been reserved; returns `409 SEAT_UNAVAILABLE`.
- Bad: a user who already has an active membership applies to the same listing again; returns `409 ACTIVE_MEMBERSHIP_EXISTS`.
- Bad: buyer or owner confirms join after `joinConfirmationDeadline`; returns `409 JOIN_CONFIRMATION_EXPIRED`.
- Bad: buyer tries to leave an already completed membership; returns `409 MEMBERSHIP_NOT_ACTIVE`.
- Good: a buyer submits an API purchase intent for an approved, online, clear API service; the `201` response includes status `open`, frozen pricing snapshots, and frozen `merchantContact.value`.
- Base: replay the exact same API purchase-intent create request with the same `Idempotency-Key`; response is reconstructed from the same intent ID and frozen merchant contact, while the idempotency row does not cache plaintext contact values.
- Good: the service owner marks the API purchase intent as contacted, then closes it with a reason; each action requires `If-Match` and `Idempotency-Key`.
- Good: service owner enables order settings only after the service is approved, online, clear, has a valid contact, has at least one enabled payment option, and has a 3-15 minute payment window; public list/search includes the service only when `isOrderable=true`.
- Good: buyer creates an API order from a purchase intent with `{paymentMethod:"wechat"}`; the order freezes server-side amount, currency, payment window, selected payment method, and service snapshots, then the buyer reads payment instructions through the audited endpoint.
- Good: after the first order is created, its intent reads as `ordered`; the buyer can create a fresh intent and a second order for the same API service.
- Good: buyer submits a payment summary, owner manually confirms off-platform payment, owner submits one structured API Key delivery credential, and buyer can later read that credential from buyer detail; each state-changing action requires `If-Match` and `Idempotency-Key`.
- Good: owner submits a `login_account` delivery credential with `panelLoginUrl`, `username`, `password`, and instructions after payment is confirmed; order status becomes `delivery_submitted`, `deliveryNote` contains only a non-sensitive summary, and detail responses include the credential only for the buyer or seller.
- Bad: a buyer submits an API purchase intent against a draft, paused, suspended, removed, or otherwise non-public API service; response is `404 OBJECT_NOT_FOUND`.
- Bad: a buyer creates an API order before order settings make the service orderable; response is `409 INVALID_STATE_TRANSITION`.
- Bad: a buyer creates another API order from the same purchase intent after cancellation, timeout, or completion; response is `409 API_PURCHASE_INTENT_HAS_ORDER`.
- Bad: buyer cancels or owner closes a purchase intent that already has any API order; response is `409 API_PURCHASE_INTENT_HAS_ORDER`.
- Bad: owner tries to submit delivery before `paid_confirmed`, repeat delivery after `delivery_submitted`, or submit delivery fields containing cookies, sessions, OAuth tokens, recovery codes, subscription URLs, proxy node URLs, owner/master account credentials, or query-string secrets; response is `409 INVALID_STATE_TRANSITION`, `422 SECRET_CONTENT_DETECTED`, or `422 VALIDATION_FAILED` as appropriate.
- Bad: a service owner submits an API purchase intent against their own service; response is `409 INVALID_STATE_TRANSITION`.
- Bad: a buyer uses a contact method owned by another user; response is `422 CONTACT_METHOD_NOT_OWNED`.
- Good: a user updates profile privacy and public profile reads omit disabled optional stats plus all contact values.
- Good: a user creates a merchant profile, publishes a store-alias API service, and public service reads expose the merchant profile slug/display name without owner contact internals.
- Good: an admin creates and publishes an announcement, a user sees it in list/home/detail, marks it read, then dismisses the home banner while detail remains readable.
- Bad: an announcement offline action without a reason returns validation failure and does not change status.
- Good: a user reports a public user, admin opens a dispute with public summary/result, public user profile shows only the sanitized dispute summary and updated unresolved count.
- Good: a user appeals a report/dispute; admin approves or rejects the appeal with `If-Match` and idempotency.
- Bad: a report description contains an API key, password, token, session, cookie, recovery code, or full contact value; response is `422 SECRET_CONTENT_DETECTED`.
- Bad: public dispute response includes reporter/admin IDs, internal notes, raw evidence, contact values, or admin reason; this violates the public DTO contract.

### 6. Tests Required

Backend contract slices must include tests for:

- Health route.
- Dev session cookie and CSRF issuance.
- Missing/invalid CSRF rejection.
- Strict JSON rejection of authority fields.
- Evidence URL validation.
- Official price lead approval and idempotent replay.
- Public official price list/detail reads.
- Product catalog category/plan reads with policy fields.
- Idempotency key conflict.
- Admin status machine rejection for invalid repeated actions.
- Contact session participant read with `Cache-Control: no-store`.
- Contact session expiry without contact value leakage.
- Carpool high-risk listing/application risk acknowledgement requirement.
- Carpool admin approve with `If-Match`.
- Carpool duplicate ongoing application rejection.
- Carpool owner accept idempotent replay and no-seat rejection.
- Carpool buyer/owner join confirmation, idempotent replay, active membership creation, and membership list reads.
- Carpool buyer/owner completion confirmation, idempotent replay, completed membership, buyer leave, owner remove, and listing cache decrement.
- API service owner create/submit/approve/publish/pause/resume/suspend/restore/remove flow, including public visibility changes.
- API service public DTO boundary, including absence of owner contact method IDs, owner user IDs, review internals, and merchant internal notes.
- API service database integrity constraints, including fixed Sub2API multiplier and owner-owned contact method selection.
- API purchase intent create flow, idempotent replay without plaintext body cache, direct merchant contact disclosure with `Cache-Control: no-store`, buyer/owner/admin detail visibility, owner mark-contacted, buyer cancel, owner close, and completed idempotency metadata rows.
- API purchase intent integrity constraints, including public service predicate rejection, owner self-intent rejection, buyer contact ownership rejection, owner contact availability, requested USD allowance cap rejection, active-intent uniqueness, and absence of API-specific contact-session columns or rows.
- API order flow, including order settings validation, public orderable list/search filtering, payment method filtering, order create from purchase intent, no payment instructions in create response, audited payment-instruction read with QR-code snapshot, buyer payment summary, owner manual payment confirmation, one-time structured delivery credentials, buyer/seller detail credential visibility, list/admin/public credential non-leakage, forbidden credential-content rejection, duplicate delivery rejection, buyer completion, dispute case creation/binding, payment timeout materialization, and one-order-ever-per-intent uniqueness.
- PostgreSQL API order regression: first order changes its intent to `ordered`, the second intent for the same buyer/service is accepted, and both order rows remain independently addressable.
- Profile/contact/merchant profile flow, including profile update, contact method list/update/verify/delete/default, public user profile privacy, public merchant profile boundary, and store-alias API service public DTO boundaries.
- Announcement user/admin flow, including create/update/publish/offline/duplicate, user list/home/detail, receipt seen/read/dismiss, unread counts, audit logs, and route parity with OpenAPI.
- Report/dispute/appeal flow, including contact/public-user report creation, admin report list/detail/actions, dispute open/request-info/resolve/close, public dispute list/profile stats, appeal create/list/admin approve/reject, idempotent replay, If-Match conflicts, and sanitized public DTO assertions.

### 7. Wrong vs Correct

#### Wrong

```go
// Silently ignores authority fields and lets public clients choose normalized prices.
decoder := json.NewDecoder(r.Body)
_ = decoder.Decode(&req)
lead.NormalizedMonthlyCNY = req.NormalizedMonthlyCNY
```

#### Correct

```go
decoder := json.NewDecoder(bytes.NewReader(body))
decoder.DisallowUnknownFields()
if err := decoder.Decode(&req); err != nil {
    return validationProblem
}
// Normalization is computed only during admin/service approval.
```

#### Wrong

```go
// Exposes contact values after a contact window expires.
return ContactSessionView{Items: session.Items}, nil
```

#### Correct

```go
if !now.Before(session.EndsAt) {
    return ContactSessionView{}, domain.NewError(http.StatusConflict, domain.CodeContactWindowExpired, "Contact window expired", "联系窗口已过期。")
}
```

#### Wrong

```go
// Public dispute API leaks internal report evidence and handler identity.
return PublicDispute{Result: report.Description, AdminID: report.HandledByAdminID}
```

#### Correct

```go
// Public dispute API uses only explicit public-safe fields.
return PublicDispute{Type: dispute.PublicSummary, Result: dispute.PublicResult}
```

## Scenario: Reports Disputes Appeals Real Integration

### 1. Scope / Trigger

- Trigger: cross-layer API and database contract for user reports, manual dispute cases, and user appeals.
- Scope: reports/disputes/appeals are manual risk records and public-safe summaries. They are not payment, refund, compensation, escrow, guarantee, fulfillment, credential delivery, file upload, email, webhook, external ticket, or automatic penalty systems.

### 2. Signatures

```text
POST /api/v1/reports
GET  /api/v1/me/reports
POST /api/v1/me/appeals
GET  /api/v1/me/appeals
GET  /api/v1/users/{username}/disputes
GET  /api/v1/admin/reports
GET  /api/v1/admin/reports/{id}
POST /api/v1/admin/reports/{id}/triage
POST /api/v1/admin/reports/{id}/reject
POST /api/v1/admin/reports/{id}/open-dispute
GET  /api/v1/admin/disputes
GET  /api/v1/admin/disputes/{id}
POST /api/v1/admin/disputes/{id}/request-info
POST /api/v1/admin/disputes/{id}/resolve
POST /api/v1/admin/disputes/{id}/close
GET  /api/v1/admin/appeals
GET  /api/v1/admin/appeals/{id}
POST /api/v1/admin/appeals/{id}/approve
POST /api/v1/admin/appeals/{id}/reject
```

Required headers:

```text
Cookie: c2c_session=<opaque session id>       # user/admin routes
X-CSRF-Token: <session CSRF token>            # all state-changing requests
Idempotency-Key: <opaque key>                 # POST create/action routes
If-Match: "<version>"                         # admin action routes
```

### 3. Contracts

- `CreateReportRequest` fields are `targetType`, `targetId`, `targetLabel`, `reportedUsername`, `reasonCode`, `title`, and `description`.
- `targetType` accepts only `contact_snapshot`, `public_user`, `carpool_membership`, `api_purchase_intent`, and `api_order`.
- `reasonCode` accepts only `invalid`, `unreachable`, `impersonation`, and `other`.
- Report content must be sanitized text. It must not include complete contact values, passwords, API keys, tokens, session IDs, cookies, recovery codes, or credential-looking material.
- Report state machine: `submitted -> triaged|rejected|dispute_opened`.
- `open-dispute` creates one `dispute_cases` row, sets report status to `dispute_opened`, and returns both report and dispute.
- API order dispute creation creates a `dispute_cases` row with `target_type='api_order'` and links `api_orders.dispute_case_id`; it does not require a `reports` row and does not mutate the order fulfillment state.
- Dispute state machine: `open -> waiting_info|resolved|closed`. `resolve` accepts `open|waiting_info`; `request-info` accepts `open`; `close` accepts any non-closed dispute.
- Appeal creation must reference `reportId` or `disputeId`; appeal state machine is `submitted -> approved|rejected`.
- Admin action responses return a mutation envelope with `report`, `dispute`, or `appeal` plus fresh `version`/`ETag`.
- Public disputes return only `id`, `username`, `type`, `result`, `handledAt`, and `unresolved`.
- Public profile dispute stats count unresolved disputes from `open|waiting_info` and resolved-last-90-days from `resolved`.

### 4. Validation & Error Matrix

| Condition | HTTP | Stable code |
| --- | ---: | --- |
| Missing/expired session on user/admin routes | 401 | `SESSION_EXPIRED` |
| Non-admin admin route | 403 | `PERMISSION_DENIED` |
| Missing or wrong CSRF token | 403 | `CSRF_TOKEN_INVALID` |
| Missing idempotency key on create/action | 400 | `VALIDATION_FAILED` |
| Same idempotency key, different request body | 409 | `IDEMPOTENCY_KEY_REUSED` |
| Missing `If-Match` on admin action | 428 | `PRECONDITION_REQUIRED` |
| Version mismatch on admin action | 412 | `VERSION_CONFLICT` |
| Unsupported target type or reason/action | 422 | `VALIDATION_FAILED` |
| Missing report/dispute reference for appeal | 422 | `VALIDATION_FAILED` |
| Credential-looking title/description/statement | 422 | `SECRET_CONTENT_DETECTED` |
| Report/dispute/appeal not found | 404 | `OBJECT_NOT_FOUND` |
| Invalid state transition | 409 | `INVALID_STATE_TRANSITION` |

### 5. Good/Base/Bad Cases

- Good: user reports a public profile, admin opens a dispute with public summary/result, public profile shows one unresolved sanitized dispute.
- Good: user reports a contact snapshot with an unreachable reason; admin rejects it with a reason and version increment.
- Good: user creates an appeal linked to a report/dispute; admin approves it with `If-Match` and idempotent replay.
- Base: replay the exact same report creation request with the same idempotency key; response returns the same report without duplicate rows or events.
- Bad: report text includes passwords, API keys, tokens, sessions, cookies, recovery codes, or complete contact values; response is `422 SECRET_CONTENT_DETECTED`.
- Bad: public dispute response contains reporter/admin IDs, raw report description, appeal statement, internal notes, admin reason, contact values, or evidence body.
- Bad: admin tries to open a dispute from a rejected or already dispute-opened report; response is `409 INVALID_STATE_TRANSITION`.

### 6. Tests Required

- OpenAPI must include all user, public, and admin report/dispute/appeal routes and schemas.
- Backend tests or smoke must cover report creation, admin list/detail/action, dispute opening, public dispute list/profile stats, dispute resolve/close, appeal creation/list/action, `If-Match`, idempotency replay, and public DTO sanitization.
- PostgreSQL migration must include `reports`, `dispute_cases`, `appeals`, and `dispute_events` with status checks, useful indexes, and one-dispute-per-report linking.
- Frontend typecheck must prove real mode `createContactReport()`, public profile report, admin reports/appeals, and public disputes use `reportBackend` without silent mock fallback.
- Product boundary scan must show no payment, escrow, guarantee, compensation, credential-storage, credential-delivery, external ticket, email, webhook, file-upload, or automatic penalty semantics added by reports/disputes/appeals.

### 7. Wrong vs Correct

#### Wrong

```go
// Treats dispute as a refund/compensation engine.
dispute.CompensationAmountCents = req.CompensationAmountCents
dispute.RefundStatus = "pending"
```

#### Correct

```go
// Store only manual state, reason, and public-safe summary/result.
input := report.AdminActionInput{PublicSummary: req.PublicSummary, PublicResult: req.PublicResult}
```

#### Wrong

```typescript
// Real backend failure is hidden behind mock admin rows.
try { return backendAdminReportRows() } catch { return mockReports }
```

#### Correct

```typescript
if (shouldUseRealBackend()) return backendAdminReportRows()
```

## Scenario: Favorites Real Integration

### 1. Scope / Trigger

- Trigger: new cross-layer API and database contract for current-user favorites.
- Scope: favorite targets are only public carpool listings and public API services. Favorites are personal markers; they do not change target state, create notifications, start contact windows, or imply payment, escrow, fulfillment, guarantee, or credential delivery.

### 2. Signatures

```text
GET    /api/v1/me/favorites
GET    /api/v1/me/favorites/{targetType}/{targetId}
PUT    /api/v1/me/favorites/{targetType}/{targetId}
DELETE /api/v1/me/favorites/{targetType}/{targetId}
```

Required headers:

```text
Cookie: c2c_session=<opaque session id>
X-CSRF-Token: <session CSRF token>       # PUT and DELETE
Idempotency-Key: <opaque key>            # PUT only
```

### 3. Contracts

- Path `targetType` accepts frontend `api-service` and backend `api_service`; the service normalizes both to durable `api_service`.
- Durable target types are `carpool` and `api_service`.
- `targetId` is a UUID string.
- `GET /me/favorites` returns `{ items: Favorite[] }`, sorted newest first.
- `GET /me/favorites/{targetType}/{targetId}` returns `{ favorited: boolean }`.
- `PUT /me/favorites/{targetType}/{targetId}` accepts `{}` and returns `{ favorited: true, favorite: Favorite }`.
- `DELETE /me/favorites/{targetType}/{targetId}` accepts `{}` and returns `{ favorited: false }`.
- `Favorite` response fields are `id`, `targetType`, `targetId`, `title`, `subtitle`, `status`, `to`, and `createdAt`.
- Favorite list queries must omit favorites whose target is no longer public-visible.
- Public-visible target predicates:
  - Carpool: `carpool_listings.status='active'`.
  - API service: approved, online, clear, accepting orders, payment window between 3 and 15 minutes, and at least one enabled payment option.

### 4. Validation & Error Matrix

| Condition | HTTP | Stable code |
| --- | ---: | --- |
| Missing/expired session | 401 | `SESSION_EXPIRED` |
| Missing or wrong CSRF token on PUT/DELETE | 403 | `CSRF_TOKEN_INVALID` |
| Missing PUT idempotency key | 400 | `VALIDATION_FAILED` |
| Same PUT idempotency key, different request body | 409 | `IDEMPOTENCY_KEY_REUSED` |
| Unsupported target type | 422 | `VALIDATION_FAILED` |
| Missing target id | 422 | `VALIDATION_FAILED` |
| Target does not exist or is not public-visible | 404 | `OBJECT_NOT_FOUND` |

### 5. Good/Base/Bad Cases

- Good: buyer favorites an active carpool listing; subsequent status is `true` and list includes a `carpool` item.
- Good: buyer favorites an approved, online, clear API service using path `api-service`; response/list stores durable `api_service`.
- Base: repeat PUT with the same idempotency key and same empty body; response replays successfully as favorited.
- Base: DELETE an already-deleted favorite; response remains `{ favorited: false }`.
- Bad: favorite a draft carpool listing or paused/suspended API service; response is `404 OBJECT_NOT_FOUND`.
- Bad: pass `official-price` as target type; response is `422 VALIDATION_FAILED`.

### 6. Tests Required

- Route parity test must include `PUT` methods from OpenAPI.
- Backend tests must cover OpenAPI route presence for all four favorite routes.
- Smoke must create one public carpool listing and one public API service, assert initial status false, PUT both, list both, DELETE both, and assert final status/list removal.
- Frontend typecheck must prove `FavoriteTargetType='api-service'` maps back from durable backend `api_service`.
- Product boundary scan must show no payment, escrow, guarantee, compensation, credential-storage, or credential-delivery semantics added by favorites.

### 7. Wrong vs Correct

#### Wrong

```go
// Treats any row ID as favorite-able and leaks non-public targets.
INSERT INTO favorites (user_id, target_type, target_id) VALUES ($1, $2, $3)
```

#### Correct

```go
// Validate public visibility before creating the favorite.
if appErr := ensureFavoriteTargetPublic(ctx, tx, targetType, targetID); appErr != nil {
    return appErr
}
```

#### Wrong

```typescript
// Real mode failure hides the backend problem behind mock state.
try { return backendFavorites() } catch { return favoriteStore }
```

#### Correct

```typescript
if (shouldUseRealBackend()) return backendFavorites()
```

## Scenario: Review Center Real Integration

### 1. Scope / Trigger

- Trigger: cross-layer API and database contract for completed carpool membership reviews.
- Scope: the first durable review source is only `carpool_membership`. A review is a buyer-to-owner public experience note after both sides complete a membership. It does not change membership state, create a dispute, create a refund, guarantee service quality, or deliver credentials.

### 2. Signatures

```text
GET /api/v1/me/reviews
PUT /api/v1/me/reviews/carpool-memberships/{membershipId}
GET /api/v1/users/{username}/reviews
```

Required headers:

```text
Cookie: c2c_session=<opaque session id>       # /me routes
X-CSRF-Token: <session CSRF token>            # PUT
Idempotency-Key: <opaque key>                 # PUT
```

### 3. Contracts

- Durable source type is `carpool_membership`; frontend `sourceType='carpool'` may be adapter-only compatibility but must not be persisted.
- `GET /me/reviews` returns `{ items: ReviewCenterRow[] }` for completed memberships where the current user is buyer.
- Review center row fields are `id`, `sourceType`, `sourceId`, `target`, `counterpartyUsername`, `counterpartyName`, `status`, `rating`, `tags`, `note`, `createdAt`, and `updatedAt`.
- Row `status` is `reviewable` when no review exists and `reviewed` after a review exists.
- `PUT /me/reviews/carpool-memberships/{membershipId}` accepts `{ rating, tags, note }` and returns a `ReviewCenterRow`.
- `rating` is integer `1..5`. `tags` are trimmed, de-duplicated, max 5 items, max 16 characters each. `note` is required and max 600 characters.
- Repeated PUT for the same `(source_type, source_id, reviewer_user_id)` updates the existing review instead of creating another public record.
- `GET /users/{username}/reviews` returns `{ items: PublicReview[] }` for reviews where the public user is the reviewee.
- Public review fields are `id`, `username`, `date`, `serviceType`, `rating`, `tags`, `note`, and `verified`.
- Public profile review reads must not expose reviewer user IDs, contact values, contact method IDs, private membership internals, or admin fields.

### 4. Validation & Error Matrix

| Condition | HTTP | Stable code |
| --- | ---: | --- |
| Missing/expired session on `/me` routes | 401 | `SESSION_EXPIRED` |
| Missing or wrong CSRF token on PUT | 403 | `CSRF_TOKEN_INVALID` |
| Missing PUT idempotency key | 400 | `VALIDATION_FAILED` |
| Same PUT idempotency key, different request body | 409 | `IDEMPOTENCY_KEY_REUSED` |
| Membership not found for the buyer | 404 | `OBJECT_NOT_FOUND` |
| Membership exists but is not completed | 409 | `INVALID_STATE_TRANSITION` |
| Reviewer is not membership buyer | 403 or 409 | `PERMISSION_DENIED` or `INVALID_STATE_TRANSITION` |
| Rating outside `1..5` | 422 | `VALIDATION_FAILED` |
| Empty or too-long note | 422 | `VALIDATION_FAILED` |
| Tags/note contain credential-looking content | 422 | `SECRET_CONTENT_DETECTED` |

### 5. Good/Base/Bad Cases

- Good: buyer and owner complete a carpool membership; buyer sees one `reviewable` row, submits a 5-star review, then public owner profile shows that review.
- Base: buyer repeats PUT for the same membership with a new note; the same logical review updates and public profile reflects the latest note.
- Base: replay the exact same idempotency key and request body; response is stable and no duplicate review is created.
- Bad: buyer tries to review an active, left, removed, or pending membership; response is `409 INVALID_STATE_TRANSITION` or not found for unauthorized readers.
- Bad: API purchase intent is used as a review source; it must not enter this route until there is an explicit platform-confirmed completed source model.
- Bad: note includes passwords, API keys, tokens, sessions, cookies, or recovery codes; response is `422 SECRET_CONTENT_DETECTED`.

### 6. Tests Required

- OpenAPI must include all three review routes and schemas: `ReviewCenterRow`, `ReviewCenterRowList`, `SubmitReviewRequest`, `PublicReview`, and `PublicReviewList`.
- Backend tests or smoke must cover completed membership reviewable row, review submission, public profile display, repeated update, and idempotent replay.
- PostgreSQL migration must enforce completed membership and buyer/owner actor consistency through constraints or a constraint trigger.
- Frontend typecheck must prove real mode `getReviewCenterRows()`, `submitReview()`, and public profile reviews use the backend adapter without silent mock fallback.
- Product boundary scan must show no payment, escrow, guarantee, compensation, credential-storage, or credential-delivery semantics added by reviews.

### 7. Wrong vs Correct

#### Wrong

```go
// Lets any source ID become a public review.
INSERT INTO carpool_reviews (source_id, reviewer_user_id, reviewee_user_id, rating, note)
VALUES ($1, $2, $3, $4, $5)
```

#### Correct

```go
// Lock and verify the completed membership before upsert.
membership, appErr := lockCompletedCarpoolMembershipForReview(ctx, tx, input)
if appErr != nil {
    return appErr
}
```

#### Wrong

```typescript
// Hides real backend failures behind mock reviews.
try { return backendReviewCenterRows() } catch { return mockReviewRows() }
```

#### Correct

```typescript
if (shouldUseRealBackend()) return backendReviewCenterRows()
```

## Scenario: Real Native/OAuth Login And Session Permissions

### 1. Scope / Trigger

- Trigger: backend work that changes auth routes, session DTOs, native password login, OAuth provider config, linux.do binding display, production startup validation, or admin permission checks.
- Owner: `backend/internal/config`, `backend/internal/server/auth_handler.go`, `backend/internal/module/auth`, `backend/internal/store/postgres/auth.go`, and `backend/migrations/*native*login*.sql`.

### 2. Signatures

```text
POST /api/v1/auth/password/login
POST /api/v1/auth/email-registration/start
POST /api/v1/auth/email-registration/confirm
GET /api/v1/auth/oauth/start?returnTo=/my
GET /api/v1/auth/oauth/callback?code=<provider-code>&state=<state>
GET /api/v1/auth/session
POST /api/v1/auth/logout
```

Environment contract:

```text
OAUTH_PROVIDER_MODE=fake|oauth2
OAUTH_CLIENT_ID=<required in production oauth2>
OAUTH_CLIENT_SECRET=<required in production oauth2>
OAUTH_AUTHORIZE_URL=<required in production oauth2>
OAUTH_TOKEN_URL=<required in production oauth2>
OAUTH_USERINFO_URL=<required in production oauth2>
OAUTH_REDIRECT_URL=<required in production oauth2>
OAUTH_SCOPES=openid profile
C2C_BOOTSTRAP_ADMIN_USERNAME=<optional first admin username>
C2C_BOOTSTRAP_ADMIN_PASSWORD=<optional first admin password>
```

Session user response includes:

```json
{
  "user": {
    "permissions": ["admin"],
    "linuxDoBinding": {
      "bound": true,
      "linuxDoUserId": "123",
      "linuxDoUsername": "orbit",
      "trustLevel": 3,
      "avatarUrl": "https://..."
    }
  },
  "csrfToken": "csrf_xxx",
  "expiresAt": "2026-06-23T00:00:00Z"
}
```

### 3. Contracts

- `password/login` must validate native credentials through salted hashes in `user_password_credentials`, create the same cookie-backed session contract as OAuth, and return `401 INVALID_CREDENTIALS` for missing users or bad passwords without revealing which field failed.
- New or changed native passwords must write `password_algorithm='argon2id_v1'`. `sha256_salted_v1` is legacy verification-only; a successful legacy login must rehash the credential to `argon2id_v1` before session creation completes.
- Native password login and set-password must require `linuxDoBinding.bound=true` for non-admin users. Admin users may use native password login without linux.do binding only to support the explicit first-admin bootstrap path.
- First-admin bootstrap is environment-driven at process startup. If `C2C_BOOTSTRAP_ADMIN_PASSWORD` is empty, bootstrap is skipped. If password is present and username is empty, username defaults to `admin`. If username is present without password, config loading must fail.
- Bootstrap must create or promote the requested user, grant `user_permissions(permission='admin')`, and write an Argon2id password credential only when no admin password credential exists. Re-running bootstrap after an admin password credential exists must not overwrite credentials.
- `email-registration/start` and `email-registration/confirm` are disabled first-release compatibility endpoints. They return `403 EMAIL_REGISTRATION_DISABLED` and must not create accounts or sessions.
- `start` must store only state plus same-origin `returnTo` in the state cookie. External URLs, protocol-relative URLs, and empty values normalize to `/`.
- `callback` must clear the state cookie after successful login.
- The PostgreSQL auth repository must upsert `users`, `auth_identities`, and `linux_do_bindings` in one transaction before creating the session.
- OAuth userinfo may include an optional `email`. Registration-success email is sent only when the OAuth upsert confirms a newly created user, the provider returned a valid email address, and the user transaction plus session persistence have succeeded. Missing/invalid email skips the registration email; send failure is logged without SMTP credentials and must not block login.
- linux.do userinfo may encode `id`/`sub` as either a JSON string or an integer. Normalize both forms to the same decimal string before identity upsert; malformed non-scalar IDs remain provider-response failures. Operational diagnostics may log only the provider host, path, method, and status/failure category, never the authorization code, access token, query string, or raw response body.
- Admin permission comes from `user_permissions(permission='admin')`; fake OAuth may grant admin only for local smoke identities that intentionally encode admin in the fake code.
- Production startup must fail if `ENABLE_DEV_AUTH=true`, `OAUTH_PROVIDER_MODE=fake`, or required oauth2 endpoint/client values are missing.
- Provider tokens are not part of the durable auth model and must not be written to PostgreSQL.

### 4. Validation & Error Matrix

| Condition | HTTP | Code |
| --- | ---: | --- |
| Bad native username/password | 401 | `INVALID_CREDENTIALS` |
| Native password set/login for non-admin user without linux.do binding | 403 | `LINUX_DO_BINDING_REQUIRED` |
| Legacy `sha256_salted_v1` password login succeeds | 200 plus credential rehash | n/a |
| Bootstrap username set without bootstrap password | startup failure | n/a |
| Bootstrap rerun after admin credential exists | no-op, no overwrite | n/a |
| Email registration start/confirm | 403 | `EMAIL_REGISTRATION_DISABLED` |
| Missing state cookie or state query | 403 | `CSRF_TOKEN_INVALID` |
| State mismatch | 403 | `CSRF_TOKEN_INVALID` |
| Missing callback code | 422 | `VALIDATION_FAILED` |
| Provider token endpoint failure | 502 | `INTERNAL_ERROR` |
| Provider userinfo endpoint failure | 502 | `INTERNAL_ERROR` |
| Production with fake provider | startup failure | n/a |
| Production with dev auth enabled | startup failure | n/a |

### 5. Good/Base/Bad Cases

- Good: linux.do-bound native user login returns the normal session response, while an incorrect password returns `401 INVALID_CREDENTIALS` and creates no session.
- Good: a legacy `sha256_salted_v1` credential logs in once and is persisted back as `argon2id_v1`; the same wrong password does not create a session or rehash.
- Good: first startup with `C2C_BOOTSTRAP_ADMIN_USERNAME=admin` and `C2C_BOOTSTRAP_ADMIN_PASSWORD=<secret>` creates or promotes admin and writes an Argon2id credential; the second startup skips without changing the existing credential.
- Good: email registration start/confirm return `EMAIL_REGISTRATION_DISABLED` and do not set `c2c_session`.
- Good: fake provider smoke logs in `fake-auth-user-*`, session shows `linuxDoBinding.bound=true`, admin route returns `403` for non-admin, and `fake-auth-admin-*` receives `permissions:["admin"]`.
- Base: existing smoke scripts may call `/auth/dev-session` only when `APP_ENV=development|test` and `ENABLE_DEV_AUTH=true`.
- Bad: real frontend mode silently calls `/auth/dev-session` to switch from buyer to admin, exposes email registration as a public sign-up path, lets an unbound non-admin user use backup password, writes new `sha256_salted_v1` credentials, overwrites an existing admin password during bootstrap, or backend stores OAuth access tokens in `auth_identities`.

### 6. Tests Required

- `cd backend && /opt/homebrew/bin/go test ./...` for config, route parity, and auth behavior.
- Auth unit tests must assert Argon2id login success, legacy login plus rehash, wrong password no session/no rehash, Argon2id set-password writes, first-admin bootstrap creation, and bootstrap no-overwrite.
- OAuth profile tests must cover linux.do userinfo with integer `id` and the existing string identifier form, and must assert both normalize to the stable string subject used by `auth_identities` and `linux_do_bindings`.
- OpenAPI YAML parse to verify auth path/schema contract.
- `scripts/auth-smoke.mjs` against PostgreSQL with `OAUTH_PROVIDER_MODE=fake` for start/callback/session/admin/logout.
- Product-boundary scan for token persistence, plaintext password storage, linux.do official endorsement, platform custody, and automatic credential delivery wording.

### 7. Wrong vs Correct

#### Wrong

```go
// Persisting provider tokens creates a credential-custody surface.
saveIdentity(userID, providerSubject, accessToken, refreshToken)
```

#### Correct

```go
// Persist only identity and binding summary; use provider tokens in memory only for userinfo.
upsertIdentity(userID, provider, providerSubject)
upsertLinuxDoBinding(userID, profile)
```

## Scenario: Notification Center Real Integration

### 1. Scope / Trigger

- Trigger: authenticated business notification inbox work.
- Scope: site inbox reads durable rows already written to `notifications` by business transactions and updates `read_at`. These inbox read/update routes must not originate external push, email, SMS, webhook, or ticketing messages. The separate `/me/events` SSE route carries only cache-invalidation topics as defined in the realtime scenario below.

### 2. Signatures

```text
GET  /api/v1/me/notifications
GET  /api/v1/me/notifications/unread-count
POST /api/v1/me/notifications/{id}/read
POST /api/v1/me/notifications/read-all
```

Required headers:

```text
Cookie: c2c_session=<opaque session id>
X-CSRF-Token: <session CSRF token>    # POST read actions
```

### 3. Contracts

- `GET /me/notifications` returns `{ items, nextCursor }` ordered by `createdAt DESC`.
- Notification response fields are `id`, `type`, `title`, `detail`, `targetType`, `targetId`, `to`, `unread`, `readAt`, `createdAt`, and `time`.
- `type` is a frontend-facing business category such as `API 意向`, `上车申请`, `审核结果`, `求车需求`, or `管理操作`; raw event names stay behind the HTTP boundary.
- `unread` is derived from `read_at IS NULL`.
- `POST /me/notifications/{id}/read` updates only the current user's notification and returns 404 when the row is absent or belongs to another user.
- `POST /me/notifications/read-all` updates only current-user unread rows and returns `{ count, items }`, where `count` is the number of rows changed in that call.
- Announcement receipts remain under announcement routes. Do not mix announcement receipts into the business inbox.
- Notification DTOs must not include contact values, passwords, API keys, tokens, sessions, cookies, recovery codes, or credential delivery material.
- Realtime invalidation must not change the notification DTO or act as a durable receipt; clients always refetch this inbox after an invalidation.

### 4. Validation & Error Matrix

| Condition | HTTP | Stable code |
| --- | ---: | --- |
| Missing/expired session | 401 | `SESSION_EXPIRED` |
| Missing or wrong CSRF token on POST | 403 | `CSRF_TOKEN_INVALID` |
| Notification missing or owned by another user | 404 | `OBJECT_NOT_FOUND` |
| Non-empty JSON body with unknown fields on read actions | 400 | `VALIDATION_FAILED` |

### 5. Tests Required

- OpenAPI must include all four notification routes and schemas.
- Backend tests must keep route/OpenAPI parity green.
- Smoke must create a real business action that writes `notifications`, then verify list, unread count, single read, and read-all.
- Frontend real mode must call `notificationBackend.ts` from the existing `api.ts` facade and must not catch real backend failures to return mock notification rows.

## Scenario: Public Search Real Backend Integration

### 1. Scope / Trigger

- Trigger: global search endpoint, backend aggregation, or frontend `/search` real-mode work.
- Scope: public-safe search only. It aggregates existing public official price records, active carpool listings, active demands, public API services, active users, and public-profile API merchants.

### 2. Signatures

```text
GET /api/v1/search?q=<keyword>
```

The endpoint is read-only and public. It does not require session, CSRF, `If-Match`, or `Idempotency-Key`.

### 3. Contracts

- Empty or whitespace-only `q` returns `{ items: [] }`.
- `q` is normalized by trimming/collapsing whitespace and must not exceed 80 characters.
- Response fields are `id`, `type`, `title`, `subtitle`, `badge`, and `to`.
- `type` is one of `官方价格`, `车源`, `求车`, `API 服务`, `用户`, or `商户`.
- Search must reuse existing public predicates: active official price records, active carpool listings, active demands, approved/online/clear API services, active users, and public-profile API merchants only.
- Store-alias API services may appear as `API 服务` results using the public merchant display name, but search must not expose the hidden owner username or create a separate `商户` result for the store alias.
- Search results must not contain contact values, contact method IDs, owner user IDs for store aliases, admin fields, review/moderation reasons, raw report/dispute text, credentials, payment, escrow, guarantee, or fulfillment material.

### 4. Validation & Error Matrix

| Condition | HTTP | Stable code |
| --- | ---: | --- |
| Empty keyword | 200 | n/a |
| Keyword longer than 80 characters | 422 | `VALIDATION_FAILED` |
| Repository failure | 500 | `INTERNAL_ERROR` |

### 5. Tests Required

- OpenAPI must include `GET /api/v1/search` and schemas.
- Backend tests must keep route/OpenAPI parity green.
- Smoke must create or reuse public business records and verify official price, carpool, demand, API service, public user, public-profile merchant, empty keyword, and too-long keyword behavior.
- Frontend real mode must call `searchBackend.ts` from the existing `api.ts` facade and must not catch real backend failures to return mock search rows.

## Scenario: Backend Production Hardening

### 1. Scope / Trigger

- Trigger: backend process startup, CORS/Origin, cookie, OAuth HTTP, rate limit, pagination, direct contact disclosure, idempotency, OpenAPI, or deployment env work.
- Scope: production hardening only. Do not add payment, escrow, guarantee, credential custody, automatic delivery, or API proxy behavior.

### 2. Signatures

```text
GET  /api/v1/auth/oauth/start
GET  /api/v1/auth/oauth/callback
GET  /api/v1/search?limit=20&cursor=<opaque>
GET  /api/v1/api-services?limit=20&cursor=<opaque>
GET  /api/v1/carpools?limit=20&cursor=<opaque>
GET  /api/v1/demands?limit=20&cursor=<opaque>
GET  /api/v1/official-prices?limit=20&cursor=<opaque>
GET  /api/v1/me/notifications?limit=20&cursor=<opaque>
GET  /api/v1/me/favorites?limit=20&cursor=<opaque>
GET  /api/v1/me/api-purchase-intents?limit=20&cursor=<opaque>
GET  /api/v1/me/carpool-applications?limit=20&cursor=<opaque>
GET  /api/v1/me/carpool-memberships?limit=20&cursor=<opaque>
GET  /api/v1/owner/api-services?limit=20&cursor=<opaque>
GET  /api/v1/owner/api-purchase-intents?limit=20&cursor=<opaque>
GET  /api/v1/owner/carpool-applications?limit=20&cursor=<opaque>
GET  /api/v1/owner/carpool-memberships?limit=20&cursor=<opaque>
GET  /api/v1/admin/api-services?limit=20&cursor=<opaque>
GET  /api/v1/admin/api-purchase-intents?limit=20&cursor=<opaque>
GET  /api/v1/admin/carpools?limit=20&cursor=<opaque>
GET  /api/v1/admin/demands?limit=20&cursor=<opaque>
GET  /api/v1/admin/reports?limit=20&cursor=<opaque>
GET  /api/v1/admin/disputes?limit=20&cursor=<opaque>
GET  /api/v1/admin/appeals?limit=20&cursor=<opaque>
```

Protected rate-limit groups:

```text
auth_dev_session, oauth_start, oauth_callback, search,
api_purchase_intent_create, api_purchase_intent_contact_read,
report_create, appeal_create, dev_contact_session, contact_read
```

Production env keys:

```text
APP_ENV=production
DATABASE_URL=<postgres URL>
FRONTEND_ORIGIN=https://app.example.com
ALLOWED_ORIGINS=https://app.example.com[,https://admin.example.com]
TRUST_X_FORWARDED_FOR=false
TRUSTED_PROXIES=<comma-separated proxy IP/CIDR list, required only when forwarding trust is enabled>
OAUTH_PROVIDER_MODE=oauth2
OAUTH_CLIENT_ID=<id>
OAUTH_CLIENT_SECRET=<secret>
OAUTH_AUTHORIZE_URL=<url>
OAUTH_TOKEN_URL=<url>
OAUTH_USERINFO_URL=<url>
OAUTH_REDIRECT_URL=<url>
CONTACT_ENCRYPTION_KEY=<secret>
CONTACT_FINGERPRINT_KEY=<secret>
CONTACT_KEY_VERSION=<version>
EMAIL_PROVIDER=aliyun_directmail
SMTP_HOST=<directmail smtp host>
SMTP_PORT=465
SMTP_USERNAME=<verified sender login>
SMTP_PASSWORD=<directmail smtp password>
MAIL_FROM_ADDRESS=<verified sender address>
MAIL_FROM_NAME=C2CMarket
```

### 3. Contracts

- `cmd/api` must use explicit `http.Server` with `ReadHeaderTimeout=5s`, `ReadTimeout=15s`, `WriteTimeout=30s`, and `IdleTimeout=60s`. It must handle `SIGINT` and `SIGTERM`, call `Shutdown(ctx)` with a bounded timeout, treat `http.ErrServerClosed` as normal, and close the PostgreSQL pool during app cleanup.
- Production cookies for `c2c_session` and OAuth state must use `Secure=true`, `HttpOnly=true`, and `SameSite=Lax`; clear cookies must use matching Path/Secure/SameSite values.
- OAuth token exchange and userinfo requests must use a dedicated `http.Client{Timeout: 10 * time.Second}` or stricter equivalent and must limit JSON response reads to 1 MiB.
- `FRONTEND_ORIGIN` is required in production and is always included in the CORS allowlist. `ALLOWED_ORIGINS` may add other explicit browser origins. Cookie-authenticated CORS responses must echo an allowlisted origin and must not use `Access-Control-Allow-Origin: *`.
- Production unsafe browser methods with an `Origin` outside the allowlist return `403 CSRF_TOKEN_INVALID` before handler logic.
- Production email uses Aliyun DirectMail SMTP over implicit TLS on port 465. Do not use Alibaba Cloud AccessKey or DirectMail API SDK for backend email. SMTP passwords are environment-only secrets and must not be printed in logs, wrapped into errors, or copied into docs beyond placeholder values.
- Email registration uses `email_verification_codes.purpose='email_registration'`, stores only code hashes, creates the verified-email user and auth session in one PostgreSQL transaction, and sends the registration-success email only after commit. Username defaults to the sanitized email prefix and appends a short random suffix on conflict. Email-registered users must return `linuxDoBinding.bound=false` until a separate linux.do binding flow exists.
- Security headers must include `X-Content-Type-Options: nosniff` and `Referrer-Policy: strict-origin-when-cross-origin`; production also sets HSTS. CSP remains a frontend/reverse-proxy concern unless the Go API starts serving pages.
- Request logging must include method, path without query string, status, duration, and request ID. It must not log request bodies, query strings, cookies, CSRF tokens, contact values, passwords, or bearer/API tokens.
- JSON request helpers must reject empty bodies, malformed JSON, unknown fields, bodies over 1 MiB, and trailing JSON values with stable Problem Details. Helpers that only own `request.Body` must use `io.LimitReader`, not `http.MaxBytesReader(nil)`.
- Rate-limit client IP keys must not trust `X-Forwarded-For` or `X-Real-IP` by default. `TRUST_X_FORWARDED_FOR=true` may read forwarding headers only when the immediate `RemoteAddr` belongs to a configured `TRUSTED_PROXIES` IP/CIDR entry; missing or invalid forwarding headers fall back to the direct peer address.
- Rate limits return HTTP `429`, Problem Details `code=RATE_LIMITED`, and `Retry-After` when available.
- Pagination `limit` defaults to 20, maxes at 100, and invalid values return `422 VALIDATION_FAILED`. `cursor` is opaque; clients must only pass through `nextCursor` and must not depend on whether a route currently uses offset or keyset internals.
- List responses using pagination return `{ "items": [...], "nextCursor": "..." }` with `nextCursor` omitted/null when there are no more results.
- Database-backed list repositories should accept `domain.PageRequest` and return `domain.Page[T]`; high-volume lists must use repository-level pagination rather than loading all rows for `server.paginateSlice`.
- API purchase intent create, buyer detail, and owner detail responses that include full contact values must set `Cache-Control: no-store` and write API purchase intent contact access audit rows without plaintext contact values.

### 4. Validation & Error Matrix

| Condition | HTTP | Stable code |
| --- | ---: | --- |
| Production missing or invalid `FRONTEND_ORIGIN` | startup fail | n/a |
| Production dev auth enabled | startup fail | n/a |
| Production fake OAuth provider | startup fail | n/a |
| Configured database migration version below expected version | 503 | readiness JSON with reason |
| Configured database migration dirty flag | 503 | readiness JSON with reason |
| Browser unsafe request from disallowed `Origin` | 403 | `CSRF_TOKEN_INVALID` |
| Empty, malformed, unknown-field, or multi-object JSON body | 400 | `VALIDATION_FAILED` |
| JSON body larger than 1 MiB | 413 | `VALIDATION_FAILED` |
| `TRUST_X_FORWARDED_FOR=true` without `TRUSTED_PROXIES` | startup fail | n/a |
| Invalid `TRUSTED_PROXIES` IP/CIDR entry | startup fail | n/a |
| Rate limit exceeded | 429 | `RATE_LIMITED` |
| Invalid `limit` or `cursor` | 422 | `VALIDATION_FAILED` |
| OAuth state missing/mismatched | 403 | `CSRF_TOKEN_INVALID` |
| OAuth code missing | 422 | `VALIDATION_FAILED` |
| OAuth token/userinfo timeout, oversized body, or provider failure | 502 | `INTERNAL_ERROR` |

### 5. Good/Base/Bad Cases

- Good: production config with `FRONTEND_ORIGIN=https://app.example.com` starts, sets secure session cookies, rejects `Origin: https://evil.example` mutations, rejects malformed/trailing JSON, ignores forged forwarding headers by default, and returns 429 for repeated protected requests.
- Good: a deployment behind a known reverse proxy sets `TRUST_X_FORWARDED_FOR=true` and `TRUSTED_PROXIES=10.0.0.0/24`; only requests from that proxy range use the first valid `X-Forwarded-For` address for rate limiting.
- Good: a configured PostgreSQL deployment whose `schema_migrations.version` equals `ExpectedMigrationVersion` returns `/readyz` 200 with `schemaVersion`, `schemaDirty=false`, and `expectedSchemaVersion`.
- Base: development/test without explicit origins defaults to local Vite origins and keeps cookies non-secure for HTTP local testing.
- Base: no-database local mode returns `/readyz` 200 with `database=not_configured`.
- Bad: production accepts wildcard CORS with cookies, trusts client-supplied `X-Forwarded-For` from the public internet, uses `http.DefaultClient` for OAuth, caches contact-containing responses, logs provider tokens/raw userinfo, logs request bodies/query strings, or reports ready while migrations are dirty or behind the expected version.

### 6. Tests Required

- Config tests for production frontend-origin validation and fake/dev-auth rejection.
- Server tests for production cookie `Secure`, clear-cookie consistency, Origin rejection, strict JSON body rejection, rate-limit `429 RATE_LIMITED`, forged forwarding-header bypass prevention, trusted-proxy forwarding behavior, OAuth oversized response rejection, and pagination validation.
- Readiness tests for configured current schema, configured behind schema, configured dirty schema, database query failure, and no-database local mode. Assertions must cover HTTP status plus `schemaVersion`, `schemaDirty`, `expectedSchemaVersion`, and reason where applicable.
- Request logging tests must prove the log line includes method, path without query string, status, duration, and request ID, and omits request body and query string content.
- Idempotency tests for completed replay, different request hash reuse conflict, non-expired processing conflict, and expired processing retry.
- PostgreSQL integration or smoke assertion that API purchase intent direct contact disclosure writes merchant-side and buyer-side access logs.
- OpenAPI route parity, YAML parse, and docs update for pagination params and `429 RATE_LIMITED`.

### 7. Wrong vs Correct

#### Wrong

```go
http.ListenAndServe(addr, handler)
http.DefaultClient.Do(oauthRequest)
w.Header().Set("Access-Control-Allow-Origin", "*")
log.Printf("request=%s", rawBody)
```

#### Correct

```go
server := &http.Server{
    Addr:              addr,
    Handler:           handler,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       15 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       60 * time.Second,
}
oauthClient := &http.Client{Timeout: 10 * time.Second}
log.Printf("method=%s path=%s status=%d duration=%s request_id=%s", method, urlPath, status, duration, requestID)
```

## Scenario: Feedback Ticket Loop Contract

### 1. Scope / Trigger

- Trigger: backend, OpenAPI, frontend adapter, notification, or admin UI work touching product problem feedback, feedback unread indicators, user supplements, or admin handling.
- Product contract: feedback tickets are for page/product issues, data correction, experience suggestions, and publish/contact blockers. They are separate from reports, disputes, and appeals.
- Storage contract: first version stores page context, associated content, text description, admin response, internal note, and follow-up supplement events only. It does not store screenshots, attachments, or object-storage references.

### 2. Signatures

```text
POST /api/v1/me/feedback-tickets
GET  /api/v1/me/feedback-tickets
GET  /api/v1/me/feedback-tickets/{id}
POST /api/v1/me/feedback-tickets/{id}/supplements
POST /api/v1/me/feedback-tickets/{id}/read
GET  /api/v1/me/feedback-tickets/unread-count

GET  /api/v1/admin/feedback-tickets
GET  /api/v1/admin/feedback-tickets/{id}
POST /api/v1/admin/feedback-tickets/{id}/handle

feedback_tickets:
  submitter_user_id, type, impact, status, title, description
  context_page_label, context_target_type, context_target_id, context_target_label, context_role_label
  admin_response, admin_internal_note, handled_by_admin_id, handled_at
  latest_admin_update_at, submitter_read_at, version

feedback_events:
  ticket_id, actor_user_id, actor_role, action, public_message, internal_note
```

### 3. Contracts

- Feedback statuses are `submitted`, `recorded`, `following_up`, `resolved`, `declined`, `needs_user_info`, and `closed`.
- Feedback types are `function_issue`, `data_correction`, `experience_suggestion`, and `publish_contact_block`.
- Impact values are `general`, `blocks_operation`, and `cannot_continue`.
- User-facing responses must omit `adminInternalNote`, `handledByAdminId`, and other internal-only handling details. Admin responses may include them.
- `contextPageLabel`, `contextTargetLabel`, and `contextRoleLabel` are human-readable product labels. Product UI must not show slash routes, API endpoints, database field names, or debug strings as feedback context.
- Admin handling requires an `If-Match` version precondition and a user-visible `response`. It writes the ticket update, `feedback_events`, `domain_events`, notification, and completed idempotency cache in one transaction.
- Any admin handling response sets `latest_admin_update_at` and clears `submitter_read_at`, making the ticket unread for its submitter until the user opens the feedback detail or marks it read.
- `POST /api/v1/me/feedback-tickets/{id}/read` sets `submitter_read_at` and marks matching feedback notifications read.
- Notifications for feedback use `target_type=feedback_ticket` and target URL `/my/feedback/{id}`. The frontend red dot must be derived from feedback unread count, not from all pending feedback count.

### 4. Validation & Error Matrix

| Condition | HTTP | Stable code |
| --- | ---: | --- |
| Missing or invalid feedback type/impact/status | 422 | `VALIDATION_FAILED` |
| Description or admin response too short | 422 | `VALIDATION_FAILED` |
| Submitter reads or supplements another user's ticket | 404 | `NOT_FOUND` |
| User supplements a closed ticket | 409 | `INVALID_STATE_TRANSITION` |
| Admin handles a closed ticket | 409 | `INVALID_STATE_TRANSITION` |
| Missing `If-Match` on admin handle | 428 | `PRECONDITION_REQUIRED` |
| Stale admin handle version | 412 | `VERSION_CONFLICT` |

### 5. Good/Base/Bad Cases

- Good: admin marks a ticket as `needs_user_info`, writes a clear response, the submitter sees a red dot in the avatar dropdown, supplements the ticket, and the ticket returns to the admin queue.
- Base: user submits a `data_correction` ticket with `contextPageLabel=API 服务详情` and `contextTargetLabel=小葵 API 服务`; admin sees the page/content labels without any route or endpoint string.
- Bad: user UI shows `/api/v1/me/feedback-tickets`, `/api-market/a1`, `context_target_id`, database column names, or an upload/screenshot control in the first feedback version.

### 6. Tests Required

- Backend route tests for create, list/detail isolation, admin handle, unread count, mark-read, user supplement, and closed-ticket rejection.
- OpenAPI route parity and YAML parse after adding or changing feedback routes.
- Frontend type/build checks after adding feedback adapter/facade/hooks/pages.
- Source scan of feedback pages for slash routes, endpoint strings, database field names, and screenshot/attachment/object-storage UI copy.

### 7. Wrong vs Correct

#### Wrong

```ts
const unreadFeedback = allTickets.filter(item => item.status === 'submitted').length
const contextLabel = route.fullPath
```

#### Correct

```ts
const unreadFeedback = await getFeedbackUnreadCount()
const contextLabel = 'API 服务详情'
```

## Scenario: Business Email Reminders

### 1. Scope / Trigger

- Trigger: backend work that sends transactional email after an existing business action succeeds.
- Current allowed reminder triggers are limited to: carpool owner accepts an application, and buyer creates an API purchase intent.
- These reminders are separate from the durable `notifications` inbox. Inbox read/update routes must still not send external email, SMS, WebSocket, webhook, or ticketing messages.

### 2. Signatures

```text
profile.EmailSender.SendCarpoolApplicationAccepted(ctx, toEmail, listingTitle, applicationID, joinDeadline)
profile.EmailSender.SendAPIPurchaseIntentCreated(ctx, toEmail, serviceTitle, intentID, buyerNote, createdAt)

No new HTTP routes, OpenAPI schemas, database tables, environment keys, queues, or background workers.
```

### 3. Contracts

- Carpool acceptance sends to the buyer only after the accept action succeeds and only when the buyer profile has a non-empty verified email.
- API purchase intent creation sends to the API service owner only after the intent create action succeeds and only when the owner profile has a non-empty verified email.
- Email sending is best-effort: profile lookup or SMTP send failure is logged with resource IDs and actor IDs, must not include contact values, note bodies, SMTP credentials, cookies, tokens, or request bodies, and must not roll back or block the business operation.
- Idempotency replay must not send duplicate reminder email. The module service should return both the business entity and an explicit `created` / `accepted` flag so the core facade can send only for a new side effect.
- SMTP templates may include public/resource titles, resource IDs, RFC3339 timestamps, reservation deadline, and short buyer note summaries. Templates must use Go `html/template` for HTML escaping and keep text bodies credential-free.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Recipient profile has no email or unverified email | Skip email, business action succeeds |
| Recipient profile lookup fails after business success | Log skip, business action succeeds |
| SMTP sender returns an error | Log failure, business action succeeds |
| Same idempotency key replays a completed action | Replay response without another email |

### 5. Good/Base/Bad Cases

- Good: a carpool owner accepts an application, the buyer has a verified profile email, one buyer reminder is sent with the listing title, application ID, and join deadline.
- Base: the same accept request is replayed with the same `Idempotency-Key`; the response is stable and no second email is sent.
- Bad: API purchase-intent creation sends email to an unverified merchant email, stores the email body in `notifications`, logs buyer note text, or adds a queue/background worker without a dedicated task.

### 6. Tests Required

- SMTP sender unit tests for both reminder methods, including HTML escaping and expected workflow copy.
- Core service tests for verified-recipient send, unverified-recipient skip, idempotent replay no-duplicate behavior, and non-blocking send failure for both reminder families.
- Full backend `go test ./...` after changing `profile.EmailSender` or idempotent service method signatures.

### 7. Wrong vs Correct

#### Wrong

```go
completion, err := acceptApplication(...)
emailSender.SendCarpoolApplicationAccepted(ctx, buyerEmail, title, applicationID, deadline)
return completion, err
```

#### Correct

```go
application, completion, accepted, err := acceptApplication(...)
if err != nil {
	return completion, err
}
if accepted && verifiedEmail != "" {
	sendBestEffortReminder(ctx, application)
}
return completion, nil
```

---

## Scenario: Product Category Uploaded Icon Contract

### 1. Scope / Trigger

- Trigger: changes to product-category persistence, admin category forms, public category responses, or category icon rendering.
- Category icons are small catalog metadata, not a general file-storage subsystem.

### 2. Signatures

```text
product_categories.icon_data_url text NOT NULL DEFAULT ''

ProductCategory / ProductCategoryRequest:
  iconDataUrl: string
```

### 3. Contracts

- Admin create/update accepts an empty string or a PNG/WebP Base64 data URL.
- The frontend limits the original file to 256 KB and supports preview, replacement, and removal.
- The catalog service validates the MIME prefix, Base64 payload, and decoded size again.
- Public and admin category responses always include `iconDataUrl`; empty string means consumers use their built-in default icon.
- PostgreSQL category reads must select and scan `icon_data_url` in the same position. Update both direct `rows.Scan(...)` sites and the shared `scanProductCategory(...)` helper whenever the category projection changes.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Empty icon | Accepted; removes the uploaded icon |
| PNG/WebP at or below 256 KB | Accepted |
| SVG/JPEG/other MIME prefix | `422 VALIDATION_FAILED`, field `iconDataUrl` |
| Invalid Base64 | `422 VALIDATION_FAILED`, field `iconDataUrl` |
| Decoded payload above 256 KB | `422 VALIDATION_FAILED`, field `iconDataUrl` |

### 5. Good/Base/Bad Cases

- Good: admin uploads a 40 KB WebP, saves the category, and public category buttons render the returned data URL.
- Base: legacy category has `iconDataUrl=""` and existing default icon rendering remains unchanged.
- Bad: SQL selects `icon_data_url` but a direct `rows.Scan` still scans the old five-column projection, causing public category reads to return 500.

### 6. Tests Required

- Backend router regression: valid PNG round-trips and SVG is rejected on `iconDataUrl`.
- Full backend `go test ./...` and a real PostgreSQL `GET /api/v1/product-categories` smoke after migration.
- Frontend unit tests for MIME and size limits, plus typecheck and real-mode production build.
- Admin browser smoke when an authenticated admin session is available.

### 7. Wrong vs Correct

#### Wrong

```go
rows.Scan(&category.ID, &category.Code, &category.DisplayName, &category.SortOrder, &category.Active)
```

#### Correct

```go
rows.Scan(&category.ID, &category.Code, &category.DisplayName, &category.IconDataURL, &category.SortOrder, &category.Active)
```

---

## Scenario: Realtime Notification Invalidation And Navigation Badges

### 1. Scope / Trigger

- Trigger: backend work touching notification creation/read state, API-order lifecycle notifications, navigation badges, PostgreSQL realtime triggers, SSE delivery, request-writer middleware, or process shutdown.
- The stream is an authenticated cache-invalidation channel. Durable notifications and REST reads remain the source of truth; this is not browser Web Push or a business-event payload API.

### 2. Signatures

```text
GET /api/v1/me/navigation-badges
GET /api/v1/me/events

PostgreSQL channel: c2c_market_realtime
Internal payloads:
  {"v":1,"audience":"user","userId":"<uuid>"}
  {"v":1,"audience":"admin"}
  {"v":1,"audience":"all"}

Client-visible named SSE events: ready | invalidate
Client-visible data: {"schemaVersion":1,"topics":["all-live"]}
```

`GET /api/v1/me/navigation-badges` returns `generatedAt`, `notificationUnread`, `importantAnnouncementUnread`, `feedbackUnread`, `buyer`, `merchant`, and nullable `admin`. Buyer/merchant fields are `carpoolActions` and `apiOrderActions`; administrator fields are `total`, `officialPrices`, `carpools`, `apiServices`, `feedbackTickets`, and `reports`.

### 3. Contracts

- Navigation badges are scalar PostgreSQL projections, not counts of a paginated frontend list. Non-admin responses must set `admin=null`; `admin.total` is the server-computed sum of the five non-overlapping administrator queues.
- Buyer API-order actions are non-expired `pending_payment`, `payment_issue`, and `delivery_submitted`. Seller actions are `payment_submitted` plus `paid_confirmed`.
- Buyer carpool actions are an unexpired reserved application not yet confirmed by the buyer, plus an active membership where the owner confirmed completion and the buyer did not. Owner actions are `pending_owner`, a reserved application confirmed by the buyer but not the owner, plus an active membership where the buyer confirmed completion and the owner did not.
- API-order transitions write `api_order_events`, a safe `domain_events` row, a deduplicated counterparty `notifications` row, and idempotency completion in one transaction. Notify on payment submit, buyer cancel, seller payment confirmation, delivery submit, buyer completion, counterparty dispute, and payment timeout. `api_order.created` does not notify again because purchase-intent creation already notified the seller.
- Payment-timeout materialization uses its own committed transaction before a payment-read or action transaction that may return a version/state conflict. A rejected request must not roll back the already-due timeout state, event, or buyer notification.
- API-order notification bodies/domain metadata must not contain payment summaries, instructions, QR data, contacts, reasons/evidence, delivery instructions, URLs, usernames, API keys, or passwords. Buyer targets use `/my/api-orders/{orderId}` and seller targets use `/merchant/api-orders/{orderId}`. Purchase-intent notifications target the matching list route, never an order-detail route built from an intent ID.
- Migration triggers publish only routing metadata after commit. Notification insert/read changes are user-scoped; official-price lead, carpool listing, API service, feedback, report, dispute, and appeal mutations are admin-scoped.
- One process-owned dedicated `pgx.Conn` performs `LISTEN`; browser connections subscribe to an in-process capacity-one coalescing Hub. Reconnect performs `LISTEN` again and publishes a broad wake because PostgreSQL notifications are not replayed.
- SSE uses cookie session auth, `Content-Type: text/event-stream`, `Cache-Control: no-cache, no-transform`, `X-Accel-Buffering: no`, `retry: 3000`, initial `ready`, and comment heartbeats. It never serializes `userId` or business data. Clear the server-wide write timeout for the stream, then apply a bounded deadline to every individual event/heartbeat write so a slow client cannot retain a handler forever.
- Every dedicated PostgreSQL connection attempt has a finite timeout before the 1-to-30-second reconnect backoff. Missing badge persistence returns `503`; it must never masquerade as a successful all-zero authoritative summary.
- Request-writer middleware must implement `Unwrap()` so `http.ResponseController` can clear the 30-second write deadline and flush. Application shutdown closes the listener and Hub before `http.Server.Shutdown`, waits for the listener, then closes the store.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Missing/expired session on either GET | `401 SESSION_EXPIRED` |
| Non-admin summary request | `200`, `admin=null` |
| Hub already shutting down | SSE returns `503 INTERNAL_ERROR` before opening |
| Badge repository unavailable | Summary returns `503 INTERNAL_ERROR`, not all-zero success |
| Invalid/unknown PostgreSQL payload field, version, audience, or padded user ID | Ignore signal, log only a sanitized parse error, keep listener alive |
| Listener connection fails | Retry from 1 second up to 30 seconds; REST/polling remains usable |
| Slow subscriber already has a wake pending | Coalesce the new wake without blocking a business transaction |
| Notification insert fails during an API-order transition | Roll back order/event/idempotency changes |

### 5. Good/Base/Bad Cases

- Good: buyer marks an order paid; the same transaction writes a seller notification, PostgreSQL signals that seller, SSE sends `invalidate`, and the browser refetches the summary/order list.
- Base: SSE disconnects during a mutation; reconnect sends `ready`, and the browser refetches authoritative REST state without replay IDs.
- Bad: API-order notification embeds `paymentSummary`, delivery credential, raw dispute reason, or contact value.
- Bad: one PostgreSQL connection is acquired per browser, or the frontend increments badge counts from event payloads.

### 6. Tests Required

- Pure API-order recipient/title/target matrix, including no notification for create and no secret-bearing copy.
- Navigation-badge service tests for non-admin hiding, administrator total recomputation, and missing dependencies.
- Hub routing/coalescing/close tests and strict PostgreSQL payload parser tests.
- Listener reconnect/backoff/re-LISTEN wake/clean shutdown tests.
- SSE authorization, headers, `ready`, user/admin `invalidate`, middleware flush, bounded writes, cancellation, and payload non-leakage tests.
- A timeout regression proving a payment-entry/action conflict cannot roll back the committed timeout state, domain event, and notification.
- PostgreSQL migration smoke must reach the expected migration version and prove both notification user triggers and administrator queue triggers wake an open stream.
- Full `go test ./...`, OpenAPI route parity/YAML parse, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
// Sends private order state and ties browser correctness to a transient event.
hub.Publish(userID, order)
badgeCount++
```

#### Correct

```go
// Coalesced wake only; the browser refetches durable REST projections.
hub.PublishUser(userID)
// SSE data: {"schemaVersion":1,"topics":["all-live"]}
```

## Scenario: API Order Releases Its Purchase Intent

### 1. Scope / Trigger

- Trigger: API purchase-intent status, API order creation, or the `api_purchase_intents` PostgreSQL constraint changes.
- Purpose: an order-backed intent must stop occupying the active `(buyer_user_id, api_service_id)` uniqueness slot, so a buyer can make a later purchase from the same service.

### 2. Signatures

```text
POST /api/v1/me/api-purchase-intents/{intentId}/orders

PostgreSQL:
  api_purchase_intents.status: open | contacted | ordered | buyer_cancelled | owner_closed
  ux_api_purchase_intents_active_buyer_service:
    (buyer_user_id, api_service_id) WHERE status IN ('open', 'contacted')
  ux_api_orders_intent:
    api_orders(api_purchase_intent_id)
```

### 3. Contracts

- Successful order creation must insert the order and update the locked intent from `open|contacted` to `ordered` in one transaction. If either write fails, neither change commits.
- `ordered` is a terminal intent state: its fulfillment is represented only by the linked order. The intent cannot be cancelled, closed, or marked contacted.
- Migration must backfill existing order-backed `open|contacted` rows to `ordered` before the new contract is used.
- A new intent for the same buyer and service is valid once the previous intent is `ordered`; the old intent must still retain its order history and its one-order-only constraint.

### 4. Validation & Error Matrix

| Condition | HTTP / result | Stable code |
| --- | --- | --- |
| A second order is requested for the same intent | 409 | `API_PURCHASE_INTENT_HAS_ORDER` |
| Cancel/close an `ordered` intent | 409 | `API_PURCHASE_INTENT_HAS_ORDER` |
| Create order from cancelled/closed intent without an order | 409 | `INVALID_STATE_TRANSITION` |
| Create a second open/contacted intent before an order exists | 409 | `ACTIVE_API_INTENT_EXISTS` |

### 5. Good/Base/Bad Cases

- Good: buyer creates intent A, creates order A, then creates intent B and order B for the same service; both orders remain visible.
- Base: owner first marks intent A contacted; order A still changes it to `ordered` and retains `contacted_at` as history.
- Bad: order creation inserts an order but leaves intent A as `open`; it permanently blocks subsequent purchases through the active-intent index.

### 6. Tests Required

- Unit test: in-memory order creation marks an active intent `ordered` and still rejects a second order for that same intent.
- Router test: order creation returns normally, intent detail is `ordered`, and cancel/close retain `API_PURCHASE_INTENT_HAS_ORDER`.
- PostgreSQL integration test: first order releases the active-intent slot, then a fresh intent and second order for the same buyer/service succeed.
- Migration smoke/read-only query: no order-backed intent remains `open` or `contacted` after migration.

### 7. Wrong vs Correct

#### Wrong

```text
create order -> insert api_orders row -> leave api_purchase_intents.status=open
```

#### Correct

```text
single transaction -> insert api_orders row + update intent.status=ordered
```

## Scenario: API Order Decimal Inventory And Admin Tracking

### 1. Scope / Trigger

- Trigger: API service quota inventory, API order pricing snapshots, order creation/cancellation/timeout, decimal response fields, or administrator order tracking changes.

### 2. Signatures

```text
api_services.available_usd_allowance numeric(18,6)
api_orders.requested_usd_allowance_snapshot numeric(18,6)
api_orders.cny_per_usd_allowance_snapshot numeric(12,4)
api_orders.pricing_snapshot jsonb
GET /api/v1/admin/api-orders -> APIOrderList
```

### 3. Contracts

- `declaredMaxUsdAllowancePerIntent` is a per-order cap; `availableUsdAllowance` is the service-level remaining inventory. They are never interchangeable.
- Decimal amount, rate, and allowance fields cross HTTP as canonical decimal strings. For `¥0.80 / $1`, an order amount of `¥10.00` freezes `12.500000` USD allowance; no layer may derive `$13` through binary floating-point rounding.
- Creating a metered order atomically decrements `available_usd_allowance` only when sufficient inventory remains. Pending-payment buyer cancellation or payment timeout releases exactly the order snapshot allowance. Payment-submitted and later states do not release inventory through ordinary timeout handling.
- `GET /admin/api-orders` returns order state and immutable decimal snapshots without contact values or `deliveryCredential`.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Requested allowance exceeds available inventory | `409 INVALID_STATE_TRANSITION` with refresh/retry guidance |
| Metered service has missing/negative available allowance | validation failure or non-orderable `quota_sold_out` |
| Two concurrent reservations exceed shared inventory | exactly one succeeds; the other returns conflict |
| Pending order cancellation | reservation is released in the same transaction |
| Admin list caller is not admin | `403 PERMISSION_DENIED` |

### 5. Good/Base/Bad Cases

- Good: two buyers concurrently request `$12.50` from `$20.00`; one order is created and `$7.50` remains.
- Base: a pending order reserves `$20.00`, buyer cancels, and inventory returns by exactly `$20.00`.
- Bad: use the per-order maximum as a fake service balance, or convert decimal strings to `float64` before reservation/comparison.

### 6. Tests Required

- In-memory `-race` test proving concurrent reservations cannot oversell.
- PostgreSQL integration assertions for reserve and pending-cancel release.
- Router test for immutable decimal snapshots and admin list credential non-leakage.
- Migration readiness at version 49, full `go test ./...`, and OpenAPI route/schema parity.

### 7. Wrong vs Correct

#### Wrong

```text
requestedUsd = round(Number(cnyAmount) / Number(rate))
available = declaredMaxUsdAllowancePerIntent
```

#### Correct

```text
requestedUsd = decimalDivide("10.00", "0.8000") // "12.500000"
UPDATE api_services SET available_usd_allowance = available_usd_allowance - requested
WHERE id = service_id AND available_usd_allowance >= requested
```

## Scenario: Authoritative Carpool Application Eligibility

### 1. Scope / Trigger
- Trigger: carpool list/detail/application code changes eligibility, seat availability, risk boundaries, or personal relationship checks.

### 2. Signatures
```text
GET /api/v1/carpools/{listingId}/eligibility -> CarpoolApplicationEligibility
EvaluateApplicationEligibility(EligibilityContext) -> ApplicationEligibility
```

### 3. Contracts
- Codes are `eligible`, `sold_out`, `paused`, `credential_risk`, `owner_action_required`, `already_applied`, `already_member`, and `self_owned`.
- Priority is exactly `credential_risk → owner_action_required → paused → self_owned → already_member → already_applied → sold_out → eligible`.
- Public list DTOs include a generic eligibility projection; authenticated detail reads include user relationships; application creation re-evaluates the same domain function before persistence.
- Public visibility remains unchanged: paused/non-public listings still return 404 from public detail/eligibility routes. PostgreSQL constraints and transactional seat checks remain the concurrency authority.

### 4. Validation & Error Matrix
| Eligibility | Create result |
| --- | --- |
| `eligible` | Continue request/contact/risk validation and create |
| `sold_out` | `409 SEAT_UNAVAILABLE` |
| `already_applied` | `409 ACTIVE_APPLICATION_EXISTS` |
| `already_member` | `409 ACTIVE_MEMBERSHIP_EXISTS` |
| Other blocked codes | `409 INVALID_STATE_TRANSITION` |

### 5. Good/Base/Bad Cases
- Good: a listing with credential risk, an old application, and no seats returns only `credential_risk`.
- Base: an active safe listing with one seat returns `eligible`.
- Bad: UI and create handler independently order risk, ownership, application, and seat checks.

### 6. Tests Required
- Table test for every code and one combined priority case.
- Router test for `eligible`, `self_owned`, and `already_applied` transitions.
- Full Go suite and OpenAPI route/schema parity.

### 7. Wrong vs Correct
#### Wrong
```text
page: status says available; button: local risk check says blocked
```
#### Correct
```text
list/detail/create -> EvaluateApplicationEligibility -> one code/reason/action
```
