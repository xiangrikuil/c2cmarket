package server

import (
	"net/http"
	"strings"

	"c2c-market/backend/internal/module/auth"
)

type passwordResetStartRequest struct {
	Email          string `json:"email"`
	TurnstileToken string `json:"turnstileToken"`
}

type passwordResetStartResponse struct {
	Accepted bool `json:"accepted"`
}

type passwordResetConfirmRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"newPassword"`
}

func (s *Server) handleStartPasswordReset(w http.ResponseWriter, r *http.Request) {
	req, appErr := decodeStrictJSONOnly[passwordResetStartRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	normalizedTarget := strings.ToLower(strings.TrimSpace(req.Email))
	if !s.allowTarget(w, r, passwordResetStartRateLimit, "email", normalizedTarget) {
		return
	}
	if appErr := s.verifyTurnstile(r, req.TurnstileToken, turnstileActionPasswordReset); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.app.StartPasswordReset(r.Context(), auth.PasswordResetStartInput{
		Email: req.Email, RequestID: requestIDFrom(r),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusAccepted, passwordResetStartResponse{Accepted: result.Accepted})
}

func (s *Server) handleConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	req, appErr := decodeStrictJSONOnly[passwordResetConfirmRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	normalizedTarget := strings.ToLower(strings.TrimSpace(req.Email))
	if !s.allowTarget(w, r, passwordResetConfirmRateLimit, "email", normalizedTarget) {
		return
	}
	if appErr := s.app.ConfirmPasswordReset(r.Context(), auth.PasswordResetConfirmInput{
		Email: req.Email, Code: req.Code, NewPassword: req.NewPassword, RequestID: requestIDFrom(r),
	}); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
