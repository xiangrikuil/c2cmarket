package modelaudit

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

func TestMaskAPIKeyDoesNotRevealMiddle(t *testing.T) {
	masked := MaskAPIKey("test-secret-value-123456")
	if strings.Contains(masked, "secret") || strings.Contains(masked, "value") {
		t.Fatalf("masked key reveals middle material: %q", masked)
	}
	if !strings.HasPrefix(masked, "test") || !strings.HasSuffix(masked, "3456") {
		t.Fatalf("masked key should keep only stable edges, got %q", masked)
	}
}

func TestOpenAICompatibleAdapterBuildsChatRequest(t *testing.T) {
	const apiKey = "test-secret-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+apiKey {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-example","choices":[{"message":{"content":"7"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":1,"total_tokens":5}}`))
	}))
	defer server.Close()

	adapter := newOpenAICompatibleAdapter(server.URL+"/v1/", apiKey, server.Client())
	response, err := adapter.Chat(context.Background(), AuditChatRequest{
		Model: "gpt-example",
		Messages: []AuditChatMessage{{
			Role:    "user",
			Content: "Return one digit.",
		}},
		MaxTokens: 1,
	})
	if err != nil {
		t.Fatalf("Chat() returned error: %v", err)
	}
	if response.Text != "7" || response.Model != "gpt-example" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Usage == nil || response.Usage.TotalTokens != 5 {
		t.Fatalf("usage was not parsed: %+v", response.Usage)
	}
}

func TestOpenAICompatibleAdapterListsModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/v1/models" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-a"},{"id":"  "},{"id":"gpt-b"}]}`))
	}))
	defer server.Close()

	adapter := newOpenAICompatibleAdapter(server.URL+"/v1", "test-secret", server.Client())
	models, err := adapter.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() returned error: %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-a" || models[1] != "gpt-b" {
		t.Fatalf("unexpected models: %v", models)
	}
}

func TestOpenAICompatibleAdapterRejectsOversizedResponses(t *testing.T) {
	tests := []struct {
		name string
		path string
		size int
		call func(context.Context, *OpenAICompatibleAdapter) error
	}{
		{
			name: "chat",
			path: "/chat/completions",
			size: chatResponseLimit + 1,
			call: func(ctx context.Context, adapter *OpenAICompatibleAdapter) error {
				_, err := adapter.Chat(ctx, AuditChatRequest{Model: "gpt-example"})
				return err
			},
		},
		{
			name: "models",
			path: "/models",
			size: modelsResponseLimit + 1,
			call: func(ctx context.Context, adapter *OpenAICompatibleAdapter) error {
				_, err := adapter.ListModels(ctx)
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				if request.URL.Path != tt.path {
					t.Fatalf("unexpected path: %s", request.URL.Path)
				}
				_, _ = w.Write([]byte(strings.Repeat("x", tt.size)))
			}))
			defer server.Close()

			adapter := newOpenAICompatibleAdapter(server.URL, "test-secret", server.Client())
			err := tt.call(context.Background(), adapter)
			if !errors.Is(err, outboundhttp.ErrResponseTooLarge) {
				t.Fatalf("expected bounded response error, got %v", err)
			}
		})
	}
}

func TestOpenAICompatibleAdapterSanitizesRequestErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	adapter := newOpenAICompatibleAdapter(server.URL, "test-secret", server.Client())
	_, err := adapter.Chat(context.Background(), AuditChatRequest{
		Model:   "gpt-example",
		Timeout: 20 * time.Millisecond,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
	if strings.Contains(err.Error(), server.URL) || strings.Contains(err.Error(), "test-secret") {
		t.Fatalf("request error leaked target or credential: %v", err)
	}
}
