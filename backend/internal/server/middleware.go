package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/auth"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func (s *Server) requireSessionAndCSRF(r *http.Request) (auth.User, auth.Session, *domain.AppError) {
	sessionToken, ok := middleware.SessionToken(r)
	if !ok {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	csrfToken := middleware.CSRFToken(r)
	if csrfToken == "" {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF token invalid", "CSRF token 无效或缺失。")
	}
	return s.app.GetSessionWithCSRF(r.Context(), sessionToken, csrfToken)
}

func (s *Server) requireSession(r *http.Request) (auth.User, auth.Session, *domain.AppError) {
	sessionToken, ok := middleware.SessionToken(r)
	if !ok {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	return s.app.GetSession(r.Context(), sessionToken)
}

func (s *Server) withIdempotency(w http.ResponseWriter, r *http.Request, userID, routeKey string, body []byte, run func() (int, any, string, string, *domain.AppError)) {
	key := middleware.IdempotencyKey(r)
	hash := requestHash(r.Method, routeKey, body)
	entry, appErr := s.app.BeginIdempotency(r.Context(), userID, routeKey, key, hash)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if entry.State == "completed" {
		w.Header().Set("Content-Type", entry.ContentType)
		w.WriteHeader(entry.Status)
		_, _ = w.Write(entry.Body)
		return
	}

	status, payload, resourceType, resourceID, appErr := run()
	if appErr != nil {
		s.app.CancelIdempotency(r.Context(), entry)
		writeProblem(w, r, appErr)
		return
	}
	responseBody, err := json.Marshal(payload)
	if err != nil {
		s.app.CancelIdempotency(r.Context(), entry)
		writeProblem(w, r, domain.NewError(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error", "响应编码失败。"))
		return
	}
	contentType := "application/json; charset=utf-8"
	if appErr := s.app.CompleteIdempotency(r.Context(), entry, status, contentType, responseBody, resourceType, resourceID); appErr != nil {
		s.app.CancelIdempotency(r.Context(), entry)
		writeProblem(w, r, appErr)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write(responseBody)
}

func (s *Server) limitHandler(routeGroup string, limit int, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if appErr := s.checkRateLimit(r, routeGroup, limit); appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
		next(w, r)
	}
}

func (s *Server) checkRateLimit(r *http.Request, routeGroup string, limit int) *domain.AppError {
	if s.rateLimiter == nil || limit <= 0 {
		return nil
	}
	keys := []string{"ip:" + routeGroup + ":" + s.clientIP(r)}
	if sessionToken, ok := middleware.SessionToken(r); ok {
		if user, _, appErr := s.app.GetSession(r.Context(), sessionToken); appErr == nil && strings.TrimSpace(user.ID) != "" {
			keys = append(keys, "user:"+routeGroup+":"+user.ID)
		}
	}
	for _, key := range keys {
		decision := s.rateLimiter.Allow(key, limit)
		if !decision.Allowed {
			return domain.NewError(http.StatusTooManyRequests, domain.CodeRateLimited, "Rate limited", "请求过于频繁，请稍后再试。")
		}
	}
	return nil
}

func (s *Server) clientIP(r *http.Request) string {
	remote := directRemoteAddr(r)
	remoteAddr, ok := parseIPAddr(remote)
	if !ok {
		return valueOrUnknown(remote)
	}
	if s.trustXForwardedFor && s.isTrustedProxy(remoteAddr) {
		if forwarded := firstForwardedClientIP(r.Header.Get("X-Forwarded-For")); forwarded != "" {
			return forwarded
		}
		if realIP := singleHeaderIP(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	return remoteAddr.String()
}

func (s *Server) isTrustedProxy(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, prefix := range s.trustedProxyPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func directRemoteAddr(r *http.Request) string {
	if r == nil {
		return ""
	}
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(remote, "[]")
}

func firstForwardedClientIP(value string) string {
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	return singleHeaderIP(parts[0])
}

func singleHeaderIP(value string) string {
	addr, ok := parseIPAddr(value)
	if !ok {
		return ""
	}
	return addr.String()
}

func parseIPAddr(value string) (netip.Addr, bool) {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	if value == "" {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func trustedProxyPrefixes(values []string) []netip.Prefix {
	prefixes := []netip.Prefix{}
	for _, value := range values {
		if prefix, ok := trustedProxyPrefix(value); ok {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

func trustedProxyPrefix(value string) (netip.Prefix, bool) {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	if value == "" {
		return netip.Prefix{}, false
	}
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked(), true
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, false
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), true
}

func valueOrUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
