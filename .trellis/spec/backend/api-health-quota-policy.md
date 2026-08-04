# API Health And Quota Policy Contract

Date: 2026-08-04
Author: Codex

## Scenario: Platform Probe Health And SKU Quota Rules

### 1. Scope / Trigger

- Trigger: changes to API health configuration, target authorization, probe execution,
  public API-market health projection, 5h/daily quota rules, or API intent/order snapshots.
- Primary owners are `internal/module/apihealth`, `internal/apihealthrunner`,
  `internal/store/postgres/api_health.go`, the API-market/quota stores and handlers,
  migration `000079`, the OpenAPI contract, and the shared frontend health/quota components.
- Health is a service-level platform measurement. Quota policy is a SKU-level seller contract.
  They are separate facts and must not share persistence or fallback rules.

### 2. Signatures

```text
GET|PUT|DELETE /api/v1/owner/api-services/{id}/health-probe
POST /api/v1/owner/api-services/{id}/health-probe/challenges
POST /api/v1/owner/api-services/{id}/health-probe/verify

GET  /api/v1/admin/api-service-health-probes
POST /api/v1/admin/api-service-health-probes/{id}/approve
POST /api/v1/admin/api-service-health-probes/{id}/reject
```

```text
QuotaUsageLimitInput { mode: limited|unlimited, amountUsd? }
QuotaUsageLimit      { mode: limited|unlimited|unspecified, amountUsd: DecimalString|null }
QuotaUsagePolicy     { fiveHour, daily, scope: per_buyer_credential,
                       dailyReset: utc_plus_8_calendar_day }

ServiceHealthSummary { state, availabilityReason, successRatePercent,
                       successfulSamples, totalSamples, medianTtftMs,
                       probeModel, lastSampledAt, samples[12] }
```

```text
api_service_probe_configs
api_service_probe_authorization_events
api_service_probe_samples

api_services/api_service_packages/api_quota_offers:
  five_hour_limit_mode, five_hour_limit_usd,
  daily_limit_mode, daily_limit_usd

api_purchase_intents/api_orders:
  five_hour_limit_mode_snapshot, five_hour_limit_usd_snapshot,
  daily_limit_mode_snapshot, daily_limit_usd_snapshot
```

Runtime environment:

```text
API_HEALTH_RUNNER_ENABLED
API_HEALTH_SCAN_INTERVAL
API_HEALTH_PROBE_TIMEOUT
API_HEALTH_MAX_CONCURRENCY
API_HEALTH_CLAIM_BATCH_SIZE
API_HEALTH_SAMPLE_RETENTION
API_HEALTH_CHALLENGE_TTL
```

### 3. Contracts

- The probe protocol is fixed to OpenAI-compatible streaming chat completions. The platform
  joins `chat/completions`, sends one bounded canary prompt, records the first non-empty content
  TTFT, and never persists response content.
- Probe credentials are dedicated low-privilege secrets encrypted with the contact codec under
  a probe-specific field/AAD. Owner responses expose only `credentialConfigured`; challenge
  tokens are returned once and only hashes are stored.
- Owner and administrator mutations require session, CSRF, `If-Match`, and private no-store
  responses. Initial owner PUT uses `If-Match: "0"`; stale versions return
  `412 VERSION_CONFLICT`. Challenge creation, challenge verification, administrator approval,
  and administrator rejection also require `Idempotency-Key`; replay restores the cached status,
  body, and ETag without repeating the authorization transition.
- SSRF validation permits only public HTTPS targets, disables proxy and redirects, re-resolves
  DNS for the actual dial, and rejects private, loopback, link-local, metadata, special-use, or
  mixed public/private results. This does not prove target ownership.
- Ownership is established by an exact-host DNS TXT challenge on the default 443 origin, an
  unauthenticated HTTP challenge at the exact origin's fixed well-known path, or administrator
  approval of the current exact origin. HTTP verification never sends the probe credential.
- Authorization binds protocol, normalized base URL including base path, model, scheme,
  hostname, and effective port. Any measurement-identity change increments
  `measurement_version`, resets authorization, clears a live challenge, and appends an
  `origin_invalidated` authorization event in the same transaction.
- The runner claims only enabled, credentialed, authorized configs whose verified origin equals
  the normalized origin. It creates at most one sample per service/five-minute slot, performs
  HTTP outside the claim transaction, finalizes only a running sample, and converges abandoned
  running samples to `internal_timeout`.
- Public summaries use the current measurement version and model, exclude running samples, and
  always return 12 ascending five-minute slots. Fewer than three final samples, data older than
  ten minutes, disabled/unauthorized/unconfigured probes, and health-query failures have explicit
  `no_sample` reasons.
- Public health enrichment batches distinct service IDs. A health read failure degrades only the
  health field to `temporarily_unavailable`; it cannot block listing, detail, publication,
  purchase, fulfillment, dispute handling, or recommendation order.
- Current public cards and service details use platform `healthSummary` for TTFT. They do not
  expose seller-declared TTFT or `performanceDisclaimer`. Historical orders retain the frozen
  seller declaration for transaction explanation only.
- 5h and daily limits are seller-declared USD amounts after the SKU's model multiplier. They are
  not token counts, upstream list-price amounts, or platform-enforced usage meters. Each buyer
  credential has its own limits, and daily means the UTC+8 calendar day.
- New service/package/offer writes require each limit to be explicitly `limited` with a strictly
  positive decimal amount or `unlimited` without an amount. `unspecified` exists only for
  historical reads and renders as `未说明`, never `不限`.
- Free-amount services own one policy, each fixed package owns one policy, and each limited quota
  offer owns one policy. Intent creation freezes the selected SKU policy; order creation copies
  the intent snapshot. Limited-offer inventory transactions freeze the same policy into the
  intent, order, and self-describing JSON snapshot atomically.

### 4. Validation & Error Matrix

| Condition | Result |
| --- | --- |
| Non-HTTPS, private, mixed-DNS, redirecting, or malformed target | `422 VALIDATION_FAILED` on `baseUrl` |
| Enabling without a configured credential | `422 VALIDATION_FAILED` on `credential` |
| DNS challenge for a non-443 origin | `422 VALIDATION_FAILED`, `port_not_supported` |
| Empty or expired/mismatched challenge | Remain unauthorized; never execute a probe |
| Owner/admin version is stale or initial PUT omits version zero | `412 VERSION_CONFLICT` or `428` |
| Administrator decision references a missing probe config | `404 OBJECT_NOT_FOUND`; do not report a version conflict |
| Administrator decision omits a reason | `422 VALIDATION_FAILED` on `reason` |
| Health repository/enrichment fails | Return product with `no_sample/temporarily_unavailable` |
| New policy uses `unspecified` | `422 VALIDATION_FAILED` on the limit mode |
| `limited` omits or supplies a non-positive amount | `422 VALIDATION_FAILED` on `amountUsd` |
| `unlimited` includes an amount | `422 VALIDATION_FAILED` on `amountUsd` |
| Historical mode is `unspecified` | Return `{ mode: unspecified, amountUsd: null }` |

### 5. Good / Base / Bad Cases

- Good: one service has three paid SKUs with different policies; all cards share one measured
  health summary while each order freezes its own policy.
- Good: changing `/v1` to `/proxy/v1` invalidates an approved target and records the invalidation
  atomically before any new sample can be claimed.
- Base: an unconfigured service remains purchasable and displays 12 no-sample slots with the
  `unconfigured` reason.
- Bad: treating SSRF validation as ownership proof, sending `Authorization` during HTTP challenge,
  or authorizing a host suffix/wildcard.
- Bad: returning unlimited for a historical null, recomputing an old order from the current SKU,
  or using platform probe TTFT in the package recommendation score.

### 6. Tests Required

- Domain: target normalization, base-path/model invalidation, credential retention/rotation,
  DNS multi-TXT/expiry/non-443, HTTP fixed path/no-auth, SSE chunking/limits/error classes,
  12-slot order, minimum samples, stale data, thresholds, and odd/even median.
- Runner/store: same-slot concurrency, authorization claim filters, credential decrypt failure,
  running timeout convergence, conditional finalize, atomic invalidation event, exact-origin
  administrator reject/approve, owner isolation, and retention of final samples only.
- Quota/order: limited/unlimited/unspecified validation, all three SKU locations, source-SKU
  mutation after purchase, and immutable dedicated plus JSON snapshots.
- HTTP/OpenAPI: route/type parity, CSRF, ETag/If-Match, no-store, write-only credential, one-time
  challenge token, secret leakage scan, public seller-TTFT absence, and fail-open enrichment.
- Frontend: generated-type adapters, owner/admin mutations, policy controls, snapshot rendering,
  three card variants, no merchant-TTFT copy, fixed 12-slot layout, and desktop/mobile overflow.
- Required gates: `go test ./...`, relevant race tests, `go vet ./...`, OpenAPI/migration guards,
  full Vitest/typecheck/build, and `scripts/ci-postgres-integration.sh`.

### 7. Wrong vs Correct

#### Wrong

```go
if publicIP(target) {
    config.AuthorizationStatus = "approved"
}

order.QuotaUsagePolicySnapshot = currentOffer.QuotaUsagePolicy
```

#### Correct

```go
if apihealth.IsAuthorized(config) && config.VerifiedOrigin == config.NormalizedOrigin {
    jobs = claimCurrentFiveMinuteSlot(config)
}

order.QuotaUsagePolicySnapshot = intent.QuotaUsagePolicySnapshot
```

Public-address validation limits where the platform connects; DNS/HTTP/admin verification proves
the exact target authorization. Orders copy frozen intent facts rather than mutable current SKUs.
