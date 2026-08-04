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
	policy, err := outboundhttp.NewPolicy(nil)
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
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	authority := net.JoinHostPort(host, port)
	if address, parseErr := netip.ParseAddr(host); parseErr != nil || !address.Is6() {
		authority = host + ":" + port
	}
	return NormalizedTarget{
		BaseURL: strings.TrimRight(normalized, "/"),
		Origin:  "https://" + authority,
	}, nil
}

func MeasurementIdentityChanged(existing Config, target NormalizedTarget, model string) bool {
	return existing.Protocol != ProtocolOpenAIChatCompletionsV1 ||
		existing.BaseURL != target.BaseURL ||
		existing.NormalizedOrigin != target.Origin ||
		existing.Model != strings.TrimSpace(model)
}
