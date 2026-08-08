package openaiapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestDiscoverModelsUsesBearerAndReturnsUniqueIDsInOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/models" {
			t.Fatalf("path=%q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer probe-key" {
			t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"gpt-4.1-mini"},{"id":"custom/model"},{"id":"gpt-4.1-mini"}]}`))
	}))
	defer server.Close()

	client := newClient(server.URL+"/v1", "probe-key", server.Client())
	models, result := client.DiscoverModels(context.Background())
	if !result.Succeeded() {
		t.Fatalf("DiscoverModels() result=%+v", result)
	}
	want := []string{"gpt-4.1-mini", "custom/model"}
	if !reflect.DeepEqual(models, want) {
		t.Fatalf("models=%v, want %v", models, want)
	}
}

func TestDiscoverModelsRequiresCompatibleEnvelopeAndNonemptyIDs(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":null}`, `{"data":{}}`, `{"data":[{}]}`, `{"data":[{"id":""}]}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				_, _ = writer.Write([]byte(body))
			}))
			defer server.Close()
			_, result := newClient(server.URL, "key", server.Client()).DiscoverModels(context.Background())
			if result.ErrorCode != ErrorInvalidResponse {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestModelCallsValidateEachProtocolIndependently(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/responses":
			_, _ = writer.Write([]byte(`{"id":"response-1","output":[]}`))
		case "/chat/completions":
			_, _ = writer.Write([]byte(`{"id":"chat-1","choices":[]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := newClient(server.URL, "key", server.Client())
	if result := client.TestResponses(context.Background(), "gpt-4.1-mini"); !result.Succeeded() {
		t.Fatalf("responses result=%+v", result)
	}
	if result := client.TestChatCompletions(context.Background(), "gpt-4.1-mini"); !result.Succeeded() {
		t.Fatalf("chat result=%+v", result)
	}
}

func TestModelCallsRejectMalformedProtocolEnvelopes(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		chatBody     string
	}{
		{name: "missing arrays", responseBody: `{}`, chatBody: `{}`},
		{name: "null arrays", responseBody: `{"output":null}`, chatBody: `{"choices":null}`},
		{name: "string arrays", responseBody: `{"output":"ok"}`, chatBody: `{"choices":"ok"}`},
		{name: "object arrays", responseBody: `{"output":{}}`, chatBody: `{"choices":{}}`},
		{name: "top level error", responseBody: `{"output":[],"error":{"message":"failed"}}`, chatBody: `{"choices":[],"error":{"message":"failed"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/responses" {
					_, _ = writer.Write([]byte(test.responseBody))
					return
				}
				_, _ = writer.Write([]byte(test.chatBody))
			}))
			defer server.Close()
			client := newClient(server.URL, "key", server.Client())
			if result := client.TestResponses(context.Background(), "gpt-4.1-mini"); result.ErrorCode != ErrorInvalidResponse {
				t.Fatalf("responses result=%+v", result)
			}
			if result := client.TestChatCompletions(context.Background(), "gpt-4.1-mini"); result.ErrorCode != ErrorInvalidResponse {
				t.Fatalf("chat result=%+v", result)
			}
		})
	}
}

func TestHTTPStatusClassification(t *testing.T) {
	tests := map[int]ErrorCode{
		http.StatusUnauthorized:        ErrorAuthentication,
		http.StatusForbidden:           ErrorAuthentication,
		http.StatusBadRequest:          ErrorProtocolUnsupported,
		http.StatusNotFound:            ErrorProtocolUnsupported,
		http.StatusTooManyRequests:     ErrorRateLimited,
		http.StatusTeapot:              ErrorRequestRejected,
		http.StatusInternalServerError: ErrorUpstream,
		http.StatusServiceUnavailable:  ErrorUpstream,
	}
	for status, expected := range tests {
		if actual := classifyHTTPStatus(status); actual != expected {
			t.Fatalf("status %d classified as %q, want %q", status, actual, expected)
		}
	}
}
