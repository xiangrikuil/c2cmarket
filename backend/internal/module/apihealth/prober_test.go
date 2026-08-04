package apihealth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/platform/outboundhttp"
)

func TestOpenAIStreamingProberMeasuresFirstContentDelta(t *testing.T) {
	startedAt := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	clock := newSequenceClock(startedAt, startedAt.Add(250*time.Millisecond), startedAt.Add(400*time.Millisecond))
	var captured *http.Request
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		captured = request
		body := &chunkReader{chunks: []string{
			": keep-alive\n",
			"data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\n",
			"data: {\"choices\":[{\"delta\":{\"content\":\"O",
			"K\"}}]}\n\n",
		}}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(body), Header: make(http.Header)}, nil
	})}
	prober := NewOpenAIStreamingProber(client, clock.Now)

	result := prober.Probe(context.Background(), ProbeJob{
		Config:     Config{BaseURL: "https://api.example.com/v1", Model: "gpt-5-mini"},
		Credential: "probe-secret",
	})

	if result.ErrorCode != "" || result.TTFTMS != 250 || result.TotalDurationMS != 400 || result.HTTPStatusClass != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if captured == nil || captured.URL.String() != "https://api.example.com/v1/chat/completions" {
		t.Fatalf("unexpected request target: %v", captured)
	}
	if got := captured.Header.Get("Authorization"); got != "Bearer probe-secret" {
		t.Fatalf("unexpected authorization header: %q", got)
	}
	var payload struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Stream    bool `json:"stream"`
		MaxTokens int  `json:"max_tokens"`
	}
	if err := json.NewDecoder(captured.Body).Decode(&payload); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if payload.Model != "gpt-5-mini" || !payload.Stream || payload.MaxTokens != 8 || len(payload.Messages) != 1 ||
		payload.Messages[0].Role != "user" || payload.Messages[0].Content != "Reply with exactly OK." {
		t.Fatalf("unexpected request payload: %+v", payload)
	}
}

func TestOpenAIStreamingProberClassifiesResponses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantCode   string
		wantClass  int
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, wantCode: ErrorAuthorizationInvalid, wantClass: 4},
		{name: "forbidden", statusCode: http.StatusForbidden, wantCode: ErrorAuthorizationInvalid, wantClass: 4},
		{name: "other client error", statusCode: http.StatusTooManyRequests, wantCode: ErrorHTTP4xx, wantClass: 4},
		{name: "server error", statusCode: http.StatusBadGateway, wantCode: ErrorHTTP5xx, wantClass: 5},
		{name: "invalid json", statusCode: http.StatusOK, body: "data: not-json\n\n", wantCode: ErrorInvalidStream, wantClass: 2},
		{name: "done without content", statusCode: http.StatusOK, body: "data: [DONE]\n\n", wantCode: ErrorEmptyResponse, wantClass: 2},
		{name: "empty delta", statusCode: http.StatusOK, body: "data: {\"choices\":[{\"delta\":{\"content\":\"\"}}]}\n\n", wantCode: ErrorEmptyResponse, wantClass: 2},
		{name: "non sse body", statusCode: http.StatusOK, body: "plain text\n", wantCode: ErrorInvalidStream, wantClass: 2},
		{name: "oversized stream", statusCode: http.StatusOK, body: "data: " + strings.Repeat("x", probeResponseLimit) + "\n", wantCode: ErrorResponseTooLarge, wantClass: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := responseClient(test.statusCode, test.body)
			prober := NewOpenAIStreamingProber(client, func() time.Time { return time.Unix(0, 0) })
			result := prober.Probe(context.Background(), ProbeJob{
				Config: Config{BaseURL: "https://api.example.com/v1", Model: "gpt"}, Credential: "key",
			})
			if result.ErrorCode != test.wantCode || result.HTTPStatusClass != test.wantClass {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestOpenAIStreamingProberHonorsContextTimeout(t *testing.T) {
	t.Parallel()
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	prober := NewOpenAIStreamingProber(client, time.Now)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	result := prober.Probe(ctx, ProbeJob{Config: Config{BaseURL: "https://api.example.com", Model: "gpt"}, Credential: "key"})

	if result.ErrorCode != ErrorTimeout {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestOpenAIStreamingProberRejectsMissingDependenciesAndCredential(t *testing.T) {
	t.Parallel()
	job := ProbeJob{Config: Config{BaseURL: "https://api.example.com", Model: "gpt"}}
	jobWithCredential := job
	jobWithCredential.Credential = "key"
	var nilProber *OpenAIStreamingProber
	if result := nilProber.Probe(context.Background(), jobWithCredential); result.ErrorCode != ErrorInternal {
		t.Fatalf("nil prober result: %+v", result)
	}
	if result := NewOpenAIStreamingProber(nil, nil).Probe(context.Background(), jobWithCredential); result.ErrorCode != ErrorInternal {
		t.Fatalf("nil client result: %+v", result)
	}
	if result := NewOpenAIStreamingProber(responseClient(http.StatusOK, ""), nil).Probe(context.Background(), job); result.ErrorCode != ErrorAuthorizationInvalid {
		t.Fatalf("missing credential result: %+v", result)
	}
}

func TestOpenAIStreamingProberRejectsChangedAuthorizedTarget(t *testing.T) {
	t.Parallel()
	prober := NewOpenAIStreamingProberWithClientFactory(NewOutboundHTTPClientFactory(time.Second), nil)
	result := prober.Probe(context.Background(), ProbeJob{
		Config: Config{
			BaseURL: "https://api.example.com/v1", NormalizedOrigin: "https://other.example.com:443", Model: "gpt",
		},
		Credential: "key",
	})
	if result.ErrorCode != ErrorBlockedTarget {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClassifyRequestError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "blocked", err: outboundhttp.ErrUnsafeAddress, want: ErrorBlockedTarget},
		{name: "dns", err: &net.DNSError{Err: "missing", Name: "api.example.com"}, want: ErrorDNSFailed},
		{name: "connect", err: outboundhttp.ErrDialFailed, want: ErrorConnectFailed},
		{name: "tls", err: tls.RecordHeaderError{Msg: "invalid record"}, want: ErrorTLSFailed},
		{name: "oversized", err: outboundhttp.ErrResponseTooLarge, want: ErrorResponseTooLarge},
		{name: "unknown", err: errors.New("unknown"), want: ErrorInternal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyRequestError(context.Background(), test.err); got != test.want {
				t.Fatalf("classifyRequestError() = %q, want %q", got, test.want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func responseClient(statusCode int, body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: statusCode, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
}

type chunkReader struct {
	chunks []string
}

func (reader *chunkReader) Read(buffer []byte) (int, error) {
	if len(reader.chunks) == 0 {
		return 0, io.EOF
	}
	chunk := reader.chunks[0]
	reader.chunks = reader.chunks[1:]
	return copy(buffer, chunk), nil
}

type sequenceClock struct {
	mu    sync.Mutex
	times []time.Time
}

func newSequenceClock(times ...time.Time) *sequenceClock {
	return &sequenceClock{times: times}
}

func (clock *sequenceClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	if len(clock.times) == 0 {
		return time.Time{}
	}
	value := clock.times[0]
	if len(clock.times) > 1 {
		clock.times = clock.times[1:]
	}
	return value
}
