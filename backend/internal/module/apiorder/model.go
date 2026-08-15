package apiorder

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"
)

const (
	MerchantConfirmWindow        = 10 * time.Minute
	DefaultDeliveryWindow        = 10 * time.Minute
	LatePaymentWindow            = 24 * time.Hour
	MinimumDeliveryValidity      = 60 * time.Minute
	QuotaDeliveryModePreimported = "preimported"
)

const (
	PurchaseKindAPIService        = "api_service"
	PurchaseKindLimitedQuotaOffer = "limited_quota_offer"

	StatusPendingPayment    = "pending_payment"
	StatusPaymentSubmitted  = "payment_submitted"
	StatusPaymentIssue      = "payment_issue"
	StatusPaidConfirmed     = "paid_confirmed"
	StatusDeliverySubmitted = "delivery_submitted"
	StatusCompleted         = "completed"
	StatusCancelled         = "cancelled"

	DisputeStatusNone                     = "none"
	DisputeStatusNegotiating              = "negotiating"
	DisputeStatusPendingSellerResponse    = "pending_seller_response"
	DisputeStatusPendingApplicantDecision = "pending_applicant_decision"
	DisputeStatusOpen                     = "open"
	DisputeStatusAwaitingFulfillment      = "awaiting_fulfillment"
	DisputeStatusFulfillmentConfirmation  = "fulfillment_confirmation"
	DisputeStatusClosed                   = "closed"

	DisputeIssueServiceUnavailable  = "service_unavailable"
	DisputeIssueDescriptionMismatch = "description_mismatch"
	DisputeIssueQuotaShortage       = "quota_shortage"
	DisputeIssueExpiredEarly        = "expired_early"
	DisputeIssueNotDelivered        = "not_delivered"
	DisputeIssueRefundNotReceived   = "refund_not_received"
	DisputeIssuePaymentDispute      = "payment_dispute"
	DisputeIssueOther               = "other"

	DisputeResolutionFullRefund          = "full_refund"
	DisputeResolutionPartialRefund       = "partial_refund"
	DisputeResolutionContinueFulfillment = "continue_fulfillment"
	DisputeResolutionOther               = "other"

	CompletionSourceBuyerConfirmed  = "buyer_confirmed"
	CompletionSourceAutoCompleted   = "auto_completed"
	CompletionSourceRemedyConfirmed = "remedy_confirmed"

	CommercialOutcomePending              = "pending"
	CommercialOutcomeCancelledUnpaid      = "cancelled_unpaid"
	CommercialOutcomeNormalFulfillment    = "normal_fulfillment"
	CommercialOutcomeFullRefund           = "full_refund"
	CommercialOutcomePartialRefund        = "partial_refund"
	CommercialOutcomeContinuedFulfillment = "continued_fulfillment"
	CommercialOutcomeClosedUnverified     = "closed_unverified"

	CancelReasonBuyer             = "buyer_cancelled"
	CancelReasonPaymentTimeout    = "payment_timeout"
	CancelReasonAccountGovernance = "ACCOUNT_GOVERNANCE_CANCELLED"

	EventCreated                 = "api_order.created"
	EventPaymentInstructionsRead = "api_order.payment_instructions_read"
	EventPaymentSubmitted        = "api_order.payment_submitted"
	EventPaymentIssueReported    = "api_order.payment_issue_reported"
	EventPaymentConfirmed        = "api_order.payment_confirmed"
	EventDeliverySubmitted       = "api_order.delivery_submitted"
	EventCompleted               = "api_order.completed"
	EventCancelled               = "api_order.cancelled"
	EventPaymentTimeoutCancelled = "api_order.payment_timeout_cancelled"
	EventGovernanceCancelled     = "api_order.governance_cancelled"
	EventDisputeOpened           = "api_order.dispute_opened"
	EventDisputeRemedyAwaiting   = "api_order.dispute_remedy_awaiting"
	EventDisputeRemedyClaimed    = "api_order.dispute_remedy_claimed"
	EventLatePaymentReported     = "api_order.late_payment_reported"
	EventLatePaymentResolved     = "api_order.late_payment_resolved"
	EventDisputeRemedyContested  = "api_order.dispute_remedy_contested"
	EventDisputeClosed           = "api_order.dispute_closed"
	EventPaymentReviewOverdue    = "api_order.payment_review_overdue"
	EventDeliveryOverdue         = "api_order.delivery_overdue"
	EventQuotaValidityIssue      = "api_order.quota_validity_issue"
	EventDeliveryDueReminder     = "api_order.delivery_due_reminder_sent"
	EventDeliveryReviewReminder  = "api_order.delivery_review_reminder_sent"
	EventAutoCompleted           = "api_order.auto_completed"
	EventCatalogRiskHoldCreated  = "api_order.catalog_risk_hold_created"
	EventCatalogRiskHoldRestored = "api_order.catalog_risk_hold_restored"
	EventCatalogRefundPending    = "api_order.catalog_risk_refund_pending"
	EventCatalogDisputeOpened    = "api_order.catalog_risk_dispute_opened"

	DeliveryKindAPIKeyEndpoint = "api_key_endpoint"
	DeliveryKindLoginAccount   = "login_account"

	PaymentIssueNotReceived    = "not_received"
	PaymentIssueAmountMismatch = "amount_mismatch"
	PaymentIssueRemarkMismatch = "remark_mismatch"

	DeliveryReviewWindow       = 24 * time.Hour
	DeliveryReviewReminderLead = 2 * time.Hour
	DeliveryDueReminderLead    = 3 * time.Minute

	QuotaValidityIssueDelivery = "delivery_insufficient"
)

const (
	LatePaymentStatusReported              = "reported"
	LatePaymentStatusNotReceived           = "not_received"
	LatePaymentStatusReceivedRefundPending = "received_refund_pending"
)

func IsDisputeActive(status string) bool {
	switch status {
	case DisputeStatusNegotiating, DisputeStatusPendingSellerResponse, DisputeStatusPendingApplicantDecision,
		DisputeStatusOpen, DisputeStatusAwaitingFulfillment, DisputeStatusFulfillmentConfirmation:
		return true
	default:
		return false
	}
}

func IsDisputeIssueCode(value string) bool {
	switch value {
	case DisputeIssueServiceUnavailable, DisputeIssueDescriptionMismatch, DisputeIssueQuotaShortage,
		DisputeIssueExpiredEarly, DisputeIssueNotDelivered, DisputeIssueRefundNotReceived,
		DisputeIssuePaymentDispute, DisputeIssueOther:
		return true
	default:
		return false
	}
}

func IsDisputeResolution(value string) bool {
	switch value {
	case DisputeResolutionFullRefund, DisputeResolutionPartialRefund,
		DisputeResolutionContinueFulfillment, DisputeResolutionOther:
		return true
	default:
		return false
	}
}

func DisputeResolutionRequiresFulfillment(value string) bool {
	switch value {
	case DisputeResolutionFullRefund, DisputeResolutionPartialRefund, DisputeResolutionContinueFulfillment:
		return true
	default:
		return false
	}
}

type DisputeProjection struct {
	CaseID             string
	Status             string
	NextActor          string
	NextUserID         string
	DueAt              *time.Time
	ActiveRemedyAction string
	ActiveRemedySource string
}

type Order struct {
	ID                            string
	OrderNo                       string
	PurchaseKind                  string
	APIPurchaseIntentID           string
	APIServiceID                  string
	BuyerUserID                   string
	SellerUserID                  string
	Status                        string
	DisputeStatus                 string
	DisputeCaseID                 string
	LatestDisputeCaseID           string
	HasDisputeHistory             bool
	DisputeNextActor              string
	DisputeNextUserID             string
	DisputeDueAt                  *time.Time
	ActiveRemedyAction            string
	ActiveRemedySource            string
	ServiceTitleSnapshot          string
	ServiceVersionSnapshot        int64
	BillingModeSnapshot           string
	SelectedPackageID             string
	SelectedPackageSnapshot       string
	QuoteVersionSnapshot          int64
	RequestedUSDAllowanceSnapshot string
	CNYPerUSDAllowanceSnapshot    string
	PricingSnapshot               string
	ProbeConnectionIDSnapshot     string
	APIBaseURLSnapshot            string
	NormalizedAPIBaseURLSnapshot  string
	QuotaUsagePolicySnapshot      apimarket.QuotaUsagePolicy
	PromptAuditEnabledSnapshot    *bool
	PackageStockReserved          bool
	PackageExpiresAt              *time.Time
	APIQuotaBatchID               string
	APIQuotaOfferID               string
	APIQuotaSaleRoundID           string
	APIQuotaAllocationID          string
	APIQuotaInventoryUnitID       string
	APIQuotaCredentialID          string
	QuotaOfferSnapshot            string
	QuotaOfferNameSnapshot        string
	QuotaUSDAllowanceSnapshot     string
	QuotaPriceCNYSnapshot         string
	QuotaCNYPerUSDSnapshot        string
	QuotaModelMultiplierSnapshot  string
	QuotaSaleCutoffAtSnapshot     *time.Time
	QuotaExpiresAtSnapshot        *time.Time
	QuotaSaleModeSnapshot         string
	QuotaRoundStartsAtSnapshot    *time.Time
	QuotaRoundEndsAtSnapshot      *time.Time
	QuotaDistributionSnapshot     string
	QuotaTTFTBandSnapshot         string
	QuotaDeclaredMaxConcurrency   int
	QuotaPerformanceConfirmedAt   *time.Time
	QuotaPerformanceUnverified    bool
	QuotaDeliveryETAMinutes       int
	QuotaDeliveryMode             string
	Amount                        string
	Currency                      string
	SelectedPaymentMethod         string
	PaymentWindowMinutesSnapshot  int
	PaymentExpiresAt              time.Time
	PaymentInstructionsSnapshot   string
	PaymentQRCodeDataURLSnapshot  string
	PaymentSummary                string
	PaymentSubmittedAt            *time.Time
	MerchantConfirmDueAt          *time.Time
	MerchantConfirmOverdue        bool
	MerchantConfirmOverdueAt      *time.Time
	PaymentIssueReason            string
	PaymentIssueNote              string
	PaymentIssueReportedAt        *time.Time
	PaidConfirmedAt               *time.Time
	DeliveryDueAt                 *time.Time
	DeliveryOverdue               bool
	DeliveryOverdueAt             *time.Time
	DeliveryDueRemindedAt         *time.Time
	DeliveryNote                  string
	DeliverySubmittedAt           *time.Time
	DeliveryReviewExpiresAt       *time.Time
	DeliveryReviewRemindedAt      *time.Time
	DeliveryCredential            *DeliveryCredential
	CommercialOutcome             string
	CommercialOutcomeUpdatedAt    *time.Time
	QuotaValidityIssueAt          *time.Time
	QuotaValidityIssueReason      string
	CompletionSource              string
	CompletedAt                   *time.Time
	CancelledAt                   *time.Time
	CancelReason                  string
	LatePaymentStatus             string
	LatePaymentReportedAt         *time.Time
	LatePaymentNote               string
	LatePaymentResolvedAt         *time.Time
	CanReportLatePayment          bool
	AfterSalesExpiresAt           *time.Time
	CanOpenDispute                bool
	DisputeEligibilityReason      string
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	Version                       int64
	BuyerReputation               *reputation.ReputationSnapshot
	SellerReputation              *reputation.ReputationSnapshot
	CatalogRiskHold               *CatalogRiskHold
}

const (
	CatalogRiskHoldActive        = "active"
	CatalogRiskHoldRestored      = "restored"
	CatalogRiskHoldRefundPending = "refund_pending"
	CatalogRiskHoldDisputeOpened = "dispute_opened"
)

type CatalogRiskHold struct {
	ID             string
	SourceType     string
	SourceID       string
	Status         string
	Reason         string
	CreatedAt      time.Time
	ResolvedBy     string
	ResolvedAt     *time.Time
	ResolutionNote string
	Version        int64
}

type Event struct {
	ID          string
	APIOrderID  string
	ActorUserID string
	EventType   string
	FromStatus  string
	ToStatus    string
	Note        string
	RequestID   string
	CreatedAt   time.Time
}

type PaymentInstructionAccessLog struct {
	ID          string
	APIOrderID  string
	BuyerUserID string
	RequestID   string
	AccessedAt  time.Time
}

type DeliveryCredential struct {
	ID            string
	APIOrderID    string
	SellerUserID  string
	BuyerUserID   string
	DeliveryKind  string
	APIBaseURL    string
	APIKey        string
	PanelLoginURL string
	Username      string
	Password      string
	Instructions  string
	SubmittedAt   time.Time
	CreatedAt     time.Time
	DestroyedAt   *time.Time
	DestroyReason string
}

type CreateInput struct {
	IntentID      string
	BuyerUserID   string
	PaymentMethod string
	RequestID     string
}

type ActionInput struct {
	OrderID                string
	ActorUserID            string
	ActorAudience          string
	ParticipantRole        string
	GovernanceActionID     string
	GovernanceVersion      int64
	RestrictionEffectiveAt time.Time
	PaymentSummary         string
	PaymentIssueReason     string
	PaymentIssueNote       string
	LatePaymentStatus      string
	LatePaymentNote        string
	DeliveryNote           string
	DeliveryCredential     DeliveryCredentialInput
	Reason                 string
	IssueCode              string
	RequestedResolution    string
	RequestedAmountCNY     string
	IssueOccurredAt        string
	ExpectedVersion        int64
	RequestID              string
	EvidenceAssetIDs       []string
}

type CatalogRiskHoldActionInput struct {
	OrderID         string
	AdminUserID     string
	Resolution      string
	ResolutionNote  string
	ExpectedVersion int64
	RequestID       string
}

type DeliveryCredentialInput struct {
	DeliveryKind  string
	APIBaseURL    string
	APIKey        string
	PanelLoginURL string
	Username      string
	Password      string
	Instructions  string
}

type PaymentInstructionsView struct {
	OrderID              string
	PaymentMethod        string
	PaymentInstructions  string
	PaymentQRCodeDataURL string
	PaymentExpiresAt     time.Time
}

type DisputeCaseInput struct {
	OrderID             string
	ServiceTitle        string
	BuyerUserID         string
	SellerUserID        string
	ActorUserID         string
	Reason              string
	IssueCode           string
	RequestedResolution string
	RequestedAmountCNY  string
	IssueOccurredAt     *time.Time
	RequestID           string
	Now                 time.Time
}

type CompletionBuilder func(Order) (idempotency.Completion, *domain.AppError)
