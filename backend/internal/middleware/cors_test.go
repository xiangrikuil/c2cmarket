package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSPreflightAllowsAccountAppealCSRFHeader(t *testing.T) {
	handler := WithCORSAndOrigin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("preflight request reached the application handler")
	}), CORSOptions{
		AllowedOrigins: []string{"https://c2cmarket.shop"},
		Production:     true,
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/account-appeal/appeals", nil)
	request.Header.Set("Origin", "https://c2cmarket.shop")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "content-type,x-account-appeal-csrf,idempotency-key")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "https://c2cmarket.shop" {
		t.Fatalf("allow origin = %q", got)
	}
	allowedHeaders := strings.ToLower(response.Header().Get("Access-Control-Allow-Headers"))
	for _, header := range []string{"content-type", "x-account-appeal-csrf", "idempotency-key", "sentry-trace", "baggage"} {
		if !strings.Contains(allowedHeaders, header) {
			t.Fatalf("preflight does not allow %s: %q", header, allowedHeaders)
		}
	}
}
