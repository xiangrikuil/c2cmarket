package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/module/profile"

	"github.com/go-chi/chi/v5"
)

type privacySettingsDTO struct {
	ShowCreatedAt               bool `json:"showCreatedAt"`
	ShowLastActiveAt            bool `json:"showLastActiveAt"`
	ShowCompletedCarpoolCount   bool `json:"showCompletedCarpoolCount"`
	ShowCompletedAPIIntentCount bool `json:"showCompletedApiIntentCount"`
	ShowResponseMedian          bool `json:"showResponseMedian"`
	ShowResolvedDisputeSummary  bool `json:"showResolvedDisputeSummary"`
	AllowPublicProfileReport    bool `json:"allowPublicProfileReport"`
}

type myProfileResponse struct {
	ID                   string                  `json:"id"`
	Username             string                  `json:"username"`
	DisplayName          string                  `json:"displayName"`
	Bio                  *string                 `json:"bio"`
	AvatarURL            *string                 `json:"avatarUrl"`
	CustomAvatarURL      *string                 `json:"customAvatarUrl"`
	Email                *string                 `json:"email"`
	EmailVerified        bool                    `json:"emailVerified"`
	EmailVerifiedAt      *string                 `json:"emailVerifiedAt"`
	PasswordConfigured   bool                    `json:"passwordConfigured"`
	RegionCode           *string                 `json:"regionCode"`
	Timezone             *string                 `json:"timezone"`
	AvatarMode           string                  `json:"avatarMode"`
	AccountStatus        string                  `json:"accountStatus"`
	Permissions          []string                `json:"permissions"`
	LinuxDoBinding       linuxDoBindingDTO       `json:"linuxDoBinding"`
	Badges               []string                `json:"badges"`
	Restrictions         []string                `json:"restrictions"`
	UsernameChangePolicy usernameChangePolicyDTO `json:"usernameChangePolicy"`
	Privacy              privacySettingsDTO      `json:"privacy"`
	CreatedAt            string                  `json:"createdAt"`
	UpdatedAt            string                  `json:"updatedAt"`
	LastActiveAt         *string                 `json:"lastActiveAt"`
	Version              int64                   `json:"version"`
}

type linuxDoBindingDTO struct {
	Bound            bool    `json:"bound"`
	LinuxDoUserID    *string `json:"linuxDoUserId"`
	LinuxDoUsername  *string `json:"linuxDoUsername"`
	LinuxDoAvatarURL *string `json:"linuxDoAvatarUrl"`
	TrustLevel       *int    `json:"trustLevel"`
	LastSyncedAt     *string `json:"lastSyncedAt"`
}

type usernameChangePolicyDTO struct {
	CanChange       bool    `json:"canChange"`
	NextAvailableAt *string `json:"nextAvailableAt"`
}

type updateMyProfileRequest struct {
	DisplayName string             `json:"displayName"`
	Username    string             `json:"username"`
	Bio         string             `json:"bio"`
	RegionCode  string             `json:"regionCode"`
	Timezone    string             `json:"timezone"`
	AvatarMode  string             `json:"avatarMode"`
	AvatarURL   string             `json:"avatarUrl"`
	Privacy     privacySettingsDTO `json:"privacy"`
}

type startEmailVerificationRequest struct {
	Email string `json:"email"`
}

type startEmailVerificationResponse struct {
	Email     string `json:"email"`
	ExpiresAt string `json:"expiresAt"`
	DevCode   string `json:"devCode,omitempty"`
}

type confirmEmailVerificationRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type publicUserProfileResponse struct {
	Profile     publicUserProfileDTO    `json:"profile"`
	Carpools    []any                   `json:"carpools"`
	Services    []any                   `json:"services"`
	Completions []any                   `json:"completions"`
	Reviews     []any                   `json:"reviews"`
	Disputes    []publicDisputeResponse `json:"disputes"`
}

type publicUserProfileDTO struct {
	ID              string             `json:"id"`
	Username        string             `json:"username"`
	DisplayName     string             `json:"displayName"`
	Bio             *string            `json:"bio"`
	AvatarURL       *string            `json:"avatarUrl"`
	AvatarText      string             `json:"avatarText"`
	LinuxDoBound    bool               `json:"linuxDoBound"`
	LinuxDoUsername *string            `json:"linuxDoUsername"`
	TrustLevel      *int               `json:"trustLevel"`
	Badges          []string           `json:"badges"`
	AccountStatus   string             `json:"accountStatus"`
	CreatedAt       *string            `json:"createdAt"`
	LastActiveAt    *string            `json:"lastActiveAt"`
	Stats           publicStatsDTO     `json:"stats"`
	Privacy         privacySettingsDTO `json:"privacy"`
}

type publicStatsDTO struct {
	CompletedCarpoolsLast30Days           *int `json:"completedCarpoolsLast30Days"`
	CompletedAPIOrdersLast30Days          *int `json:"completedApiOrdersLast30Days"`
	ResponseMedianMinutes                 *int `json:"responseMedianMinutes"`
	BuyerResponsibilityCancellationCount  int  `json:"buyerResponsibilityCancellationCount"`
	SellerResponsibilityCancellationCount int  `json:"sellerResponsibilityCancellationCount"`
	UnresolvedDisputeCount                int  `json:"unresolvedDisputeCount"`
	ResolvedDisputeCountLast90Days        *int `json:"resolvedDisputeCountLast90Days"`
}

type merchantProfileRequest struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
}

type merchantProfileResponse struct {
	ID          string  `json:"id"`
	OwnerUserID string  `json:"ownerUserId,omitempty"`
	Slug        string  `json:"slug"`
	DisplayName string  `json:"displayName"`
	AvatarURL   *string `json:"avatarUrl"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
	Version     int64   `json:"version"`
}

type publicMerchantProfileResponse struct {
	Profile     publicMerchantProfileDTO `json:"profile"`
	Services    []any                    `json:"services"`
	Completions []any                    `json:"completions"`
	Reviews     []any                    `json:"reviews"`
	Disputes    []any                    `json:"disputes"`
}

type publicMerchantProfileDTO struct {
	Username                         string `json:"username"`
	DisplayName                      string `json:"displayName"`
	AvatarText                       string `json:"avatarText"`
	MerchantID                       string `json:"merchantId"`
	Identity                         string `json:"identity"`
	TrustLevel                       int    `json:"trustLevel"`
	LinuxDoBound                     bool   `json:"linuxdoBound"`
	OriginalPostBound                bool   `json:"originalPostBound"`
	JoinedAt                         string `json:"joinedAt"`
	LastActiveAt                     string `json:"lastActiveAt"`
	LinuxDoURL                       string `json:"linuxdoUrl"`
	Completed30d                     int    `json:"completed30d"`
	ResponseMedianMinutes            int    `json:"responseMedianMinutes"`
	MerchantResponsibleCancellations int    `json:"merchantResponsibleCancellations"`
	UnresolvedDisputes               int    `json:"unresolvedDisputes"`
	HandledDisputes90d               int    `json:"handledDisputes90d"`
}

func (s *Server) handleMyProfile(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	profile, appErr := s.app.MyProfile(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toMyProfileResponse(profile))
}

func (s *Server) handleUpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[updateMyProfileRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	updated, appErr := s.app.UpdateMyProfile(r.Context(), user, profile.UpdateUserProfileInput{
		DisplayName: req.DisplayName,
		Username:    req.Username,
		Bio:         req.Bio,
		RegionCode:  req.RegionCode,
		Timezone:    req.Timezone,
		AvatarMode:  req.AvatarMode,
		AvatarURL:   req.AvatarURL,
		Privacy:     fromPrivacyDTO(req.Privacy),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toMyProfileResponse(updated))
}

func (s *Server) handleStartEmailVerification(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[startEmailVerificationRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	challenge, appErr := s.app.StartEmailVerification(r.Context(), user, profile.EmailVerificationStartInput{Email: req.Email})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, startEmailVerificationResponse{
		Email:     challenge.Email,
		ExpiresAt: challenge.ExpiresAt.UTC().Format(time.RFC3339),
		DevCode:   challenge.DevCode,
	})
}

func (s *Server) handleConfirmEmailVerification(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[confirmEmailVerificationRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	updated, appErr := s.app.ConfirmEmailVerification(r.Context(), user, profile.EmailVerificationConfirmInput{
		Email: req.Email,
		Code:  req.Code,
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toMyProfileResponse(updated))
}

func (s *Server) handlePublicUserProfile(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	publicProfile, appErr := s.app.PublicUserProfile(r.Context(), username)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	disputes, appErr := s.app.PublicUserDisputes(r.Context(), username)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, publicUserProfileResponse{
		Profile:     toPublicUserProfileDTO(publicProfile),
		Carpools:    []any{},
		Services:    []any{},
		Completions: []any{},
		Reviews:     []any{},
		Disputes:    toPublicDisputeResponses(disputes),
	})
}

func (s *Server) handleMyMerchantProfile(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	merchant, appErr := s.app.MyMerchantProfile(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toMerchantProfileResponse(merchant, true))
}

func (s *Server) handleUpsertMyMerchantProfile(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[merchantProfileRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	merchant, appErr := s.app.UpsertMyMerchantProfile(r.Context(), user, profile.UpsertMerchantProfileInput{
		Slug:        req.Slug,
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toMerchantProfileResponse(merchant, true))
}

func (s *Server) handlePublicMerchantProfile(w http.ResponseWriter, r *http.Request) {
	merchant, appErr := s.app.PublicMerchantProfile(r.Context(), chi.URLParam(r, "slug"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, publicMerchantProfileResponse{
		Profile:     toPublicMerchantProfileDTO(merchant),
		Services:    []any{},
		Completions: []any{},
		Reviews:     []any{},
		Disputes:    []any{},
	})
}

func toMyProfileResponse(value profile.UserProfile) myProfileResponse {
	lastActive := formatOptionalTime(value.LastActiveAt)
	return myProfileResponse{
		ID:                 value.ID,
		Username:           value.Username,
		DisplayName:        value.DisplayName,
		Bio:                stringPtrOrNil(value.Bio),
		AvatarURL:          stringPtrOrNil(value.AvatarURL),
		CustomAvatarURL:    stringPtrOrNil(value.CustomAvatarURL),
		Email:              stringPtrOrNil(value.Email),
		EmailVerified:      value.EmailVerifiedAt != nil,
		EmailVerifiedAt:    formatOptionalTime(value.EmailVerifiedAt),
		PasswordConfigured: value.PasswordConfigured,
		RegionCode:         stringPtrOrNil(value.RegionCode),
		Timezone:           stringPtrOrNil(value.Timezone),
		AvatarMode:         value.AvatarMode,
		AccountStatus:      normalizeAccountStatus(value.AccountStatus),
		Permissions:        permissionsFor(value.IsAdmin),
		LinuxDoBinding: linuxDoBindingDTO{
			Bound:            value.LinuxDoBound,
			LinuxDoUserID:    stringPtrOrNil(value.LinuxDoUserID),
			LinuxDoUsername:  stringPtrOrNil(value.LinuxDoUsername),
			LinuxDoAvatarURL: stringPtrOrNil(value.LinuxDoAvatarURL),
			TrustLevel:       value.LinuxDoTrustLevel,
			LastSyncedAt:     formatOptionalTime(value.LinuxDoLastSyncedAt),
		},
		Badges:       badgesFor(value),
		Restrictions: value.Restrictions,
		UsernameChangePolicy: usernameChangePolicyDTO{
			CanChange:       value.UsernameCanChange,
			NextAvailableAt: formatOptionalTime(value.UsernameNextChangeAt),
		},
		Privacy:      toPrivacyDTO(value.Privacy),
		CreatedAt:    value.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    value.UpdatedAt.UTC().Format(time.RFC3339),
		LastActiveAt: lastActive,
		Version:      value.Version,
	}
}

func toPublicUserProfileDTO(value profile.PublicUserProfile) publicUserProfileDTO {
	return publicUserProfileDTO{
		ID:              value.ID,
		Username:        value.Username,
		DisplayName:     value.DisplayName,
		Bio:             stringPtrOrNil(value.Bio),
		AvatarURL:       stringPtrOrNil(value.AvatarURL),
		AvatarText:      value.AvatarText,
		LinuxDoBound:    value.LinuxDoBound,
		LinuxDoUsername: stringPtrOrNil(value.LinuxDoUsername),
		TrustLevel:      value.TrustLevel,
		Badges:          value.Badges,
		AccountStatus:   normalizeAccountStatus(value.AccountStatus),
		CreatedAt:       formatOptionalTime(value.CreatedAt),
		LastActiveAt:    formatOptionalTime(value.LastActiveAt),
		Stats: publicStatsDTO{
			CompletedCarpoolsLast30Days:           value.Stats.CompletedCarpoolsLast30Days,
			CompletedAPIOrdersLast30Days:          value.Stats.CompletedAPIIntentsLast30Days,
			ResponseMedianMinutes:                 value.Stats.ResponseMedianMinutes,
			BuyerResponsibilityCancellationCount:  value.Stats.BuyerResponsibilityCancellationCount,
			SellerResponsibilityCancellationCount: value.Stats.SellerResponsibilityCancellationCount,
			UnresolvedDisputeCount:                value.Stats.UnresolvedDisputeCount,
			ResolvedDisputeCountLast90Days:        value.Stats.ResolvedDisputeCountLast90Days,
		},
		Privacy: toPrivacyDTO(value.Privacy),
	}
}

func toMerchantProfileResponse(value profile.MerchantProfile, includeOwner bool) merchantProfileResponse {
	ownerID := ""
	if includeOwner {
		ownerID = value.OwnerUserID
	}
	return merchantProfileResponse{
		ID:          value.ID,
		OwnerUserID: ownerID,
		Slug:        value.Slug,
		DisplayName: value.DisplayName,
		AvatarURL:   stringPtrOrNil(value.AvatarURL),
		Status:      value.Status,
		CreatedAt:   value.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   value.UpdatedAt.UTC().Format(time.RFC3339),
		Version:     value.Version,
	}
}

func toPublicMerchantProfileDTO(value profile.PublicMerchantProfile) publicMerchantProfileDTO {
	lastActive := ""
	if value.LastActiveAt != nil {
		lastActive = value.LastActiveAt.UTC().Format(time.RFC3339)
	}
	return publicMerchantProfileDTO{
		Username:                         value.Slug,
		DisplayName:                      value.DisplayName,
		AvatarText:                       value.AvatarText,
		MerchantID:                       value.ID,
		Identity:                         value.Identity,
		TrustLevel:                       value.TrustLevel,
		LinuxDoBound:                     value.LinuxDoBound,
		OriginalPostBound:                value.OriginalPostBound,
		JoinedAt:                         value.JoinedAt.UTC().Format(time.RFC3339),
		LastActiveAt:                     lastActive,
		LinuxDoURL:                       "",
		Completed30d:                     value.Completed30d,
		ResponseMedianMinutes:            value.ResponseMedianMinutes,
		MerchantResponsibleCancellations: value.MerchantResponsibleCancellations,
		UnresolvedDisputes:               value.UnresolvedDisputes,
		HandledDisputes90d:               value.HandledDisputes90d,
	}
}

func toPrivacyDTO(value profile.PrivacySettings) privacySettingsDTO {
	return privacySettingsDTO{
		ShowCreatedAt:               value.ShowCreatedAt,
		ShowLastActiveAt:            value.ShowLastActiveAt,
		ShowCompletedCarpoolCount:   value.ShowCompletedCarpoolCount,
		ShowCompletedAPIIntentCount: value.ShowCompletedAPIIntentCount,
		ShowResponseMedian:          value.ShowResponseMedian,
		ShowResolvedDisputeSummary:  value.ShowResolvedDisputeSummary,
		AllowPublicProfileReport:    value.AllowPublicProfileReport,
	}
}

func fromPrivacyDTO(value privacySettingsDTO) profile.PrivacySettings {
	return profile.PrivacySettings{
		ShowCreatedAt:               value.ShowCreatedAt,
		ShowLastActiveAt:            value.ShowLastActiveAt,
		ShowCompletedCarpoolCount:   value.ShowCompletedCarpoolCount,
		ShowCompletedAPIIntentCount: value.ShowCompletedAPIIntentCount,
		ShowResponseMedian:          value.ShowResponseMedian,
		ShowResolvedDisputeSummary:  value.ShowResolvedDisputeSummary,
		AllowPublicProfileReport:    value.AllowPublicProfileReport,
	}
}

func permissionsFor(isAdmin bool) []string {
	if isAdmin {
		return []string{"admin"}
	}
	return []string{}
}

func badgesFor(value profile.UserProfile) []string {
	badges := []string{}
	if value.LinuxDoBound {
		badges = append(badges, "linuxdo_bound")
	}
	if value.IsAdmin {
		badges = append(badges, "admin")
	}
	return badges
}

func normalizeAccountStatus(value string) string {
	switch value {
	case "suspended", "banned":
		return "restricted"
	default:
		return "normal"
	}
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
