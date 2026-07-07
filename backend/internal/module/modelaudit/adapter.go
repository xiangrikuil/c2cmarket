package modelaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AuditChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AuditChatRequest struct {
	Model       string             `json:"model"`
	Messages    []AuditChatMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Logprobs    bool               `json:"logprobs,omitempty"`
	TopLogprobs int                `json:"top_logprobs,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Timeout     time.Duration      `json:"-"`
}

type AuditChatResponse struct {
	Text                string
	Raw                 map[string]any
	Model               string
	FinishReason        string
	Usage               *AuditUsage
	Logprobs            []AuditLogprob
	LatencyMS           int
	FirstTokenLatencyMS int
	StatusCode          int
	Headers             map[string]string
}

type AuditUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type AuditLogprob struct {
	Token       string
	Logprob     float64
	TopLogprobs []AuditTopLogprob
}

type AuditTopLogprob struct {
	Token   string
	Logprob float64
}

type ProviderAdapter interface {
	Chat(ctx context.Context, request AuditChatRequest) (AuditChatResponse, error)
}

type OpenAICompatibleAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewOpenAICompatibleAdapter(baseURL, apiKey string) *OpenAICompatibleAdapter {
	return &OpenAICompatibleAdapter{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *OpenAICompatibleAdapter) Chat(ctx context.Context, request AuditChatRequest) (AuditChatResponse, error) {
	if a == nil || a.baseURL == "" {
		return AuditChatResponse{}, fmt.Errorf("provider adapter is not configured")
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	request.Stream = false
	body, err := json.Marshal(request)
	if err != nil {
		return AuditChatResponse{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return AuditChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	start := time.Now()
	resp, err := a.client.Do(httpReq)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return AuditChatResponse{}, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return AuditChatResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AuditChatResponse{}, fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	var parsed openAIChatCompletionResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return AuditChatResponse{}, err
	}
	raw := map[string]any{}
	_ = json.Unmarshal(rawBody, &raw)
	headers := map[string]string{}
	for name, values := range resp.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}
	result := AuditChatResponse{
		Raw:        raw,
		Model:      parsed.Model,
		LatencyMS:  latency,
		StatusCode: resp.StatusCode,
		Headers:    headers,
	}
	if parsed.Usage != nil {
		result.Usage = &AuditUsage{
			PromptTokens:     parsed.Usage.PromptTokens,
			CompletionTokens: parsed.Usage.CompletionTokens,
			TotalTokens:      parsed.Usage.TotalTokens,
		}
	}
	if len(parsed.Choices) > 0 {
		result.Text = parsed.Choices[0].Message.Content
		result.FinishReason = parsed.Choices[0].FinishReason
	}
	return result, nil
}

func (a *OpenAICompatibleAdapter) ListModels(ctx context.Context) ([]string, error) {
	if a == nil || a.baseURL == "" {
		return nil, fmt.Errorf("provider adapter is not configured")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("models endpoint returned status %d", resp.StatusCode)
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&parsed); err != nil {
		return nil, err
	}
	models := make([]string, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		if strings.TrimSpace(item.ID) != "" {
			models = append(models, item.ID)
		}
	}
	return models, nil
}

func (a *OpenAICompatibleAdapter) SupportsLogprobs(ctx context.Context) bool {
	return true
}

type openAIChatCompletionResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
