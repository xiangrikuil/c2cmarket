package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const unknownClientIP = "unknown"

type clientIPContextKey struct{}

// ClientIPResolver 在请求入口解析并固定唯一的规范客户端地址。
type ClientIPResolver struct {
	trustProxyHeaders bool
	trustedProxies    []netip.Prefix
}

func NewClientIPResolver(trustProxyHeaders bool, trustedProxies []string) ClientIPResolver {
	prefixes := make([]netip.Prefix, 0, len(trustedProxies))
	for _, value := range trustedProxies {
		if prefix, ok := parseTrustedProxyPrefix(value); ok {
			prefixes = append(prefixes, prefix)
		}
	}
	return ClientIPResolver{
		trustProxyHeaders: trustProxyHeaders,
		trustedProxies:    prefixes,
	}
}

func (resolver ClientIPResolver) Resolve(r *http.Request) string {
	direct, ok := parseRemoteAddr(r)
	if !ok {
		return unknownClientIP
	}
	if !resolver.trustProxyHeaders || !resolver.isTrustedProxy(direct) {
		return direct.String()
	}
	if client, ok := parseSingleForwardedHeader(r.Header.Values("CF-Connecting-IP")); ok {
		return client.String()
	}
	if client, ok := resolver.parseForwardedFor(r.Header.Values("X-Forwarded-For")); ok {
		return client.String()
	}
	if client, ok := parseSingleForwardedHeader(r.Header.Values("X-Real-IP")); ok {
		return client.String()
	}
	return direct.String()
}

func (resolver ClientIPResolver) parseForwardedFor(values []string) (netip.Addr, bool) {
	if len(values) == 0 {
		return netip.Addr{}, false
	}
	parts := strings.Split(strings.Join(values, ","), ",")
	addresses := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		addr, ok := parseIPAddr(part)
		if !ok {
			return netip.Addr{}, false
		}
		addresses = append(addresses, addr)
	}
	for index := len(addresses) - 1; index >= 0; index-- {
		if resolver.isTrustedProxy(addresses[index]) {
			continue
		}
		return addresses[index], true
	}
	return netip.Addr{}, false
}

func (resolver ClientIPResolver) isTrustedProxy(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range resolver.trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func WithClientIP(resolver ClientIPResolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := resolver.Resolve(r)
		ctx := context.WithValue(r.Context(), clientIPContextKey{}, clientIP)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ClientIPFromContext(ctx context.Context) string {
	if ctx == nil {
		return unknownClientIP
	}
	value, _ := ctx.Value(clientIPContextKey{}).(string)
	value = strings.TrimSpace(value)
	if value == "" {
		return unknownClientIP
	}
	return value
}

func ClientIPFromRequest(r *http.Request) string {
	if r == nil {
		return unknownClientIP
	}
	return ClientIPFromContext(r.Context())
}

func parseRemoteAddr(r *http.Request) (netip.Addr, bool) {
	if r == nil {
		return netip.Addr{}, false
	}
	value := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	return parseIPAddr(value)
}

func parseSingleForwardedHeader(values []string) (netip.Addr, bool) {
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return netip.Addr{}, false
	}
	return parseIPAddr(values[0])
}

func parseIPAddr(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '[' && value[len(value)-1] == ']' {
		value = value[1 : len(value)-1]
	}
	if value == "" {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(value)
	if err != nil || addr.Zone() != "" {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func parseTrustedProxyPrefix(value string) (netip.Prefix, bool) {
	value = strings.TrimSpace(value)
	if prefix, err := netip.ParsePrefix(value); err == nil {
		addr := prefix.Addr()
		bits := prefix.Bits()
		if addr.Zone() != "" {
			return netip.Prefix{}, false
		}
		if addr.Is4In6() {
			if bits < 96 {
				return netip.Prefix{}, false
			}
			addr = addr.Unmap()
			bits -= 96
		}
		return netip.PrefixFrom(addr, bits).Masked(), true
	}
	addr, ok := parseIPAddr(value)
	if !ok {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, addr.BitLen()), true
}
