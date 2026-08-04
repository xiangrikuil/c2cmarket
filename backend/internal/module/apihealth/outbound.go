package apihealth

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"c2c-market/backend/internal/platform/outboundhttp"
)

var ErrTargetIdentityMismatch = errors.New("probe target no longer matches its authorized identity")

type HTTPClientFactory interface {
	ClientFor(config Config) (*http.Client, error)
}

type OutboundHTTPClientFactory struct {
	timeout time.Duration
}

func NewOutboundHTTPClientFactory(timeout time.Duration) *OutboundHTTPClientFactory {
	return &OutboundHTTPClientFactory{timeout: timeout}
}

func (factory *OutboundHTTPClientFactory) ClientFor(config Config) (*http.Client, error) {
	if factory == nil {
		return nil, ErrTargetIdentityMismatch
	}
	target, err := NormalizeTarget(config.BaseURL)
	if err != nil || target.BaseURL != config.BaseURL || target.Origin != config.NormalizedOrigin {
		return nil, ErrTargetIdentityMismatch
	}
	parsed, err := url.Parse(target.Origin)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return nil, ErrTargetIdentityMismatch
	}
	policy, err := outboundhttp.NewPolicy([]string{parsed.Hostname()})
	if err != nil {
		return nil, ErrTargetIdentityMismatch
	}
	return outboundhttp.NewClient(policy, outboundhttp.WithClientTimeout(factory.timeout)), nil
}
