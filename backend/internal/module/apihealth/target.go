package apihealth

import (
	"net/url"
	"strings"
)

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
