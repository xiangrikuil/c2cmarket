package server

import (
	"bytes"
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/auth"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const oauthStateCookieName = "c2c_oauth_state"
const oauthMaxResponseBodyBytes = 1 << 20

type devSessionRequest struct {
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
}

type passwordLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type emailRegistrationStartRequest struct {
	Email string `json:"email"`
}

type emailRegistrationStartResponse struct {
	Email     string `json:"email"`
	ExpiresAt string `json:"expiresAt"`
	DevCode   string `json:"devCode,omitempty"`
}

type emailRegistrationConfirmRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type setPasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type oauthStartResponse struct {
	AuthorizationURL string `json:"authorizationUrl"`
}

type sessionResponse struct {
	User      userDTO `json:"user"`
	CSRFToken string  `json:"csrfToken"`
	ExpiresAt string  `json:"expiresAt"`
}

type userDTO struct {
	ID          string                   `json:"id"`
	Username    string                   `json:"username"`
	DisplayName string                   `json:"displayName"`
	IsAdmin     bool                     `json:"isAdmin"`
	Permissions []string                 `json:"permissions"`
	LinuxDo     sessionLinuxDoBindingDTO `json:"linuxDoBinding"`
}

type sessionLinuxDoBindingDTO struct {
	Bound           bool    `json:"bound"`
	LinuxDoUserID   *string `json:"linuxDoUserId,omitempty"`
	LinuxDoUsername *string `json:"linuxDoUsername,omitempty"`
	TrustLevel      *int    `json:"trustLevel,omitempty"`
	AvatarURL       *string `json:"avatarUrl,omitempty"`
}

func (s *Server) handleDevSession(w http.ResponseWriter, r *http.Request) {
	if !s.enableDevAuth {
		writeProblem(w, r, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Not found", "接口不存在。"))
		return
	}

	var req devSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeProblem(w, r, err)
		return
	}

	user, session, appErr := s.app.CreateDevSession(r.Context(), req.Username, req.Admin)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.setSessionCookie(w, session)

	writeJSON(w, http.StatusOK, sessionResponse{
		User:      toUserDTO(user),
		CSRFToken: session.CSRFToken,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handlePasswordLogin(w http.ResponseWriter, r *http.Request) {
	var req passwordLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeProblem(w, r, err)
		return
	}

	user, session, appErr := s.app.LoginWithPassword(r.Context(), req.Username, req.Password)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.setSessionCookie(w, session)

	writeJSON(w, http.StatusOK, sessionResponse{
		User:      toUserDTO(user),
		CSRFToken: session.CSRFToken,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleStartEmailRegistration(w http.ResponseWriter, r *http.Request) {
	req, appErr := decodeStrictJSONOnly[emailRegistrationStartRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	challenge, appErr := s.app.StartEmailRegistration(r.Context(), auth.EmailRegistrationStartInput{Email: req.Email})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, emailRegistrationStartResponse{
		Email:     challenge.Email,
		ExpiresAt: challenge.ExpiresAt.UTC().Format(time.RFC3339),
		DevCode:   challenge.DevCode,
	})
}

func (s *Server) handleConfirmEmailRegistration(w http.ResponseWriter, r *http.Request) {
	req, appErr := decodeStrictJSONOnly[emailRegistrationConfirmRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	user, session, appErr := s.app.ConfirmEmailRegistration(r.Context(), auth.EmailRegistrationConfirmInput{
		Email: req.Email,
		Code:  req.Code,
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.setSessionCookie(w, session)
	writeJSON(w, http.StatusOK, sessionResponse{
		User:      toUserDTO(user),
		CSRFToken: session.CSRFToken,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[setPasswordRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr := s.app.SetPassword(r.Context(), auth.SetPasswordInput{
		UserID:          user.ID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	state := newOAuthState()
	returnTo := cleanReturnTo(r.URL.Query().Get("returnTo"))
	cookieValue := state
	if returnTo != "" {
		cookieValue += "|" + returnTo
	}
	s.setOAuthStateCookie(w, cookieValue)
	writeJSON(w, http.StatusOK, oauthStartResponse{AuthorizationURL: s.oauthAuthorizationURL(r, state, returnTo)})
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || state == "" {
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "OAuth state invalid", "登录 state 无效或已过期。"))
		return
	}
	expectedState, returnTo := splitOAuthStateCookie(stateCookie.Value)
	if expectedState == "" || expectedState != state {
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "OAuth state invalid", "登录 state 无效或已过期。"))
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "OAuth code required", "OAuth 回调缺少 code。", "code", "required", "OAuth 回调缺少 code。"))
		return
	}
	profile, appErr := s.oauthProfile(r.Context(), code)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	user, session, appErr := s.app.LoginWithOAuthProfile(r.Context(), profile)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.setSessionCookie(w, session)
	s.clearOAuthStateCookie(w)
	_ = user
	http.Redirect(w, r, cleanReturnTo(returnTo), http.StatusFound)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionToken, ok := middleware.SessionToken(r)
	if !ok {
		writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。"))
		return
	}
	user, session, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	csrfToken, appErr := s.app.RefreshSessionCSRF(r.Context(), sessionToken)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	session.CSRFToken = csrfToken
	writeJSON(w, http.StatusOK, sessionResponse{
		User:      toUserDTO(user),
		CSRFToken: session.CSRFToken,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	_, session, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.app.Logout(r.Context(), session.ID)
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func toUserDTO(user auth.User) userDTO {
	permissions := []string{}
	if user.IsAdmin {
		permissions = append(permissions, "admin")
	}
	return userDTO{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
		Permissions: permissions,
		LinuxDo:     toLinuxDoBindingDTO(user.LinuxDoBinding),
	}
}

func toLinuxDoBindingDTO(binding *auth.LinuxDoBinding) sessionLinuxDoBindingDTO {
	if binding == nil || !binding.Bound {
		return sessionLinuxDoBindingDTO{Bound: false}
	}
	return sessionLinuxDoBindingDTO{
		Bound:           true,
		LinuxDoUserID:   stringPtr(binding.LinuxDoUserID),
		LinuxDoUsername: stringPtr(binding.LinuxDoUsername),
		TrustLevel:      intPtr(binding.TrustLevel),
		AvatarURL:       stringPtr(binding.AvatarURL),
	}
}

func (s *Server) setSessionCookie(w http.ResponseWriter, session auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) setOAuthStateCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(10 * time.Minute),
	})
}

func (s *Server) clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func newOAuthState() string {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return "oauth_" + hex.EncodeToString(buf[:])
}

func (s *Server) oauthAuthorizationURL(r *http.Request, state, returnTo string) string {
	mode := strings.TrimSpace(s.oauth.ProviderMode)
	if mode == "" || mode == "fake" {
		callback := s.oauth.RedirectURL
		if callback == "" {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			callback = scheme + "://" + r.Host + "/api/v1/auth/oauth/callback"
		}
		values := url.Values{}
		values.Set("code", "fake-user")
		values.Set("state", state)
		if returnTo != "" {
			values.Set("returnTo", returnTo)
		}
		return callback + "?" + values.Encode()
	}
	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", s.oauth.ClientID)
	values.Set("redirect_uri", s.oauth.RedirectURL)
	values.Set("scope", s.oauth.Scopes)
	values.Set("state", state)
	return strings.TrimRight(s.oauth.AuthorizeURL, "?") + "?" + values.Encode()
}

func (s *Server) oauthProfile(ctx context.Context, code string) (auth.OAuthProfile, *domain.AppError) {
	if strings.TrimSpace(s.oauth.ProviderMode) == "" || s.oauth.ProviderMode == "fake" {
		return fakeOAuthProfile(code), nil
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", s.oauth.RedirectURL)
	form.Set("client_id", s.oauth.ClientID)
	form.Set("client_secret", s.oauth.ClientSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.oauth.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return auth.OAuthProfile{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "OAuth token request failed", "OAuth token 请求创建失败。")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if appErr := s.fetchOAuthJSON(request, &token); appErr != nil {
		return auth.OAuthProfile{}, appErr
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return auth.OAuthProfile{}, domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth token missing", "OAuth provider 未返回 access token。")
	}
	userRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, s.oauth.UserInfoURL, nil)
	if err != nil {
		return auth.OAuthProfile{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "OAuth userinfo request failed", "OAuth 用户资料请求创建失败。")
	}
	userRequest.Header.Set("Authorization", "Bearer "+token.AccessToken)
	var info struct {
		Subject           string `json:"sub"`
		ID                string `json:"id"`
		Username          string `json:"username"`
		PreferredUsername string `json:"preferred_username"`
		Login             string `json:"login"`
		Name              string `json:"name"`
		DisplayName       string `json:"display_name"`
		Email             string `json:"email"`
		AvatarURL         string `json:"avatar_url"`
		Picture           string `json:"picture"`
		TrustLevel        int    `json:"trust_level"`
		TrustLevelCamel   int    `json:"trustLevel"`
	}
	if appErr := s.fetchOAuthJSON(userRequest, &info); appErr != nil {
		return auth.OAuthProfile{}, appErr
	}
	subject := firstNonEmpty(info.Subject, info.ID)
	username := firstNonEmpty(info.Username, info.PreferredUsername, info.Login, subject)
	displayName := firstNonEmpty(info.DisplayName, info.Name, username)
	avatarURL := firstNonEmpty(info.AvatarURL, info.Picture)
	trustLevel := info.TrustLevel
	if trustLevel == 0 {
		trustLevel = info.TrustLevelCamel
	}
	return auth.OAuthProfile{
		Provider:         "linux_do",
		Subject:          subject,
		Username:         username,
		DisplayName:      displayName,
		Email:            info.Email,
		AvatarURL:        avatarURL,
		TrustLevel:       trustLevel,
		LinuxDoUserID:    subject,
		LinuxDoUsername:  username,
		LinuxDoAvatarURL: avatarURL,
	}, nil
}

func fakeOAuthProfile(code string) auth.OAuthProfile {
	username := strings.TrimSpace(strings.ToLower(code))
	grantAdmin := strings.Contains(username, "admin")
	username = strings.TrimPrefix(username, "fake-")
	if username == "" {
		username = "oauth-user"
	}
	return auth.OAuthProfile{
		Provider:         "linux_do",
		Subject:          "fake-" + username,
		Username:         username,
		DisplayName:      username,
		Email:            username + "@example.test",
		TrustLevel:       3,
		GrantAdmin:       grantAdmin,
		LinuxDoUserID:    "fake-" + username,
		LinuxDoUsername:  username,
		LinuxDoAvatarURL: "",
	}
}

func (s *Server) fetchOAuthJSON(request *http.Request, target any) *domain.AppError {
	client := s.oauthHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider unavailable", "OAuth provider 请求失败。")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider rejected request", "OAuth provider 返回失败状态。")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, oauthMaxResponseBodyBytes+1))
	if err != nil {
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider invalid response", "OAuth provider 响应解析失败。")
	}
	if len(body) > oauthMaxResponseBodyBytes {
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider response too large", "OAuth provider 响应过大。")
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(target); err != nil {
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider invalid response", "OAuth provider 响应解析失败。")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func splitOAuthStateCookie(value string) (string, string) {
	parts := strings.SplitN(value, "|", 2)
	if len(parts) == 1 {
		return parts[0], "/"
	}
	return parts[0], cleanReturnTo(parts[1])
}

func cleanReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/"
	}
	return value
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func intPtr(value int) *int {
	return &value
}
