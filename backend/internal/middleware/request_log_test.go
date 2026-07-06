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
	handler := WithRequestID(WithRequestLogging(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})))
	request := httptest.NewRequest(http.MethodPost, "/log-target?token=secret-token", strings.NewReader("secret-body"))
	request.Header.Set(RequestIDHeader, "req_test_request_log")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	line := output.String()
	for _, expected := range []string{
		"method=POST",
		"path=/log-target",
		"status=201",
		"request_id=req_test_request_log",
		"duration=",
	} {
		if !strings.Contains(line, expected) {
			t.Fatalf("expected log line to contain %q, got %q", expected, line)
		}
	}
	for _, forbidden := range []string{"secret-body", "secret-token", "token=secret-token"} {
		if strings.Contains(line, forbidden) {
			t.Fatalf("request log leaked %q in %q", forbidden, line)
		}
	}
}
