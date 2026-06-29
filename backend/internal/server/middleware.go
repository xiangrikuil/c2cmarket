package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/auth"
	"encoding/json"
	"net/http"
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
	keys := []string{"ip:" + routeGroup + ":" + clientIP(r)}
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

func clientIP(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			return first
		}
	}
	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, ok := strings.Cut(remote, ":"); ok && host != "" {
		return host
	}
	return remote
}
