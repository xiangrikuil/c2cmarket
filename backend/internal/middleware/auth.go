package middleware

import (
	"net/http"
	"strings"
)

const SessionCookieName = "c2c_session"
const RestrictedBusinessSessionCookieName = "c2c_restricted_business_session"
const SessionAudienceHeaderName = "X-Session-Audience"

func SessionToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(cookie.Value)
	return value, value != ""
}

func RestrictedBusinessSessionToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(RestrictedBusinessSessionCookieName)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(cookie.Value)
	return value, value != ""
}

func SessionAudience(r *http.Request) string {
	return strings.TrimSpace(strings.ToLower(r.Header.Get(SessionAudienceHeaderName)))
}
