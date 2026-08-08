# API Probe Connections, Model Testing, And Quota Policy

Date: 2026-08-08
Author: Codex

## Scenario: Reusable Seller Probes, Frozen Delivery Targets, And Temporary Buyer Tests

### 1. Scope / Trigger

- Trigger: changes to seller probe connections, API service publication/orderability, probe
  execution, public health projection, API delivery validation, the API model tester, model-key
  snapshots, or 5h/daily quota rules.
- Primary owners are `internal/module/apihealth`, `internal/apihealthrunner`,
  `internal/platform/openaiapi`, `internal/module/apiorder`, `internal/module/apimodeltest`, the
  PostgreSQL API-market stores, migration `000081`, OpenAPI, and the matching frontend adapters.
- Probe health, order delivery credentials, temporary model-test credentials, and SKU quota policy
  are separate facts. They may share the stateless OpenAI-compatible HTTP adapter, but never share
  secrets, samples, or persistence lifecycles.

### 2. Signatures

```text
GET    /api/v1/owner/api-probe-connections
POST   /api/v1/owner/api-probe-connections
GET    /api/v1/owner/api-probe-connections/{id}
PATCH  /api/v1/owner/api-probe-connections/{id}
DELETE /api/v1/owner/api-probe-connections/{id}
POST   /api/v1/owner/api-probe-connections/{id}/verify

PATCH  /api/v1/owner/api-services/{id}/probe-connection

GET    /api/v1/tools/api-model-tester/order-sources
POST   /api/v1/tools/api-model-tester/discover
POST   /api/v1/tools/api-model-tester/test
```

```text
APIProbeConnection:
  id, name, baseUrl, credentialConfigured, enabled, verificationStatus,
  verifiedAt, lastVerificationErrorCode, measurementVersion, version,
  referencedServices[], healthSummary, createdAt, updatedAt

ServiceHealthSummary:
  state, availabilityReason, successRatePercent, successfulSamples,
  totalSamples, transportSecurity, lastSampledAt, samples[12]

APIModelTesterCredentialSource:
  manual { kind, baseUrl, apiKey, acknowledgeInsecureHttp }
  order  { kind, orderId, acknowledgeInsecureHttp }

QuotaUsagePolicy:
  fiveHour, daily, scope: per_buyer_credential,
  dailyReset: utc_plus_8_calendar_day
```

```text
api_probe_connections
api_probe_connection_samples

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
API_HEALTH_RUNNER_ENABLED
API_HEALTH_SCAN_INTERVAL
API_HEALTH_PROBE_TIMEOUT
API_HEALTH_MAX_CONCURRENCY
API_HEALTH_CLAIM_BATCH_SIZE
API_HEALTH_SAMPLE_RETENTION
```

There is no challenge TTL. DNS TXT, HTTP challenge, and administrator probe approval are removed.

### 3. Contracts

#### 3.1 Reusable seller connections

- A probe connection belongs to one seller and can be bound to any number of that seller's API
  services. A service binds at most one connection. Cross-owner reads and bindings behave as not
  found or unavailable and must not reveal another seller's connection.
- Creation requires a name, complete Base URL, dedicated Bearer key, enabled choice, and explicit
  HTTP acknowledgement when applicable. It immediately performs authenticated
  `GET {BaseURL}/models`; only HTTP 2xx with an OpenAI-compatible `data[]` envelope verifies the
  connection. Model IDs are not stored or compared with service declarations.
- Verification proves only that the seller supplied a working Endpoint + Key and authorized the
  platform to use that key. It does not prove ownership of an IP, domain, server, upstream account,
  or official model.
- Base URLs are trimmed but otherwise preserved for display and order snapshots. Never append
  `/v1`, change case, or rewrite the business path. A separate canonical value normalizes scheme,
  host, default port, and trailing slash only for strict comparison.
- Changing the Base URL or key, or re-enabling a disabled connection, performs a fresh `/models`
  verification and increments `measurementVersion`. A failed verification leaves the connection
  disabled/unverified and pauses every bound service from taking new orders. It does not alter old
  orders.
- Connection writes require session and CSRF. PATCH/DELETE require `If-Match`; create and explicit
  verify require `Idempotency-Key`. Responses are `private, no-store` and never include the key or
  its fingerprint.
- Deleting a referenced connection returns `409` with the affected service references. Disabling
  remains allowed and immediately stops scheduling and new-order eligibility. Deleting an
  unreferenced connection cascades only its samples.

#### 3.2 Runner and public health

- The runner performs only authenticated `GET {BaseURL}/models`. It never specifies or tests a
  model, never calls Responses or Chat Completions, and never records response bodies or TTFT.
- Each enabled, verified, credentialed connection is claimed at most once per five-minute slot,
  regardless of how many services reference it. HTTP executes outside the claim transaction.
  Abandoned running rows converge to `internal_timeout`; finalization updates only running rows.
- Every dial re-resolves and validates all DNS answers. Private, loopback, link-local, metadata,
  special-use, and mixed public/private results are rejected. Environment proxies and redirects
  are disabled. These protections remain active for HTTP and HTTPS.
- A service reads the summary of its currently bound connection. Multiple services bound to the
  same connection therefore share the same current sample set without duplicate outbound calls.
- Public health contains connection authentication availability only. It never exposes Base URL,
  key, model list, configured model, TTFT, or connection ownership claims. Summaries retain the
  fixed 12 ascending five-minute slots and explicit no-sample reasons.
- A health-enrichment read failure degrades only `healthSummary` to temporarily unavailable. It
  must not fail the surrounding list/detail response.

#### 3.3 Service and order target snapshots

- A service can be published or accept new standard/package/quota orders only while its bound
  connection is enabled and verified. Publication and order creation check the same persisted
  readiness predicate.
- Standard purchase-intent creation and limited-quota order creation freeze the connection ID,
  seller-entered Base URL, and canonical comparison value. Standard order creation copies the
  intent snapshot rather than reading the current service.
- Seller `api_key_endpoint` delivery must provide a Base URL canonically equal to the frozen order
  target. Scheme, host kind, host value, effective port, and path must match. A domain and one of
  its resolved IPs are intentionally different targets. The seller's submitted spelling is not
  rewritten or given an automatic `/v1`.
- Before accepting delivery, the backend calls `GET {submitted BaseURL}/models` with the delivered
  buyer key. This verifies only current authentication and compatible list structure; it does not
  compare returned models with seller declarations or copy the key into the probe connection.
- Order credentials, probe keys, and temporary tester keys have independent encryption and
  retention boundaries. The periodic runner can read only probe-connection credentials.

#### 3.4 Temporary API model tester

- The tester accepts manual credentials or a current buyer's eligible delivered order. Order
  sources expose order metadata and Base URL only; the key remains server-side and is re-authorized
  and decrypted for every discover/test request.
- HTTP requires `acknowledgeInsecureHttp=true` for both manual and order sources. The UI shows an
  unchecked warning for the current HTTP target and resets it when the source target changes.
- `discover` returns every unique non-empty model ID from `/models` in provider order. It does not
  restrict results to the platform catalog or seller-declared models.
- `test` accepts one discovered model ID and independently performs one minimal non-streaming
  Responses call and one Chat Completions call. Success requires HTTP 2xx, the protocol array
  (`output` or `choices`), and no non-null top-level `error`; response text is never returned.
- The frontend offers one-model, selected-model, and all-model actions, with at most three model
  requests in flight. Each model means two outbound calls. There is no product cooldown because
  the user owns the credential and quota, but normal body, timeout, response-size, and
  infrastructure limits still apply.
- Results and manual keys stay in page memory only. Source change, refresh, navigation, or
  unmount clears them. Do not put keys or results in URL queries, storage, analytics, logs, or
  persistent tables. Testing never changes order, dispute, completion, or public health state.

#### 3.5 Model keys and quota policy

- `modelKey` is the one canonical public catalog name and real request identifier, for example
  `gpt-4.1-mini`. Do not create a decorative display-name variant such as `GPT-4.1 mini`.
- Service/package API DTOs expose canonical names as `modelKeySnapshot`. Intent/order
  `pricingSnapshot.models[]` stores the same canonical identifier as `modelKey`; APIs and frontend
  projections must not restore `displayName` or `modelNameSnapshot` compatibility fields.
- New service/package/offer writes require each 5h/daily limit to be explicitly `limited` with a
  positive decimal amount or `unlimited` without an amount. `unspecified` is historical-read only.
- Intent/order creation freezes the selected SKU quota policy and prompt-audit declaration. Never
  recompute a historical order from the current service, package, or quota offer.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| HTTP source omits current-request acknowledgement | `422 VALIDATION_FAILED` on `acknowledgeInsecureHttp` |
| Base URL contains only an origin | Preserve it; do not append `/v1` |
| Base URL already contains a path | Preserve the path; structured joins append only the endpoint |
| Target is malformed, private, mixed-DNS, or redirecting | Stable target/network error; never dial an unsafe address |
| Connection verification returns invalid `/models` JSON | Connection remains failed/disabled with `invalid_response` |
| Connection is disabled, failed, missing a key, or belongs to another seller | Cannot bind/publish/order |
| Referenced connection delete | `409 INVALID_STATE_TRANSITION` with service references |
| Stale connection mutation | `412 VERSION_CONFLICT`; missing precondition is `428` |
| Delivery target differs only by default port/trailing slash/case | Canonical match succeeds |
| Delivery swaps domain for resolved IP, IP for domain, port, scheme, or path | `422 VALIDATION_FAILED` on delivery Base URL |
| Delivered key cannot authenticate `/models` | Reject delivery without persisting the credential |
| Tester order is missing/foreign | `404`; do not reveal existence |
| Tester credential was destroyed or is unsupported | `409 API_MODEL_TEST_CREDENTIAL_UNAVAILABLE` |
| Tester `/models` gets 429/5xx/timeout | `429`/`502`/`504` with stable problem details |
| Model protocol returns 2xx with non-array `output`/`choices` or top-level error | `invalid_response` for that protocol |
| New quota limit uses `unspecified`, invalid amount, or mismatched mode/amount | `422 VALIDATION_FAILED` |

### 5. Good / Base / Bad Cases

- Good: one seller creates one verified connection and binds three separately sold services. The
  runner emits one sample per slot and all three public services read the same health summary.
- Good: an order freezes `http://155.103.116.134:31238/`; delivery with the same target and a buyer
  key authenticates `/models`, while a domain resolving to that IP is rejected as a different host.
- Good: a buyer imports an eligible order into the tester, explicitly acknowledges its HTTP target,
  discovers every returned model ID, and tests selected IDs without changing the order or health.
- Base: a verified connection is disabled. Existing orders remain intact, runner claims stop, and
  bound services become unavailable for new publication/orders until it is re-enabled and verified.
- Base: `/models` returns an empty valid `data` array. Connection authentication may be verified and
  the tester shows zero discovered models; neither flow invents platform catalog entries.
- Bad: creating one probe row per service, using returned model IDs as service verification, or
  issuing Responses/Chat calls from the periodic runner.
- Bad: adding `/v1`, treating a domain and its resolved IP as equal, persisting temporary tester
  results, or presenting a successful call as proof of official model identity.

### 6. Tests Required

- Domain and handlers: connection create/update/verify/delete, HTTP acknowledgement, no secret
  projection, service readiness, binding ownership, publication/orderability, and canonical order
  target comparisons.
- PostgreSQL: owner isolation, optimistic version conflict, referenced-delete rejection,
  same-slot concurrent claim deduplication, credential decryption failure, finalization, shared
  service summary input, running-timeout convergence, retention, and cascade deletion.
- PostgreSQL fixtures that exercise public/orderable services must bind an enabled, verified probe
  connection owned by the seller. Keep draft-only fixtures unbound, and never weaken the production
  orderability predicate to preserve an old test fixture.
- Credential-destruction concurrency tests must commit credential setup before starting the
  lifecycle lock transaction. The destructive update must run inside the transaction holding that
  lock so concurrent reads observe the intended block-then-unavailable sequence.
- OpenAI adapter: Base URL preservation, no automatic `/v1`, `/models` envelope and deduplication,
  both protocol request shapes, array response validation, top-level error rejection, error
  classification, response limits, and safe outbound dialing.
- Tester: manual/order authorization, destroyed credentials, CSRF/no-store, no key response,
  HTTP acknowledgement, three-worker frontend queue, cancellation, and memory-only state.
- Contracts: OpenAPI route/status/type parity, generated frontend types, the explicit
  service/package `modelKeySnapshot` versus intent/order pricing `modelKey` boundary, and scans
  excluding removed challenge/admin/model/TTFT fields.
- Required gates: full Go test/vet, focused race tests, PostgreSQL integration when configured,
  OpenAPI generate/check/route parity, full Vitest/typecheck/build, and `git diff --check`.

### 7. Wrong vs Correct

```go
// Wrong: one runner call per service and implicit HTTP permission.
for _, service := range services {
	probe(service.BaseURL, service.APIKey, service.Model)
}

// Correct: one claimed slot per reusable connection; no model is selected.
jobs := claimDueConnections(slot)
for _, job := range jobs {
	discoverModels(job.Connection.BaseURL, job.Credential)
}
```

```go
// Wrong: accept a delivered domain because it resolves to the frozen IP.
equal := resolvedIP(deliveryURL) == frozenIP

// Correct: compare canonical URL facts without DNS equivalence.
equal := canonicalDeliveryURL(deliveryURL) == order.NormalizedAPIBaseURLSnapshot
```
