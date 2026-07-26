package middleware

import "net/http"

const (
	apiContentSecurityPolicy = "default-src 'none'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'none'"
	permissionsPolicy        = "camera=(), geolocation=(), microphone=(), payment=(), usb=()"
)

type SecurityHeadersOptions struct {
	HSTS bool
}

func WithSecurityHeaders(next http.Handler, options SecurityHeadersOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", apiContentSecurityPolicy)
		w.Header().Set("Permissions-Policy", permissionsPolicy)
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if options.HSTS {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}
