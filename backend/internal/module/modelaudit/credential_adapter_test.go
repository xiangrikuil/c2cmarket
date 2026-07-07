package modelaudit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+apiKey {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-example","choices":[{"message":{"content":"7"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":1,"total_tokens":5}}`))
	}))
	defer server.Close()

	adapter := NewOpenAICompatibleAdapter(server.URL, apiKey)
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
