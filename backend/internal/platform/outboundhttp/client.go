package outboundhttp

import (
	"crypto/tls"
	"io"
	"net/http"
	"time"
)

const (
	defaultClientTimeout         = 30 * time.Second
	defaultDialTimeout           = 5 * time.Second
	defaultTLSHandshakeTimeout   = 5 * time.Second
	defaultResponseHeaderTimeout = 10 * time.Second
	defaultIdleConnTimeout       = 30 * time.Second
)

type clientOptions struct {
	timeout   time.Duration
	tlsConfig *tls.Config
}

type ClientOption func(*clientOptions)

func withClientTimeout(timeout time.Duration) ClientOption {
	return func(options *clientOptions) {
		if timeout > 0 {
			options.timeout = timeout
		}
	}
}

func withTLSConfig(config *tls.Config) ClientOption {
	return func(options *clientOptions) {
		if config != nil {
			options.tlsConfig = config.Clone()
		}
	}
}

func NewClient(policy *Policy, options ...ClientOption) *http.Client {
	settings := clientOptions{timeout: defaultClientTimeout}
	for _, option := range options {
		option(&settings)
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           policy.dialContext,
		ForceAttemptHTTP2:     true,
		TLSClientConfig:       settings.tlsConfig,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout,
		ResponseHeaderTimeout: defaultResponseHeaderTimeout,
		ExpectContinueTimeout: time.Second,
		IdleConnTimeout:       defaultIdleConnTimeout,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   5,
	}
	return &http.Client{
		Transport: &validatingTransport{
			policy:    policy,
			transport: transport,
		},
		Timeout: settings.timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return ErrRedirectNotAllowed
		},
	}
}

type validatingTransport struct {
	policy    *Policy
	transport *http.Transport
}

func (t *validatingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t == nil || t.policy == nil || t.transport == nil || request == nil {
		return nil, ErrInvalidTarget
	}
	if err := t.policy.validateRequestURL(request.URL); err != nil {
		return nil, err
	}
	return t.transport.RoundTrip(request)
}

func ReadBody(body io.Reader, limit int64) ([]byte, error) {
	if body == nil || limit < 0 {
		return nil, ErrInvalidTarget
	}
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}
