# API Probe Connections, Real-Model Health, And Quota Policy

Date: 2026-08-08
Author: Codex

## Scenario: Reusable Real-Model Probes, Frozen Delivery Targets, And Temporary Buyer Tests

### 1. Scope / Trigger

- Trigger: changes to seller probe connections, probe preflight/save behavior, scheduled probe
  execution, public health projection, latency calibration, API delivery validation, the buyer API
  model tester, model-key snapshots, or 5h/daily quota rules.
- Primary owners are `internal/module/apihealth`, `internal/apihealthrunner`,
  `internal/platform/openaiapi`, `internal/module/apiorder`, `internal/module/apimodeltest`, the
  PostgreSQL API-market stores, migrations `000082` and `000092`, OpenAPI, and the matching frontend adapters.
- A seller probe, an order delivery credential, and a temporary buyer test are separate facts.
  They may reuse the stateless OpenAI-compatible adapter, but never share credentials, samples, or
  persistence lifecycles.

### 2. Signatures

```text
POST   /api/v1/owner/api-probe-connections/preflight
GET    /api/v1/owner/api-probe-connections
POST   /api/v1/owner/api-probe-connections
GET    /api/v1/owner/api-probe-connections/{id}
PUT    /api/v1/owner/api-probe-connections/{id}
DELETE /api/v1/owner/api-probe-connections/{id}
POST   /api/v1/owner/api-probe-connections/{id}/verify
POST   /api/v1/owner/api-probe-connections/{id}/preflight

PATCH  /api/v1/owner/api-services/{id}/probe-connection

GET    /api/v1/admin/api-health/latency-calibration
POST   /api/v1/admin/api-health/latency-rules/preview
GET    /api/v1/admin/api-health/latency-rules
POST   /api/v1/admin/api-health/latency-rules

GET    /api/v1/tools/api-model-tester/order-sources
POST   /api/v1/tools/api-model-tester/discover
POST   /api/v1/tools/api-model-tester/test
```

```text
APIProbeConnectionRequest:
  name, baseUrl, credential?, probeModel?, preflightToken?, enabled,
  acknowledgeInsecureHttp

APIProbeConnectionPreflight:
  errorCode, availableModels[], probeModel, probeProtocol, probeEnvironment,
  dailyBaseCostUpperBoundUsd, priceUnavailable, preflightToken

OwnerAPIProbeConnection:
  id, name, baseUrl, credentialConfigured, enabled, verificationStatus,
  verifiedAt, lastVerificationErrorCode, probeModel, probeProtocol,
  availableModels[], probeEnvironment, probeModelChangedAt,
  dailyBaseCostUpperBoundUsd, priceUnavailable, measurementVersion, version,
  referencedServices[], healthSummary, createdAt, updatedAt

ServiceHealthSummary:
  state, availabilityReason, transportSecurity,
  stabilityPercent, finalSuccessPercent, coveragePercent,
  completedCycles, theoreticalSlots,
  firstAttemptSuccesses, retryRecoveries, finalFailures,
  averageTtftMs, p50TtftMs, p95TtftMs, lastSampledAt,
  probeModel, probeProtocol, probeEnvironment, probeEnvironmentLabel,
  probeModelChangedAt, accumulatingSamples, hourlyBuckets[24], cost

APIProbeLatencyCalibration:
  model, protocol, environment, environmentLabel,
  observationStartedAt, observationEndedAt, completeCalendarDays,
  connectionCount, sampleCount, p50TtftMs, p90TtftMs, p95TtftMs,
  p99TtftMs, ready
```

```text
api_probe_connections
api_probe_connection_samples
api_probe_connection_attempts
api_probe_connection_events
api_probe_latency_rules

api_services.probe_connection_id
api_purchase_intents:
  probe_connection_id_snapshot, api_base_url_snapshot,
  normalized_api_base_url_snapshot
api_orders:
  probe_connection_id_snapshot, api_base_url_snapshot,
  normalized_api_base_url_snapshot
```

Runtime environment:

```text
API_HEALTH_RUNNER_ENABLED=true
API_HEALTH_SCAN_INTERVAL=1m
API_HEALTH_PROBE_TIMEOUT=30s
API_HEALTH_MAX_CONCURRENCY=4
API_HEALTH_CLAIM_BATCH_SIZE=50
API_HEALTH_SAMPLE_RETENTION=192h
```

The 192-hour retention preserves at least seven complete UTC calendar days for calibration even
when the current partial day is also present. There is no DNS TXT or HTTP ownership challenge.

### 3. Contracts

#### 3.1 Reusable seller connections and preflight

- A probe connection belongs to one seller and can be bound to any number of that seller's API
  services. A service binds at most one connection. Scheduling is connection-scoped, so shared
  services do not duplicate paid model calls.
- Create and measurement-changing updates use a two-step contract. Preflight first calls
  `GET {BaseURL}/models`, requires an exact model ID from that response, then sends one fixed real
  model request to select a protocol. Responses is preferred; Chat Completions is tried only when
  Responses is definitively unsupported.
- If no model is requested, preflight selects `gpt-5.6-luna` only when that exact ID exists. It does
  not guess aliases. Otherwise the seller must choose from `availableModels` and preflight again.
- A successful preflight issues a random, short-lived, one-time `preflightToken`. The token is bound
  to the owner, connection and expected version when updating, canonical Base URL, credential
  fingerprint, selected model, and selected protocol. Create/update consumes it instead of issuing
  a second paid verification request. Missing, expired, reused, or mismatched tokens return a
  recoverable `preflightToken` field error.
- Preflight tokens and credentials are private, `no-store`, never projected publicly, and never
  persisted into analytics or probe samples. An explicit `verify` action may execute a fresh
  verification without exposing the stored credential.
- Base URLs are trimmed but otherwise preserved for display and order snapshots. Never append
  `/v1`, change case, or rewrite the business path. A separate canonical value normalizes only the
  scheme, host, default port, and trailing slash for strict target comparison.
- A measurement identity change means Base URL, credential, model, protocol, or probe environment
  changed. It increments `measurementVersion`. Model/protocol changes also append permanent change
  history and keep old samples isolated from current statistics.
- Create, update, verify, enable, disable, and delete are idempotent commands. Their connection
  mutation, one append-only `api_probe_connection_events` row, and idempotency completion commit in
  the same PostgreSQL transaction. Update and delete require `Idempotency-Key` in addition to
  `If-Match`; completed replay returns the stored response without another network verification.
- Preflight and explicit verify perform external HTTP before the short mutation transaction. A
  transaction never stays open across network I/O. Verification then atomically persists the
  success/failure state, its safe event, and idempotency completion.
- Probe events contain actor/request identifiers, action, versions, timestamps, and safe field-name
  metadata only. Credentials, Base URLs, preflight tokens, provider responses, and request bodies
  never enter the ledger. The legacy model-change view remains write-compatible while the event
  ledger is authoritative.
- The owner UI shows exact model IDs, such as `gpt-5.6-luna`. It shows the conservative 1.00x daily
  base-cost upper bound for 288 scheduled requests when the model catalog has a current price;
  missing pricing is explicitly unknown.
- API service and limited-quota publish workflows create a missing probe connection through the
  shared `ApiProbeConnectionDialog` without routing away from the publish page. The dialog owns the
  same preflight/save form used by the connection-management page; publish pages own only the open
  state and selection result.
- A successful inline create refreshes the owner connection query through the existing mutation,
  closes the dialog, assigns only the returned connection ID to the current publish form, clears
  that field's error, and preserves the current step and every other draft field. Inline creation
  requires the connection to be enabled. An existing same-target connection can be reused inline
  only when it is enabled and verified.
- A failed preflight or save keeps the dialog and its inputs open. The publish workflow must not
  encode its draft in a route query, navigate to `/my/api-probe-connections`, or use storage as a
  handoff mechanism.

#### 3.2 Scheduled real-model runner

- Every enabled, verified, credentialed connection is claimed at most once per five-minute slot.
  HTTP execution happens outside the claim transaction; finalization updates only running rows.
- Each cycle sends exactly one protocol chosen at preflight. It never probes both protocols and
  never silently changes protocol at runtime.
- The request is fixed by the platform: `Reply with OK.`, no additional system/instructions,
  streaming enabled, storage disabled, and at most 32 output tokens. Seller input cannot change the
  request shape.
- TTFT is measured from request dispatch to the first non-empty visible text delta. A probe succeeds
  only after a valid stream completes. The output need not equal `OK`; response text is never stored.
- Each cycle retries at most once. Network/connect failures, first-text timeout, HTTP 429,
  HTTP 500/502/503/504, and interrupted valid streams are retryable. Authentication, model,
  parameter, protocol, malformed-response, and other deterministic failures are not retried.
- Retry delay uses a valid `Retry-After` capped at three seconds; otherwise it is randomized between
  one and three seconds. Each attempt is bounded by the configured timeout or the active rule's
  lower hard-timeout limit.
- Every attempt stores HTTP status, error code, TTFT, total duration, retryability, and available
  usage counters without storing the response body. Unknown usage remains unknown. Base and retry
  cost are tracked separately using the connection's price snapshot.
- Every network dial re-resolves and validates all DNS answers. Redirects and environment proxies
  remain disabled. HTTP requires the seller's explicit risk acknowledgement.
- Runtime configuration defaults the runner to enabled. Health summaries receive the runner's
  enabled and last-successful-scan facts. An enabled connection reports `runner_disabled` when the
  runner is off and `stale` when both scheduling and samples stop advancing; it must not remain
  silently grey.

#### 3.3 Twenty-four-hour health projection

- Current health includes only the connection's active `measurementVersion` and the 24-hour window.
  It exposes 24 ascending UTC hourly buckets and a theoretical maximum of 288 five-minute cycles.
- `stabilityPercent` is first-attempt successes divided by completed cycles. Retry recovery does not
  inflate stability. `finalSuccessPercent` includes first successes and retry recoveries and is used
  only for severe hourly-health decisions.
- Average, P50, and P95 TTFT include only first-attempt successes, including first successes marked
  slow by a published fixed rule. Retry recovery duration is separate and never enters TTFT
  distributions.
- Hour state is grey with no completed sample; green when every sample succeeds first try with no
  fixed-rule slow result; yellow for retry recovery, a limited final failure, or fixed-rule slow
  success; red when final success is below 80% or the current sequence contains two consecutive
  final failures. Consecutive failure detection crosses hour boundaries.
- Color never uses relative seller ranking or a live percentile. No published latency rule means a
  slow but successful first attempt remains green while its TTFT is still recorded.
- Public cards show stability, average TTFT, coverage, and the 24-hour strip. Details/tooltip show
  model, protocol fallback, US West environment, detection counts, P50/P95, recovery/failure counts,
  cost facts, and per-hour detail.
- Fewer than three current-version samples is `accumulatingSamples` and stays grey. A model change is
  disclosed for 24 hours, and old/new samples never mix.
- Public wording describes end-to-end measurements from the platform's US West VPS. It does not
  claim official model identity, an SLA, or equal latency for every buyer region.

#### 3.4 Fixed latency calibration and rules

- Calibration dimensions are exact `(model, protocol, environment)` tuples. V1 environment is
  `us-west-v1`; changing the outbound region requires a new environment/version and fresh data.
- Calibration uses only successful first-attempt TTFT. Retry recoveries and failures remain health
  facts but do not enter P50/P90/P95/P99.
- A dimension is ready only after at least seven complete UTC calendar days and five independent
  connections. Insufficient data keeps observation mode active and cannot make successful calls
  yellow for speed.
- Readiness never publishes a rule automatically. An administrator supplies `0 < X < Y <= 30000`,
  previews slow/over-timeout impact, and explicitly publishes an immutable version with the exact
  observation window, counts, percentiles, actor, and timestamp.
- Publishing supersedes the prior active rule for the same dimension transactionally. A new sample
  snapshots the active rule ID; historical samples are never recolored.
- With a published rule, TTFT `<= X` is normal, `X < TTFT <= Y` is a successful slow result, and an
  attempt exceeding `Y` is cancelled and enters the normal failure/retry policy.

#### 3.5 Service and order target snapshots

- A service can be published or accept new standard/package/quota orders only while its bound
  connection is enabled and verified. Publication and order creation use the same persisted
  readiness predicate.
- Standard purchase-intent creation and limited-quota order creation freeze the connection ID,
  seller-entered Base URL, and canonical comparison value. Standard order creation copies the
  intent snapshot instead of rereading the current service.
- Seller `api_key_endpoint` delivery must provide a Base URL canonically equal to the frozen order
  target. A domain and one of its resolved IPs are intentionally different targets; neither is
  rewritten or granted an automatic `/v1`.
- Before accepting delivery, the backend calls `/models` with the delivered buyer key. This checks
  current authentication and list compatibility only; it does not copy the buyer key into the
  seller probe or claim every listed model works.

#### 3.6 Temporary buyer model tester and quota policy

- The tester accepts manual credentials or one of the current buyer's eligible delivered orders.
  An order source exposes metadata and Base URL only; the key remains server-side and is reauthorized
  and decrypted for each request.
- `discover` returns every unique non-empty model ID from `/models`. `test` accepts a discovered ID
  and independently tests Responses and Chat Completions. Results and manual keys stay in page
  memory and never change order, dispute, completion, or public-health state.
- The frontend supports one-model, selected-model, and all-model actions with at most three model
  tests in flight. There is no product cooldown because the buyer owns the credential and quota.
- `modelKey` is the canonical catalog and request identifier. Service/package DTOs expose
  `modelKeySnapshot`; intent/order pricing snapshots use `models[].modelKey`. Decorative model names
  and compatibility aliases must not return.
- New 5h/daily quota limits must be explicitly `limited` with a positive decimal amount or
  `unlimited` without an amount. Intent/order creation freezes the selected quota policy and prompt
  audit declaration.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| HTTP target lacks current acknowledgement | `422 VALIDATION_FAILED` on `acknowledgeInsecureHttp` |
| Preflight lacks a credential on create | `422 VALIDATION_FAILED` on `credential` |
| Requested model is not an exact `/models` ID | Preflight returns `model_unavailable`; no token is issued |
| Responses is explicitly unsupported, Chat works | Fix `openai_chat_completions_v1`; disclose Chat fallback |
| Responses gets auth, rate, network, or upstream failure | Do not hide it by falling back to Chat |
| Save omits, reuses, expires, or mutates a preflight token binding | `422 VALIDATION_FAILED` on `preflightToken` |
| Inline publish create succeeds with an enabled verified connection | Close dialog, refresh connections, select the returned ID, preserve the publish step and draft |
| Inline publish create fails | Keep dialog and inputs open; publish page and draft remain unchanged |
| Same-target connection is disabled or unverified during inline create | Show it as unavailable for reuse; require a ready connection or a separately verified create |
| Target is malformed, private, mixed-DNS, or redirecting | Stable target/network error; never dial the rejected address |
| Referenced connection delete | `409 INVALID_STATE_TRANSITION` with service references |
| Stale connection mutation or preflight version | `412 VERSION_CONFLICT`; missing `If-Match` is `428` |
| Create/update/verify/delete omits an idempotency key | `400 IDEMPOTENCY_KEY_REQUIRED`; no connection or event write |
| Same completed mutation key is replayed | Return the stored completion; do not dial, mutate, or append another event |
| Runner is configured off for an enabled verified connection | `availabilityReason=runner_disabled` |
| Runner scan and latest sample are both stale | `availabilityReason=stale` |
| Retryable first attempt recovers | `retry_recovered`, yellow, stability still counts first attempt as failed |
| Deterministic 400/401/403/model/protocol error | One attempt, `final_failure`, no second paid request |
| Rule calibration has fewer than 7 complete days or 5 connections | `ready=false`; publish rejected with `calibration_incomplete` |
| Thresholds do not satisfy `0 < X < Y <= 30000` | `422 VALIDATION_FAILED` |
| Delivery swaps domain/IP, scheme, port, or business path | `422 VALIDATION_FAILED` on delivery Base URL |
| Tester order is missing/foreign | `404`; do not reveal existence |
| New quota mode/amount is inconsistent | `422 VALIDATION_FAILED` |

### 5. Good / Base / Bad Cases

- Good: one seller preflights `gpt-5.6-luna`, saves with the returned token, and binds the same
  connection to three services. One real Responses request is scheduled per slot and all three
  services display the same health strip.
- Good: Responses is explicitly unsupported during preflight, Chat succeeds, and all later cycles
  use only the fixed Chat protocol until the seller revalidates a new measurement version.
- Good: 288 planned requests are shown as the base daily estimate; a rare 429 retry is separately
  visible as recovery and retry cost.
- Base: calibration has eight retained days but only four independent connections. TTFT percentiles
  remain visible, `ready=false`, and slow successful calls are not colored yellow.
- Base: a verified connection is disabled. Existing orders remain intact, runner claims stop, and
  bound services cannot receive new orders until re-enabled and freshly verified.
- Good: a seller reaches the probe step while publishing a service, creates and verifies a
  connection in the inline dialog, and continues on the same step with that connection selected
  and all model, pricing, payment, and quota fields unchanged.
- Base: inline save fails. The seller corrects the dialog input and retries without rebuilding the
  publish draft.
- Bad: save repeats the paid protocol verification after a successful preflight, runs both protocols
  every five minutes, or creates one probe row per bound service.
- Bad: the publish page links to `?create=1`, duplicates the probe form, or reuses a disabled or
  unverified same-target connection.
- Bad: dynamically color the slowest sellers yellow, mix TTFT from different model/protocol/environment
  versions, infer domain/IP equality from DNS, or persist buyer tester keys/results.

### 6. Tests Required

- Domain/service: default Luna selection, exact model matching, Responses-first fixed fallback,
  preflight token issue/binding/expiry/one-time use, measurement version changes, retry classification,
  fixed request contract, TTFT distribution, 24-hour buckets, cross-hour consecutive failures,
  cost/unknown usage, and runner disabled/stale projection.
- OpenAI adapter: Base URL preservation, `/models` deduplication, Responses and Chat SSE parsing,
  first visible text, normal completion, stream interruption, usage extraction, response bounds,
  error mapping, and no redirect/proxy behavior.
- PostgreSQL: owner isolation, same-slot claim deduplication, attempts/finalization atomicity, rule
  snapshotting, append-only connection-event/model history, mutation/event/idempotency fault rollback,
  completed replay without another verify call, seven-complete-day calibration, five-connection readiness, empty
  calibration count, threshold preview, immutable publication, advisory-lock types, and 192-hour
  retention.
- Handlers/contracts: CSRF, idempotency, `If-Match`, private/no-store preflight, no credential
  projection, admin authorization, OpenAPI route/status/type parity, and generated frontend types.
- Frontend: preflight then save without duplicate verification, model selection, default Luna,
  price unknown state, Chat disclosure, inline create from both publish workflows without route
  navigation, returned-ID selection without draft reset, failed-save dialog retention, unavailable
  duplicate-reuse blocking, 24-hour strip and tooltip, runner warnings, calibration preview/publish,
  responsive desktop/mobile layout, and no mock fallback in real mode.
- Full gates: `go test ./...`, `go vet ./...`, PostgreSQL migration/integration, full Vitest,
  frontend typecheck and production build, OpenAPI checks, migration-doc guard, browser QA, and
  `git diff --check`.

### 7. Wrong vs Correct

```go
// Wrong: save pays for the same protocol check again.
preflight := verifier.Verify(baseURL, key, model)
connection := service.Create(input) // verifier.Verify runs again

// Correct: a bound, one-time token carries the successful preflight fact into save.
preflight := service.Preflight(input)
input.PreflightToken = preflight.PreflightToken
connection := service.Create(input)
```

```go
// Wrong: issue one real-model probe for every service that references a connection.
for _, service := range services {
	probe(service.ProbeConnectionID)
}

// Correct: claim the reusable connection once per five-minute slot.
for _, job := range claimDueConnections(slot) {
	probeFixedModelAndProtocol(job)
}
```

```text
Wrong:  full-site P95 changes every seller's color dynamically
Correct: full-site percentiles only inform an explicitly published, immutable X/Y rule version
```
