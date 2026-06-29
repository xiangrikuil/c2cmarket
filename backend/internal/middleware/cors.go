package middleware

import (
	"c2c-market/backend/internal/domain"
	httpresponse "c2c-market/backend/internal/response"
	"net/http"
	"strings"
)

type CORSOptions struct {
	AllowedOrigins []string
	Production     bool
}

func WithCORSAndOrigin(next http.Handler, options CORSOptions) http.Handler {
	allowed := map[string]struct{}{}
	for _, origin := range options.AllowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" && origin != "*" {
			allowed[origin] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		originAllowed := false
		if origin != "" {
			_, originAllowed = allowed[origin]
			if originAllowed {
				setCORSHeaders(w, origin)
			}
		}
		if r.Method == http.MethodOptions && strings.TrimSpace(r.Header.Get("Access-Control-Request-Method")) != "" {
			if origin == "" || !originAllowed {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if options.Production && isUnsafeMethod(r.Method) && origin != "" && !originAllowed {
			httpresponse.WriteProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "Origin not allowed", "请求来源不被允许。"), RequestIDFromRequest(r))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Add("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token, Idempotency-Key, If-Match, X-Request-Id")
	w.Header().Set("Access-Control-Expose-Headers", "ETag, Location, X-Request-Id")
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}
