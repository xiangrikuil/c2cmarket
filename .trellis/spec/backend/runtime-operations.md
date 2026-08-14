# Runtime Security And Observability

Date: 2026-08-14
Author: Codex

## Scenario: API/Pages Headers And Protected Runtime Metrics

### 1. Scope / Trigger

- Trigger: changing backend response-header middleware,
  `frontend/public/_headers`, `/metrics`, `METRICS_BEARER_TOKEN`, Sentry SDK or
  DSN configuration, metric labels, or public/operator route exposure.
- This is a cross-layer infrastructure contract. The Go API policy and the
  Cloudflare Pages policy serve different content and must not be collapsed
  into one permissive CSP.

### 2. Signatures

```go
middleware.WithSecurityHeaders(
    next,
    middleware.SecurityHeadersOptions{HSTS: appEnv == config.EnvProduction},
)

GET /metrics
ServerOptions{
    AppEnv:             appEnv,
    MetricsBearerToken: token,
    Metrics:            collector,
}
```

```text
METRICS_BEARER_TOKEN=<distinct secret, at least 32 bytes in production>
frontend/public/_headers
node scripts/check-security-headers.mjs
```

```text
SENTRY_ENABLED=true
SENTRY_DSN=<public backend project DSN>
SENTRY_ENVIRONMENT=production|staging
SENTRY_RELEASE=<deployed Git commit>
SENTRY_TRACES_SAMPLE_RATE=0.1
```

### 3. Contracts

- Every API response sets:
  - `Content-Security-Policy: default-src 'none'; base-uri 'none'; object-src
    'none'; frame-ancestors 'none'; form-action 'none'`
  - `Permissions-Policy: camera=(), geolocation=(), microphone=(), payment=(),
    usb=()`
  - `X-Frame-Options: DENY`
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: strict-origin-when-cross-origin`
- Production API responses additionally set
  `Strict-Transport-Security: max-age=31536000; includeSubDomains`. Development
  and tests omit HSTS so local HTTP remains usable.
- Cloudflare Pages owns the SPA CSP. It allows only committed asset/API origins,
  forbids wildcard, HTTP, and `unsafe-eval` sources, and keeps
  `style-src 'unsafe-inline'` only while runtime Vue style bindings require it.
- Production configuration requires a distinct `METRICS_BEARER_TOKEN` of at
  least 32 bytes. Metrics authentication is enabled in production regardless
  of other options, and is also enabled outside production when a token is
  configured.
- Metrics accepts exactly `Authorization: Bearer <token>`. Comparison uses
  fixed-size SHA-256 digests with constant-time comparison. Missing, malformed,
  empty, or wrong credentials return `401 METRICS_AUTH_REQUIRED` with
  `WWW-Authenticate: Bearer realm="metrics"` and never echo the token.
- An authorized response uses the Prometheus/OpenMetrics text format. Metric
  names use the `c2c_market_` namespace and bounded labels such as method,
  normalized chi route, status, result, or decision. Raw URLs, request IDs,
  user IDs, contacts, credentials, and tokens are forbidden labels.
- `/health` remains public liveness. `/readyz`, `/version`, and `/metrics` are
  operator routes and stay outside public ingress unless an explicit
  authenticated operational consumer requires them. Metrics uses both bearer
  authentication and a network/authenticated-proxy boundary in production.
- Outside production, an empty token may leave `/metrics` open for local tests.
  This is a local convenience, not a production fallback.
- Go Sentry is disabled by default outside deployment environments. Its HTTP
  middleware runs after request-ID assignment, reports panics and 5xx Problem
  Details, and does not report expected business 4xx responses.
- Sentry events keep bounded `request_id`, status, error-code, route-pattern,
  environment, and release context. They must remove request URL, query, body,
  Cookie, headers, remote address, user PII, contacts, and delivery credentials.
- Browser-to-API trace continuation uses only `Sentry-Trace` and `Baggage` CORS
  headers. The Pages CSP adds only the exact Sentry ingest origin.
- `SENTRY_AUTH_TOKEN` is build-only and must never enter Compose, Wrangler,
  source-controlled env files, runtime config, logs, or browser bundles.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Production metrics token missing or under 32 bytes | Configuration load fails |
| Missing, malformed, or wrong metrics bearer | `401 METRICS_AUTH_REQUIRED` plus bearer challenge |
| Correct production bearer | `200` Prometheus/OpenMetrics text |
| Development/test with no token | Metrics may be read locally without auth |
| Development/test with a token | Bearer authentication is required |
| Production API response lacks HSTS | Middleware test fails |
| Development response contains HSTS | Middleware test fails |
| Pages CSP gains wildcard, HTTP, or `unsafe-eval` | Header guard fails |
| Metric label contains raw URL/user/request/secret data | Review and observability tests fail |
| Sentry enabled without a valid DSN or with an out-of-range sample rate | Configuration load fails |
| Expected 4xx Problem Details response | No Sentry error event |
| Panic or 5xx Problem Details response | Redacted event with request ID and route trace |

### 5. Good / Base / Bad Cases

- Good: an internal monitoring proxy sends the bearer token over HTTPS, receives
  bounded route-pattern metrics, and never places the token in labels or logs.
- Base: a local test process has no metrics token and reads `/metrics` on
  loopback.
- Bad: exposing `/metrics` through the public frontend ingress with only a
  bearer token and no network boundary.
- Bad: reusing the API deny-by-default CSP for the SPA, or weakening the SPA
  policy with `script-src *` to make a new asset load.
- Bad: sending AppError detail, raw URL, query, body, Cookie, authorization
  header, contact data, or user identity to Sentry.

### 6. Tests Required

```bash
cd backend
go test -count=1 ./internal/config ./internal/middleware ./internal/observability ./internal/server
go test -race -count=1 ./internal/middleware ./internal/observability ./internal/server
cd ..
node scripts/check-security-headers.mjs
node scripts/check-openapi-routes.mjs
node scripts/check-compose-exposure.mjs
```

Assertions:

- API headers are exact in development and production, with HSTS differing only
  by environment.
- Metrics rejects missing/wrong tokens without disclosure, accepts the correct
  token, and records a normalized route/status counter.
- OpenAPI declares `/metrics` and its bearer scheme.
- Pages headers contain the exact API `connect-src` origins and forbidden CSP
  sources are absent.
- Production/staging Compose keep the backend loopback-only and PostgreSQL
  private.
- Sentry configuration parsing, event redaction, request-ID tagging, 5xx-only
  reporting, trace CORS headers, and exact ingest CSP origin are covered.

### 7. Wrong vs Correct

#### Wrong

```go
router.Get("/metrics", metrics.Handler().ServeHTTP)
```

This bypasses the production bearer contract and its stable error response.

#### Correct

```go
router.Get("/metrics", server.handleMetrics)
```

`handleMetrics` enforces environment-aware authentication before delegating to
the shared metrics registry, while outer middleware still applies request IDs,
logging, and security headers.
