package middleware

import (
	"bytes"
	"encoding/json"
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
	var entry struct {
		Event      string  `json:"event"`
		Method     string  `json:"method"`
		Path       string  `json:"path"`
		Status     int     `json:"status"`
		DurationMS float64 `json:"duration_ms"`
		RequestID  string  `json:"request_id"`
		ClientIP   string  `json:"client_ip"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry); err != nil {
		t.Fatalf("request log is not one-line JSON: %v, output %q", err, line)
	}
	if entry.Event != "http_request" || entry.Method != http.MethodPost ||
		entry.Path != "/log-target" || entry.Status != http.StatusCreated ||
		entry.RequestID != "req_test_request_log" || entry.ClientIP != "203.0.113.10" ||
		entry.DurationMS < 0 {
		t.Fatalf("unexpected request log entry: %+v", entry)
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

func TestRequestLoggingReplacesUnsafeRequestID(t *testing.T) {
	var output bytes.Buffer
	handler := WithRequestID(WithRequestLogging(
		log.New(&output, "", 0),
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	))
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set(RequestIDHeader, "attacker\nforged")

	handler.ServeHTTP(httptest.NewRecorder(), request)

	if strings.Contains(output.String(), "attacker") || strings.Contains(output.String(), "forged") {
		t.Fatalf("unsafe request ID reached logs: %q", output.String())
	}
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	requestID, _ := entry["request_id"].(string)
	if !strings.HasPrefix(requestID, "req_") {
		t.Fatalf("expected generated request ID, got %q", requestID)
	}
}
