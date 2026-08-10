package openaiapi

import "testing"

func TestNormalizeBaseURLPreservesRawValueAndBuildsCanonicalComparison(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		allowHTTP bool
		stored    string
		canonical string
	}{
		{name: "https default port", raw: " HTTPS://API.Example.COM:443/v1/ ", stored: "HTTPS://API.Example.COM:443/v1/", canonical: "https://api.example.com/v1"},
		{name: "http default port", raw: "http://API.Example.COM:80/", allowHTTP: true, stored: "http://API.Example.COM:80/", canonical: "http://api.example.com"},
		{name: "non-default port", raw: "http://155.103.116.134:31238/", allowHTTP: true, stored: "http://155.103.116.134:31238/", canonical: "http://155.103.116.134:31238"},
		{name: "ipv6", raw: "https://[2606:4700:4700::1111]:443/v1/", stored: "https://[2606:4700:4700::1111]:443/v1/", canonical: "https://[2606:4700:4700::1111]/v1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target, err := NormalizeBaseURL(test.raw, test.allowHTTP)
			if err != nil {
				t.Fatalf("NormalizeBaseURL() error: %v", err)
			}
			if target.Raw != test.stored || target.Canonical != test.canonical {
				t.Fatalf("target=%+v, want raw=%q canonical=%q", target, test.stored, test.canonical)
			}
		})
	}
}

func TestNormalizeBaseURLRejectsAmbiguousOrUnsafeSyntax(t *testing.T) {
	for _, raw := range []string{
		"http://api.example.com/v1",
		"https://user:secret@api.example.com/v1",
		"https://api.example.com/v1?debug=1",
		"https://api.example.com/v1#fragment",
		"ftp://api.example.com/v1",
		"api.example.com/v1",
	} {
		if _, err := NormalizeBaseURL(raw, false); err == nil {
			t.Fatalf("NormalizeBaseURL(%q) succeeded", raw)
		}
	}
}

func TestJoinEndpointNeverInsertsVersionPath(t *testing.T) {
	tests := map[string]string{
		"https://api.example.com":     "https://api.example.com/models",
		"https://api.example.com/":    "https://api.example.com/models",
		"https://api.example.com/v1":  "https://api.example.com/v1/models",
		"https://api.example.com/v1/": "https://api.example.com/v1/models",
	}
	for baseURL, expected := range tests {
		actual, err := JoinEndpoint(baseURL, "models")
		if err != nil || actual != expected {
			t.Fatalf("JoinEndpoint(%q)=%q err=%v, want %q", baseURL, actual, err, expected)
		}
	}
}
