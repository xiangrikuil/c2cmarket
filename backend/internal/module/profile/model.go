package profile

import "time"

type PrivacySettings struct {
	ShowCreatedAt               bool
	ShowLastActiveAt            bool
	ShowCompletedCarpoolCount   bool
	ShowCompletedAPIIntentCount bool
	ShowResponseMedian          bool
	ShowResolvedDisputeSummary  bool
	AllowPublicProfileReport    bool
}

type UserProfile struct {
	ID                   string
	Username             string
	DisplayName          string
	Bio                  string
	AvatarURL            string
	CustomAvatarURL      string
	Email                string
	EmailVerifiedAt      *time.Time
	PasswordConfigured   bool
	AccountStatus        string
	IsAdmin              bool
	RegionCode           string
	Timezone             string
	AvatarMode           string
	Privacy              PrivacySettings
	CreatedAt            time.Time
	UpdatedAt            time.Time
	LastActiveAt         *time.Time
	Version              int64
	LinuxDoBound         bool
	LinuxDoUserID        string
	LinuxDoUsername      string
	LinuxDoAvatarURL     string
	LinuxDoTrustLevel    *int
	LinuxDoLastSyncedAt  *time.Time
	Restrictions         []string
	UsernameCanChange    bool
	UsernameNextChangeAt *time.Time
}

type UpdateUserProfileInput struct {
	UserID      string
	DisplayName string
	Username    string
	Bio         string
	RegionCode  string
	Timezone    string
	AvatarMode  string
	AvatarURL   string
	Privacy     PrivacySettings
}

type EmailVerificationStartInput struct {
	UserID string
	Email  string
}

type EmailVerificationConfirmInput struct {
	UserID string
	Email  string
	Code   string
}

type EmailVerificationChallenge struct {
	Email     string
	ExpiresAt time.Time
	DevCode   string
}

type PublicStats struct {
	CompletedCarpoolsLast30Days           *int
	CompletedAPIIntentsLast30Days         *int
	ResponseMedianMinutes                 *int
	BuyerResponsibilityCancellationCount  int
	SellerResponsibilityCancellationCount int
	UnresolvedDisputeCount                int
	ResolvedDisputeCountLast90Days        *int
}

type PublicUserProfile struct {
	ID              string
	Username        string
	DisplayName     string
	Bio             string
	AvatarURL       string
	AvatarText      string
	LinuxDoBound    bool
	LinuxDoUsername string
	TrustLevel      *int
	AccountStatus   string
	CreatedAt       *time.Time
	LastActiveAt    *time.Time
	Privacy         PrivacySettings
	Stats           PublicStats
	Badges          []string
}

type MerchantProfile struct {
	ID          string
	OwnerUserID string
	Slug        string
	DisplayName string
	AvatarURL   string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int64
}

type UpsertMerchantProfileInput struct {
	OwnerUserID string
	Slug        string
	DisplayName string
	AvatarURL   string
}

type PublicMerchantProfile struct {
	ID                               string
	Slug                             string
	DisplayName                      string
	AvatarURL                        string
	AvatarText                       string
	Identity                         string
	TrustLevel                       int
	LinuxDoBound                     bool
	OriginalPostBound                bool
	JoinedAt                         time.Time
	LastActiveAt                     *time.Time
	Completed30d                     int
	ResponseMedianMinutes            int
	MerchantResponsibleCancellations int
	UnresolvedDisputes               int
	HandledDisputes90d               int
}
