package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIPResolverUsesOnlyNormalizedDirectPeerByDefault(t *testing.T) {
	resolver := NewClientIPResolver(false, nil)

	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{name: "IPv4 with port", remoteAddr: "203.0.113.10:4321", want: "203.0.113.10"},
		{name: "IPv6 with port", remoteAddr: "[2001:db8::10]:4321", want: "2001:db8::10"},
		{name: "mapped IPv4", remoteAddr: "[::ffff:203.0.113.10]:4321", want: "203.0.113.10"},
		{name: "raw address", remoteAddr: "198.51.100.20", want: "198.51.100.20"},
		{name: "zone rejected", remoteAddr: "[fe80::1%25en0]:4321", want: unknownClientIP},
		{name: "malformed address", remoteAddr: "not-an-ip:4321", want: unknownClientIP},
		{name: "empty address", remoteAddr: "", want: unknownClientIP},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = tc.remoteAddr
			request.Header.Set("CF-Connecting-IP", "192.0.2.1")
			request.Header.Set("X-Forwarded-For", "192.0.2.2")
			request.Header.Set("X-Real-IP", "192.0.2.3")

			if got := resolver.Resolve(request); got != tc.want {
				t.Fatalf("Resolve() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientIPResolverIgnoresHeadersFromUntrustedPeer(t *testing.T) {
	resolver := NewClientIPResolver(true, []string{"10.0.0.0/24"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.10:4321"
	request.Header.Set("CF-Connecting-IP", "192.0.2.1")
	request.Header.Set("X-Forwarded-For", "192.0.2.2")
	request.Header.Set("X-Real-IP", "192.0.2.3")

	if got := resolver.Resolve(request); got != "203.0.113.10" {
		t.Fatalf("Resolve() = %q, want direct peer", got)
	}
}

func TestClientIPResolverUsesTrustedForwardingChain(t *testing.T) {
	resolver := NewClientIPResolver(true, []string{"10.0.0.0/24"})

	tests := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{
			name: "Cloudflare header has priority",
			headers: map[string][]string{
				"CF-Connecting-IP": {"::ffff:198.51.100.10"},
				"X-Forwarded-For":  {"198.51.100.11"},
				"X-Real-IP":        {"198.51.100.12"},
			},
			want: "198.51.100.10",
		},
		{
			name: "nearest untrusted XFF hop defeats forged far-left value",
			headers: map[string][]string{
				"X-Forwarded-For": {"192.0.2.200, 198.51.100.20, 10.0.0.8"},
			},
			want: "198.51.100.20",
		},
		{
			name: "multiple XFF field lines form one chain",
			headers: map[string][]string{
				"X-Forwarded-For": {"192.0.2.200, 198.51.100.21", "10.0.0.8"},
			},
			want: "198.51.100.21",
		},
		{
			name: "invalid Cloudflare header falls back to XFF",
			headers: map[string][]string{
				"CF-Connecting-IP": {"invalid"},
				"X-Forwarded-For":  {"198.51.100.22, 10.0.0.8"},
			},
			want: "198.51.100.22",
		},
		{
			name: "invalid XFF invalidates the entire header",
			headers: map[string][]string{
				"X-Forwarded-For": {"198.51.100.23, invalid, 10.0.0.8"},
				"X-Real-IP":       {"198.51.100.24"},
			},
			want: "198.51.100.24",
		},
		{
			name: "all-trusted XFF falls back to real IP",
			headers: map[string][]string{
				"X-Forwarded-For": {"10.0.0.7, 10.0.0.8"},
				"X-Real-IP":       {"::ffff:198.51.100.25"},
			},
			want: "198.51.100.25",
		},
		{
			name: "multi-value Cloudflare header is rejected",
			headers: map[string][]string{
				"CF-Connecting-IP": {"198.51.100.26", "198.51.100.27"},
				"X-Forwarded-For":  {"198.51.100.28"},
			},
			want: "198.51.100.28",
		},
		{
			name: "invalid fallbacks use direct peer",
			headers: map[string][]string{
				"CF-Connecting-IP": {"fe80::1%en0"},
				"X-Forwarded-For":  {"invalid"},
				"X-Real-IP":        {"also-invalid"},
			},
			want: "10.0.0.9",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = "10.0.0.9:4321"
			for name, values := range tc.headers {
				for _, value := range values {
					request.Header.Add(name, value)
				}
			}

			if got := resolver.Resolve(request); got != tc.want {
				t.Fatalf("Resolve() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClientIPResolverNormalizesMappedTrustedProxyPrefix(t *testing.T) {
	resolver := NewClientIPResolver(true, []string{"::ffff:10.0.0.0/120"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.9:4321"
	request.Header.Set("CF-Connecting-IP", "198.51.100.30")

	if got := resolver.Resolve(request); got != "198.51.100.30" {
		t.Fatalf("Resolve() = %q, want trusted forwarded client", got)
	}
}

func TestWithClientIPStoresOneRequestScopedValue(t *testing.T) {
	resolver := NewClientIPResolver(true, []string{"10.0.0.0/24"})
	var fromContext string
	var fromRequest string
	handler := WithClientIP(resolver, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("CF-Connecting-IP", "192.0.2.200")
		fromContext = ClientIPFromContext(r.Context())
		fromRequest = ClientIPFromRequest(r)
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.9:4321"
	request.Header.Set("CF-Connecting-IP", "198.51.100.31")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if fromContext != "198.51.100.31" || fromRequest != fromContext {
		t.Fatalf("context=%q request=%q, want one resolved value", fromContext, fromRequest)
	}
	if got := ClientIPFromContext(context.Background()); got != unknownClientIP {
		t.Fatalf("missing context value = %q, want %q", got, unknownClientIP)
	}
	if got := ClientIPFromRequest(nil); got != unknownClientIP {
		t.Fatalf("nil request value = %q, want %q", got, unknownClientIP)
	}
}
