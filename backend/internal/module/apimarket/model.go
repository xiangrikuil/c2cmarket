package apimarket

import "time"

const (
	ServiceReviewStatusDraft            = "draft"
	ServiceReviewStatusPendingReview    = "pending_review"
	ServiceReviewStatusChangesRequested = "changes_requested"
	ServiceReviewStatusApproved         = "approved"
	ServiceReviewStatusRejected         = "rejected"

	ServicePublicationStatusOffline     = "offline"
	ServicePublicationStatusOnline      = "online"
	ServicePublicationStatusOwnerPaused = "owner_paused"
	ServicePublicationStatusArchived    = "archived"

	ServiceModerationStatusClear          = "clear"
	ServiceModerationStatusAdminSuspended = "admin_suspended"
	ServiceModerationStatusRemoved        = "removed"

	ServiceDistributionSub2API     = "sub2api"
	ServiceBillingModeMetered      = "metered_usd_quota"
	ServiceBillingModeManual       = "manual_usage_check"
	ServiceBillingModeFixedPackage = "fixed_package"

	PaymentMethodWechat = "wechat"
	PaymentMethodAlipay = "alipay"
	PaymentMethodUSDT   = "usdt"
)

type Service struct {
	ID                               string
	OwnerUserID                      string
	MerchantProfileID                string
	MerchantIdentityMode             string
	MerchantDisplayName              string
	MerchantProfileSlug              string
	OwnerContactMethodID             string
	Title                            string
	ShortDescription                 string
	DistributionSystem               string
	BillingMode                      string
	DeclaredCNYPerUSDAllowance       string
	DeclaredMaxUSDAllowancePerIntent string
	MinimumIntentCNY                 string
	MaximumIntentCNY                 string
	UsageVisibility                  string
	PublicAccessNote                 string
	MerchantNote                     string
	MerchantSupportNote              string
	AcceptingOrders                  bool
	PaymentWindowMinutes             int
	ReviewStatus                     string
	PublicationStatus                string
	ModerationStatus                 string
	ApprovedByAdminID                string
	ApprovedAt                       *time.Time
	ModerationReason                 string
	AccessModes                      []ServiceAccessMode
	Models                           []ServiceModel
	Packages                         []ServicePackage
	PaymentOptions                   []PaymentOption
	IsOrderable                      bool
	OrderableReasons                 []string
	CreatedAt                        time.Time
	UpdatedAt                        time.Time
	Version                          int64
}

type ServiceAccessMode struct {
	APIServiceID string
	AccessMode   string
	PublicNote   string
}

type ServiceModel struct {
	ID                                  string
	APIServiceID                        string
	DistributionSystem                  string
	ModelCatalogID                      string
	ModelPriceVersionID                 string
	ModelNameSnapshot                   string
	ProviderSnapshot                    string
	CapabilitiesSnapshot                []string
	MerchantMultiplier                  string
	EffectiveInputPricePerMillion       string
	EffectiveCachedInputPricePerMillion string
	EffectiveOutputPricePerMillion      string
	Enabled                             bool
	CreatedAt                           time.Time
	UpdatedAt                           time.Time
}

type ServicePackage struct {
	ID           string
	APIServiceID string
	Name         string
	PriceCNY     string
	DurationDays *int
	Description  string
	Enabled      bool
	SortOrder    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PaymentOption struct {
	ID                  string
	APIServiceID        string
	PaymentMethod       string
	Enabled             bool
	PaymentInstructions string
	Version             int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateServiceInput struct {
	OwnerUserID                      string
	MerchantProfileID                string
	MerchantIdentityMode             string
	OwnerContactMethodID             string
	Title                            string
	ShortDescription                 string
	DistributionSystem               string
	BillingMode                      string
	DeclaredCNYPerUSDAllowance       string
	DeclaredMaxUSDAllowancePerIntent string
	MinimumIntentCNY                 string
	MaximumIntentCNY                 string
	UsageVisibility                  string
	PublicAccessNote                 string
	MerchantNote                     string
	MerchantSupportNote              string
	AccessModes                      []ServiceAccessModeInput
	Models                           []ServiceModelInput
	Packages                         []ServicePackageInput
}

type UpdateServiceInput struct {
	ServiceID                        string
	OwnerUserID                      string
	MerchantProfileID                string
	MerchantIdentityMode             string
	OwnerContactMethodID             string
	Title                            string
	ShortDescription                 string
	DistributionSystem               string
	BillingMode                      string
	DeclaredCNYPerUSDAllowance       string
	DeclaredMaxUSDAllowancePerIntent string
	MinimumIntentCNY                 string
	MaximumIntentCNY                 string
	UsageVisibility                  string
	PublicAccessNote                 string
	MerchantNote                     string
	MerchantSupportNote              string
	AccessModes                      []ServiceAccessModeInput
	Models                           []ServiceModelInput
	Packages                         []ServicePackageInput
	ExpectedVersion                  int64
	RequestID                        string
}

type ServiceAccessModeInput struct {
	AccessMode string
	PublicNote string
}

type ServiceModelInput struct {
	ModelCatalogID      string
	ModelPriceVersionID string
	MerchantMultiplier  string
	Enabled             bool
}

type ServicePackageInput struct {
	Name         string
	PriceCNY     string
	DurationDays *int
	Description  string
	Enabled      bool
	SortOrder    int
}

type ServiceOwnerActionInput struct {
	ServiceID       string
	OwnerUserID     string
	ExpectedVersion int64
	RequestID       string
}

type ServiceAdminActionInput struct {
	ServiceID       string
	AdminUserID     string
	Action          string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type PublicServiceFilter struct {
	PaymentMethod string
}

type UpdateOrderSettingsInput struct {
	ServiceID            string
	OwnerUserID          string
	AcceptingOrders      bool
	PaymentWindowMinutes int
	PaymentOptions       []PaymentOptionInput
	ExpectedVersion      int64
	RequestID            string
}

type PaymentOptionInput struct {
	PaymentMethod       string
	Enabled             bool
	PaymentInstructions string
}
