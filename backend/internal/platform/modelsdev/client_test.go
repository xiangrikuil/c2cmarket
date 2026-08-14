package modelsdev

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/platform/outboundhttp"
)

func TestFixedSourceClientRejectsRedirects(t *testing.T) {
	redirected := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			redirected = true
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/redirected", http.StatusFound)
	}))
	defer server.Close()

	_, err := newFixedSourceHTTPClient(time.Second).Get(server.URL)
	if !errors.Is(err, outboundhttp.ErrRedirectNotAllowed) {
		t.Fatalf("expected redirect rejection, got %v", err)
	}
	if redirected {
		t.Fatal("fixed-source client followed redirect")
	}
}

func TestClientFetchParsesCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Accept") != "application/json" {
			t.Fatalf("unexpected request method=%s accept=%q", r.Method, r.Header.Get("Accept"))
		}
		_, _ = w.Write([]byte(`{"openai":{"id":"openai","name":"OpenAI","models":{"gpt-4.1-mini":{"id":"gpt-4.1-mini","last_updated":"2026-08-01","reasoning":false,"modalities":{"input":["text"],"output":["text"]},"cost":{"input":0.4,"cache_read":0.1,"output":1.6}}}}}`))
	}))
	defer server.Close()

	catalog, err := NewClientForTest(server.Client(), server.URL, 1<<20).Fetch(context.Background())
	if err != nil {
		t.Fatalf("fetch catalog: %v", err)
	}
	model := catalog["openai"].Models["gpt-4.1-mini"]
	if model.ID != "gpt-4.1-mini" || model.Cost == nil || model.Cost.Input.String() != "0.4" || model.Cost.CacheRead.String() != "0.1" {
		t.Fatalf("unexpected model: %+v", model)
	}
}

func TestClientFetchRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		bodyLimit int64
		expected  error
	}{
		{name: "http failure", status: http.StatusBadGateway, body: `{}`, bodyLimit: 1024, expected: ErrUnavailable},
		{name: "empty catalog", status: http.StatusOK, body: `{}`, bodyLimit: 1024, expected: ErrInvalidData},
		{name: "trailing json", status: http.StatusOK, body: `{"openai":{"models":{}}} {}`, bodyLimit: 1024, expected: ErrInvalidData},
		{name: "oversized", status: http.StatusOK, body: strings.Repeat("x", 32), bodyLimit: 8, expected: ErrInvalidData},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()

			_, err := NewClientForTest(server.Client(), server.URL, test.bodyLimit).Fetch(context.Background())
			if !errors.Is(err, test.expected) {
				t.Fatalf("expected %v, got %v", test.expected, err)
			}
		})
	}
}

func TestClientFetchHonorsClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"openai":{"models":{}}}`))
	}))
	defer server.Close()

	client := NewClientForTest(&http.Client{Timeout: 10 * time.Millisecond}, server.URL, 1024)
	_, err := client.Fetch(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected timeout to report unavailable, got %v", err)
	}
}

func TestNewClientUsesDefaultTimeout(t *testing.T) {
	client := NewClient(0)
	if client.httpClient.Timeout != defaultTimeout {
		t.Fatalf("unexpected default timeout %s", client.httpClient.Timeout)
	}
}
