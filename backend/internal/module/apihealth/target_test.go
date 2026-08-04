package apihealth

import "testing"

func TestNormalizeTargetBindsEffectiveOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		raw       string
		baseURL   string
		origin    string
		wantError bool
	}{
		{name: "default port", raw: "https://API.Example.com/v1/", baseURL: "https://api.example.com/v1", origin: "https://api.example.com:443"},
		{name: "explicit port", raw: "https://api.example.com:8443/proxy/v1", baseURL: "https://api.example.com:8443/proxy/v1", origin: "https://api.example.com:8443"},
		{name: "ipv6", raw: "https://[2606:4700:4700::1111]/v1", baseURL: "https://[2606:4700:4700::1111]/v1", origin: "https://[2606:4700:4700::1111]:443"},
		{name: "http rejected", raw: "http://api.example.com/v1", wantError: true},
		{name: "userinfo rejected", raw: "https://user@api.example.com/v1", wantError: true},
		{name: "query rejected", raw: "https://api.example.com/v1?token=x", wantError: true},
		{name: "invalid port rejected", raw: "https://api.example.com:0/v1", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			target, err := NormalizeTarget(test.raw)
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
