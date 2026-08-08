package openaiapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/platform/outboundhttp"
)

const (
	ProtocolResponsesV1       = "openai_responses_v1"
	ProtocolChatCompletionsV1 = "openai_chat_completions_v1"
	ProbePrompt               = "Reply with OK."
	ProbeMaxOutputTokens      = 32
	streamResponseLimit       = 2 * 1024 * 1024
	streamLineLimit           = 256 * 1024
)

type Usage struct {
	InputTokens       *int64
	CachedInputTokens *int64
	OutputTokens      *int64
	ReasoningTokens   *int64
}

func (usage Usage) Complete() bool {
	return usage.InputTokens != nil && usage.OutputTokens != nil
}

type StreamResult struct {
	Result
	TTFTMS      *int
	FirstTextAt *time.Time
	Usage       Usage
}

func (client *Client) StreamProbe(ctx context.Context, protocol, model string) StreamResult {
	model = strings.TrimSpace(model)
	if model == "" {
		return StreamResult{Result: Result{ErrorCode: ErrorInternal}}
	}
	var path string
	var payload any
	switch protocol {
	case ProtocolResponsesV1:
		path = "responses"
		payload = struct {
			Model           string `json:"model"`
			Input           string `json:"input"`
			Stream          bool   `json:"stream"`
			Store           bool   `json:"store"`
			MaxOutputTokens int    `json:"max_output_tokens"`
		}{model, ProbePrompt, true, false, ProbeMaxOutputTokens}
	case ProtocolChatCompletionsV1:
		path = "chat/completions"
		payload = struct {
			Model     string              `json:"model"`
			Messages  []map[string]string `json:"messages"`
			Stream    bool                `json:"stream"`
			MaxTokens int                 `json:"max_tokens"`
			Options   map[string]bool     `json:"stream_options,omitempty"`
		}{model, []map[string]string{{"role": "user", "content": ProbePrompt}}, true, ProbeMaxOutputTokens, map[string]bool{"include_usage": true}}
	default:
		return StreamResult{Result: Result{ErrorCode: ErrorProtocolUnsupported}}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return StreamResult{Result: Result{ErrorCode: ErrorInternal}}
	}
	return client.doStream(ctx, path, body, protocol)
}

func (client *Client) doStream(ctx context.Context, path string, body []byte, protocol string) StreamResult {
	if client == nil || client.httpClient == nil || client.baseURL == "" || client.apiKey == "" {
		return StreamResult{Result: Result{ErrorCode: ErrorInternal}}
	}
	now := client.now
	if now == nil {
		now = time.Now
	}
	startedAt := now()
	finish := func(result *StreamResult, code ErrorCode, status, retryAfterMS int) StreamResult {
		duration := int(now().Sub(startedAt).Milliseconds())
		if duration < 0 {
			duration = 0
		}
		result.Result = Result{HTTPStatus: status, HTTPStatusClass: status / 100, DurationMS: duration, RetryAfterMS: retryAfterMS, ErrorCode: code}
		return *result
	}
	endpoint, err := JoinEndpoint(client.baseURL, path)
	if err != nil {
		return finish(&StreamResult{}, ErrorInternal, 0, 0)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return finish(&StreamResult{}, ErrorInternal, 0, 0)
	}
	request.Header.Set("Authorization", "Bearer "+client.apiKey)
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return finish(&StreamResult{}, classifyRequestError(ctx, err), 0, 0)
	}
	defer response.Body.Close()
	retryAfterMS := parseRetryAfter(response.Header.Get("Retry-After"), now())
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, readErr := outboundhttp.ReadBody(response.Body, callResponseLimit)
		if readErr != nil {
			return finish(&StreamResult{}, classifyRequestError(ctx, readErr), response.StatusCode, retryAfterMS)
		}
		return finish(&StreamResult{}, classifyHTTPResponse(response.StatusCode, responseBody), response.StatusCode, retryAfterMS)
	}
	result := StreamResult{}
	completed, parseErr := readSSE(response.Body, protocol, func(text string, usage *Usage, done bool) {
		if strings.TrimSpace(text) != "" && result.TTFTMS == nil {
			firstTextAt := now().UTC()
			ttft := int(firstTextAt.Sub(startedAt).Milliseconds())
			if ttft < 0 {
				ttft = 0
			}
			result.FirstTextAt = &firstTextAt
			result.TTFTMS = &ttft
		}
		if usage != nil {
			result.Usage = *usage
		}
	})
	if parseErr != nil {
		code := ErrorInvalidResponse
		if errors.Is(parseErr, io.ErrUnexpectedEOF) {
			code = ErrorStreamInterrupted
		} else if errors.Is(parseErr, outboundhttp.ErrResponseTooLarge) {
			code = ErrorResponseTooLarge
		} else if ctx.Err() != nil {
			code = classifyRequestError(ctx, ctx.Err())
		}
		return finish(&result, code, response.StatusCode, retryAfterMS)
	}
	if !completed || result.TTFTMS == nil {
		return finish(&result, ErrorInvalidResponse, response.StatusCode, retryAfterMS)
	}
	return finish(&result, ErrorNone, response.StatusCode, retryAfterMS)
}

type streamEvent struct {
	Event string
	Data  []byte
}

func readSSE(reader io.Reader, protocol string, consume func(text string, usage *Usage, done bool)) (bool, error) {
	buffered := bufio.NewReaderSize(reader, 32*1024)
	total := 0
	eventName := ""
	dataLines := make([]string, 0, 1)
	completed := false
	flush := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		data := []byte(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		name := eventName
		eventName = ""
		if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
			if protocol != ProtocolChatCompletionsV1 {
				return errors.New("unexpected done marker")
			}
			completed = true
			consume("", nil, true)
			return nil
		}
		text, usage, done, err := parseStreamEvent(streamEvent{Event: name, Data: data}, protocol)
		if err != nil {
			return err
		}
		if done {
			completed = true
		}
		consume(text, usage, done)
		return nil
	}
	for {
		line, err := buffered.ReadString('\n')
		total += len(line)
		if len(line) > streamLineLimit || total > streamResponseLimit {
			return false, outboundhttp.ErrResponseTooLarge
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			if flushErr := flush(); flushErr != nil {
				return false, flushErr
			}
		} else if !strings.HasPrefix(line, ":") {
			field, value, found := strings.Cut(line, ":")
			if found && strings.HasPrefix(value, " ") {
				value = value[1:]
			}
			switch field {
			case "event":
				eventName = value
			case "data":
				dataLines = append(dataLines, value)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(dataLines) > 0 {
					if flushErr := flush(); flushErr != nil {
						return false, flushErr
					}
				}
				if completed {
					return true, nil
				}
				return false, io.ErrUnexpectedEOF
			}
			return false, err
		}
	}
}

func parseStreamEvent(event streamEvent, protocol string) (string, *Usage, bool, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(event.Data, &envelope); err != nil || envelope == nil {
		return "", nil, false, errors.New("invalid stream event")
	}
	if raw := envelope["error"]; len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", nil, false, errors.New("stream error event")
	}
	switch protocol {
	case ProtocolResponsesV1:
		var eventType string
		_ = json.Unmarshal(envelope["type"], &eventType)
		if eventType == "" {
			eventType = event.Event
		}
		switch eventType {
		case "response.output_text.delta":
			var delta string
			if err := json.Unmarshal(envelope["delta"], &delta); err != nil {
				return "", nil, false, errors.New("invalid response text delta")
			}
			return delta, nil, false, nil
		case "response.completed":
			usage := responsesUsage(envelope["response"])
			return "", &usage, true, nil
		case "response.failed", "response.incomplete", "error":
			return "", nil, false, errors.New("response stream failed")
		default:
			return "", nil, false, nil
		}
	case ProtocolChatCompletionsV1:
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content json.RawMessage `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage json.RawMessage `json:"usage"`
		}
		if err := json.Unmarshal(event.Data, &chunk); err != nil {
			return "", nil, false, errors.New("invalid chat stream chunk")
		}
		text := ""
		for _, choice := range chunk.Choices {
			if len(choice.Delta.Content) == 0 || bytes.Equal(bytes.TrimSpace(choice.Delta.Content), []byte("null")) {
				continue
			}
			var value string
			if err := json.Unmarshal(choice.Delta.Content, &value); err != nil {
				return "", nil, false, errors.New("invalid chat text delta")
			}
			text += value
		}
		if len(chunk.Usage) > 0 && !bytes.Equal(bytes.TrimSpace(chunk.Usage), []byte("null")) {
			usage := chatUsage(chunk.Usage)
			return text, &usage, false, nil
		}
		return text, nil, false, nil
	default:
		return "", nil, false, errors.New("unsupported stream protocol")
	}
}

func responsesUsage(raw json.RawMessage) Usage {
	var value struct {
		Usage struct {
			InputTokens  *int64 `json:"input_tokens"`
			OutputTokens *int64 `json:"output_tokens"`
			InputDetails struct {
				CachedTokens *int64 `json:"cached_tokens"`
			} `json:"input_tokens_details"`
			OutputDetails struct {
				ReasoningTokens *int64 `json:"reasoning_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	}
	_ = json.Unmarshal(raw, &value)
	return Usage{InputTokens: value.Usage.InputTokens, CachedInputTokens: value.Usage.InputDetails.CachedTokens, OutputTokens: value.Usage.OutputTokens, ReasoningTokens: value.Usage.OutputDetails.ReasoningTokens}
}

func chatUsage(raw json.RawMessage) Usage {
	var value struct {
		PromptTokens     *int64 `json:"prompt_tokens"`
		CompletionTokens *int64 `json:"completion_tokens"`
		PromptDetails    struct {
			CachedTokens *int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionDetails struct {
			ReasoningTokens *int64 `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	}
	_ = json.Unmarshal(raw, &value)
	return Usage{InputTokens: value.PromptTokens, CachedInputTokens: value.PromptDetails.CachedTokens, OutputTokens: value.CompletionTokens, ReasoningTokens: value.CompletionDetails.ReasoningTokens}
}
