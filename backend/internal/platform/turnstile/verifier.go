package turnstile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultSiteverifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	maxResponseBodyBytes = 64 << 10
	maxTokenBytes        = 2048
)

var ErrVerificationFailed = errors.New("turnstile verification failed")

type Verification struct {
	Token    string
	Action   string
	RemoteIP string
}

type Verifier interface {
	Verify(ctx context.Context, input Verification) error
}

type Options struct {
	HTTPClient *http.Client
	Endpoint   string
}

type Client struct {
	secret           string
	allowedHostnames map[string]struct{}
	httpClient       *http.Client
	endpoint         string
}

type siteverifyResponse struct {
	Success  bool     `json:"success"`
	Hostname string   `json:"hostname"`
	Action   string   `json:"action"`
	Errors   []string `json:"error-codes"`
}

func New(secret string, allowedHostnames []string, options Options) (*Client, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, fmt.Errorf("Turnstile secret is required")
	}
	hostnames := make(map[string]struct{}, len(allowedHostnames))
	for _, hostname := range allowedHostnames {
		hostname = normalizeHostname(hostname)
		if hostname != "" {
			hostnames[hostname] = struct{}{}
		}
	}
	if len(hostnames) == 0 {
		return nil, fmt.Errorf("at least one Turnstile hostname is required")
	}
	endpoint := strings.TrimSpace(options.Endpoint)
	if endpoint == "" {
		endpoint = defaultSiteverifyURL
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		httpClient = &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &Client{
		secret:           secret,
		allowedHostnames: hostnames,
		httpClient:       httpClient,
		endpoint:         endpoint,
	}, nil
}

func (client *Client) Verify(ctx context.Context, input Verification) error {
	token := strings.TrimSpace(input.Token)
	expectedAction := strings.TrimSpace(input.Action)
	if token == "" || len(token) > maxTokenBytes || expectedAction == "" {
		return ErrVerificationFailed
	}
	form := url.Values{
		"secret":   {client.secret},
		"response": {token},
	}
	if remoteIP := strings.TrimSpace(input.RemoteIP); remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrVerificationFailed
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ErrVerificationFailed
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ErrVerificationFailed
	}
	body, err := readBounded(response.Body, maxResponseBodyBytes)
	if err != nil {
		return ErrVerificationFailed
	}
	var result siteverifyResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&result); err != nil {
		return ErrVerificationFailed
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return ErrVerificationFailed
	}
	if !result.Success || result.Action != expectedAction {
		return ErrVerificationFailed
	}
	if _, ok := client.allowedHostnames[normalizeHostname(result.Hostname)]; !ok {
		return ErrVerificationFailed
	}
	return nil
}

func normalizeHostname(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, ErrVerificationFailed
	}
	return body, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return ErrVerificationFailed
		}
		return err
	}
	return nil
}
