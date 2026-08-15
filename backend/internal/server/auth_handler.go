package server

import (
	"bytes"
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/devpersona"
	"c2c-market/backend/internal/module/promotionreward"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const oauthStateCookieName = "c2c_oauth_state"
const oauthLinkStateCookieName = "c2c_oauth_link_state"
const restrictedBusinessOAuthStateCookieName = "c2c_restricted_business_oauth_state"
const adminReauthenticationOAuthStateCookieName = "c2c_admin_reauthentication_oauth_state"
const accountAppealOAuthStateCookieName = "c2c_account_appeal_oauth_state"
const oauthMaxResponseBodyBytes = 1 << 20
const oauthPurposeAccountAppeal = "account_appeal"
const oauthPurposeLinkLinuxDo = auth.OAuthPurposeLinkLinuxDo
const oauthPurposeRestrictedBusiness = auth.SessionAudienceRestrictedBusiness
const oauthPurposeGrantAdminReauthentication = auth.OAuthPurposeGrantAdminReauthentication
const accountAppealFrontendPath = "/account-appeal"

type devSessionRequest struct {
	Username string `json:"username"`
	Admin    bool   `json:"admin"`
}

type devPersonaSessionRequest struct {
	Persona string `json:"persona"`
}

type passwordLoginRequest struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	TurnstileToken string `json:"turnstileToken"`
}

type emailRegistrationStartRequest struct {
	Email          string `json:"email"`
	TurnstileToken string `json:"turnstileToken"`
}

type emailRegistrationStartResponse struct {
	Email     string `json:"email"`
	ExpiresAt string `json:"expiresAt"`
	DevCode   string `json:"devCode,omitempty"`
}

type emailRegistrationConfirmRequest struct {
	Email       string                         `json:"email"`
	Code        string                         `json:"code"`
	Username    string                         `json:"username"`
	Password    string                         `json:"password"`
	Attribution registrationAttributionRequest `json:"attribution"`
}

type passwordReauthenticateRequest struct {
	Password string `json:"password"`
	Purpose  string `json:"purpose"`
}

type emailRegistrationConfigResponse struct {
	Enabled      bool                                   `json:"enabled"`
	Institutions []emailRegistrationInstitutionResponse `json:"institutions"`
}

type usernameAvailabilityResponse struct {
	Username  string `json:"username"`
	Available bool   `json:"available"`
}

type emailRegistrationInstitutionResponse struct {
	Domain          string `json:"domain"`
	InstitutionName string `json:"institutionName"`
}

type registrationAttributionRequest struct {
	Source       string `json:"source"`
	Medium       string `json:"medium"`
	Campaign     string `json:"campaign"`
	ReferrerHost string `json:"referrerHost"`
	LandingPath  string `json:"landingPath"`
}

func (request registrationAttributionRequest) model() auth.RegistrationAttribution {
	return auth.RegistrationAttribution{
		Source:       request.Source,
		Medium:       request.Medium,
		Campaign:     request.Campaign,
		ReferrerHost: request.ReferrerHost,
		LandingPath:  request.LandingPath,
	}
}

type oauthStateCookiePayload struct {
	State       string                       `json:"state"`
	ReturnTo    string                       `json:"returnTo"`
	Purpose     string                       `json:"purpose,omitempty"`
	Attribution auth.RegistrationAttribution `json:"attribution"`
	InviteCode  string                       `json:"inviteCode,omitempty"`
}

type setPasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type oauthStartResponse struct {
	AuthorizationURL string `json:"authorizationUrl"`
}

type oauthProviderID string

func (id *oauthProviderID) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	switch typed := value.(type) {
	case string:
		*id = oauthProviderID(strings.TrimSpace(typed))
	case json.Number:
		if _, err := typed.Int64(); err != nil {
			return fmt.Errorf("oauth provider numeric id must be an integer: %w", err)
		}
		*id = oauthProviderID(typed.String())
	case nil:
		*id = ""
	default:
		return fmt.Errorf("oauth provider id must be a string or number, got %T", value)
	}
	return nil
}

type sessionResponse struct {
	User      userDTO `json:"user"`
	Audience  string  `json:"audience"`
	CSRFToken string  `json:"csrfToken"`
	ExpiresAt string  `json:"expiresAt"`
}

type devPersonaSessionResponse struct {
	Persona   devpersona.Persona `json:"persona"`
	User      userDTO            `json:"user"`
	Audience  string             `json:"audience"`
	CSRFToken string             `json:"csrfToken"`
	ExpiresAt string             `json:"expiresAt"`
}

type userDTO struct {
	ID              string                   `json:"id"`
	AnalyticsUserID string                   `json:"analyticsUserId"`
	Username        string                   `json:"username"`
	DisplayName     string                   `json:"displayName"`
	IsAdmin         bool                     `json:"isAdmin"`
	Permissions     []string                 `json:"permissions"`
	Capabilities    []string                 `json:"capabilities"`
	StudentClaim    *sessionStudentClaimDTO  `json:"studentClaim"`
	LinuxDo         sessionLinuxDoBindingDTO `json:"linuxDoBinding"`
}

// sessionStudentClaimDTO intentionally omits the canonical email and internal
// identifiers. Session consumers only need the safe institution proof and its
// timestamp; authorization always reloads the durable claim on the backend.
type sessionStudentClaimDTO struct {
	InstitutionDomain string `json:"institutionDomain"`
	InstitutionName   string `json:"institutionName"`
	ClaimedAt         string `json:"claimedAt"`
}

type sessionLinuxDoBindingDTO struct {
	Bound           bool    `json:"bound"`
	LinuxDoUserID   *string `json:"linuxDoUserId,omitempty"`
	LinuxDoUsername *string `json:"linuxDoUsername,omitempty"`
	TrustLevel      *int    `json:"trustLevel,omitempty"`
	AvatarURL       *string `json:"avatarUrl,omitempty"`
}

type adminUserResponse struct {
	ID            string  `json:"id"`
	Username      string  `json:"username"`
	DisplayName   string  `json:"displayName"`
	AccountStatus string  `json:"accountStatus"`
	IsAdmin       bool    `json:"isAdmin"`
	LinuxDoBound  bool    `json:"linuxDoBound"`
	TrustLevel    *int    `json:"trustLevel,omitempty"`
	CreatedAt     string  `json:"createdAt"`
	LastActiveAt  *string `json:"lastActiveAt,omitempty"`
	Version       int64   `json:"version"`
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
		Audience:  auth.SessionAudienceNormal,
		CSRFToken: session.CSRFToken,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDevPersonaSession(w http.ResponseWriter, r *http.Request) {
	if !s.enableDevAuth {
		writeProblem(w, r, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Not found", "接口不存在。"))
		return
	}
	req, appErr := decodeStrictJSONOnly[devPersonaSessionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.devPersonas.PrepareDevPersonaSession(r.Context(), req.Persona)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.setSessionCookie(w, result.Session)
	writeJSON(w, http.StatusOK, devPersonaSessionResponse{
		Persona:   result.Persona,
		User:      toUserDTO(result.User),
		Audience:  auth.SessionAudienceNormal,
		CSRFToken: result.Session.CSRFToken,
		ExpiresAt: result.Session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	query, appErr := parseAdminUserDirectoryQuery(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	directory, appErr := s.adminUsers.AdminUsers(r.Context(), user, query)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserDirectoryResponse(directory))
}

func (s *Server) handlePasswordLogin(w http.ResponseWriter, r *http.Request) {
	var req passwordLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeProblem(w, r, err)
		return
	}
	if appErr := s.verifyTurnstile(r, req.TurnstileToken, turnstileActionPasswordLogin); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	result, appErr := s.app.AuthenticateWithPassword(r.Context(), req.Username, req.Password)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.writeAuthenticationResult(w, result)
}

func (s *Server) handlePasswordReauthenticate(w http.ResponseWriter, r *http.Request) {
	sessionToken, ok := middleware.SessionToken(r)
	if !ok {
		writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。"))
		return
	}
	_, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[passwordReauthenticateRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr := s.app.ReauthenticatePasswordForPurpose(r.Context(), sessionToken, middleware.CSRFToken(r), req.Password, req.Purpose); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleEmailRegistrationConfig(w http.ResponseWriter, r *http.Request) {
	config, appErr := s.app.StudentRegistrationConfig(r.Context())
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	institutions := make([]emailRegistrationInstitutionResponse, 0, len(config.Institutions))
	for _, institution := range config.Institutions {
		if !institution.Enabled {
			continue
		}
		institutions = append(institutions, emailRegistrationInstitutionResponse{
			Domain: institution.Domain, InstitutionName: institution.InstitutionName,
		})
	}
	writeJSON(w, http.StatusOK, emailRegistrationConfigResponse{Enabled: config.Enabled, Institutions: institutions})
}

func (s *Server) handleUsernameAvailability(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	username := r.URL.Query().Get("username")
	available, appErr := s.app.UsernameAvailable(r.Context(), username)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, usernameAvailabilityResponse{Username: username, Available: available})
}

func (s *Server) handleStartEmailRegistration(w http.ResponseWriter, r *http.Request) {
	req, appErr := decodeStrictJSONOnly[emailRegistrationStartRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !s.allowTarget(w, r, emailRegistrationStartRateLimit, "email", req.Email) {
		return
	}
	if appErr := s.verifyTurnstile(r, req.TurnstileToken, turnstileActionStudentSignup); appErr != nil {
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
	if !s.allowTarget(w, r, emailRegistrationConfirmRateLimit, "email", req.Email) {
		return
	}
	user, session, appErr := s.app.ConfirmEmailRegistration(r.Context(), auth.EmailRegistrationConfirmInput{
		Email:       req.Email,
		Code:        req.Code,
		Username:    req.Username,
		Password:    req.Password,
		Attribution: req.Attribution.model(),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.setSessionCookie(w, session)
	writeJSON(w, http.StatusOK, sessionResponse{
		User:      toUserDTO(user),
		Audience:  auth.SessionAudienceNormal,
		CSRFToken: session.CSRFToken,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
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
	purpose := strings.TrimSpace(r.URL.Query().Get("purpose"))
	state := newOAuthState()
	if purpose != "" && purpose != oauthPurposeLinkLinuxDo && purpose != oauthPurposeRestrictedBusiness && purpose != oauthPurposeGrantAdminReauthentication {
		writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "OAuth purpose invalid", "OAuth purpose 无效。", "purpose", "invalid", "OAuth purpose 无效。"))
		return
	}
	if purpose == oauthPurposeLinkLinuxDo {
		sessionToken, ok := middleware.SessionToken(r)
		if !ok {
			writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。"))
			return
		}
		var appErr *domain.AppError
		state, appErr = s.app.StartLinuxDoLink(r.Context(), sessionToken)
		if appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
	}
	if purpose == oauthPurposeGrantAdminReauthentication {
		sessionToken, ok := middleware.SessionToken(r)
		if !ok {
			writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。"))
			return
		}
		var appErr *domain.AppError
		state, appErr = s.app.StartAdminReauthenticationOAuth(r.Context(), sessionToken)
		if appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
	}
	if purpose == oauthPurposeRestrictedBusiness {
		var appErr *domain.AppError
		state, appErr = s.app.StartRestrictedBusinessOAuth(r.Context())
		if appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
	}
	returnTo := cleanReturnTo(r.URL.Query().Get("returnTo"))
	attribution := auth.NormalizeRegistrationAttribution(auth.RegistrationAttribution{
		Source:       r.URL.Query().Get("utmSource"),
		Medium:       r.URL.Query().Get("utmMedium"),
		Campaign:     r.URL.Query().Get("utmCampaign"),
		ReferrerHost: r.URL.Query().Get("referrerHost"),
		LandingPath:  r.URL.Query().Get("landingPath"),
	})
	s.setOAuthStateCookie(w, oauthStateCookieNameForPurpose(purpose), encodeOAuthStateCookie(oauthStateCookiePayload{
		State:       state,
		ReturnTo:    returnTo,
		Purpose:     purpose,
		Attribution: attribution,
		InviteCode:  promotionreward.CanonicalReferralCode(r.URL.Query().Get("inviteCode")),
	}))
	writeJSON(w, http.StatusOK, oauthStartResponse{AuthorizationURL: s.oauthAuthorizationURL(r, state, returnTo)})
}

func (s *Server) handleAccountAppealOAuthStart(w http.ResponseWriter, r *http.Request) {
	state, appErr := s.app.StartAccountAppealOAuth(r.Context())
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.setOAuthStateCookie(w, accountAppealOAuthStateCookieName, encodeOAuthStateCookie(oauthStateCookiePayload{
		State:    state,
		ReturnTo: accountAppealFrontendPath,
		Purpose:  oauthPurposeAccountAppeal,
	}))
	writeJSON(w, http.StatusOK, oauthStartResponse{
		AuthorizationURL: s.oauthAuthorizationURL(r, state, accountAppealFrontendPath),
	})
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	payload, stateCookieName, ok := oauthStatePayloadFromRequest(r, state)
	if !ok {
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "OAuth state invalid", "登录 state 无效或已过期。"))
		return
	}
	if payload.Purpose != "" && payload.Purpose != oauthPurposeAccountAppeal && payload.Purpose != oauthPurposeLinkLinuxDo && payload.Purpose != oauthPurposeRestrictedBusiness && payload.Purpose != oauthPurposeGrantAdminReauthentication {
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
	if payload.Purpose == oauthPurposeAccountAppeal {
		s.handleAccountAppealOAuthCallback(w, r, stateCookieName, state, profile)
		return
	}
	if payload.Purpose == oauthPurposeLinkLinuxDo {
		sessionToken, ok := middleware.SessionToken(r)
		if !ok {
			writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。"))
			return
		}
		user, session, appErr := s.app.CompleteLinuxDoLink(r.Context(), sessionToken, state, profile)
		if appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
		s.setSessionCookie(w, session)
		s.clearOAuthStateCookie(w, stateCookieName)
		_ = user
		http.Redirect(w, r, s.oauthRedirectTarget(appendAuthOutcome(payload.ReturnTo, "linked")), http.StatusFound)
		return
	}
	if payload.Purpose == oauthPurposeGrantAdminReauthentication {
		sessionToken, ok := middleware.SessionToken(r)
		if !ok {
			writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。"))
			return
		}
		if appErr := s.app.CompleteAdminReauthenticationOAuth(r.Context(), sessionToken, state, profile); appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
		s.clearOAuthStateCookie(w, stateCookieName)
		http.Redirect(w, r, s.oauthRedirectTarget(appendAuthOutcome(payload.ReturnTo, "admin_reauthenticated")), http.StatusFound)
		return
	}
	var result auth.AuthenticationResult
	if payload.Purpose == oauthPurposeRestrictedBusiness {
		result, appErr = s.app.CompleteRestrictedBusinessOAuth(r.Context(), state, profile)
	} else {
		profile.Attribution = payload.Attribution
		profile.ReferralCode = payload.InviteCode
		result, appErr = s.app.AuthenticateWithOAuthProfile(r.Context(), profile)
	}
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if payload.Purpose == oauthPurposeRestrictedBusiness && result.Audience != auth.SessionAudienceRestrictedBusiness {
		s.clearOAuthStateCookie(w, stateCookieName)
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Restricted business unavailable", "当前账号没有可用的受限业务入口。"))
		return
	}
	if result.Audience == auth.SessionAudienceRestrictedBusiness {
		s.setRestrictedBusinessSessionCookie(w, result.RestrictedSession)
	} else {
		s.setSessionCookie(w, result.Session)
	}
	s.clearOAuthStateCookie(w, stateCookieName)
	outcome := "logged_in"
	if result.Session.NewRegistration {
		outcome = "registered"
	}
	if result.Audience == auth.SessionAudienceRestrictedBusiness {
		outcome = "restricted_business"
	}
	http.Redirect(w, r, s.oauthRedirectTarget(appendAuthOutcome(payload.ReturnTo, outcome)), http.StatusFound)
}

func (s *Server) handleAccountAppealOAuthCallback(w http.ResponseWriter, r *http.Request, stateCookieName, state string, profile auth.OAuthProfile) {
	_, session, appErr := s.app.CompleteAccountAppealOAuth(r.Context(), state, profile)
	if appErr != nil {
		if appErr.Code == domain.CodeAccountAppealIneligible {
			s.clearOAuthStateCookie(w, stateCookieName)
			s.clearAccountAppealCookie(w)
			http.Redirect(w, r, s.oauthRedirectTarget(appendAccountAppealOutcome("ineligible")), http.StatusFound)
			return
		}
		writeProblem(w, r, appErr)
		return
	}
	s.setAccountAppealCookie(w, session)
	s.clearOAuthStateCookie(w, stateCookieName)
	http.Redirect(w, r, s.oauthRedirectTarget(appendAccountAppealOutcome("verified")), http.StatusFound)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionToken, ok := middleware.SessionToken(r)
	if !ok {
		writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。"))
		return
	}
	user, session, appErr := s.requireSession(w, r)
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
		Audience:  auth.SessionAudienceNormal,
		CSRFToken: session.CSRFToken,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleGetRestrictedBusinessSession(w http.ResponseWriter, r *http.Request) {
	sessionToken, ok := middleware.RestrictedBusinessSessionToken(r)
	if !ok {
		writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Restricted business session required", "请先完成受限业务身份验证。"))
		return
	}
	user, session, appErr := s.app.GetRestrictedBusinessSession(r.Context(), sessionToken)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	csrfToken, appErr := s.app.RefreshRestrictedBusinessSessionCSRF(r.Context(), sessionToken)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, sessionResponse{
		User:      toUserDTO(user),
		Audience:  auth.SessionAudienceRestrictedBusiness,
		CSRFToken: csrfToken,
		ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleRestrictedBusinessLogout(w http.ResponseWriter, r *http.Request) {
	sessionToken, ok := middleware.RestrictedBusinessSessionToken(r)
	if !ok {
		writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Restricted business session required", "请先完成受限业务身份验证。"))
		return
	}
	csrfToken := middleware.RestrictedBusinessCSRFToken(r)
	if csrfToken == "" {
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF token invalid", "受限业务 CSRF token 无效或缺失。"))
		return
	}
	if _, _, appErr := s.app.GetRestrictedBusinessSessionWithCSRF(r.Context(), sessionToken, csrfToken); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.app.LogoutRestrictedBusinessSession(r.Context(), sessionToken)
	s.clearRestrictedBusinessSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeAuthenticationResult(w http.ResponseWriter, result auth.AuthenticationResult) {
	response := sessionResponse{User: toUserDTO(result.User), Audience: result.Audience}
	if result.Audience == auth.SessionAudienceRestrictedBusiness {
		s.setRestrictedBusinessSessionCookie(w, result.RestrictedSession)
		response.CSRFToken = result.RestrictedSession.CSRFToken
		response.ExpiresAt = result.RestrictedSession.ExpiresAt.UTC().Format(time.RFC3339)
	} else {
		s.setSessionCookie(w, result.Session)
		response.CSRFToken = result.Session.CSRFToken
		response.ExpiresAt = result.Session.ExpiresAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	sessionToken, ok := middleware.SessionToken(r)
	if !ok {
		writeProblem(w, r, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。"))
		return
	}
	_, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.app.Logout(r.Context(), sessionToken)
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func toUserDTO(user auth.User) userDTO {
	permissions := []string{}
	if user.IsAdmin {
		permissions = append(permissions, "admin")
	}
	return userDTO{
		ID:              user.ID,
		AnalyticsUserID: user.AnalyticsUserID,
		Username:        user.Username,
		DisplayName:     user.DisplayName,
		IsAdmin:         user.IsAdmin,
		Permissions:     permissions,
		Capabilities:    append([]string(nil), auth.ProjectCapabilities(user)...),
		StudentClaim:    toStudentClaimDTO(user.StudentClaim),
		LinuxDo:         toLinuxDoBindingDTO(user.LinuxDoBinding),
	}
}

func toStudentClaimDTO(claim *auth.StudentEmailClaim) *sessionStudentClaimDTO {
	if claim == nil {
		return nil
	}
	return &sessionStudentClaimDTO{
		InstitutionDomain: claim.InstitutionDomain,
		InstitutionName:   claim.InstitutionName,
		ClaimedAt:         claim.ClaimedAt.UTC().Format(time.RFC3339),
	}
}

func toAdminUserResponses(items []auth.AdminUser) []adminUserResponse {
	result := make([]adminUserResponse, 0, len(items))
	for _, item := range items {
		result = append(result, adminUserResponse{
			ID:            item.ID,
			Username:      item.Username,
			DisplayName:   item.DisplayName,
			AccountStatus: item.Status,
			IsAdmin:       item.IsAdmin,
			LinuxDoBound:  item.LinuxDoBound,
			TrustLevel:    item.TrustLevel,
			CreatedAt:     item.CreatedAt.UTC().Format(time.RFC3339),
			LastActiveAt:  formatOptionalTime(item.LastActiveAt),
			Version:       item.Version,
		})
	}
	return result
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
	maxAge := 0
	if !session.RenewedAt.IsZero() && session.ExpiresAt.After(session.RenewedAt) {
		maxAge = int(session.ExpiresAt.Sub(session.RenewedAt) / time.Second)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
		MaxAge:   maxAge,
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

func (s *Server) setRestrictedBusinessSessionCookie(w http.ResponseWriter, session auth.RestrictedBusinessSession) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.RestrictedBusinessSessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
		MaxAge:   max(0, int(time.Until(session.ExpiresAt)/time.Second)),
	})
}

func (s *Server) clearRestrictedBusinessSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.RestrictedBusinessSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) setOAuthStateCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(10 * time.Minute),
	})
}

func (s *Server) clearOAuthStateCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func oauthStateCookieNameForPurpose(purpose string) string {
	switch purpose {
	case oauthPurposeLinkLinuxDo:
		return oauthLinkStateCookieName
	case oauthPurposeRestrictedBusiness:
		return restrictedBusinessOAuthStateCookieName
	case oauthPurposeGrantAdminReauthentication:
		return adminReauthenticationOAuthStateCookieName
	case oauthPurposeAccountAppeal:
		return accountAppealOAuthStateCookieName
	default:
		return oauthStateCookieName
	}
}

func oauthStatePayloadFromRequest(r *http.Request, state string) (oauthStateCookiePayload, string, bool) {
	state = strings.TrimSpace(state)
	if state == "" {
		return oauthStateCookiePayload{}, "", false
	}
	type candidate struct {
		name    string
		purpose string
	}
	candidates := []candidate{
		{name: oauthStateCookieName, purpose: ""},
		{name: oauthLinkStateCookieName, purpose: oauthPurposeLinkLinuxDo},
		{name: restrictedBusinessOAuthStateCookieName, purpose: oauthPurposeRestrictedBusiness},
		{name: adminReauthenticationOAuthStateCookieName, purpose: oauthPurposeGrantAdminReauthentication},
		{name: accountAppealOAuthStateCookieName, purpose: oauthPurposeAccountAppeal},
	}
	var matched oauthStateCookiePayload
	matchedName := ""
	for _, item := range candidates {
		cookie, err := r.Cookie(item.name)
		if err != nil {
			continue
		}
		payload, ok := decodeOAuthStateCookie(cookie.Value)
		if !ok || payload.State != state || payload.Purpose != item.purpose || matchedName != "" {
			if ok && payload.State == state {
				return oauthStateCookiePayload{}, "", false
			}
			continue
		}
		matched = payload
		matchedName = item.name
	}
	return matched, matchedName, matchedName != ""
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
		Subject           oauthProviderID `json:"sub"`
		ID                oauthProviderID `json:"id"`
		Username          string          `json:"username"`
		PreferredUsername string          `json:"preferred_username"`
		Login             string          `json:"login"`
		Name              string          `json:"name"`
		DisplayName       string          `json:"display_name"`
		Email             string          `json:"email"`
		AvatarURL         string          `json:"avatar_url"`
		AvatarTemplate    string          `json:"avatar_template"`
		Picture           string          `json:"picture"`
		TrustLevel        int             `json:"trust_level"`
		TrustLevelCamel   int             `json:"trustLevel"`
	}
	if appErr := s.fetchOAuthJSON(userRequest, &info); appErr != nil {
		return auth.OAuthProfile{}, appErr
	}
	subject := firstNonEmpty(string(info.Subject), string(info.ID))
	username := firstNonEmpty(info.Username, info.PreferredUsername, info.Login, subject)
	displayName := firstNonEmpty(info.DisplayName, info.Name, username)
	avatarURL := normalizeLinuxDoAvatarURL(firstNonEmpty(info.AvatarURL, info.Picture, info.AvatarTemplate))
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
		log.Printf("oauth_provider_request_failed method=%s host=%s path=%s", request.Method, request.URL.Host, request.URL.Path)
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider unavailable", "OAuth provider 请求失败。")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf("oauth_provider_rejected_request method=%s host=%s path=%s status=%d", request.Method, request.URL.Host, request.URL.Path, response.StatusCode)
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider rejected request", "OAuth provider 返回失败状态。")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, oauthMaxResponseBodyBytes+1))
	if err != nil {
		log.Printf("oauth_provider_response_read_failed method=%s host=%s path=%s", request.Method, request.URL.Host, request.URL.Path)
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider invalid response", "OAuth provider 响应解析失败。")
	}
	if len(body) > oauthMaxResponseBodyBytes {
		log.Printf("oauth_provider_response_too_large method=%s host=%s path=%s", request.Method, request.URL.Host, request.URL.Path)
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "OAuth provider response too large", "OAuth provider 响应过大。")
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(target); err != nil {
		log.Printf("oauth_provider_response_invalid_json method=%s host=%s path=%s", request.Method, request.URL.Host, request.URL.Path)
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

func normalizeLinuxDoAvatarURL(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "{size}", "288")
}

func encodeOAuthStateCookie(payload oauthStateCookiePayload) string {
	payload.ReturnTo = cleanReturnTo(payload.ReturnTo)
	payload.Attribution = auth.NormalizeRegistrationAttribution(payload.Attribution)
	payload.InviteCode = promotionreward.CanonicalReferralCode(payload.InviteCode)
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeOAuthStateCookie(value string) (oauthStateCookiePayload, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(decoded) == 0 || len(decoded) > 2048 {
		return oauthStateCookiePayload{}, false
	}
	var payload oauthStateCookiePayload
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || strings.TrimSpace(payload.State) == "" {
		return oauthStateCookiePayload{}, false
	}
	payload.ReturnTo = cleanReturnTo(payload.ReturnTo)
	payload.Attribution = auth.NormalizeRegistrationAttribution(payload.Attribution)
	payload.InviteCode = promotionreward.CanonicalReferralCode(payload.InviteCode)
	return payload, true
}

func cleanReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/"
	}
	return value
}

func appendAuthOutcome(returnTo, outcome string) string {
	path := cleanReturnTo(returnTo)
	parsed, err := url.Parse(path)
	if err != nil {
		parsed = &url.URL{Path: "/"}
	}
	query := parsed.Query()
	query.Set("authOutcome", outcome)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func appendAccountAppealOutcome(outcome string) string {
	parsed := &url.URL{Path: accountAppealFrontendPath}
	query := parsed.Query()
	query.Set("accountAppealOutcome", outcome)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (s *Server) oauthRedirectTarget(returnTo string) string {
	path := cleanReturnTo(returnTo)
	if s.frontendOrigin == "" {
		return path
	}
	return strings.TrimRight(s.frontendOrigin, "/") + path
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
