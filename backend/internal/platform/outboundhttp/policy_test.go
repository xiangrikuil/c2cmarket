package outboundhttp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type staticResolver struct {
	addresses map[string][]netip.Addr
	err       error
}

func (r staticResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	if r.err != nil {
		return nil, r.err
	}
	return append([]netip.Addr(nil), r.addresses[host]...), nil
}

type recordingDialer struct {
	addresses []string
	dial      func(context.Context, string, string) (net.Conn, error)
}

func (d *recordingDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d.addresses = append(d.addresses, address)
	if d.dial != nil {
		return d.dial(ctx, network, address)
	}
	return nil, errors.New("test dial failed")
}

func TestValidateURLRejectsInvalidTargets(t *testing.T) {
	policy := testPolicy(t, nil)
	tests := []string{
		"http://api.example.com/v1",
		"https://user:secret@api.example.com/v1",
		"https://api.example.com/v1?debug=1",
		"https://api.example.com/v1#fragment",
		"https://api.example.com:0/v1",
		"https://api.example.com:65536/v1",
		"https://api.example.com:/v1",
		"https://[fe80::1%25eth0]/v1",
		"https://0x7f000001/v1",
		"https://0x7f.0.0.1/v1",
		"api.example.com/v1",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			if _, err := policy.ValidateURL(context.Background(), target); err == nil {
				t.Fatalf("ValidateURL(%q) succeeded", target)
			}
		})
	}
}

func TestValidateURLRejectsUnsafeLiteralAndResolvedAddresses(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]netip.Addr{
		"localhost":       {netip.MustParseAddr("127.0.0.1")},
		"private.example": {netip.MustParseAddr("10.0.0.2")},
		"mixed.example": {
			netip.MustParseAddr("93.184.216.34"),
			netip.MustParseAddr("192.168.1.10"),
		},
	}}
	policy := testPolicy(t, resolver)
	targets := []string{
		"https://127.0.0.1",
		"https://localhost",
		"https://0.0.0.0",
		"https://169.254.169.254",
		"https://10.0.0.1",
		"https://172.16.0.1",
		"https://192.168.1.1",
		"https://100.64.0.1",
		"https://192.0.2.1",
		"https://198.18.0.1",
		"https://224.0.0.1",
		"https://240.0.0.1",
		"https://[::1]",
		"https://[fc00::1]",
		"https://[fe80::1]",
		"https://[fec0::1]",
		"https://[2001:db8::1]",
		"https://[ff02::1]",
		"https://[::ffff:127.0.0.1]",
		"https://private.example",
		"https://mixed.example",
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			if _, err := policy.ValidateURL(context.Background(), target); err == nil {
				t.Fatalf("ValidateURL(%q) succeeded", target)
			}
		})
	}
}

func TestValidateURLNormalizesPublicTargetAndEnforcesAllowlist(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]netip.Addr{
		"api.example.com": {netip.MustParseAddr("93.184.216.34")},
	}}
	policy, err := NewPolicy([]string{"API.EXAMPLE.COM."}, WithResolver(resolver))
	if err != nil {
		t.Fatalf("NewPolicy() error: %v", err)
	}
	normalized, err := policy.ValidateURL(context.Background(), " HTTPS://API.EXAMPLE.COM./v1/ ")
	if err != nil {
		t.Fatalf("ValidateURL() error: %v", err)
	}
	if normalized != "https://api.example.com/v1" {
		t.Fatalf("unexpected normalized URL %q", normalized)
	}
	if _, err := policy.ValidateURL(context.Background(), "https://other.example.com/v1"); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("expected allowlist error, got %v", err)
	}
	if _, err := NewPolicy([]string{"*.example.com"}); err == nil {
		t.Fatal("wildcard allowlist entry was accepted")
	}
}

func TestDialContextUsesOnlyValidatedIPsAndCapsAttempts(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]netip.Addr{
		"api.example.com": {
			netip.MustParseAddr("93.184.216.34"),
			netip.MustParseAddr("1.1.1.1"),
			netip.MustParseAddr("8.8.8.8"),
			netip.MustParseAddr("9.9.9.9"),
		},
	}}
	dialer := &recordingDialer{}
	policy, err := NewPolicy(nil, WithResolver(resolver), WithDialer(dialer))
	if err != nil {
		t.Fatalf("NewPolicy() error: %v", err)
	}
	if _, err := policy.dialContext(context.Background(), "tcp", "api.example.com:443"); !errors.Is(err, ErrDialFailed) {
		t.Fatalf("expected dial failure, got %v", err)
	}
	want := []string{"93.184.216.34:443", "1.1.1.1:443", "8.8.8.8:443"}
	if fmt.Sprint(dialer.addresses) != fmt.Sprint(want) {
		t.Fatalf("dialer received %v, want %v", dialer.addresses, want)
	}
}

func TestDialContextRejectsMixedDNSBeforeDial(t *testing.T) {
	resolver := staticResolver{addresses: map[string][]netip.Addr{
		"api.example.com": {
			netip.MustParseAddr("93.184.216.34"),
			netip.MustParseAddr("10.0.0.1"),
		},
	}}
	dialer := &recordingDialer{}
	policy, err := NewPolicy(nil, WithResolver(resolver), WithDialer(dialer))
	if err != nil {
		t.Fatalf("NewPolicy() error: %v", err)
	}
	if _, err := policy.dialContext(context.Background(), "tcp", "api.example.com:443"); !errors.Is(err, ErrUnsafeAddress) {
		t.Fatalf("expected unsafe address error, got %v", err)
	}
	if len(dialer.addresses) != 0 {
		t.Fatalf("dialer was called with %v", dialer.addresses)
	}
}

func TestClientSupportsPublicHTTPSAndRejectsRedirects(t *testing.T) {
	var tlsServerName string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/ok":
			_, _ = io.WriteString(w, `{"ok":true}`)
		case "/redirect":
			http.Redirect(w, request, "https://api.example.com/ok", http.StatusFound)
		default:
			http.NotFound(w, request)
		}
	}))
	server.TLS = &tls.Config{GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		tlsServerName = info.ServerName
		return nil, nil
	}}
	server.StartTLS()
	defer server.Close()

	client, dialer := testTLSClient(t, server, 2*time.Second)
	response, err := client.Get("https://api.example.com/ok")
	if err != nil {
		t.Fatalf("GET public target: %v", err)
	}
	_ = response.Body.Close()
	if len(dialer.addresses) != 1 || !strings.HasPrefix(dialer.addresses[0], "93.184.216.34:") {
		t.Fatalf("dialer did not receive validated public IP: %v", dialer.addresses)
	}
	if tlsServerName != "api.example.com" {
		t.Fatalf("TLS ServerName was %q, want original host", tlsServerName)
	}

	_, err = client.Get("https://api.example.com/redirect")
	if !errors.Is(err, ErrRedirectNotAllowed) {
		t.Fatalf("expected redirect rejection, got %v", err)
	}
}

func TestClientTotalTimeoutBoundsSlowResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()

	client, _ := testTLSClient(t, server, 20*time.Millisecond)
	_, err := client.Get("https://api.example.com/slow")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
}

func TestReadBodyRejectsOversizedResponse(t *testing.T) {
	if _, err := ReadBody(strings.NewReader("12345"), 4); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected oversized response error, got %v", err)
	}
	data, err := ReadBody(strings.NewReader("1234"), 4)
	if err != nil {
		t.Fatalf("ReadBody() error: %v", err)
	}
	if string(data) != "1234" {
		t.Fatalf("unexpected body %q", data)
	}
}

func testPolicy(t *testing.T, resolver Resolver) *Policy {
	t.Helper()
	if resolver == nil {
		resolver = staticResolver{addresses: map[string][]netip.Addr{
			"api.example.com": {netip.MustParseAddr("93.184.216.34")},
		}}
	}
	policy, err := NewPolicy(nil, WithResolver(resolver))
	if err != nil {
		t.Fatalf("NewPolicy() error: %v", err)
	}
	return policy
}

func testTLSClient(t *testing.T, server *httptest.Server, timeout time.Duration) (*http.Client, *recordingDialer) {
	t.Helper()
	targetAddress := server.Listener.Addr().String()
	dialer := &recordingDialer{dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, targetAddress)
	}}
	resolver := staticResolver{addresses: map[string][]netip.Addr{
		"api.example.com": {netip.MustParseAddr("93.184.216.34")},
	}}
	policy, err := NewPolicy(nil, WithResolver(resolver), WithDialer(dialer))
	if err != nil {
		t.Fatalf("NewPolicy() error: %v", err)
	}
	return NewClient(policy,
		withClientTimeout(timeout),
		withTLSConfig(&tls.Config{InsecureSkipVerify: true}),
	), dialer
}
