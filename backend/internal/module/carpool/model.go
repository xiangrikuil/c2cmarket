package carpool

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"
)

const (
	ListingStatusDraft            = "draft"
	ListingStatusPendingReview    = "pending_review"
	ListingStatusChangesRequested = "changes_requested"
	ListingStatusActive           = "active"
	ListingStatusStopped          = "stopped"
	ListingStatusPaused           = "paused"
	ListingStatusRejected         = "rejected"
	ListingStatusRemoved          = "removed"

	ApplicationStatusPendingOwner     = "pending_owner"
	ApplicationStatusJoined           = "joined"
	ApplicationStatusRejected         = "rejected"
	ApplicationStatusCancelledByBuyer = "cancelled_by_buyer"

	JoinActorBuyer = "buyer"
	JoinActorOwner = "owner"

	MembershipStatusActive  = "active"
	MembershipStatusLeft    = "left"
	MembershipStatusRemoved = "removed"

	OwnerListingViewAll        = ""
	OwnerListingViewRecruiting = "recruiting"
	OwnerListingViewServing    = "serving"
	OwnerListingViewHistory    = "history"
	OwnerListingViewNeedsEdit  = "needs_edit"

	ListingDistributionMethodSub2API      = "sub2api"
	ListingDistributionMethodAccountLogin = "account_login"
	ListingDistributionMethodOther        = "other"

	ListingOpeningChannelWeb         = "web"
	ListingOpeningChannelIOSAppStore = "ios_app_store"
	ListingOpeningChannelGooglePlay  = "google_play"
	ListingOpeningChannelTeamSeat    = "team_seat"
	ListingOpeningChannelOther       = "other"

	ListingPaymentMethodCreditCard         = "credit_card"
	ListingPaymentMethodVirtualCard        = "virtual_card"
	ListingPaymentMethodApplePay           = "apple_pay"
	ListingPaymentMethodGooglePay          = "google_pay"
	ListingPaymentMethodAppStoreGiftCard   = "app_store_gift_card"
	ListingPaymentMethodGooglePlayGiftCard = "google_play_gift_card"
	ListingPaymentMethodPayPal             = "paypal"
	ListingPaymentMethodUCard              = "u_card"
	ListingPaymentMethodOther              = "other"
)

type RiskAcknowledgement struct {
	RiskNoticeCode string
	PolicyVersion  int64
	AcknowledgedAt time.Time
}

type Listing struct {
	ID                                    string
	OwnerUserID                           string
	ProductPlanID                         string
	OwnerContactMethodID                  string
	CycleTerm                             *CycleTerm
	Title                                 string
	Summary                               string
	AccessArrangement                     string
	DistributionMethod                    string
	DistributionMethodNote                string
	ProvidesAdminAccount                  bool
	RegionCode                            string
	RegionName                            string
	SourceURL                             string
	PriceMonthlyCNY                       string
	ServiceMultiplier                     string
	DailyQuotaAmount                      *string
	WeeklyQuotaAmount                     *string
	FollowsOfficialQuotaReset             *bool
	VPSRegion                             *string
	SupportsMainlandChinaDirectConnection *bool
	OpeningChannelCode                    *string
	CustomOpeningChannel                  *string
	PaymentMethodCode                     *string
	CustomPaymentMethod                   *string
	QuotaLabel                            string
	QuotaUnit                             string
	QuotaPeriod                           string
	BuyerSeatCapacity                     int
	OfflineOccupiedSeats                  int
	ActiveBuyerMembers                    int
	Status                                string
	GovernanceStatus                      string
	RecruitmentStopReason                 string
	ConditionsVersion                     int64
	ReviewedByAdminID                     string
	ReviewedAt                            *time.Time
	ReviewReason                          string
	PolicyVersion                         int64
	RiskNoticeCode                        string
	RiskAckRequired                       bool
	AvailableSeats                        int
	RequestID                             string
	CreatedAt                             time.Time
	UpdatedAt                             time.Time
	Version                               int64
	SellerReputation                      *reputation.ReputationSnapshot
	SourceAuthorVerification              reputation.SourceAuthorResourceSummary
}

type CycleTerm struct {
	ID               string
	CarpoolListingID string
	OwnerUserID      string
	BillingPeriod    string
	CycleStartDay    *int
	NoticeDays       int
	ExitPolicy       string
	UsageRules       string
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Application struct {
	ID                        string
	CarpoolListingID          string
	BuyerUserID               string
	OwnerUserID               string
	ProductPlanID             string
	BuyerContactMethodID      string
	Status                    string
	SeatCount                 int
	ListingTitleSnapshot      string
	PriceMonthlyCNY           string
	PolicyVersionSnapshot     int64
	RiskNoticeCode            string
	ConditionsVersionSnapshot int64
	ConditionsSnapshot        ListingConditionsSnapshot
	AcceptedConditionsVersion int64
	ConditionsAcceptedAt      time.Time
	ContactSessionID          string
	JoinedAt                  *time.Time
	DecisionReason            string
	DecidedAt                 *time.Time
	RequestID                 string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	Version                   int64
	BuyerReputation           *reputation.ReputationSnapshot
}

type Membership struct {
	ID                        string
	CarpoolListingID          string
	CarpoolApplicationID      string
	CycleTermID               string
	BuyerUserID               string
	OwnerUserID               string
	ProductPlanID             string
	Status                    string
	SeatCount                 int
	PriceMonthlyCNY           string
	PolicyVersionSnapshot     int64
	RiskNoticeCode            string
	ConditionsVersionSnapshot int64
	ConditionsSnapshot        ListingConditionsSnapshot
	JoinedAt                  time.Time
	EndedAt                   *time.Time
	EndedReason               string
	EndedByUserID             string
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	Version                   int64
}

type ListingConditionsSnapshot struct {
	Title                                 string     `json:"title"`
	PriceMonthlyCNY                       string     `json:"priceMonthlyCny"`
	DailySpendLimitUSD                    *string    `json:"dailySpendLimitUsd"`
	WeeklySpendLimitUSD                   *string    `json:"weeklySpendLimitUsd"`
	FollowsOfficialQuotaReset             bool       `json:"followsOfficialQuotaReset"`
	BuyerSeatCapacity                     int        `json:"buyerSeatCapacity"`
	OfflineOccupiedSeats                  int        `json:"offlineOccupiedSeats"`
	RegionCode                            string     `json:"regionCode"`
	RegionName                            string     `json:"regionName"`
	VPSRegion                             *string    `json:"vpsRegion"`
	SupportsMainlandChinaDirectConnection *bool      `json:"supportsMainlandChinaDirectConnection"`
	OpeningChannelCode                    string     `json:"openingChannelCode"`
	CustomOpeningChannel                  string     `json:"customOpeningChannel"`
	PaymentMethodCode                     string     `json:"paymentMethodCode"`
	CustomPaymentMethod                   string     `json:"customPaymentMethod"`
	DistributionMethod                    string     `json:"distributionMethod"`
	DistributionMethodNote                string     `json:"distributionMethodNote"`
	ProvidesAdminAccount                  bool       `json:"providesAdminAccount"`
	AccessArrangement                     string     `json:"accessArrangement"`
	CycleTerm                             *CycleTerm `json:"cycleTerm,omitempty"`
	PolicyVersion                         int64      `json:"policyVersion"`
	RiskNoticeCode                        string     `json:"riskNoticeCode"`
}

type CreateListingInput struct {
	OwnerUserID                           string
	ProductPlanID                         string
	OwnerContactMethodID                  string
	CycleTerm                             CycleTermInput
	Title                                 string
	Summary                               string
	AccessArrangement                     string
	DistributionMethod                    string
	DistributionMethodNote                string
	ProvidesAdminAccount                  bool
	RegionCode                            string
	RegionName                            string
	SourceURL                             string
	PriceMonthlyCNY                       string
	ServiceMultiplier                     string
	DailyQuotaAmount                      string
	WeeklyQuotaAmount                     string
	FollowsOfficialQuotaReset             *bool
	VPSRegion                             string
	SupportsMainlandChinaDirectConnection *bool
	OpeningChannelCode                    string
	CustomOpeningChannel                  string
	PaymentMethodCode                     string
	CustomPaymentMethod                   string
	BuyerSeatCapacity                     int
	OfflineOccupiedSeats                  int
	RiskAcknowledgement                   *RiskAcknowledgement
	RequestID                             string
}

type PublishListingInput = CreateListingInput

type CycleTermInput struct {
	BillingPeriod string
	CycleStartDay *int
	NoticeDays    int
	ExitPolicy    string
	UsageRules    string
}

type ReviewInput struct {
	ListingID       string
	AdminUserID     string
	Action          string
	Status          string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type UpdateListingInput struct {
	ListingID                             string
	OwnerUserID                           string
	ProductPlanID                         string
	OwnerContactMethodID                  string
	CycleTerm                             CycleTermInput
	Title                                 string
	Summary                               string
	AccessArrangement                     string
	DistributionMethod                    string
	DistributionMethodNote                string
	ProvidesAdminAccount                  bool
	RegionCode                            string
	RegionName                            string
	SourceURL                             string
	PriceMonthlyCNY                       string
	ServiceMultiplier                     string
	DailyQuotaAmount                      string
	WeeklyQuotaAmount                     string
	FollowsOfficialQuotaReset             *bool
	VPSRegion                             string
	SupportsMainlandChinaDirectConnection *bool
	OpeningChannelCode                    string
	CustomOpeningChannel                  string
	PaymentMethodCode                     string
	CustomPaymentMethod                   string
	BuyerSeatCapacity                     int
	OfflineOccupiedSeats                  int
	RiskAcknowledgement                   *RiskAcknowledgement
	ExpectedVersion                       int64
	RequestID                             string
}

type SubmitListingReviewInput struct {
	ListingID       string
	OwnerUserID     string
	ExpectedVersion int64
	RequestID       string
}

type RecruitmentInput struct {
	ListingID       string
	OwnerUserID     string
	ExpectedVersion int64
	RequestID       string
}

// ListingAuditEvent 是内存仓储与 PostgreSQL domain_events 对齐的安全操作事实。
type ListingAuditEvent struct {
	ListingID        string
	EventType        string
	ActorUserID      string
	ActorKind        string
	AggregateVersion int64
	RequestID        string
	Status           string
	GovernanceStatus string
	CreatedAt        time.Time
}

type ListingCompletionBuilder func(Listing) (idempotency.Completion, *domain.AppError)

type CreateApplicationInput struct {
	ListingID            string
	BuyerUserID          string
	BuyerContactMethodID string
	RiskAcknowledgement  *RiskAcknowledgement
	RequestID            string
}

type AcceptApplicationInput struct {
	ApplicationID   string
	OwnerUserID     string
	ExpectedVersion int64
	RequestID       string
}

type ConfirmApplicationConditionsInput struct {
	ApplicationID   string
	BuyerUserID     string
	ExpectedVersion int64
	RequestID       string
}

type RejectApplicationInput struct {
	ApplicationID   string
	OwnerUserID     string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type CancelApplicationInput struct {
	ApplicationID   string
	BuyerUserID     string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type ApplicationCompletionBuilder func(Application) (idempotency.Completion, *domain.AppError)

type ApplicationAuditEvent struct {
	ApplicationID    string
	EventType        string
	ActorUserID      string
	ActorKind        string
	AggregateVersion int64
	RequestID        string
	Status           string
	CreatedAt        time.Time
}

type EndMembershipInput struct {
	MembershipID           string
	ActorUserID            string
	ActorRole              string
	ActorAudience          string
	GovernanceActionID     string
	GovernanceVersion      int64
	RestrictionEffectiveAt time.Time
	TargetStatus           string
	Reason                 string
	ExpectedVersion        int64
	RequestID              string
}

type MembershipCompletionBuilder func(Membership) (idempotency.Completion, *domain.AppError)
