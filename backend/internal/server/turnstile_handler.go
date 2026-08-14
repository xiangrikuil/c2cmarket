package server

import (
	"net/http"
	"net/netip"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/platform/turnstile"
)

const (
	turnstileActionPasswordLogin = "password_login"
	turnstileActionStudentSignup = "student_signup"
	turnstileActionPasswordReset = "password_reset"
)

func (s *Server) verifyTurnstile(r *http.Request, token, action string) *domain.AppError {
	if s.turnstile == nil {
		return turnstileVerificationError()
	}
	remoteIP := middleware.ClientIPFromRequest(r)
	if _, err := netip.ParseAddr(remoteIP); err != nil {
		remoteIP = ""
	}
	if err := s.turnstile.Verify(r.Context(), turnstile.Verification{
		Token:    token,
		Action:   action,
		RemoteIP: remoteIP,
	}); err != nil {
		return turnstileVerificationError()
	}
	return nil
}

func turnstileVerificationError() *domain.AppError {
	return domain.NewError(
		http.StatusForbidden,
		domain.CodeTurnstileVerificationFailed,
		"Turnstile verification failed",
		"人机验证失败，请重新验证后再试。",
	)
}
