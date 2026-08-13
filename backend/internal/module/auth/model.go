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

	SessionAudienceNormal             = "normal"
	SessionAudienceRestrictedBusiness = "restricted_business"
	SessionAudienceAccountAppeal      = "account_appeal"

	GovernanceActionSuspend          = "suspend"
	GovernanceActionExtendSuspension = "extend_suspension"
	GovernanceActionBan              = "ban"
	GovernanceActionRestore          = "restore"
	GovernanceReasonManual           = "MANUAL_ACCOUNT_GOVERNANCE"

	AdminReauthenticationPurposeGrantAdmin  = "grant_admin"
	AdminReauthenticationMethodPassword     = "password"
	AdminReauthenticationMethodLinuxDoOAuth = "linux_do_oauth"
	OAuthPurposeGrantAdminReauthentication  = "grant_admin_reauth"

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
	ID                        string
	AnalyticsUserID           string
	Username                  string
	DisplayName               string
	IsAdmin                   bool
	Status                    string
	LinuxDoBinding            *LinuxDoBinding
	StudentClaim              *StudentEmailClaim
	Capabilities              []string
	GovernanceVersion         int64
	CurrentGovernanceActionID string
	SecurityLockedAt          *time.Time
}

type AuthenticationResult struct {
	User              User
	Audience          string
	Session           Session
	RestrictedSession RestrictedBusinessSession
}

type RestrictedBusinessSession struct {
	ID                     string
	UserID                 string
	CSRFToken              string
	GovernanceActionID     string
	GovernanceVersion      int64
	RestrictionEffectiveAt time.Time
	CreatedAt              time.Time
	ExpiresAt              time.Time
	RevokedAt              *time.Time
	LastSeenAt             time.Time
}

type BusinessActor struct {
	UserID                 string
	Username               string
	DisplayName            string
	Audience               string
	AccountStatus          string
	Capabilities           []string
	GovernanceActionID     string
	GovernanceVersion      int64
	RestrictionEffectiveAt time.Time
}

type AdminUser struct {
	ID                        string
	Username                  string
	DisplayName               string
	IsAdmin                   bool
	Status                    string
	LinuxDoBound              bool
	TrustLevel                *int
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	LastActiveAt              *time.Time
	Version                   int64
	GovernanceVersion         int64
	CurrentGovernanceActionID string
	SecurityLockedAt          *time.Time
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

type AdminAuditLogFilter struct {
	Search      string
	Action      string
	TargetType  string
	ActorUserID string
	TargetID    string
}

type AdminAuditLog struct {
	ID            string
	ActorUserID   string
	ActorUsername string
	Action        string
	TargetType    string
	TargetID      string
	Reason        string
	RequestID     string
	BeforeStatus  *string
	AfterStatus   *string
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
	ReasonCode      string
	PublicReason    string
	InternalNote    string
	ExpiresAt       *time.Time
	IsIndefinite    bool
	LinkedCaseType  string
	LinkedCaseID    string
	RequestID       string
}

type AdminUserPermissionInput struct {
	TargetUserID          string
	AdminUserID           string
	Grant                 bool
	ExpectedVersion       int64
	Reason                string
	AdminSessionTokenHash string
	RequestID             string
}

type AccountGovernanceAction struct {
	ID                 string
	TargetUserID       string
	ActionType         string
	Status             string
	GovernanceVersion  int64
	ReasonCode         string
	PublicReason       string
	InternalNote       string
	LinkedCaseType     string
	LinkedCaseID       string
	EffectiveAt        time.Time
	ExpiresAt          *time.Time
	IsIndefinite       bool
	SupersedesActionID string
	SupersededAt       *time.Time
	ActorUserID        string
	RequestID          string
	CreatedAt          time.Time
}

type AdminReauthenticationGrant struct {
	ID            string
	AdminUserID   string
	AuthSessionID string
	Purpose       string
	Method        string
	VerifiedAt    time.Time
	ExpiresAt     time.Time
	ConsumedAt    *time.Time
	RevokedAt     *time.Time
}

func IsRestrictedBusinessAccountStatus(status string) bool {
	return status == AccountStatusSuspended || status == AccountStatusBanned
}

type AdminUserMutationResult struct {
	Detail AdminUserDetail
}

type AdminUserCompletionBuilder func(AdminUserMutationResult) (idempotency.Completion, *domain.AppError)

type Session struct {
	ID                        string
	UserID                    string
	CSRFToken                 string
	ExpiresAt                 time.Time
	RenewedAt                 time.Time
	AbsoluteExpiresAt         time.Time
	RevokedAt                 *time.Time
	NewRegistration           bool
	PasswordReauthenticatedAt *time.Time
	OAuthLinkStateHash        string
	OAuthLinkStatePurpose     string
	OAuthLinkStateExpiresAt   *time.Time
	OAuthLinkStateConsumedAt  *time.Time
}

type AccountAppealSession struct {
	ID        string
	UserID    string
	CSRFToken string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
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

type StudentEmailClaim struct {
	ID                  string
	UserID              string
	NormalizedEmail     string
	InstitutionDomainID string
	InstitutionDomain   string
	InstitutionName     string
	ClaimedAt           time.Time
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
	Attribution      RegistrationAttribution
	ReferralCode     string
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
	Email       string
	Code        string
	Username    string
	Password    string
	Attribution RegistrationAttribution
}

type StudentRegistrationConfig struct {
	Enabled      bool
	Version      int64
	Institutions []StudentInstitutionDomain
}

type StudentInstitutionDomain struct {
	ID              string
	Domain          string
	InstitutionName string
	Enabled         bool
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type StudentRegistrationSettingUpdate struct {
	Enabled         bool
	ExpectedVersion int64
	AdminUserID     string
	Reason          string
	RequestID       string
}

type StudentInstitutionDomainCreateInput struct {
	Domain          string
	InstitutionName string
	Enabled         bool
	AdminUserID     string
	Reason          string
	RequestID       string
}

type StudentInstitutionDomainUpdateInput struct {
	ID              string
	InstitutionName string
	Enabled         bool
	ExpectedVersion int64
	AdminUserID     string
	Reason          string
	RequestID       string
}

type StudentRegistrationCompletionBuilder func(StudentRegistrationConfig) (idempotency.Completion, *domain.AppError)

type StudentInstitutionDomainCompletionBuilder func(StudentInstitutionDomain) (idempotency.Completion, *domain.AppError)

type OAuthLinkResult struct {
	User    User
	Session Session
}

type RegistrationAttribution struct {
	SourceType   string
	Source       string
	Medium       string
	Campaign     string
	ReferrerHost string
	LandingPath  string
}
