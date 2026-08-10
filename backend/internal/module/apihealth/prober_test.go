package apihealth

import (
	"context"
	"errors"
	"testing"
	"time"

	"c2c-market/backend/internal/platform/openaiapi"
)

func TestMapOpenAIErrorUsesStableLowCardinalityCodes(t *testing.T) {
	tests := map[openaiapi.ErrorCode]string{
		openaiapi.ErrorNone:                "",
		openaiapi.ErrorAuthentication:      ErrorAuthorizationInvalid,
		openaiapi.ErrorBlockedTarget:       ErrorBlockedTarget,
		openaiapi.ErrorDNS:                 ErrorDNSFailed,
		openaiapi.ErrorConnect:             ErrorConnectFailed,
		openaiapi.ErrorTLS:                 ErrorTLSFailed,
		openaiapi.ErrorTimeout:             ErrorTimeout,
		openaiapi.ErrorRateLimited:         ErrorRateLimited,
		openaiapi.ErrorUpstream:            ErrorHTTP5xx,
		openaiapi.ErrorInvalidResponse:     ErrorInvalidResponse,
		openaiapi.ErrorResponseTooLarge:    ErrorResponseTooLarge,
		openaiapi.ErrorProtocolUnsupported: ErrorProtocolUnavailable,
		openaiapi.ErrorRequestRejected:     ErrorHTTP4xx,
		openaiapi.ErrorStreamInterrupted:   ErrorStreamInterrupted,
	}
	for input, expected := range tests {
		if actual := mapOpenAIError(input); actual != expected {
			t.Fatalf("mapOpenAIError(%q)=%q, want %q", input, actual, expected)
		}
	}
}

func TestVerifyPrefersResponsesAndFallsBackOnlyWhenProtocolUnavailable(t *testing.T) {
	client := &fakeOpenAIProbeClient{
		models:          []string{DefaultGPTProbeModel},
		discoveryResult: openaiapi.Result{HTTPStatus: 200},
		streamResults: []openaiapi.StreamResult{
			{Result: openaiapi.Result{HTTPStatus: 404, ErrorCode: openaiapi.ErrorProtocolUnsupported}},
			streamSuccess(25),
		},
	}
	prober := testRealModelProber(client)

	result := prober.Verify(context.Background(), "https://api.example.test/v1", "key", "", false)

	if result.ErrorCode != "" || result.ProbeModel != DefaultGPTProbeModel || result.ProbeProtocol != ProtocolChatCompletionsV1 {
		t.Fatalf("unexpected verification: %+v", result)
	}
	if len(client.protocols) != 2 || client.protocols[0] != ProtocolResponsesV1 || client.protocols[1] != ProtocolChatCompletionsV1 {
		t.Fatalf("unexpected protocol order: %v", client.protocols)
	}
}

func TestVerifyDoesNotFallbackForDeterministicRequestError(t *testing.T) {
	client := &fakeOpenAIProbeClient{
		models:          []string{DefaultGPTProbeModel},
		discoveryResult: openaiapi.Result{HTTPStatus: 200},
		streamResults: []openaiapi.StreamResult{{
			Result: openaiapi.Result{HTTPStatus: 400, ErrorCode: openaiapi.ErrorRequestRejected},
		}},
	}
	prober := testRealModelProber(client)

	result := prober.Verify(context.Background(), "https://api.example.test/v1", "key", DefaultGPTProbeModel, false)

	if result.ErrorCode != ErrorHTTP4xx || len(client.protocols) != 1 {
		t.Fatalf("unexpected verification/fallback: result=%+v protocols=%v", result, client.protocols)
	}
}

func TestProbeRetriesTransientFailureOnceAndKeepsRecoverySeparate(t *testing.T) {
	client := &fakeOpenAIProbeClient{streamResults: []openaiapi.StreamResult{
		{Result: openaiapi.Result{HTTPStatus: 429, RetryAfterMS: 2500, ErrorCode: openaiapi.ErrorRateLimited}},
		streamSuccess(40),
	}}
	prober := testRealModelProber(client)
	var waits []time.Duration
	prober.sleep = func(_ context.Context, duration time.Duration) error {
		waits = append(waits, duration)
		return nil
	}

	result := prober.Probe(context.Background(), probeJob())

	if result.Outcome != OutcomeRetryRecovered || result.ErrorCode != "" || len(result.Attempts) != 2 || result.RecoveryDurationMS == nil {
		t.Fatalf("unexpected retry result: %+v", result)
	}
	if result.FirstAttemptTTFTMS != nil || len(waits) != 1 || waits[0] != 2500*time.Millisecond {
		t.Fatalf("retry contaminated TTFT or wait: ttft=%v waits=%v", result.FirstAttemptTTFTMS, waits)
	}
}

func TestProbeDoesNotRetryDeterministicFailure(t *testing.T) {
	client := &fakeOpenAIProbeClient{streamResults: []openaiapi.StreamResult{{
		Result: openaiapi.Result{HTTPStatus: 401, ErrorCode: openaiapi.ErrorAuthentication},
	}}}
	prober := testRealModelProber(client)
	prober.sleep = func(context.Context, time.Duration) error { return errors.New("must not sleep") }

	result := prober.Probe(context.Background(), probeJob())

	if result.Outcome != OutcomeFinalFailure || result.ErrorCode != ErrorAuthorizationInvalid || len(result.Attempts) != 1 || len(client.protocols) != 1 {
		t.Fatalf("unexpected deterministic result: %+v protocols=%v", result, client.protocols)
	}
}

func TestProbeAppliesPublishedSlowThresholdOnlyToFirstSuccess(t *testing.T) {
	client := &fakeOpenAIProbeClient{streamResults: []openaiapi.StreamResult{streamSuccess(5100)}}
	prober := testRealModelProber(client)
	job := probeJob()
	job.LatencyRule = &LatencyRule{SlowTTFTMS: 5000, HardTimeoutMS: 10000}

	result := prober.Probe(context.Background(), job)

	if result.Outcome != OutcomeFirstSuccessSlow || result.FirstAttemptTTFTMS == nil || *result.FirstAttemptTTFTMS != 5100 || len(result.Attempts) != 1 {
		t.Fatalf("unexpected slow result: %+v", result)
	}
}

type fakeOpenAIProbeClient struct {
	models          []string
	discoveryResult openaiapi.Result
	streamResults   []openaiapi.StreamResult
	protocols       []string
}

func (client *fakeOpenAIProbeClient) DiscoverModels(context.Context) ([]string, openaiapi.Result) {
	return append([]string(nil), client.models...), client.discoveryResult
}

func (client *fakeOpenAIProbeClient) StreamProbe(_ context.Context, protocol, _ string) openaiapi.StreamResult {
	client.protocols = append(client.protocols, protocol)
	if len(client.streamResults) == 0 {
		return openaiapi.StreamResult{Result: openaiapi.Result{ErrorCode: openaiapi.ErrorInternal}}
	}
	result := client.streamResults[0]
	client.streamResults = client.streamResults[1:]
	return result
}

func testRealModelProber(client openAIProbeClient) *OpenAIRealModelProber {
	prober := NewOpenAIRealModelProber(30 * time.Second)
	prober.newClient = func(string, string, openaiapi.Options) (openAIProbeClient, error) { return client, nil }
	current := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	prober.now = func() time.Time {
		current = current.Add(10 * time.Millisecond)
		return current
	}
	prober.randomWait = func() time.Duration { return time.Second }
	return prober
}

func probeJob() ProbeJob {
	return ProbeJob{
		Connection: Connection{
			BaseURL: "https://api.example.test/v1", ProbeModel: DefaultGPTProbeModel,
			ProbeProtocol: ProtocolResponsesV1, ProbeEnvironment: ProbeEnvironmentUSWestV1,
		},
		Credential: "key",
	}
}

func streamSuccess(ttft int) openaiapi.StreamResult {
	input, output := int64(6), int64(2)
	return openaiapi.StreamResult{
		Result: openaiapi.Result{HTTPStatus: 200, HTTPStatusClass: 2, DurationMS: ttft + 10},
		TTFTMS: &ttft, Usage: openaiapi.Usage{InputTokens: &input, OutputTokens: &output},
	}
}
