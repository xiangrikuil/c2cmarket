package middleware

import (
	"net/http"
	"strings"
)

const CSRFHeaderName = "X-CSRF-Token"
const RestrictedBusinessCSRFHeaderName = "X-Restricted-Business-CSRF"

func CSRFToken(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(CSRFHeaderName))
}

func RestrictedBusinessCSRFToken(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(RestrictedBusinessCSRFHeaderName))
}
