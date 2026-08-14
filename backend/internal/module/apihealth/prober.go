package apihealth

import (
	"context"
	cryptorand "crypto/rand"
	"math/big"
	"strings"
	"time"

	"c2c-market/backend/internal/platform/openaiapi"
)

type Prober interface {
	Probe(ctx context.Context, job ProbeJob) ProbeResult
}

type openAIProbeClient interface {
	DiscoverModels(ctx context.Context) ([]string, openaiapi.Result)
	StreamProbe(ctx context.Context, protocol, model string) openaiapi.StreamResult
}

type openAIProbeClientFactory func(baseURL, credential string, options openaiapi.Options) (openAIProbeClient, error)

type OpenAIRealModelProber struct {
	timeout    time.Duration
	now        func() time.Time
	sleep      func(context.Context, time.Duration) error
	randomWait func() time.Duration
	newClient  openAIProbeClientFactory
}

func NewOpenAIRealModelProber(timeout time.Duration) *OpenAIRealModelProber {
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	return &OpenAIRealModelProber{
		timeout: timeout,
		now:     time.Now,
		sleep: func(ctx context.Context, duration time.Duration) error {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
		randomWait: func() time.Duration {
			value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(3))
			if err != nil {
				return 2 * time.Second
			}
			return time.Duration(value.Int64()+1) * time.Second
		},
		newClient: func(baseURL, credential string, options openaiapi.Options) (openAIProbeClient, error) {
			return openaiapi.NewClient(baseURL, credential, options)
		},
	}
}

func (prober *OpenAIRealModelProber) Verify(ctx context.Context, baseURL, credential, requestedModel string, allowInsecureHTTP bool) VerificationResult {
	if prober == nil {
		return VerificationResult{ErrorCode: ErrorInternal}
	}
	client, err := prober.newClient(baseURL, credential, openaiapi.Options{
		AllowInsecureHTTP: allowInsecureHTTP,
		Timeout:           prober.timeout,
	})
	if err != nil {
		return VerificationResult{ErrorCode: mapOpenAIError(openaiapi.ErrorBlockedTarget)}
	}
	models, discovery := client.DiscoverModels(ctx)
	if !discovery.Succeeded() {
		return VerificationResult{TotalDurationMS: discovery.DurationMS, HTTPStatus: discovery.HTTPStatus, ErrorCode: mapOpenAIError(discovery.ErrorCode)}
	}
	model := strings.TrimSpace(requestedModel)
	if model == "" && containsModel(models, DefaultGPTProbeModel) {
		model = DefaultGPTProbeModel
	}
	if model == "" || !containsModel(models, model) {
		return VerificationResult{TotalDurationMS: discovery.DurationMS, HTTPStatus: discovery.HTTPStatus, ErrorCode: ErrorModelUnavailable, AvailableModels: models}
	}
	responses := prober.streamAttempt(ctx, client, ProtocolResponsesV1, model, 1)
	if responses.Succeeded {
		return VerificationResult{
			TotalDurationMS: discovery.DurationMS + responses.TotalDurationMS, HTTPStatus: responses.HTTPStatus,
			AvailableModels: models, ProbeModel: model, ProbeProtocol: ProtocolResponsesV1, Attempt: responses,
		}
	}
	if responses.ErrorCode != ErrorProtocolUnavailable {
		return VerificationResult{
			TotalDurationMS: discovery.DurationMS + responses.TotalDurationMS, HTTPStatus: responses.HTTPStatus,
			ErrorCode: responses.ErrorCode, AvailableModels: models, ProbeModel: model, Attempt: responses,
		}
	}
	chat := prober.streamAttempt(ctx, client, ProtocolChatCompletionsV1, model, 1)
	if !chat.Succeeded {
		return VerificationResult{
			TotalDurationMS: discovery.DurationMS + responses.TotalDurationMS + chat.TotalDurationMS, HTTPStatus: chat.HTTPStatus,
			ErrorCode: chat.ErrorCode, AvailableModels: models, ProbeModel: model, Attempt: chat,
		}
	}
	return VerificationResult{
		TotalDurationMS: discovery.DurationMS + responses.TotalDurationMS + chat.TotalDurationMS, HTTPStatus: chat.HTTPStatus,
		AvailableModels: models, ProbeModel: model, ProbeProtocol: ProtocolChatCompletionsV1, Attempt: chat,
	}
}

func (prober *OpenAIRealModelProber) Probe(ctx context.Context, job ProbeJob) ProbeResult {
	if prober == nil || job.CredentialError {
		return ProbeResult{Outcome: OutcomeFinalFailure, ErrorCode: ErrorDecryptFailed}
	}
	now := prober.now
	if now == nil {
		now = time.Now
	}
	cycleStartedAt := now()
	first := prober.executeAttempt(ctx, job, 1)
	first.CostUSD = AttemptCostUSD(job.Connection.Price, first.Usage)
	result := ProbeResult{Attempts: []ProbeAttempt{first}, HTTPStatus: first.HTTPStatus, HTTPStatusClass: first.HTTPStatus / 100, ErrorCode: first.ErrorCode}
	firstDuration := first.TotalDurationMS
	result.FirstAttemptTotalDurationMS = &firstDuration
	if first.Succeeded {
		result.Outcome = OutcomeFirstSuccess
		result.ErrorCode = ""
		result.FirstAttemptTTFTMS = first.TTFTMS
		if job.LatencyRule != nil && first.TTFTMS != nil && *first.TTFTMS > job.LatencyRule.SlowTTFTMS {
			result.Outcome = OutcomeFirstSuccessSlow
		}
		result.BaseCostUSD = first.CostUSD
		result.Usage = first.Usage
		result.UsageComplete = first.Usage.Complete()
		result.TotalDurationMS = elapsedMilliseconds(now().Sub(cycleStartedAt))
		return result
	}
	if !first.Retryable {
		result.Outcome = OutcomeFinalFailure
		result.BaseCostUSD = first.CostUSD
		result.Usage = first.Usage
		result.UsageComplete = first.Usage.Complete()
		result.TotalDurationMS = elapsedMilliseconds(now().Sub(cycleStartedAt))
		return result
	}
	wait := time.Duration(first.RetryAfterMS) * time.Millisecond
	if wait <= 0 || wait > 3*time.Second {
		wait = prober.randomWait()
	}
	if err := prober.sleep(ctx, wait); err != nil {
		result.Outcome = OutcomeFinalFailure
		result.TotalDurationMS = elapsedMilliseconds(now().Sub(cycleStartedAt))
		return result
	}
	second := prober.executeAttempt(ctx, job, 2)
	second.CostUSD = AttemptCostUSD(job.Connection.Price, second.Usage)
	result.Attempts = append(result.Attempts, second)
	result.HTTPStatus = second.HTTPStatus
	result.HTTPStatusClass = second.HTTPStatus / 100
	result.ErrorCode = second.ErrorCode
	result.BaseCostUSD = first.CostUSD
	result.RetryCostUSD = second.CostUSD
	result.TotalDurationMS = elapsedMilliseconds(now().Sub(cycleStartedAt))
	result.Usage, result.UsageComplete = aggregateUsage(result.Attempts)
	if second.Succeeded {
		result.Outcome = OutcomeRetryRecovered
		result.ErrorCode = ""
		recovery := result.TotalDurationMS
		result.RecoveryDurationMS = &recovery
		return result
	}
	result.Outcome = OutcomeFinalFailure
	return result
}

func (prober *OpenAIRealModelProber) executeAttempt(ctx context.Context, job ProbeJob, number int) ProbeAttempt {
	timeout := prober.timeout
	if job.LatencyRule != nil && job.LatencyRule.HardTimeoutMS > 0 {
		ruleTimeout := time.Duration(job.LatencyRule.HardTimeoutMS) * time.Millisecond
		if ruleTimeout < timeout {
			timeout = ruleTimeout
		}
	}
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client, err := prober.newClient(job.Connection.BaseURL, job.Credential, openaiapi.Options{
		AllowInsecureHTTP: UsesInsecureHTTP(job.Connection.BaseURL),
		Timeout:           timeout,
	})
	if err != nil {
		now := prober.now().UTC()
		return ProbeAttempt{AttemptNumber: number, StartedAt: now, FinishedAt: now, ErrorCode: ErrorBlockedTarget}
	}
	return prober.streamAttempt(attemptCtx, client, job.Connection.ProbeProtocol, job.Connection.ProbeModel, number)
}

func (prober *OpenAIRealModelProber) streamAttempt(ctx context.Context, client openAIProbeClient, protocol, model string, number int) ProbeAttempt {
	startedAt := prober.now().UTC()
	stream := client.StreamProbe(ctx, protocol, model)
	finishedAt := prober.now().UTC()
	usage := TokenUsage{
		InputTokens: stream.Usage.InputTokens, CachedInputTokens: stream.Usage.CachedInputTokens,
		OutputTokens: stream.Usage.OutputTokens, ReasoningTokens: stream.Usage.ReasoningTokens,
	}
	errorCode := mapOpenAIError(stream.ErrorCode)
	return ProbeAttempt{
		AttemptNumber: number, StartedAt: startedAt, FirstTextAt: stream.FirstTextAt, FinishedAt: finishedAt,
		HTTPStatus: stream.HTTPStatus, TTFTMS: stream.TTFTMS, TotalDurationMS: stream.DurationMS,
		Succeeded: stream.Succeeded(), Retryable: retryableStreamResult(stream), ErrorCode: errorCode,
		RetryAfterMS: stream.RetryAfterMS, Usage: usage,
	}
}

func containsModel(models []string, target string) bool {
	for _, model := range models {
		if model == target {
			return true
		}
	}
	return false
}

func retryableStreamResult(result openaiapi.StreamResult) bool {
	switch result.ErrorCode {
	case openaiapi.ErrorDNS, openaiapi.ErrorConnect, openaiapi.ErrorTimeout, openaiapi.ErrorRateLimited, openaiapi.ErrorStreamInterrupted:
		return true
	case openaiapi.ErrorUpstream:
		return result.HTTPStatus == 500 || result.HTTPStatus == 502 || result.HTTPStatus == 503 || result.HTTPStatus == 504
	default:
		return false
	}
}

func aggregateUsage(attempts []ProbeAttempt) (TokenUsage, bool) {
	var input, cached, output, reasoning int64
	cacheKnown := true
	reasoningKnown := true
	for _, attempt := range attempts {
		if !attempt.Usage.Complete() {
			return TokenUsage{}, false
		}
		input += *attempt.Usage.InputTokens
		output += *attempt.Usage.OutputTokens
		if attempt.Usage.CachedInputTokens == nil {
			cacheKnown = false
		} else {
			cached += *attempt.Usage.CachedInputTokens
		}
		if attempt.Usage.ReasoningTokens == nil {
			reasoningKnown = false
		} else {
			reasoning += *attempt.Usage.ReasoningTokens
		}
	}
	usage := TokenUsage{InputTokens: &input, OutputTokens: &output}
	if cacheKnown {
		usage.CachedInputTokens = &cached
	}
	if reasoningKnown {
		usage.ReasoningTokens = &reasoning
	}
	return usage, true
}

func elapsedMilliseconds(duration time.Duration) int {
	value := int(duration.Milliseconds())
	if value < 0 {
		return 0
	}
	return value
}

func mapOpenAIError(code openaiapi.ErrorCode) string {
	switch code {
	case openaiapi.ErrorNone:
		return ""
	case openaiapi.ErrorAuthentication:
		return ErrorAuthorizationInvalid
	case openaiapi.ErrorBlockedTarget:
		return ErrorBlockedTarget
	case openaiapi.ErrorDNS:
		return ErrorDNSFailed
	case openaiapi.ErrorConnect:
		return ErrorConnectFailed
	case openaiapi.ErrorTLS:
		return ErrorTLSFailed
	case openaiapi.ErrorTimeout:
		return ErrorTimeout
	case openaiapi.ErrorRateLimited:
		return ErrorRateLimited
	case openaiapi.ErrorUpstream:
		return ErrorHTTP5xx
	case openaiapi.ErrorProtocolUnsupported:
		return ErrorProtocolUnavailable
	case openaiapi.ErrorRequestRejected:
		return ErrorHTTP4xx
	case openaiapi.ErrorResponseTooLarge:
		return ErrorResponseTooLarge
	case openaiapi.ErrorInvalidResponse:
		return ErrorInvalidResponse
	case openaiapi.ErrorStreamInterrupted:
		return ErrorStreamInterrupted
	default:
		return ErrorInternal
	}
}
