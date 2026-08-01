package auth

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	AccountStatusActive    = "active"
	AccountStatusSuspended = "suspended"
	AccountStatusBanned    = "banned"
	AccountStatusArchived  = "archived"

	AdminUserStatusAll      = "all"
	AdminUserRoleAll        = "all"
	AdminUserRoleAdmin      = "admin"
	AdminUserRoleUser       = "user"
	AdminUserLinuxDoAll     = "all"
	AdminUserLinuxDoBound   = "bound"
	AdminUserLinuxDoUnbound = "unbound"

	AdminUserSortCreatedDesc  = "created_desc"
	AdminUserSortCreatedAsc   = "created_asc"
	AdminUserSortActiveDesc   = "active_desc"
	AdminUserSortUsernameAsc  = "username_asc"
	AdminUserSortUsernameDesc = "username_desc"

	AdminUserActionSuspend     = "suspend"
	AdminUserActionBan         = "ban"
	AdminUserActionArchive     = "archive"
	AdminUserActionRestore     = "restore"
	AdminUserActionGrantAdmin  = "grant_admin"
	AdminUserActionRevokeAdmin = "revoke_admin"
)

type User struct {
	ID             string
	Username       string
	DisplayName    string
	IsAdmin        bool
	Status         string
	LinuxDoBinding *LinuxDoBinding
}

type AdminUser struct {
	ID           string
	Username     string
	DisplayName  string
	IsAdmin      bool
	Status       string
	LinuxDoBound bool
	TrustLevel   *int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastActiveAt *time.Time
	Version      int64
}

type AdminUserDirectoryQuery struct {
	Page    int
	Limit   int
	Search  string
	Status  string
	Role    string
	LinuxDo string
	Sort    string
}

type AdminUserPagination struct {
	Page       int
	Limit      int
	TotalItems int
	TotalPages int
}

type AdminUserDirectorySummary struct {
	TotalUsers        int
	AdminUsers        int
	LinuxDoBoundUsers int
	ActiveUsers       int
	SuspendedUsers    int
	BannedUsers       int
	ArchivedUsers     int
}

type AdminUserDirectory struct {
	Items      []AdminUser
	Pagination AdminUserPagination
	Summary    AdminUserDirectorySummary
}

type AdminLinuxDoBinding struct {
	Bound        bool
	Username     string
	TrustLevel   int
	BoundAt      *time.Time
	LastSyncedAt *time.Time
}

type AdminAuthProvider struct {
	Provider    string
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

type AdminAccountAuditEntry struct {
	ID            string
	AdminUserID   string
	AdminUsername string
	Action        string
	Reason        string
	BeforeStatus  string
	AfterStatus   string
	BeforeIsAdmin *bool
	AfterIsAdmin  *bool
	RequestID     string
	CreatedAt     time.Time
}

type AdminUserGovernanceAction struct {
	Action               string
	Kind                 string
	TargetStatus         string
	TargetIsAdmin        *bool
	Allowed              bool
	Severity             string
	RequiresReason       bool
	RequiresConfirmation bool
	BlockedCode          string
	BlockedReason        string
}

type AdminUserGovernanceImpact struct {
	ActiveSessions          int
	ActiveCarpoolListings   int
	OnlineAPIServices       int
	OpenCarpoolApplications int
	OpenAPIOrders           int
	OpenDisputes            int
}

type AdminUserAccountCapabilities struct {
	CanLogin                        bool
	PubliclyVisible                 bool
	CanPublish                      bool
	CanCreateOrders                 bool
	CanRevealContact                bool
	CanAccessHistoricalTransactions bool
}

type AdminUserDetail struct {
	User                     AdminUser
	LinuxDoBinding           AdminLinuxDoBinding
	EmailVerified            bool
	BackupPasswordConfigured bool
	Providers                []AdminAuthProvider
	ActiveSessionCount       int
	LatestSessionActivityAt  *time.Time
	RecentAuditEntries       []AdminAccountAuditEntry
	AvailableActions         []AdminUserGovernanceAction
	ImpactPreview            AdminUserGovernanceImpact
	AccountCapabilities      AdminUserAccountCapabilities
	ActiveAdminCount         int
}

type AdminUserStatusInput struct {
	TargetUserID    string
	AdminUserID     string
	Status          string
	ExpectedVersion int64
	Reason          string
	RequestID       string
}

type AdminUserPermissionInput struct {
	TargetUserID    string
	AdminUserID     string
	Grant           bool
	ExpectedVersion int64
	Reason          string
	RequestID       string
}

type AdminUserMutationResult struct {
	Detail AdminUserDetail
}

type AdminUserCompletionBuilder func(AdminUserMutationResult) (idempotency.Completion, *domain.AppError)

type Session struct {
	ID                string
	UserID            string
	CSRFToken         string
	ExpiresAt         time.Time
	RenewedAt         time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
}

type LinuxDoBinding struct {
	Bound           bool
	LinuxDoUserID   string
	LinuxDoUsername string
	TrustLevel      int
	AvatarURL       string
	BoundAt         time.Time
	LastSyncedAt    time.Time
}

type OAuthProfile struct {
	Provider         string
	Subject          string
	Username         string
	DisplayName      string
	Email            string
	AvatarURL        string
	TrustLevel       int
	LinuxDoUserID    string
	LinuxDoUsername  string
	LinuxDoAvatarURL string
}

type PasswordCredential struct {
	User      User
	Algorithm string
	Salt      string
	Hash      string
}

type BootstrapAdminInput struct {
	Username string
	Password string
}

type BootstrapAdminResult struct {
	User    User
	Created bool
}

type OAuthUserResult struct {
	User    User
	Created bool
}

type SetPasswordInput struct {
	UserID          string
	CurrentPassword string
	NewPassword     string
}

type EmailRegistrationStartInput struct {
	Email string
}

type EmailRegistrationChallenge struct {
	Email     string
	ExpiresAt time.Time
	DevCode   string
}

type EmailRegistrationConfirmInput struct {
	Email              string
	Code               string
	UsernameCandidates []string
}
