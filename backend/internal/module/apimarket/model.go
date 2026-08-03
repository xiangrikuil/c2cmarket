package apimarket

import (
	"time"

	"c2c-market/backend/internal/module/reputation"
)

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

	AccountPoolGPTPro20x = "gpt_pro_20x"
	AccountPoolGPTPro5x  = "gpt_pro_5x"
	AccountPoolGPTPlus   = "gpt_plus"
	AccountPoolCustom    = "custom"

	MerchantRefundPolicyVersion = "api-merchant-refund-v1"

	PaymentMethodWechat = "wechat"
	PaymentMethodAlipay = "alipay"

	DefaultPaymentWindowMinutes = 10

	OwnerSalesViewActive  = "active"
	OwnerSalesViewExpired = "expired"
	OwnerSalesViewPaused  = "paused"
	OwnerSalesViewDraft   = "draft"
	OwnerSalesViewAll     = "all"

	ServiceSalesStateSelling  = "selling"
	ServiceSalesStateUpcoming = "upcoming"
	ServiceSalesStatePaused   = "paused"
	ServiceSalesStateSoldOut  = "sold_out"
	ServiceSalesStateExpired  = "expired"
	ServiceSalesStateDraft    = "draft"
	ServiceSalesStateOffline  = "offline"
	ServiceSalesStateArchived = "archived"

	ServiceSalesChannelFlexibleQuota = "flexible_quota"
	ServiceSalesChannelLimitedQuota  = "limited_quota"
)

type Service struct {
	ID                               string
	OwnerUserID                      string
	MerchantProfileID                string
	MerchantIdentityMode             string
	MerchantDisplayName              string
	MerchantProfileSlug              string
	MerchantAvatarURL                string
	OwnerContactMethodID             string
	Title                            string
	ShortDescription                 string
	SourceURL                        string
	DistributionSystem               string
	BillingMode                      string
	DeclaredCNYPerUSDAllowance       string
	DeclaredMaxUSDAllowancePerIntent string
	AvailableUSDAllowance            string
	QuotaExpiresAt                   *time.Time
	MinimumIntentCNY                 string
	MaximumIntentCNY                 string
	UsageVisibility                  string
	PublicAccessNote                 string
	MerchantNote                     string
	MerchantSupportNote              string
	AccountPoolType                  string
	AccountPoolCustomName            string
	MerchantRefundCommitment         bool
	DeclaredTTFTBand                 string
	DeclaredMaxConcurrency           int
	PerformanceConfirmedAt           *time.Time
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
	Completed30d                     int
	UnresolvedDisputes               int
	ResponseMedianMinutes            *float64
	IsOrderable                      bool
	OrderableReasons                 []string
	CreatedAt                        time.Time
	UpdatedAt                        time.Time
	Version                          int64
	SellerReputation                 *reputation.ReputationSnapshot
	SourceAuthorVerification         reputation.SourceAuthorResourceSummary
	SalesSummary                     ServiceSalesSummary
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
	ID             string
	APIServiceID   string
	Name           string
	PriceCNY       string
	PanelAllowance string
	DurationDays   *int
	StockTotal     int
	StockAvailable int
	Description    string
	Enabled        bool
	SortOrder      int
	Models         []ServicePackageModel
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ServicePackageModel struct {
	ServiceModelID      string
	ModelCatalogID      string
	ModelPriceVersionID string
	ModelNameSnapshot   string
	ProviderSnapshot    string
	MerchantMultiplier  string
}

type PaymentOption struct {
	ID                   string
	APIServiceID         string
	PaymentMethod        string
	Enabled              bool
	PaymentInstructions  string
	PaymentQRCodeDataURL string
	Version              int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CreateServiceInput struct {
	OwnerUserID                      string
	MerchantProfileID                string
	MerchantIdentityMode             string
	OwnerContactMethodID             string
	Title                            string
	ShortDescription                 string
	SourceURL                        string
	DistributionSystem               string
	BillingMode                      string
	DeclaredCNYPerUSDAllowance       string
	DeclaredMaxUSDAllowancePerIntent string
	AvailableUSDAllowance            string
	QuotaExpiresAt                   string
	MinimumIntentCNY                 string
	MaximumIntentCNY                 string
	UsageVisibility                  string
	PublicAccessNote                 string
	MerchantNote                     string
	AccountPoolType                  string
	AccountPoolCustomName            string
	MerchantRefundCommitment         *bool
	DeclaredTTFTBand                 string
	DeclaredMaxConcurrency           int
	PerformanceConfirmedAt           string
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
	SourceURL                        string
	DistributionSystem               string
	BillingMode                      string
	DeclaredCNYPerUSDAllowance       string
	DeclaredMaxUSDAllowancePerIntent string
	AvailableUSDAllowance            string
	QuotaExpiresAt                   string
	MinimumIntentCNY                 string
	MaximumIntentCNY                 string
	UsageVisibility                  string
	PublicAccessNote                 string
	MerchantNote                     string
	AccountPoolType                  string
	AccountPoolCustomName            string
	MerchantRefundCommitment         *bool
	DeclaredTTFTBand                 string
	DeclaredMaxConcurrency           int
	PerformanceConfirmedAt           string
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
	ID              string
	Name            string
	PriceCNY        string
	PanelAllowance  string
	DurationDays    *int
	StockTotal      int
	Description     string
	Enabled         bool
	SortOrder       int
	ModelCatalogIDs []string
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

type OwnerServiceFilter struct {
	SalesView string
}

type ServiceSalesSummary struct {
	OverallState string                `json:"overallState"`
	Channels     []ServiceSalesChannel `json:"channels"`
}

type ServiceSalesChannel struct {
	Kind                  string     `json:"kind"`
	State                 string     `json:"state"`
	AvailableUSDAllowance string     `json:"availableUsdAllowance,omitempty"`
	AvailableCopies       int        `json:"availableCopies,omitempty"`
	NextStartsAt          *time.Time `json:"nextStartsAt,omitempty"`
	SaleCutoffAt          *time.Time `json:"saleCutoffAt,omitempty"`
	ExpiresAt             *time.Time `json:"expiresAt,omitempty"`
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
	PaymentMethod        string
	Enabled              bool
	PaymentInstructions  string
	PaymentQRCodeDataURL string
}

type AccountPaymentSettings struct {
	UserID               string
	PaymentWindowMinutes int
	PaymentOptions       []PaymentOptionInput
	UpdatedAt            time.Time
}

type UpdateAccountPaymentSettingsInput struct {
	UserID               string
	PaymentWindowMinutes int
	PaymentOptions       []PaymentOptionInput
}
