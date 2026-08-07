package apihealth

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"

	"c2c-market/backend/internal/platform/outboundhttp"
)

type NormalizedTarget struct {
	BaseURL string
	Origin  string
}

func NormalizeTarget(raw string) (NormalizedTarget, error) {
	return normalizeTarget(raw, false)
}

func normalizeTarget(raw string, allowInsecureHTTP bool) (NormalizedTarget, error) {
	options := make([]outboundhttp.PolicyOption, 0, 1)
	if allowInsecureHTTP {
		options = append(options, outboundhttp.WithInsecureHTTP())
	}
	policy, err := outboundhttp.NewPolicy(nil, options...)
	if err != nil {
		return NormalizedTarget{}, err
	}
	normalized, err := policy.NormalizeURL(raw)
	if err != nil {
		return NormalizedTarget{}, err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return NormalizedTarget{}, fmt.Errorf("parse normalized target: %w", err)
	}
	if parsed.Path == "" {
		parsed.Path = "/v1"
		normalized = parsed.String()
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "80"
		if parsed.Scheme == "https" {
			port = "443"
		}
	}
	authority := net.JoinHostPort(host, port)
	if address, parseErr := netip.ParseAddr(host); parseErr != nil || !address.Is6() {
		authority = host + ":" + port
	}
	return NormalizedTarget{
		BaseURL: normalized,
		Origin:  parsed.Scheme + "://" + authority,
	}, nil
}

func UsesInsecureHTTP(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && strings.EqualFold(parsed.Scheme, "http")
}

func TargetTransportSecurity(raw string) string {
	if UsesInsecureHTTP(raw) {
		return TransportSecurityHTTP
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err == nil && strings.EqualFold(parsed.Scheme, "https") {
		return TransportSecurityHTTPS
	}
	return TransportSecurityUnknown
}

func MeasurementIdentityChanged(existing Config, target NormalizedTarget, model string) bool {
	return existing.Protocol != ProtocolOpenAIChatCompletionsV1 ||
		existing.BaseURL != target.BaseURL ||
		existing.NormalizedOrigin != target.Origin ||
		existing.Model != strings.TrimSpace(model)
}

func AuthorizationIdentityChanged(existing Config, target NormalizedTarget) bool {
	return existing.NormalizedOrigin != target.Origin
}
