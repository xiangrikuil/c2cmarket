package apihealth

import "testing"

func TestNormalizeTargetBindsEffectiveOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		raw       string
		baseURL   string
		origin    string
		allowHTTP bool
		wantError bool
	}{
		{name: "root", raw: "https://api.example.com", baseURL: "https://api.example.com/v1", origin: "https://api.example.com:443"},
		{name: "root trailing slash", raw: "https://api.example.com/", baseURL: "https://api.example.com/v1", origin: "https://api.example.com:443"},
		{name: "existing v1", raw: "https://API.Example.com/v1/", baseURL: "https://api.example.com/v1", origin: "https://api.example.com:443"},
		{name: "custom api v1 path", raw: "https://api.example.com/api/v1/", baseURL: "https://api.example.com/api/v1", origin: "https://api.example.com:443"},
		{name: "custom openai v1 path", raw: "https://api.example.com/openai/v1", baseURL: "https://api.example.com/openai/v1", origin: "https://api.example.com:443"},
		{name: "explicit port root", raw: "https://api.example.com:8443/", baseURL: "https://api.example.com:8443/v1", origin: "https://api.example.com:8443"},
		{name: "ipv6 root", raw: "https://[2606:4700:4700::1111]", baseURL: "https://[2606:4700:4700::1111]/v1", origin: "https://[2606:4700:4700::1111]:443"},
		{name: "http rejected", raw: "http://api.example.com/v1", wantError: true},
		{name: "http root acknowledged", raw: "http://api.example.com", baseURL: "http://api.example.com/v1", origin: "http://api.example.com:80", allowHTTP: true},
		{name: "http root port acknowledged", raw: "http://api.example.com:8080/", baseURL: "http://api.example.com:8080/v1", origin: "http://api.example.com:8080", allowHTTP: true},
		{name: "http existing path acknowledged", raw: "http://api.example.com/openai/v1/", baseURL: "http://api.example.com/openai/v1", origin: "http://api.example.com:80", allowHTTP: true},
		{name: "userinfo rejected", raw: "https://user@api.example.com/v1", wantError: true},
		{name: "query rejected", raw: "https://api.example.com/v1?token=x", wantError: true},
		{name: "invalid port rejected", raw: "https://api.example.com:0/v1", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			target, err := normalizeTarget(test.raw, test.allowHTTP)
			if test.wantError {
				if err == nil {
					t.Fatalf("expected error, got %+v", target)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize target: %v", err)
			}
			if target.BaseURL != test.baseURL || target.Origin != test.origin {
				t.Fatalf("unexpected target: %+v", target)
			}
		})
	}
}

func TestTargetTransportSecurityDoesNotExposeTargetDetails(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"https://api.example.com/v1": TransportSecurityHTTPS,
		"http://api.example.com/v1":  TransportSecurityHTTP,
		"api.example.com/v1":         TransportSecurityUnknown,
	}
	for target, want := range tests {
		if got := TargetTransportSecurity(target); got != want {
			t.Fatalf("TargetTransportSecurity(%q) = %q, want %q", target, got, want)
		}
	}
}
