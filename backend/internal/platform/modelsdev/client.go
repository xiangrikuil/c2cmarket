package modelsdev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/platform/outboundhttp"
)

const (
	SourceURL         = "https://models.dev/api.json"
	responseBodyLimit = 16 << 20
)

var (
	ErrUnavailable = errors.New("models.dev is unavailable")
	ErrInvalidData = errors.New("models.dev returned invalid data")
)

type Source interface {
	Fetch(ctx context.Context) (Catalog, error)
}

type Catalog map[string]Provider

type Provider struct {
	ID     string           `json:"id"`
	Name   string           `json:"name"`
	Models map[string]Model `json:"models"`
}

type Model struct {
	ID          string     `json:"id"`
	LastUpdated string     `json:"last_updated"`
	Attachment  bool       `json:"attachment"`
	Reasoning   bool       `json:"reasoning"`
	Modalities  Modalities `json:"modalities"`
	Cost        *Cost      `json:"cost"`
}

type Modalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type Cost struct {
	Input       json.Number `json:"input"`
	Output      json.Number `json:"output"`
	CacheRead   json.Number `json:"cache_read"`
	InputAudio  json.Number `json:"input_audio"`
	OutputAudio json.Number `json:"output_audio"`
}

type Client struct {
	httpClient *http.Client
	endpoint   string
	bodyLimit  int64
}

func NewClient(timeout time.Duration) *Client {
	policy, err := outboundhttp.NewPolicy([]string{"models.dev"})
	if err != nil {
		panic(fmt.Sprintf("invalid fixed models.dev policy: %v", err))
	}
	return &Client{
		httpClient: outboundhttp.NewClient(policy, outboundhttp.WithClientTimeout(timeout)),
		endpoint:   SourceURL,
		bodyLimit:  responseBodyLimit,
	}
}

func NewClientForTest(httpClient *http.Client, endpoint string, bodyLimit int64) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if bodyLimit <= 0 {
		bodyLimit = responseBodyLimit
	}
	return &Client{httpClient: httpClient, endpoint: endpoint, bodyLimit: bodyLimit}
}

func (c *Client) Fetch(ctx context.Context) (Catalog, error) {
	if c == nil || c.httpClient == nil || strings.TrimSpace(c.endpoint) == "" {
		return nil, ErrUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: http status %d", ErrUnavailable, response.StatusCode)
	}
	body, err := outboundhttp.ReadBody(response.Body, c.bodyLimit)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidData, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var catalog Catalog
	if err := decoder.Decode(&catalog); err != nil || len(catalog) == 0 {
		return nil, fmt.Errorf("%w: malformed catalog", ErrInvalidData)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing data", ErrInvalidData)
	}
	return catalog, nil
}
