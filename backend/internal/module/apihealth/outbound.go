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
	allowInsecureHTTP := UsesInsecureHTTP(config.BaseURL)
	target, err := normalizeTarget(config.BaseURL, allowInsecureHTTP)
	if err != nil || target.BaseURL != config.BaseURL || target.Origin != config.NormalizedOrigin {
		return nil, ErrTargetIdentityMismatch
	}
	parsed, err := url.Parse(target.Origin)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return nil, ErrTargetIdentityMismatch
	}
	options := make([]outboundhttp.PolicyOption, 0, 1)
	if allowInsecureHTTP {
		options = append(options, outboundhttp.WithInsecureHTTP())
	}
	policy, err := outboundhttp.NewPolicy([]string{parsed.Hostname()}, options...)
	if err != nil {
		return nil, ErrTargetIdentityMismatch
	}
	return outboundhttp.NewClient(policy, outboundhttp.WithClientTimeout(factory.timeout)), nil
}
