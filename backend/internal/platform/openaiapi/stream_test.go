package openaiapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestReadSSEResponsesHandlesFragmentedCRLFMultilineDataAndUsage(t *testing.T) {
	body := ": keep-alive\r\n\r\n" +
		"event: response.output_text.delta\r\n" +
		"data: {\"type\":\"response.output_text.delta\",\r\n" +
		"data: \"delta\":\"OK\"}\r\n\r\n" +
		"event: response.completed\r\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":7,\"output_tokens\":2,\"input_tokens_details\":{\"cached_tokens\":3},\"output_tokens_details\":{\"reasoning_tokens\":1}}}}\r\n\r\n"
	reader := &fragmentedReader{data: []byte(body), chunkSize: 3}
	var text string
	var usage Usage
	completed, err := readSSE(reader, ProtocolResponsesV1, func(delta string, current *Usage, _ bool) {
		text += delta
		if current != nil {
			usage = *current
		}
	})
	if err != nil || !completed {
		t.Fatalf("readSSE() completed=%v error=%v", completed, err)
	}
	if text != "OK" || !usage.Complete() || value(usage.InputTokens) != 7 || value(usage.CachedInputTokens) != 3 || value(usage.OutputTokens) != 2 || value(usage.ReasoningTokens) != 1 {
		t.Fatalf("unexpected text/usage: text=%q usage=%+v", text, usage)
	}
}

func TestReadSSEChatExtractsUsageAndRequiresDone(t *testing.T) {
	body := "data: {\"choices\":[{\"delta\":{\"content\":\"O\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"K\"}}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2,\"prompt_tokens_details\":{\"cached_tokens\":1},\"completion_tokens_details\":{\"reasoning_tokens\":1}}}\n\n" +
		"data: [DONE]\n\n"
	var text string
	var usage Usage
	completed, err := readSSE(strings.NewReader(body), ProtocolChatCompletionsV1, func(delta string, current *Usage, _ bool) {
		text += delta
		if current != nil {
			usage = *current
		}
	})
	if err != nil || !completed || text != "OK" {
		t.Fatalf("readSSE() completed=%v text=%q error=%v", completed, text, err)
	}
	if !usage.Complete() || value(usage.InputTokens) != 5 || value(usage.OutputTokens) != 2 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestStreamProbeKeepsTTFTButFailsWhenStreamInterruptsAfterVisibleText(t *testing.T) {
	stream := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n"
	client := newClient("https://api.example.test/v1", "key", &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: &interruptingReader{data: []byte(stream)}}, nil
	})})
	client.now = steppedClock(time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC), 10*time.Millisecond)

	result := client.StreamProbe(context.Background(), ProtocolResponsesV1, "gpt-5.6-luna")

	if result.ErrorCode != ErrorStreamInterrupted || result.TTFTMS == nil || *result.TTFTMS <= 0 || result.FirstTextAt == nil {
		t.Fatalf("unexpected interrupted result: %+v", result)
	}
}

func TestStreamProbeExtractsResponsesAndChatUsage(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		body     string
	}{
		{
			name: "responses", protocol: ProtocolResponsesV1,
			body: "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"OK\"}\n\n" +
				"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":6,\"output_tokens\":2}}}\n\n",
		},
		{
			name: "chat", protocol: ProtocolChatCompletionsV1,
			body: "data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\n" +
				"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":6,\"completion_tokens\":2}}\n\n" +
				"data: [DONE]\n\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newClient("https://api.example.test/v1", "key", &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})})
			client.now = steppedClock(time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC), time.Millisecond)
			result := client.StreamProbe(context.Background(), test.protocol, "gpt-5.6-luna")
			if !result.Succeeded() || !result.Usage.Complete() || value(result.Usage.InputTokens) != 6 || value(result.Usage.OutputTokens) != 2 {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestClassifyHTTPStatusDoesNotTreatBadRequestAsProtocolFallback(t *testing.T) {
	if code := classifyHTTPStatus(http.StatusBadRequest); code != ErrorRequestRejected {
		t.Fatalf("400 classified as %q", code)
	}
	if code := classifyHTTPStatus(http.StatusNotFound); code != ErrorProtocolUnsupported {
		t.Fatalf("404 classified as %q", code)
	}
}

func TestClassifyHTTPResponseDoesNotTreatModelNotFoundAsProtocolFallback(t *testing.T) {
	body := []byte(`{"error":{"type":"invalid_request_error","param":"model","code":"model_not_found"}}`)
	if code := classifyHTTPResponse(http.StatusNotFound, body); code != ErrorRequestRejected {
		t.Fatalf("model-specific 404 classified as %q", code)
	}
	if code := classifyHTTPResponse(http.StatusNotFound, []byte("route not found")); code != ErrorProtocolUnsupported {
		t.Fatalf("path 404 classified as %q", code)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type fragmentedReader struct {
	data      []byte
	offset    int
	chunkSize int
}

func (reader *fragmentedReader) Read(target []byte) (int, error) {
	if reader.offset >= len(reader.data) {
		return 0, io.EOF
	}
	limit := reader.offset + reader.chunkSize
	if limit > len(reader.data) {
		limit = len(reader.data)
	}
	n := copy(target, reader.data[reader.offset:limit])
	reader.offset += n
	return n, nil
}

type interruptingReader struct {
	data []byte
	done bool
}

func (reader *interruptingReader) Read(target []byte) (int, error) {
	if reader.done {
		return 0, io.ErrUnexpectedEOF
	}
	reader.done = true
	return copy(target, reader.data), nil
}

func (reader *interruptingReader) Close() error { return nil }

func steppedClock(start time.Time, step time.Duration) func() time.Time {
	current := start.Add(-step)
	return func() time.Time {
		current = current.Add(step)
		return current
	}
}

func value(pointer *int64) int64 {
	if pointer == nil {
		return -1
	}
	return *pointer
}
