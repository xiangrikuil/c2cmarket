package middleware

import (
	"net/http"
	"strings"
)

const SessionCookieName = "c2c_session"

func SessionToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(cookie.Value)
	return value, value != ""
}
