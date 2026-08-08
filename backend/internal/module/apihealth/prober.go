package apihealth

import (
	"context"
	"time"

	"c2c-market/backend/internal/platform/openaiapi"
)

type Prober interface {
	Probe(ctx context.Context, job ProbeJob) ProbeResult
}

type OpenAIModelsProber struct {
	timeout time.Duration
}

func NewOpenAIModelsProber(timeout time.Duration) *OpenAIModelsProber {
	return &OpenAIModelsProber{timeout: timeout}
}

func (prober *OpenAIModelsProber) Verify(ctx context.Context, baseURL, credential string, allowInsecureHTTP bool) ProbeResult {
	if prober == nil {
		return ProbeResult{ErrorCode: ErrorInternal}
	}
	client, err := openaiapi.NewClient(baseURL, credential, openaiapi.Options{
		AllowInsecureHTTP: allowInsecureHTTP,
		Timeout:           prober.timeout,
	})
	if err != nil {
		return ProbeResult{ErrorCode: mapOpenAIError(openaiapi.ErrorBlockedTarget)}
	}
	_, result := client.DiscoverModels(ctx)
	return probeResult(result)
}

func (prober *OpenAIModelsProber) Probe(ctx context.Context, job ProbeJob) ProbeResult {
	if job.CredentialError {
		return ProbeResult{ErrorCode: ErrorDecryptFailed}
	}
	return prober.Verify(ctx, job.Connection.BaseURL, job.Credential, UsesInsecureHTTP(job.Connection.BaseURL))
}

func probeResult(result openaiapi.Result) ProbeResult {
	return ProbeResult{
		TotalDurationMS: result.DurationMS,
		HTTPStatusClass: result.HTTPStatusClass,
		ErrorCode:       mapOpenAIError(result.ErrorCode),
	}
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
	case openaiapi.ErrorRequestRejected, openaiapi.ErrorProtocolUnsupported:
		return ErrorHTTP4xx
	case openaiapi.ErrorResponseTooLarge:
		return ErrorResponseTooLarge
	case openaiapi.ErrorInvalidResponse:
		return ErrorInvalidResponse
	default:
		return ErrorInternal
	}
}
