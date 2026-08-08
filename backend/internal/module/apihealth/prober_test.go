package apihealth

import (
	"testing"

	"c2c-market/backend/internal/platform/openaiapi"
)

func TestMapOpenAIErrorUsesStableLowCardinalityCodes(t *testing.T) {
	tests := map[openaiapi.ErrorCode]string{
		openaiapi.ErrorNone:             "",
		openaiapi.ErrorAuthentication:   ErrorAuthorizationInvalid,
		openaiapi.ErrorBlockedTarget:    ErrorBlockedTarget,
		openaiapi.ErrorDNS:              ErrorDNSFailed,
		openaiapi.ErrorConnect:          ErrorConnectFailed,
		openaiapi.ErrorTLS:              ErrorTLSFailed,
		openaiapi.ErrorTimeout:          ErrorTimeout,
		openaiapi.ErrorRateLimited:      ErrorRateLimited,
		openaiapi.ErrorUpstream:         ErrorHTTP5xx,
		openaiapi.ErrorInvalidResponse:  ErrorInvalidResponse,
		openaiapi.ErrorResponseTooLarge: ErrorResponseTooLarge,
	}
	for input, expected := range tests {
		if actual := mapOpenAIError(input); actual != expected {
			t.Fatalf("mapOpenAIError(%q)=%q, want %q", input, actual, expected)
		}
	}
}
