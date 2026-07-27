package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"c2c-market/backend/internal/config"
	app "c2c-market/backend/internal/module/core"
)

func TestMetricsEndpointRequiresProductionBearerToken(t *testing.T) {
	const token = "test-only-metrics-token-at-least-32-bytes"
	handler := NewServer(app.NewService(), ServerOptions{
		AppEnv:             config.EnvProduction,
		MetricsBearerToken: token,
	})

	for _, authorization := range []string{"", "Bearer wrong-token"} {
		request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q status = %d, want 401", authorization, response.Code)
		}
		if response.Header().Get("WWW-Authenticate") != `Bearer realm="metrics"` {
			t.Fatalf("missing metrics bearer challenge")
		}
		if strings.Contains(response.Body.String(), token) {
			t.Fatal("metrics authentication response leaked the configured token")
		}
	}

	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, httptest.NewRequest(http.MethodGet, "/health", nil))

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("authorized metrics status = %d body %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("metrics content type = %q", response.Header().Get("Content-Type"))
	}
	if !strings.Contains(
		response.Body.String(),
		`c2c_market_http_requests_total{method="GET",route="/health",status="200"} 1`,
	) {
		t.Fatalf("metrics output is missing route-pattern HTTP counter:\n%s", response.Body.String())
	}
}

func TestMetricsEndpointAllowsEmptyTokenOutsideProduction(t *testing.T) {
	handler := NewServer(app.NewService(), ServerOptions{AppEnv: config.EnvTest})
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("development metrics status = %d body %s", response.Code, response.Body.String())
	}
}
