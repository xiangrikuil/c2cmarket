package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"
)

const (
	accountAppealCookieName = "c2c_account_appeal"
	accountAppealCookiePath = "/api/v1/account-appeal"
	accountAppealCSRFHeader = "X-Account-Appeal-CSRF"
)

type accountAppealSessionResponse struct {
	AccountStatus string `json:"accountStatus"`
	CSRFToken     string `json:"csrfToken"`
	ExpiresAt     string `json:"expiresAt"`
}

type createAccountGovernanceAppealRequest struct {
	Statement string `json:"statement"`
}

type accountGovernanceAppealResponse struct {
	ID         string `json:"id"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	Version    int64  `json:"version"`
}

func (s *Server) handleGetAccountAppealSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	rawSessionID, ok := accountAppealSessionToken(r)
	if !ok {
		writeProblem(w, r, accountAppealSessionRequiredError())
		return
	}
	user, session, appErr := s.app.GetAccountAppealSession(r.Context(), rawSessionID)
	if appErr != nil {
		s.clearAccountAppealCookie(w)
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, accountAppealSessionResponse{
		AccountStatus: user.Status,
		CSRFToken:     session.CSRFToken,
		ExpiresAt:     session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleCreateAccountGovernanceAppeal(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireAccountAppealSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createAccountGovernanceAppealRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	const routeKey = "POST /api/v1/account-appeal/appeals"
	completion, appErr := s.app.CreateAccountGovernanceAppealWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		report.CreateAccountGovernanceAppealInput{Statement: req.Statement},
		accountGovernanceAppealCompletionBuilder,
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) requireAccountAppealSessionAndCSRF(r *http.Request) (auth.User, auth.AccountAppealSession, *domain.AppError) {
	rawSessionID, ok := accountAppealSessionToken(r)
	if !ok {
		return auth.User{}, auth.AccountAppealSession{}, accountAppealSessionRequiredError()
	}
	rawCSRF := strings.TrimSpace(r.Header.Get(accountAppealCSRFHeader))
	if rawCSRF == "" {
		return auth.User{}, auth.AccountAppealSession{}, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF token invalid", "CSRF token 无效或缺失。")
	}
	return s.app.GetAccountAppealSessionWithCSRF(r.Context(), rawSessionID, rawCSRF)
}

func accountAppealSessionToken(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(accountAppealCookieName)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(cookie.Value)
	return value, value != ""
}

func accountAppealSessionRequiredError() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Account appeal session required", "申诉验证已失效，请重新验证账号。")
}

func (s *Server) setAccountAppealCookie(w http.ResponseWriter, session auth.AccountAppealSession) {
	http.SetCookie(w, &http.Cookie{
		Name:     accountAppealCookieName,
		Value:    session.ID,
		Path:     accountAppealCookiePath,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
		MaxAge:   int((15 * time.Minute) / time.Second),
	})
}

func (s *Server) clearAccountAppealCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     accountAppealCookieName,
		Value:    "",
		Path:     accountAppealCookiePath,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func accountGovernanceAppealCompletionBuilder(item report.Appeal) (idempotency.Completion, *domain.AppError) {
	payload := accountGovernanceAppealResponse{
		ID:         item.ID,
		TargetType: item.TargetType,
		TargetID:   item.TargetID,
		Title:      item.Title,
		Status:     item.Status,
		CreatedAt:  item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:    item.Version,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	return idempotency.Completion{
		Status:       http.StatusCreated,
		ContentType:  "application/json; charset=utf-8",
		Body:         body,
		ResourceType: "appeal",
		ResourceID:   item.ID,
		Headers:      map[string]string{"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`},
	}, nil
}
