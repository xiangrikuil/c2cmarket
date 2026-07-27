package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hsts     bool
		wantHSTS bool
	}{
		{name: "development", hsts: false, wantHSTS: false},
		{name: "production", hsts: true, wantHSTS: true},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			handler := WithSecurityHeaders(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
				SecurityHeadersOptions{HSTS: test.hsts},
			)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

			want := map[string]string{
				"Content-Security-Policy": apiContentSecurityPolicy,
				"Permissions-Policy":      permissionsPolicy,
				"X-Frame-Options":         "DENY",
				"X-Content-Type-Options":  "nosniff",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
			}
			for name, value := range want {
				if got := response.Header().Get(name); got != value {
					t.Errorf("%s = %q, want %q", name, got, value)
				}
			}

			gotHSTS := response.Header().Get("Strict-Transport-Security")
			if test.wantHSTS && gotHSTS != "max-age=31536000; includeSubDomains" {
				t.Errorf("Strict-Transport-Security = %q", gotHSTS)
			}
			if !test.wantHSTS && gotHSTS != "" {
				t.Errorf("development Strict-Transport-Security = %q, want empty", gotHSTS)
			}
		})
	}
}
