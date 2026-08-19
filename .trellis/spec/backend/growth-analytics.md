# Registered-User Growth And Umami Analytics

Date: 2026-08-02
Executor: Codex

## Scenario: Authoritative Growth Dashboard And Diagnostic Browser Analytics

### 1. Scope / Trigger

- Trigger: changes to registration, authenticated activity, activation, retention, completed-transaction metrics, registration attribution, the administrator growth API/dashboard, or Umami identity/events.
- PostgreSQL owns registered-user and business KPI truth. Umami owns diagnostic traffic and funnel behavior; Umami Visitors are never treated as registered users.
- This is a cross-layer contract covering migration `000073_growth_analytics`, the backend `growth` module, auth/session projection, OpenAPI/generated types, `/admin/growth`, and the browser analytics wrapper.

### 2. Signatures

```http
GET /api/v1/admin/growth-overview?days=7|30|90
Cookie: c2cmarket_session=<opaque>
```

```go
const BusinessTimezone = "Asia/Shanghai"

type Repository interface {
	GrowthOverview(ctx context.Context, asOf time.Time, windowDays int) (Overview, *domain.AppError)
	RecordActivity(ctx context.Context, userID, activityDate string, seenAt time.Time) *domain.AppError
}

func (s *Service) AdminOverview(ctx context.Context, user auth.User, windowDays int) (Overview, *domain.AppError)
func (s *Service) RecordActivity(ctx context.Context, userID string) *domain.AppError
```

```sql
users.analytics_user_id uuid NOT NULL DEFAULT gen_random_uuid() UNIQUE

user_registration_attributions(
  user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  registration_method text,
  source_type text,
  source text,
  medium text NULL,
  campaign text NULL,
  referrer_host text NULL,
  landing_path text,
  captured_at timestamptz
)

user_activity_daily(
  user_id uuid REFERENCES users(id) ON DELETE CASCADE,
  activity_date date,
  first_seen_at timestamptz,
  PRIMARY KEY (user_id, activity_date)
)

carpool_listings.first_published_at timestamptz NULL
api_services.first_published_at timestamptz NULL
```

```ts
type GrowthWindowDays = 7 | 30 | 90

function backendAdminGrowthOverview(days: GrowthWindowDays): Promise<GrowthOverview>
function growthOverviewQueryKey(days: GrowthWindowDays): readonly ['admin', 'growth-overview', GrowthWindowDays]
function identifyAnalyticsUser(analyticsUserId: unknown): void
function clearAnalyticsIdentity(): void
function trackAnalytics(eventName: AnalyticsEventName, props?: Record<string, unknown>): void
```

### 3. Contracts

#### Registered-user truth

| Metric | Authoritative fact |
| --- | --- |
| Registration and cohort membership | `users.created_at`; this is the only registration timestamp used for growth KPIs. |
| Cumulative effective users | Users whose current `account_status <> 'archived'` at the snapshot. |
| Qualified activity | One `user_activity_daily` row per authenticated user and Shanghai calendar date. Background polling, health, auth callback/session, logout, and asset routes do not create activity. |
| Buyer activation | Earliest `carpool_applications.created_at` or `api_orders.created_at`, within registration through registration plus 7 times 24 hours. |
| Seller activation | Earliest immutable `carpool_listings.first_published_at` or `api_services.first_published_at`, within registration through registration plus 7 times 24 hours. |
| Completed carpool transaction | Retired compatibility metric; migration 111 removed the `completed` membership state, so the value remains zero until a replacement activation metric is explicitly designed. |
| Completed API transaction | `api_orders.status = 'completed'` and `api_orders.completed_at` in the selected window. |

- All business-day boundaries and date labels use `Asia/Shanghai`; storage timestamps remain `timestamptz`/UTC instants.
- `days` defaults to `30`. Supported values are exactly `7`, `30`, and `90`.
- The endpoint uses one read-only repeatable-read transaction and one `generatedAt` value so cards, trends, attribution, activation, and retention reconcile.
- Date series contain exactly `windowDays` ordered rows and are zero-filled when no facts exist.
- Ratios are in `[0, 1]`. An unavailable activation median or retention rate is JSON `null`, never a fabricated zero.
- A D1 cohort is mature only after its complete D+1 Shanghai calendar day has ended (`cohort_date <= today - 2`). A D7 cohort is mature only after its complete D+7 day has ended (`cohort_date <= today - 8`). Headline retention is the weighted ratio across mature cohort users, not an average of daily percentages.
- The registration/cumulative trend counts all registrations through each date. The headline `cumulativeEffectiveUsers` separately excludes currently archived accounts.

#### Durable identity and attribution

- Every user has a randomly generated `analytics_user_id`. It must be unique and must not equal or be derived from the business user UUID, username, email, linux.do identity, contact value, or another public identifier.
- The authenticated session response may expose `analyticsUserId` to the first-party frontend solely for analytics identification. Only this random identifier may be passed to Umami `identify()`.
- Registration attribution is first-touch and create-only. OAuth and email account creation insert it in the same transaction as the user/session; subsequent logins use `ON CONFLICT (user_id) DO NOTHING` and cannot overwrite it.
- `source`, `medium`, and `campaign` are stripped of control characters and limited to 100 Unicode code points. `referrer_host` is host-only, lowercase, and limited to 255 characters. `landing_path` is a normalized low-cardinality route without query, fragment, or dynamic entity ID.
- Historical users are backfilled with an inferable registration method and `unknown` acquisition values. Historical UTM/referrer data must not be invented.
- `first_published_at` is set once by database triggers on the first public transition. Later pause, restore, review, moderation, or direct column update cannot change it.

#### Umami and dashboard

- Automatic Umami page views remain enabled. `normalized_page_view` is an additional event for route-template aggregation.
- Auth funnel events are `login_page_view`, `oauth_login_start`, `login_success`, and `registration_success`. Their properties are limited to an allowlisted auth method and normalized source route.
- `contact_window_reveal` is emitted only after the contact API successfully returns disclosed values. Intent, blocked windows, cancellations, and failed requests do not emit it.
- Analytics is best effort: disabled, blocked, absent, synchronous failure, or asynchronous rejection never changes business behavior.
- Logout, session invalidation, or a rejected analytics ID clears the desired browser analytics identity. Identification starts when an authenticated session is known; the system does not claim retroactive anonymous-to-user stitching.
- `/admin/growth` uses the backend endpoint and generated OpenAPI types. It does not query Umami, synthesize real-mode mock success data, or copy device/browser/country reports into the product dashboard.

### 4. Validation & Error Matrix

| Condition | Expected result |
| --- | --- |
| Missing or expired session | Existing `401` Problem Details session contract. |
| Authenticated non-administrator | `403 PERMISSION_DENIED`. |
| `days` is not an integer or is outside `7,30,90` | `422 VALIDATION_FAILED`, field `days`, code `invalid`. |
| Empty database window | `200`; zero-filled trends, empty attribution, zero counts, and nullable unavailable rates/median. |
| D1/D7 observation day is still in progress | Cohort retained count and rate are `null`; it does not enter the headline denominator. |
| Growth snapshot query fails | Visible backend Problem Details error; the dashboard shows retry UI and no mock fallback. |
| Daily activity insert fails | Log the observability failure; preserve the authenticated business response. |
| Attribution contains a full URL, query, fragment, control character, or unknown path | Normalize/drop unsafe parts before persistence; never persist the raw value. |
| Analytics ID is blank, malformed, or a business ID | Do not identify with it; clear the pending analytics identity. |
| Umami is disabled, missing, blocked, or throws | Continue the login, navigation, disclosure, and business workflow unchanged. |

### 5. Good/Base/Bad Cases

- Good: an OAuth-created user gets a random analytics ID, a single first-touch attribution row, a Shanghai activity row, and a one-time `registration_success` event containing no business identity.
- Good: active/left/removed carpool memberships do not increment the retired completion metric; an API order completed at `completed_at` increments the API transaction counter.
- Good: on 2026-08-02 Shanghai time, the 2026-08-01 D1 and 2026-07-26 D7 observation days remain `null` because those observation days have not ended.
- Base: a period with registrations but no mature cohorts shows registration counts while activation/retention values remain unavailable.
- Bad: use Umami Visitors as the denominator for registration conversion, registered DAU, activation, or retention.
- Bad: reinterpret `left` or `removed` as a completed carpool transaction, or count rows because an end timestamp happens to be populated.
- Bad: call `identify(user.id)`, send email/username/order/listing IDs to Umami, persist a full referrer URL, or overwrite first-touch attribution on login.

### 6. Tests Required

- Migration contract: schema, constraints, conservative backfill, trigger creation, and down-migration dependency order.
- PostgreSQL migration gate: empty database migrates through version 72; local `72 -> 71 -> 72` round trip succeeds.
- Growth service unit tests: administrator authorization, default/allowed/invalid windows, Shanghai dates, empty output, and activity date conversion.
- PostgreSQL integration tests: empty snapshot, registration/cumulative series, attribution grouping, activity deduplication, buyer/seller activation boundaries, median hours, completed transaction columns/statuses, mature and in-progress D1/D7 cohorts, weighted rates, immutable first publication, random analytics ID, first-touch attribution, and repeatable snapshots.
- Server/OpenAPI tests: route registration, admin-only response, `days` validation, session `analyticsUserId`, OpenAPI route parity, and generated type drift.
- Frontend tests: 7/30/90 query keys, loading/error/empty/null states, route/navigation guard, normalized paths, allowlisted event properties, attribution bounds, queued identity/clear behavior, analytics failure no-op, and successful-only contact reveal.
- Full local gates: `go test ./...`, `go vet ./...`, full frontend tests, frontend type-check, real-backend production build, OpenAPI checks, PostgreSQL integration script, `git diff --check`, and desktop/390px browser verification.

### 7. Wrong vs Correct

#### Wrong

```sql
SELECT count(*)
FROM carpool_memberships
WHERE completed_at >= $1 AND completed_at < $2;
```

```ts
window.umami?.identify(session.user.id)
```

```sql
CASE WHEN cohort_date <= today - 7 THEN d7_retained END
```

#### Correct

```sql
SELECT count(*)
FROM carpool_memberships
WHERE status = 'completed'
  AND ended_at >= $1
  AND ended_at < $2;
```

```ts
identifyAnalyticsUser(session.user.analyticsUserId)
```

```sql
CASE WHEN cohort_date <= today - 8 THEN d7_retained END
```
