package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type adminUserDirectoryResponse struct {
	Items      []adminUserResponse       `json:"items"`
	Pagination adminUserPaginationDTO    `json:"pagination"`
	Summary    adminUserDirectorySummary `json:"summary"`
}

type adminUserPaginationDTO struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

type adminUserDirectorySummary struct {
	TotalUsers        int `json:"totalUsers"`
	AdminUsers        int `json:"adminUsers"`
	LinuxDoBoundUsers int `json:"linuxDoBoundUsers"`
	ActiveUsers       int `json:"activeUsers"`
	SuspendedUsers    int `json:"suspendedUsers"`
	BannedUsers       int `json:"bannedUsers"`
	ArchivedUsers     int `json:"archivedUsers"`
}

type adminUserDetailResponse struct {
	User                     adminUserResponse               `json:"user"`
	UpdatedAt                string                          `json:"updatedAt"`
	LinuxDoBinding           adminLinuxDoBindingDTO          `json:"linuxDoBinding"`
	EmailVerified            bool                            `json:"emailVerified"`
	BackupPasswordConfigured bool                            `json:"backupPasswordConfigured"`
	Providers                []adminAuthProviderDTO          `json:"providers"`
	Sessions                 adminUserSessionSummaryDTO      `json:"sessions"`
	RecentAuditEntries       []adminAccountAuditEntryDTO     `json:"recentAuditEntries"`
	AvailableActions         []adminUserGovernanceActionDTO  `json:"availableActions"`
	ImpactPreview            adminUserGovernanceImpactDTO    `json:"impactPreview"`
	AccountCapabilities      adminUserAccountCapabilitiesDTO `json:"accountCapabilities"`
}

type adminUserGovernanceActionDTO struct {
	Action               string `json:"action"`
	Kind                 string `json:"kind"`
	TargetStatus         string `json:"targetStatus,omitempty"`
	TargetIsAdmin        *bool  `json:"targetIsAdmin,omitempty"`
	Allowed              bool   `json:"allowed"`
	Severity             string `json:"severity"`
	RequiresReason       bool   `json:"requiresReason"`
	RequiresConfirmation bool   `json:"requiresConfirmation"`
	BlockedCode          string `json:"blockedCode,omitempty"`
	BlockedReason        string `json:"blockedReason,omitempty"`
}

type adminUserGovernanceImpactDTO struct {
	ActiveSessions          int `json:"activeSessions"`
	ActiveCarpoolListings   int `json:"activeCarpoolListings"`
	OnlineAPIServices       int `json:"onlineApiServices"`
	OpenCarpoolApplications int `json:"openCarpoolApplications"`
	OpenAPIOrders           int `json:"openApiOrders"`
	OpenDisputes            int `json:"openDisputes"`
}

type adminUserAccountCapabilitiesDTO struct {
	CanLogin                        bool `json:"canLogin"`
	PubliclyVisible                 bool `json:"publiclyVisible"`
	CanPublish                      bool `json:"canPublish"`
	CanCreateOrders                 bool `json:"canCreateOrders"`
	CanRevealContact                bool `json:"canRevealContact"`
	CanAccessHistoricalTransactions bool `json:"canAccessHistoricalTransactions"`
}

type adminLinuxDoBindingDTO struct {
	Bound        bool    `json:"bound"`
	Username     *string `json:"username,omitempty"`
	TrustLevel   *int    `json:"trustLevel,omitempty"`
	BoundAt      *string `json:"boundAt,omitempty"`
	LastSyncedAt *string `json:"lastSyncedAt,omitempty"`
}

type adminAuthProviderDTO struct {
	Provider    string  `json:"provider"`
	CreatedAt   string  `json:"createdAt"`
	LastLoginAt *string `json:"lastLoginAt,omitempty"`
}

type adminUserSessionSummaryDTO struct {
	ActiveCount      int     `json:"activeCount"`
	LatestActivityAt *string `json:"latestActivityAt,omitempty"`
}

type adminAccountAuditEntryDTO struct {
	ID            string `json:"id"`
	AdminUserID   string `json:"adminUserId"`
	AdminUsername string `json:"adminUsername"`
	Action        string `json:"action"`
	Reason        string `json:"reason"`
	BeforeStatus  string `json:"beforeStatus,omitempty"`
	AfterStatus   string `json:"afterStatus,omitempty"`
	BeforeIsAdmin *bool  `json:"beforeIsAdmin,omitempty"`
	AfterIsAdmin  *bool  `json:"afterIsAdmin,omitempty"`
	RequestID     string `json:"requestId"`
	CreatedAt     string `json:"createdAt"`
}

type adminUserStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type adminUserPermissionRequest struct {
	IsAdmin *bool  `json:"isAdmin"`
	Reason  string `json:"reason"`
}

func parseAdminUserDirectoryQuery(r *http.Request) (auth.AdminUserDirectoryQuery, *domain.AppError) {
	page, appErr := parseAdminUserQueryInteger(r, "page")
	if appErr != nil {
		return auth.AdminUserDirectoryQuery{}, appErr
	}
	limit, appErr := parseAdminUserQueryInteger(r, "limit")
	if appErr != nil {
		return auth.AdminUserDirectoryQuery{}, appErr
	}
	values := r.URL.Query()
	return auth.AdminUserDirectoryQuery{
		Page:    page,
		Limit:   limit,
		Search:  values.Get("search"),
		Status:  values.Get("status"),
		Role:    values.Get("role"),
		LinuxDo: values.Get("linuxDo"),
		Sort:    values.Get("sort"),
	}, nil
}

func parseAdminUserQueryInteger(r *http.Request, field string) (int, *domain.AppError) {
	raw := strings.TrimSpace(r.URL.Query().Get(field))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		detail := field + " 必须是整数。"
		return 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid user directory query", detail, field, "invalid", detail)
	}
	return value, nil
}

func (s *Server) handleAdminUser(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	detail, appErr := s.adminUsers.AdminUser(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, detail.User.Version)
	writeJSON(w, http.StatusOK, toAdminUserDetailResponse(detail))
}

func (s *Server) handleUpdateAdminUserStatus(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[adminUserStatusRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	userID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/users/{id}/status:" + userID
	completion, appErr := s.adminUsers.UpdateAdminUserStatusWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		auth.AdminUserStatusInput{
			TargetUserID:    userID,
			Status:          request.Status,
			ExpectedVersion: version,
			Reason:          request.Reason,
			RequestID:       requestIDFrom(r),
		},
		adminUserMutationCompletionBuilder(),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleUpdateAdminUserPermission(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[adminUserPermissionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if request.IsAdmin == nil {
		writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Administrator permission required", "必须明确指定是否授予管理员权限。", "isAdmin", "required", "必须提供 isAdmin。"))
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	userID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/users/{id}/admin-permission:" + userID
	completion, appErr := s.adminUsers.UpdateAdminUserPermissionWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		auth.AdminUserPermissionInput{
			TargetUserID:    userID,
			Grant:           *request.IsAdmin,
			ExpectedVersion: version,
			Reason:          request.Reason,
			RequestID:       requestIDFrom(r),
		},
		adminUserMutationCompletionBuilder(),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func adminUserMutationCompletionBuilder() auth.AdminUserCompletionBuilder {
	return func(result auth.AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toAdminUserDetailResponse(result.Detail))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "账号治理响应编码失败。")
		}
		return idempotency.Completion{
			Status:       http.StatusOK,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "user",
			ResourceID:   result.Detail.User.ID,
			Headers: map[string]string{
				"ETag": `"` + strconv.FormatInt(result.Detail.User.Version, 10) + `"`,
			},
		}, nil
	}
}

func toAdminUserDirectoryResponse(directory auth.AdminUserDirectory) adminUserDirectoryResponse {
	return adminUserDirectoryResponse{
		Items: toAdminUserResponses(directory.Items),
		Pagination: adminUserPaginationDTO{
			Page:       directory.Pagination.Page,
			Limit:      directory.Pagination.Limit,
			TotalItems: directory.Pagination.TotalItems,
			TotalPages: directory.Pagination.TotalPages,
		},
		Summary: adminUserDirectorySummary{
			TotalUsers:        directory.Summary.TotalUsers,
			AdminUsers:        directory.Summary.AdminUsers,
			LinuxDoBoundUsers: directory.Summary.LinuxDoBoundUsers,
			ActiveUsers:       directory.Summary.ActiveUsers,
			SuspendedUsers:    directory.Summary.SuspendedUsers,
			BannedUsers:       directory.Summary.BannedUsers,
			ArchivedUsers:     directory.Summary.ArchivedUsers,
		},
	}
}

func toAdminUserDetailResponse(detail auth.AdminUserDetail) adminUserDetailResponse {
	providers := make([]adminAuthProviderDTO, 0, len(detail.Providers))
	for _, provider := range detail.Providers {
		providers = append(providers, adminAuthProviderDTO{
			Provider:    provider.Provider,
			CreatedAt:   provider.CreatedAt.UTC().Format(timeLayoutRFC3339),
			LastLoginAt: formatOptionalTime(provider.LastLoginAt),
		})
	}
	auditEntries := make([]adminAccountAuditEntryDTO, 0, len(detail.RecentAuditEntries))
	for _, entry := range detail.RecentAuditEntries {
		auditEntries = append(auditEntries, adminAccountAuditEntryDTO{
			ID:            entry.ID,
			AdminUserID:   entry.AdminUserID,
			AdminUsername: entry.AdminUsername,
			Action:        entry.Action,
			Reason:        entry.Reason,
			BeforeStatus:  entry.BeforeStatus,
			AfterStatus:   entry.AfterStatus,
			BeforeIsAdmin: entry.BeforeIsAdmin,
			AfterIsAdmin:  entry.AfterIsAdmin,
			RequestID:     entry.RequestID,
			CreatedAt:     entry.CreatedAt.UTC().Format(timeLayoutRFC3339),
		})
	}
	actions := make([]adminUserGovernanceActionDTO, 0, len(detail.AvailableActions))
	for _, action := range detail.AvailableActions {
		actions = append(actions, adminUserGovernanceActionDTO{
			Action:               action.Action,
			Kind:                 action.Kind,
			TargetStatus:         action.TargetStatus,
			TargetIsAdmin:        action.TargetIsAdmin,
			Allowed:              action.Allowed,
			Severity:             action.Severity,
			RequiresReason:       action.RequiresReason,
			RequiresConfirmation: action.RequiresConfirmation,
			BlockedCode:          action.BlockedCode,
			BlockedReason:        action.BlockedReason,
		})
	}
	linuxDoBinding := adminLinuxDoBindingDTO{Bound: detail.LinuxDoBinding.Bound}
	if detail.LinuxDoBinding.Bound {
		linuxDoBinding.Username = stringPointer(detail.LinuxDoBinding.Username)
		linuxDoBinding.TrustLevel = intPointer(detail.LinuxDoBinding.TrustLevel)
		linuxDoBinding.BoundAt = formatOptionalTime(detail.LinuxDoBinding.BoundAt)
		linuxDoBinding.LastSyncedAt = formatOptionalTime(detail.LinuxDoBinding.LastSyncedAt)
	}
	return adminUserDetailResponse{
		User:                     toAdminUserResponses([]auth.AdminUser{detail.User})[0],
		UpdatedAt:                detail.User.UpdatedAt.UTC().Format(timeLayoutRFC3339),
		LinuxDoBinding:           linuxDoBinding,
		EmailVerified:            detail.EmailVerified,
		BackupPasswordConfigured: detail.BackupPasswordConfigured,
		Providers:                providers,
		Sessions: adminUserSessionSummaryDTO{
			ActiveCount:      detail.ActiveSessionCount,
			LatestActivityAt: formatOptionalTime(detail.LatestSessionActivityAt),
		},
		RecentAuditEntries: auditEntries,
		AvailableActions:   actions,
		ImpactPreview: adminUserGovernanceImpactDTO{
			ActiveSessions:          detail.ImpactPreview.ActiveSessions,
			ActiveCarpoolListings:   detail.ImpactPreview.ActiveCarpoolListings,
			OnlineAPIServices:       detail.ImpactPreview.OnlineAPIServices,
			OpenCarpoolApplications: detail.ImpactPreview.OpenCarpoolApplications,
			OpenAPIOrders:           detail.ImpactPreview.OpenAPIOrders,
			OpenDisputes:            detail.ImpactPreview.OpenDisputes,
		},
		AccountCapabilities: adminUserAccountCapabilitiesDTO{
			CanLogin:                        detail.AccountCapabilities.CanLogin,
			PubliclyVisible:                 detail.AccountCapabilities.PubliclyVisible,
			CanPublish:                      detail.AccountCapabilities.CanPublish,
			CanCreateOrders:                 detail.AccountCapabilities.CanCreateOrders,
			CanRevealContact:                detail.AccountCapabilities.CanRevealContact,
			CanAccessHistoricalTransactions: detail.AccountCapabilities.CanAccessHistoricalTransactions,
		},
	}
}

const timeLayoutRFC3339 = "2006-01-02T15:04:05Z07:00"

func stringPointer(value string) *string {
	return &value
}

func intPointer(value int) *int {
	return &value
}
