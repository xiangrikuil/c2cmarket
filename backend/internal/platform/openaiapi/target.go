package openaiapi

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"strings"

	"c2c-market/backend/internal/platform/outboundhttp"
)

type BaseURL struct {
	Raw       string
	Canonical string
}

func NormalizeBaseURL(raw string, allowInsecureHTTP bool) (BaseURL, error) {
	stored := strings.TrimSpace(raw)
	options := policyOptions(allowInsecureHTTP)
	policy, err := outboundhttp.NewPolicy(nil, options...)
	if err != nil {
		return BaseURL{}, err
	}
	canonical, err := policy.NormalizeURL(stored)
	if err != nil {
		return BaseURL{}, err
	}
	parsed, err := url.Parse(canonical)
	if err != nil {
		return BaseURL{}, outboundhttp.ErrInvalidTarget
	}
	port := parsed.Port()
	if (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80") {
		parsed.Host = canonicalAuthority(parsed.Hostname(), "")
	}
	return BaseURL{Raw: stored, Canonical: parsed.String()}, nil
}

func ValidateBaseURL(ctx context.Context, raw string, allowInsecureHTTP bool) (BaseURL, error) {
	target, err := NormalizeBaseURL(raw, allowInsecureHTTP)
	if err != nil {
		return BaseURL{}, err
	}
	policy, err := outboundhttp.NewPolicy(nil, policyOptions(allowInsecureHTTP)...)
	if err != nil {
		return BaseURL{}, err
	}
	if _, err := policy.ValidateURL(ctx, target.Canonical); err != nil {
		return BaseURL{}, err
	}
	return target, nil
}

func JoinEndpoint(baseURL, endpoint string) (string, error) {
	return url.JoinPath(strings.TrimSpace(baseURL), strings.TrimPrefix(endpoint, "/"))
}

func policyOptions(allowInsecureHTTP bool) []outboundhttp.PolicyOption {
	if allowInsecureHTTP {
		return []outboundhttp.PolicyOption{outboundhttp.WithInsecureHTTP()}
	}
	return nil
}

func canonicalAuthority(host, port string) string {
	if port != "" {
		return net.JoinHostPort(host, port)
	}
	if address, err := netip.ParseAddr(host); err == nil && address.Is6() {
		return "[" + host + "]"
	}
	return host
}
