package modelaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"c2c-market/backend/internal/platform/outboundhttp"
)

const (
	chatResponseLimit   = 4 * 1024 * 1024
	modelsResponseLimit = 2 * 1024 * 1024
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
	policy, err := outboundhttp.NewPolicy(nil)
	if err != nil {
		panic(err)
	}
	return newOpenAICompatibleAdapter(baseURL, apiKey, outboundhttp.NewClient(policy))
}

func newOpenAICompatibleAdapter(baseURL, apiKey string, client *http.Client) *OpenAICompatibleAdapter {
	if client == nil {
		policy, err := outboundhttp.NewPolicy(nil)
		if err != nil {
			panic(err)
		}
		client = outboundhttp.NewClient(policy)
	}
	return &OpenAICompatibleAdapter{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		client:  client,
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
		return AuditChatResponse{}, fmt.Errorf("provider request encoding failed")
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	endpoint, err := url.JoinPath(a.baseURL, "chat/completions")
	if err != nil {
		return AuditChatResponse{}, fmt.Errorf("provider endpoint is invalid")
	}
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return AuditChatResponse{}, fmt.Errorf("provider endpoint is invalid")
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	start := time.Now()
	resp, err := a.client.Do(httpReq)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return AuditChatResponse{}, sanitizeProviderRequestError(err)
	}
	defer resp.Body.Close()
	rawBody, err := outboundhttp.ReadBody(resp.Body, chatResponseLimit)
	if err != nil {
		return AuditChatResponse{}, sanitizeProviderBodyError(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AuditChatResponse{}, fmt.Errorf("provider returned status %d", resp.StatusCode)
	}
	var parsed openAIChatCompletionResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return AuditChatResponse{}, fmt.Errorf("provider response is invalid")
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
	endpoint, err := url.JoinPath(a.baseURL, "models")
	if err != nil {
		return nil, fmt.Errorf("provider endpoint is invalid")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("provider endpoint is invalid")
	}
	if a.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, sanitizeProviderRequestError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("models endpoint returned status %d", resp.StatusCode)
	}
	rawBody, err := outboundhttp.ReadBody(resp.Body, modelsResponseLimit)
	if err != nil {
		return nil, sanitizeProviderBodyError(err)
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return nil, fmt.Errorf("models response is invalid")
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

func sanitizeProviderRequestError(err error) error {
	switch {
	case errors.Is(err, outboundhttp.ErrRedirectNotAllowed):
		return fmt.Errorf("provider redirect is not allowed: %w", outboundhttp.ErrRedirectNotAllowed)
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("provider request timed out: %w", context.DeadlineExceeded)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("provider request canceled: %w", context.Canceled)
	default:
		return fmt.Errorf("provider request failed")
	}
}

func sanitizeProviderBodyError(err error) error {
	if errors.Is(err, outboundhttp.ErrResponseTooLarge) {
		return fmt.Errorf("provider response is too large: %w", outboundhttp.ErrResponseTooLarge)
	}
	return fmt.Errorf("provider response read failed")
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
