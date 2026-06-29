package middleware

import (
	"net/http"
	"strings"
)

const CSRFHeaderName = "X-CSRF-Token"

func CSRFToken(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(CSRFHeaderName))
}
