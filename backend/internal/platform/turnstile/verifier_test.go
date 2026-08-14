package turnstile

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestVerifierAcceptsMatchingSuccessfulResponse(t *testing.T) {
	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}
		received, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(siteverifyResponse{
			Success:  true,
			Hostname: "C2CMarket.Shop",
			Action:   "password_login",
		})
	}))
	defer server.Close()

	verifier := newTestVerifier(t, server.URL)
	if err := verifier.Verify(context.Background(), Verification{
		Token:    "one-time-token",
		Action:   "password_login",
		RemoteIP: "203.0.113.10",
	}); err != nil {
		t.Fatalf("verify matching response: %v", err)
	}
	if received.Get("secret") != "test-secret" || received.Get("response") != "one-time-token" || received.Get("remoteip") != "203.0.113.10" {
		t.Fatalf("unexpected Siteverify form fields: %v", received)
	}
}

func TestVerifierFailsClosed(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		action     string
		hostname   string
		token      string
		bodySuffix string
	}{
		{name: "missing token", status: http.StatusOK, body: `{}`, token: ""},
		{name: "oversized token", status: http.StatusOK, body: `{}`, token: strings.Repeat("x", maxTokenBytes+1)},
		{name: "provider rejection", status: http.StatusOK, body: `{"success":false}`, token: "token"},
		{name: "wrong action", status: http.StatusOK, body: `{"success":true,"hostname":"c2cmarket.shop","action":"student_signup"}`, token: "token"},
		{name: "wrong hostname", status: http.StatusOK, body: `{"success":true,"hostname":"attacker.example","action":"password_login"}`, token: "token"},
		{name: "malformed response", status: http.StatusOK, body: `{`, token: "token"},
		{name: "trailing response", status: http.StatusOK, body: `{"success":true,"hostname":"c2cmarket.shop","action":"password_login"}`, bodySuffix: `{}`, token: "token"},
		{name: "non success status", status: http.StatusBadGateway, body: `{}`, token: "token"},
		{name: "oversized response", status: http.StatusOK, body: strings.Repeat("x", maxResponseBodyBytes+1), token: "token"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body+test.bodySuffix)
			}))
			defer server.Close()
			verifier := newTestVerifier(t, server.URL)
			if err := verifier.Verify(context.Background(), Verification{Token: test.token, Action: "password_login"}); err == nil {
				t.Fatal("expected verification failure")
			}
		})
	}
}

func TestVerifierSanitizesTransportFailure(t *testing.T) {
	client := &http.Client{
		Timeout: time.Second,
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}
	verifier, err := New("test-secret", []string{"c2cmarket.shop"}, Options{HTTPClient: client})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	err = verifier.Verify(context.Background(), Verification{Token: "sensitive-token", Action: "password_login"})
	if err == nil || strings.Contains(err.Error(), "sensitive-token") || strings.Contains(err.Error(), "test-secret") {
		t.Fatalf("expected sanitized transport failure, got %v", err)
	}
}

func newTestVerifier(t *testing.T, endpoint string) *Client {
	t.Helper()
	verifier, err := New("test-secret", []string{"c2cmarket.shop"}, Options{
		HTTPClient: &http.Client{Timeout: time.Second},
		Endpoint:   endpoint,
	})
	if err != nil {
		t.Fatalf("create verifier: %v", err)
	}
	return verifier
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
