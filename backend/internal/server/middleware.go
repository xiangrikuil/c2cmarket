package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/auth"
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) requireSessionAndCSRF(w http.ResponseWriter, r *http.Request) (auth.User, auth.Session, *domain.AppError) {
	sessionToken, ok := middleware.SessionToken(r)
	if !ok {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	csrfToken := middleware.CSRFToken(r)
	if csrfToken == "" {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF token invalid", "CSRF token 无效或缺失。")
	}
	user, session, appErr := s.app.GetSessionWithCSRF(r.Context(), sessionToken, csrfToken)
	return s.renewAuthenticatedSession(w, r, sessionToken, user, session, appErr)
}

func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (auth.User, auth.Session, *domain.AppError) {
	sessionToken, ok := middleware.SessionToken(r)
	if !ok {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	user, session, appErr := s.app.GetSession(r.Context(), sessionToken)
	return s.renewAuthenticatedSession(w, r, sessionToken, user, session, appErr)
}

func (s *Server) renewAuthenticatedSession(w http.ResponseWriter, r *http.Request, sessionToken string, user auth.User, session auth.Session, appErr *domain.AppError) (auth.User, auth.Session, *domain.AppError) {
	if appErr != nil || !shouldRenewSessionForRequest(r) {
		return user, session, appErr
	}
	renewedSession, renewed, appErr := s.app.RenewSession(r.Context(), sessionToken)
	if appErr != nil {
		return auth.User{}, auth.Session{}, appErr
	}
	if renewed {
		s.setSessionCookie(w, renewedSession)
		session.ExpiresAt = renewedSession.ExpiresAt
		session.RenewedAt = renewedSession.RenewedAt
	}
	return user, session, nil
}

func shouldRenewSessionForRequest(r *http.Request) bool {
	if r == nil || r.Method == http.MethodOptions {
		return false
	}
	switch r.URL.Path {
	case "/health",
		"/readyz",
		"/api/v1/auth/dev-session",
		"/api/v1/auth/password/login",
		"/api/v1/auth/email-registration/start",
		"/api/v1/auth/email-registration/confirm",
		"/api/v1/auth/oauth/start",
		"/api/v1/auth/oauth/callback",
		"/api/v1/auth/session",
		"/api/v1/auth/session/renew",
		"/api/v1/auth/logout",
		"/api/v1/me/events",
		"/api/v1/me/navigation-badges",
		"/api/v1/me/feedback-tickets/unread-count",
		"/api/v1/me/notifications/unread-count",
		"/api/v1/me/announcements/unread-count",
		"/api/v1/me/announcements/important-unread-count":
		return false
	default:
		return !strings.HasPrefix(r.URL.Path, "/assets/")
	}
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
	return s.limitHandlerByActor(routeGroup, limit, limit, next)
}

func (s *Server) limitHandlerByActor(routeGroup string, ipLimit, userLimit int, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if appErr := s.checkRateLimitByActor(r, routeGroup, ipLimit, userLimit); appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
		next(w, r)
	}
}

func (s *Server) checkRateLimit(r *http.Request, routeGroup string, limit int) *domain.AppError {
	return s.checkRateLimitByActor(r, routeGroup, limit, limit)
}

func (s *Server) checkRateLimitByActor(r *http.Request, routeGroup string, ipLimit, userLimit int) *domain.AppError {
	if s.rateLimiter == nil {
		return nil
	}
	type limitKey struct {
		value string
		limit int
	}
	keys := make([]limitKey, 0, 2)
	if sessionToken, ok := middleware.SessionToken(r); ok {
		if user, _, appErr := s.app.GetSession(r.Context(), sessionToken); appErr == nil && strings.TrimSpace(user.ID) != "" {
			keys = append(keys, limitKey{value: "user:" + routeGroup + ":" + user.ID, limit: userLimit})
		}
	}
	keys = append(keys, limitKey{value: "ip:" + routeGroup + ":" + middleware.ClientIPFromRequest(r), limit: ipLimit})
	for _, key := range keys {
		decision := s.rateLimiter.Allow(key.value, key.limit)
		if !decision.Allowed {
			return domain.NewError(http.StatusTooManyRequests, domain.CodeRateLimited, "Rate limited", "请求过于频繁，请稍后再试。")
		}
	}
	return nil
}
