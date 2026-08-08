package openaiapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/platform/outboundhttp"
)

const (
	modelsResponseLimit = 2 * 1024 * 1024
	callResponseLimit   = 1024 * 1024
)

type ErrorCode string

const (
	ErrorNone                ErrorCode = ""
	ErrorAuthentication      ErrorCode = "authentication_failed"
	ErrorProtocolUnsupported ErrorCode = "protocol_unsupported"
	ErrorRateLimited         ErrorCode = "rate_limited"
	ErrorRequestRejected     ErrorCode = "request_rejected"
	ErrorUpstream            ErrorCode = "upstream_error"
	ErrorTimeout             ErrorCode = "timeout"
	ErrorBlockedTarget       ErrorCode = "blocked_target"
	ErrorDNS                 ErrorCode = "dns_failed"
	ErrorConnect             ErrorCode = "connect_failed"
	ErrorTLS                 ErrorCode = "tls_failed"
	ErrorResponseTooLarge    ErrorCode = "response_too_large"
	ErrorInvalidResponse     ErrorCode = "invalid_response"
	ErrorStreamInterrupted   ErrorCode = "stream_interrupted"
	ErrorInternal            ErrorCode = "internal"
)

type Result struct {
	HTTPStatus      int
	HTTPStatusClass int
	DurationMS      int
	RetryAfterMS    int
	ErrorCode       ErrorCode
}

func (result Result) Succeeded() bool {
	return result.ErrorCode == ErrorNone
}

type Options struct {
	AllowInsecureHTTP bool
	Timeout           time.Duration
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	now        func() time.Time
}

func NewClient(baseURL, apiKey string, options Options) (*Client, error) {
	target, err := NormalizeBaseURL(baseURL, options.AllowInsecureHTTP)
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(target.Canonical)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return nil, outboundhttp.ErrInvalidTarget
	}
	policyOptions := policyOptions(options.AllowInsecureHTTP)
	policy, err := outboundhttp.NewPolicy([]string{parsed.Hostname()}, policyOptions...)
	if err != nil {
		return nil, err
	}
	clientOptions := make([]outboundhttp.ClientOption, 0, 1)
	if options.Timeout > 0 {
		clientOptions = append(clientOptions, outboundhttp.WithClientTimeout(options.Timeout))
	}
	return newClient(target.Raw, apiKey, outboundhttp.NewClient(policy, clientOptions...)), nil
}

func newClient(baseURL, apiKey string, client *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimSpace(baseURL),
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: client,
		now:        time.Now,
	}
}

func (client *Client) DiscoverModels(ctx context.Context) ([]string, Result) {
	response, result := client.do(ctx, http.MethodGet, "models", nil)
	if !result.Succeeded() {
		return nil, result
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		result.ErrorCode = ErrorInvalidResponse
		return nil, result
	}
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(envelope.Data, &items); err != nil {
		result.ErrorCode = ErrorInvalidResponse
		return nil, result
	}
	models := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			result.ErrorCode = ErrorInvalidResponse
			return nil, result
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		models = append(models, item.ID)
	}
	return models, result
}

func (client *Client) TestResponses(ctx context.Context, model string) Result {
	body, err := json.Marshal(map[string]any{
		"model":             strings.TrimSpace(model),
		"input":             "Reply briefly.",
		"max_output_tokens": 16,
		"stream":            false,
	})
	if err != nil {
		return Result{ErrorCode: ErrorInternal}
	}
	response, result := client.do(ctx, http.MethodPost, "responses", body)
	if !result.Succeeded() {
		return result
	}
	if !validObjectResponse(response, "output") {
		result.ErrorCode = ErrorInvalidResponse
	}
	return result
}

func (client *Client) TestChatCompletions(ctx context.Context, model string) Result {
	body, err := json.Marshal(map[string]any{
		"model":      strings.TrimSpace(model),
		"messages":   []map[string]string{{"role": "user", "content": "Reply briefly."}},
		"max_tokens": 16,
		"stream":     false,
	})
	if err != nil {
		return Result{ErrorCode: ErrorInternal}
	}
	response, result := client.do(ctx, http.MethodPost, "chat/completions", body)
	if !result.Succeeded() {
		return result
	}
	if !validObjectResponse(response, "choices") {
		result.ErrorCode = ErrorInvalidResponse
	}
	return result
}

func (client *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, Result) {
	if client == nil || client.httpClient == nil || client.baseURL == "" || client.apiKey == "" {
		return nil, Result{ErrorCode: ErrorInternal}
	}
	now := client.now
	if now == nil {
		now = time.Now
	}
	startedAt := now()
	finish := func(code ErrorCode, status int, retryAfterMS int) Result {
		duration := int(now().Sub(startedAt).Milliseconds())
		if duration < 0 {
			duration = 0
		}
		return Result{HTTPStatus: status, HTTPStatusClass: status / 100, DurationMS: duration, RetryAfterMS: retryAfterMS, ErrorCode: code}
	}
	endpoint, err := JoinEndpoint(client.baseURL, path)
	if err != nil {
		return nil, finish(ErrorInternal, 0, 0)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, finish(ErrorInternal, 0, 0)
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, finish(classifyRequestError(ctx, err), 0, 0)
	}
	defer response.Body.Close()
	retryAfterMS := parseRetryAfter(response.Header.Get("Retry-After"), now())
	limit := int64(callResponseLimit)
	if path == "models" {
		limit = modelsResponseLimit
	}
	responseBody, err := outboundhttp.ReadBody(response.Body, limit)
	if err != nil {
		return nil, finish(classifyRequestError(ctx, err), response.StatusCode, retryAfterMS)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, finish(classifyHTTPResponse(response.StatusCode, responseBody), response.StatusCode, retryAfterMS)
	}
	return responseBody, finish(ErrorNone, response.StatusCode, retryAfterMS)
}

func parseRetryAfter(value string, now time.Time) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		if seconds > 3 {
			seconds = 3
		}
		return seconds * 1000
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	delay := when.Sub(now)
	if delay > 3*time.Second {
		delay = 3 * time.Second
	}
	return int(delay.Milliseconds())
}

func validObjectResponse(body []byte, requiredField string) bool {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(body, &value); err != nil || value == nil {
		return false
	}
	if raw, exists := value["error"]; exists && len(raw) > 0 && string(raw) != "null" {
		return false
	}
	raw, exists := value[requiredField]
	if !exists {
		return false
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false
	}
	var items []json.RawMessage
	return json.Unmarshal(raw, &items) == nil
}

func classifyHTTPStatus(status int) ErrorCode {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ErrorAuthentication
	case status == http.StatusTooManyRequests:
		return ErrorRateLimited
	case status == http.StatusNotFound || status == http.StatusMethodNotAllowed:
		return ErrorProtocolUnsupported
	case status >= 400 && status < 500:
		return ErrorRequestRejected
	case status >= 500 && status < 600:
		return ErrorUpstream
	default:
		return ErrorInvalidResponse
	}
}

func classifyHTTPResponse(status int, body []byte) ErrorCode {
	if status != http.StatusNotFound {
		return classifyHTTPStatus(status)
	}
	var envelope struct {
		Error struct {
			Type  string `json:"type"`
			Param string `json:"param"`
			Code  string `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		code := strings.ToLower(strings.TrimSpace(envelope.Error.Code))
		typeName := strings.ToLower(strings.TrimSpace(envelope.Error.Type))
		param := strings.ToLower(strings.TrimSpace(envelope.Error.Param))
		if param == "model" || strings.Contains(code, "model") || strings.Contains(typeName, "model") {
			return ErrorRequestRejected
		}
	}
	return ErrorProtocolUnsupported
}

func classifyRequestError(ctx context.Context, err error) ErrorCode {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrorTimeout
	}
	if errors.Is(err, outboundhttp.ErrInvalidTarget) || errors.Is(err, outboundhttp.ErrHostNotAllowed) ||
		errors.Is(err, outboundhttp.ErrUnsafeAddress) || errors.Is(err, outboundhttp.ErrRedirectNotAllowed) {
		return ErrorBlockedTarget
	}
	if errors.Is(err, outboundhttp.ErrResolutionFailed) {
		return ErrorDNS
	}
	if errors.Is(err, outboundhttp.ErrDialFailed) {
		return ErrorConnect
	}
	if errors.Is(err, outboundhttp.ErrResponseTooLarge) {
		return ErrorResponseTooLarge
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return ErrorDNS
	}
	var certificateError *tls.CertificateVerificationError
	var unknownAuthorityError x509.UnknownAuthorityError
	var hostnameError x509.HostnameError
	var recordHeaderError tls.RecordHeaderError
	if errors.As(err, &certificateError) || errors.As(err, &unknownAuthorityError) ||
		errors.As(err, &hostnameError) || errors.As(err, &recordHeaderError) {
		return ErrorTLS
	}
	var networkError *net.OpError
	if errors.As(err, &networkError) {
		return ErrorConnect
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return ErrorTimeout
	}
	return ErrorInternal
}
