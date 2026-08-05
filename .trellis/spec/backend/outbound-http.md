# Safe Outbound HTTP

Date: 2026-07-26
Author: Codex

## Scenario: Variable-Destination Outbound Requests

### 1. Scope / Trigger

- Trigger: an outbound HTTP destination is stored, configured, or otherwise
  variable instead of being a fixed source-code constant.
- Use `internal/platform/outboundhttp` for URL policy, DNS validation, bound
  dialing, redirect control, timeouts, and bounded response reads.
- Model audit targets are the first enforced consumer. Fixed OAuth provider
  endpoints remain outside this scenario until a task explicitly migrates them.

### 2. Signatures

```go
type Resolver interface {
    LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type Dialer interface {
    DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func NewPolicy(allowedHosts []string, options ...PolicyOption) (*Policy, error)
func WithInsecureHTTP() PolicyOption
func (p *Policy) ValidateURL(ctx context.Context, raw string) (string, error)
func NewClient(policy *Policy, options ...ClientOption) *http.Client
func ReadBody(body io.Reader, limit int64) ([]byte, error)

func (s *modelaudit.Service) SetOutboundPolicy(policy *outboundhttp.Policy)
func (s *core.Service) ConfigureModelAuditOutbound(policy *outboundhttp.Policy)
```

Environment and configuration boundary:

```text
MODEL_AUDIT_ALLOWED_HOSTS=<comma-separated exact hosts>
config.Config.ModelAuditAllowedHosts []string
```

### 3. Contracts

- Accept only absolute HTTPS URLs by default.
- `WithInsecureHTTP` is a narrow opt-in owned by the API-health probe flow. It may be used only
  after that flow has enforced `acknowledgeInsecureHttp=true` for every HTTP save and disclosed
  the transport risk. Model audit and all other consumers remain HTTPS-only.
- Reject URL credentials, queries, fragments, opaque URLs, zone identifiers,
  malformed ports, alternative numeric IP forms, and invalid DNS labels.
- Treat `MODEL_AUDIT_ALLOWED_HOSTS` as an optional exact-host allowlist. Do not
  add wildcard or suffix matching. An empty list permits any host that passes
  the public-address policy.
- Resolve every address before persistence. Reject the whole target if any
  result is private, loopback, link-local, multicast, unspecified, metadata,
  documentation, benchmark, reserved, translated, or otherwise special-use.
- Resolve and validate again inside `http.Transport.DialContext`. Pass only
  validated IP literals to the underlying dialer while retaining the original
  request host for TLS certificate and Server Name validation.
- Disable environment proxies and redirects. Limit a connection to three
  validated IP attempts without replaying the HTTP request.
- Keep explicit dial, TLS handshake, response header, total request, and idle
  connection timeouts. Reuse one policy and one client per consumer service.
- Read remote bodies with `ReadBody`, which checks `limit+1` and returns
  `ErrResponseTooLarge` instead of silently truncating.
- Never return or log a raw `url.Error` for a variable target. Map it to a
  stable error so complete URLs, credentials, authorization headers, and
  third-party response bodies cannot enter logs or API responses.
- Production must keep standard TLS verification. Local tests may inject a
  test-only TLS configuration.

### 4. Validation & Error Matrix

| Condition | Required result |
| --- | --- |
| Scheme is HTTP without `WithInsecureHTTP`, is neither HTTP nor HTTPS, URL is relative/opaque, or authority is malformed | `ErrInvalidTarget`; do not resolve or dial |
| API-health HTTP request omits its request-layer acknowledgement | Reject before policy validation; do not resolve, persist, or dial |
| URL contains credentials, query, fragment, zone ID, or invalid port | `ErrInvalidTarget`; do not persist |
| Host is absent from a non-empty exact allowlist | `ErrHostNotAllowed`; do not resolve or dial |
| Literal IP is private or special-use | `ErrUnsafeAddress`; do not dial |
| DNS returns no addresses or fails | `ErrResolutionFailed`; do not dial |
| Any DNS answer is private or special-use | `ErrUnsafeAddress`; reject the complete result set |
| More than three safe DNS answers are returned | Attempt at most the first three IP literals |
| Provider returns a redirect | `ErrRedirectNotAllowed`; never follow it |
| Body exceeds its consumer limit | `ErrResponseTooLarge`; do not parse truncated data |
| Connect, TLS, header, or total timeout expires | Return a sanitized timeout/request error without the complete URL |

### 5. Good / Base / Bad Cases

- Good: `https://api.provider.example/v1` is allowlisted, all DNS answers are
  public, and the dialer receives an IP literal such as `93.184.216.34:443`.
  Documentation ranges are intentionally blocked.
- Good: API health creates its own policy with `WithInsecureHTTP` only after owner acknowledgement;
  `http://api.provider.example/v1` dials a validated public IP on port 80 with redirects disabled.
- Base: `MODEL_AUDIT_ALLOWED_HOSTS` is empty. Any syntactically valid public
  HTTPS target may be saved, but private and special-use addresses still fail.
- Bad: validating DNS only when saving, then letting `http.Transport` resolve
  the hostname again. DNS rebinding can change the second answer to a private
  address.
- Bad: accepting one public answer when the same lookup also returns a private
  answer. The entire result set must fail before dialing.
- Bad: using `io.LimitReader` without checking one extra byte. That silently
  turns an oversized JSON response into malformed or partial data.
- Bad: adding `WithInsecureHTTP` to a shared/global policy or a consumer without an explicit
  product acknowledgement and public disclosure contract.

### 6. Tests Required

- URL table tests: HTTP rejected by default, explicitly enabled HTTP, credentials, query, fragment, invalid/zero/overflow
  ports, zone IDs, relative URLs, and alternative numeric IPv4 syntax.
- Address table tests: loopback, unspecified, metadata, RFC1918, CGNAT,
  link-local, multicast, benchmark, documentation, reserved, IPv6 ULA,
  deprecated site-local, and IPv4-mapped IPv6.
- Resolver tests: private-only, mixed public/private, empty/failure, duplicate
  public answers, and exact allowlist mismatch.
- Dial tests: assert the underlying dialer receives IP literals, never the
  hostname; unsafe mixed DNS must call the dialer zero times; attempts must not
  exceed three.
- TLS test: assert the ClientHello ServerName remains the original DNS host even
  though the socket dial uses an IP literal.
- HTTP tests: successful public HTTPS and explicitly enabled public HTTP, default HTTP rejection,
  private/mixed-DNS HTTP rejection, redirect rejection, slow response timeout, and oversized
  Chat/models bodies.
- Consumer tests: model audit create and update both reject unsafe targets,
  preserve a normalized `/v1` base path, and sanitize request errors.
- Final verification: focused tests and race tests for `outboundhttp` and
  `modelaudit`, then full backend tests, `go vet`, `gofmt`, Compose expansion,
  OpenAPI/migration guards, and `git diff --check`.

### 7. Wrong vs Correct

#### Wrong

```go
client := &http.Client{Timeout: 30 * time.Second}
response, err := client.Get(targetURL)
body, err := io.ReadAll(io.LimitReader(response.Body, maxBody))
```

This leaves DNS rebinding, redirect behavior, proxy behavior, and silent body
truncation outside the policy boundary.

#### Correct

```go
policy, err := outboundhttp.NewPolicy(configuredHosts)
if err != nil {
    return err
}
normalized, err := policy.ValidateURL(ctx, targetURL)
if err != nil {
    return err
}
client := outboundhttp.NewClient(policy)
response, err := client.Get(normalized)
body, err := outboundhttp.ReadBody(response.Body, maxBody)
```

Persistence validation improves feedback, while the same policy inside the
transport remains the actual connection-time security boundary.

For the API-health-only exception, construct a separate policy instance after request-layer
acknowledgement:

```go
if usesHTTP(input.BaseURL) && !input.AcknowledgeInsecureHTTP {
    return validationError("acknowledgeInsecureHttp")
}
policy, err := outboundhttp.NewPolicy(configuredHosts, outboundhttp.WithInsecureHTTP())
```

Do not add the option to model-audit or a process-wide shared policy.
