package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggingIncludesRequestMetadataWithoutSensitivePayload(t *testing.T) {
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	handler := WithRequestID(
		WithClientIP(
			NewClientIPResolver(false, nil),
			WithRequestLogging(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			})),
		),
	)
	request := httptest.NewRequest(http.MethodPost, "/log-target?token=secret-token", strings.NewReader("secret-body"))
	request.RemoteAddr = "203.0.113.10:4321"
	request.Header.Set(RequestIDHeader, "req_test_request_log")
	request.Header.Set("CF-Connecting-IP", "192.0.2.10")
	request.Header.Set("X-Forwarded-For", "192.0.2.11, 10.0.0.8")
	request.Header.Set("X-Real-IP", "192.0.2.12")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	line := output.String()
	for _, expected := range []string{
		"method=POST",
		"path=/log-target",
		"status=201",
		"request_id=req_test_request_log",
		"client_ip=203.0.113.10",
		"duration=",
	} {
		if !strings.Contains(line, expected) {
			t.Fatalf("expected log line to contain %q, got %q", expected, line)
		}
	}
	for _, forbidden := range []string{
		"secret-body",
		"secret-token",
		"token=secret-token",
		"192.0.2.10",
		"192.0.2.11",
		"192.0.2.12",
		"X-Forwarded-For",
	} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("request log leaked %q in %q", forbidden, line)
		}
	}
}
