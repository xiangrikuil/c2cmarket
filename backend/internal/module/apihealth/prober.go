package apihealth

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"c2c-market/backend/internal/platform/outboundhttp"
)

const probeResponseLimit = 64 * 1024

var (
	errEmptyProbeResponse = errors.New("probe stream contained no content")
	errInvalidProbeStream = errors.New("probe stream is invalid")
)

type Prober interface {
	Probe(ctx context.Context, job ProbeJob) ProbeResult
}

type OpenAIStreamingProber struct {
	client        *http.Client
	clientFactory HTTPClientFactory
	now           func() time.Time
}

func NewOpenAIStreamingProberWithClientFactory(factory HTTPClientFactory, now func() time.Time) *OpenAIStreamingProber {
	if now == nil {
		now = time.Now
	}
	return &OpenAIStreamingProber{clientFactory: factory, now: now}
}

func NewOpenAIStreamingProber(client *http.Client, now func() time.Time) *OpenAIStreamingProber {
	if now == nil {
		now = time.Now
	}
	return &OpenAIStreamingProber{client: client, now: now}
}

func (p *OpenAIStreamingProber) Probe(ctx context.Context, job ProbeJob) ProbeResult {
	if p == nil {
		return ProbeResult{ErrorCode: ErrorInternal}
	}
	now := p.now
	if now == nil {
		now = time.Now
	}
	startedAt := now()
	fail := func(code string, statusClass int) ProbeResult {
		duration := int(now().Sub(startedAt).Milliseconds())
		if duration < 0 {
			duration = 0
		}
		return ProbeResult{TotalDurationMS: duration, HTTPStatusClass: statusClass, ErrorCode: code}
	}
	if strings.TrimSpace(job.Credential) == "" {
		return fail(ErrorAuthorizationInvalid, 0)
	}
	client := p.client
	if p.clientFactory != nil {
		var err error
		client, err = p.clientFactory.ClientFor(job.Config)
		if err != nil {
			return fail(classifyRequestError(ctx, err), 0)
		}
	}
	if client == nil {
		return fail(ErrorInternal, 0)
	}
	body, err := json.Marshal(map[string]any{
		"model":      job.Config.Model,
		"messages":   []map[string]string{{"role": "user", "content": "Reply with exactly OK."}},
		"max_tokens": 8,
		"stream":     true,
	})
	if err != nil {
		return fail(ErrorInternal, 0)
	}
	endpoint, err := url.JoinPath(job.Config.BaseURL, "chat/completions")
	if err != nil {
		return fail(ErrorInternal, 0)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fail(ErrorInternal, 0)
	}
	request.Header.Set("Authorization", "Bearer "+job.Credential)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	response, err := client.Do(request)
	if err != nil {
		return fail(classifyRequestError(ctx, err), 0)
	}
	defer response.Body.Close()
	statusClass := response.StatusCode / 100
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fail(classifyHTTPStatus(response.StatusCode), statusClass)
	}
	ttft, parseErr := readFirstStreamingContent(ctx, response.Body, startedAt, now)
	if parseErr != nil {
		if errors.Is(parseErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fail(ErrorTimeout, statusClass)
		}
		if errors.Is(parseErr, outboundhttp.ErrResponseTooLarge) {
			return fail(ErrorResponseTooLarge, statusClass)
		}
		if errors.Is(parseErr, errEmptyProbeResponse) {
			return fail(ErrorEmptyResponse, statusClass)
		}
		return fail(ErrorInvalidStream, statusClass)
	}
	duration := int(now().Sub(startedAt).Milliseconds())
	if duration < ttft {
		duration = ttft
	}
	return ProbeResult{TTFTMS: ttft, TotalDurationMS: duration, HTTPStatusClass: statusClass}
}

func readFirstStreamingContent(ctx context.Context, body io.Reader, startedAt time.Time, now func() time.Time) (int, error) {
	if body == nil || now == nil {
		return 0, errInvalidProbeStream
	}
	scanner := bufio.NewScanner(io.LimitReader(body, probeResponseLimit+1))
	scanner.Buffer(make([]byte, 4096), probeResponseLimit+1)
	total := 0
	dataLines := make([]string, 0, 1)
	sawInvalidLine := false
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		line := scanner.Text()
		total += len(line) + 1
		if total > probeResponseLimit {
			return 0, outboundhttp.ErrResponseTooLarge
		}
		if line == "" {
			ttft, found, err := parseStreamingEvent(dataLines, startedAt, now)
			if err != nil || found {
				return ttft, err
			}
			dataLines = dataLines[:0]
			continue
		}
		if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			sawInvalidLine = true
			continue
		}
		dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return 0, outboundhttp.ErrResponseTooLarge
		}
		return 0, err
	}
	if len(dataLines) > 0 {
		ttft, found, err := parseStreamingEvent(dataLines, startedAt, now)
		if err != nil || found {
			return ttft, err
		}
	}
	if sawInvalidLine {
		return 0, errInvalidProbeStream
	}
	return 0, errEmptyProbeResponse
}

func parseStreamingEvent(dataLines []string, startedAt time.Time, now func() time.Time) (int, bool, error) {
	if len(dataLines) == 0 {
		return 0, false, nil
	}
	data := strings.Join(dataLines, "\n")
	if data == "[DONE]" {
		return 0, false, errEmptyProbeResponse
	}
	var event struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return 0, false, errInvalidProbeStream
	}
	for _, choice := range event.Choices {
		if choice.Delta.Content != "" {
			ttft := int(now().Sub(startedAt).Milliseconds())
			if ttft < 0 {
				ttft = 0
			}
			return ttft, true, nil
		}
	}
	return 0, false, nil
}

func classifyHTTPStatus(status int) string {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return ErrorAuthorizationInvalid
	}
	if status >= 400 && status < 500 {
		return ErrorHTTP4xx
	}
	if status >= 500 && status < 600 {
		return ErrorHTTP5xx
	}
	return ErrorInvalidStream
}

func classifyRequestError(ctx context.Context, err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
		return ErrorTimeout
	}
	if errors.Is(err, outboundhttp.ErrInvalidTarget) || errors.Is(err, outboundhttp.ErrHostNotAllowed) ||
		errors.Is(err, outboundhttp.ErrUnsafeAddress) || errors.Is(err, outboundhttp.ErrRedirectNotAllowed) ||
		errors.Is(err, ErrTargetIdentityMismatch) {
		return ErrorBlockedTarget
	}
	if errors.Is(err, outboundhttp.ErrResolutionFailed) {
		return ErrorDNSFailed
	}
	if errors.Is(err, outboundhttp.ErrDialFailed) {
		return ErrorConnectFailed
	}
	if errors.Is(err, outboundhttp.ErrResponseTooLarge) {
		return ErrorResponseTooLarge
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrorDNSFailed
	}
	var certificateErr *tls.CertificateVerificationError
	var unknownAuthorityErr x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var recordHeaderErr tls.RecordHeaderError
	if errors.As(err, &certificateErr) || errors.As(err, &unknownAuthorityErr) ||
		errors.As(err, &hostnameErr) || errors.As(err, &recordHeaderErr) {
		return ErrorTLSFailed
	}
	var networkErr *net.OpError
	if errors.As(err, &networkErr) {
		return ErrorConnectFailed
	}
	return ErrorInternal
}
