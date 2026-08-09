package apiorder

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"
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

	DisputeStatusNone                    = "none"
	DisputeStatusNegotiating             = "negotiating"
	DisputeStatusOpen                    = "open"
	DisputeStatusAwaitingFulfillment     = "awaiting_fulfillment"
	DisputeStatusFulfillmentConfirmation = "fulfillment_confirmation"
	DisputeStatusClosed                  = "closed"

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

	CompletionSourceBuyerConfirmed = "buyer_confirmed"
	CompletionSourceAutoCompleted  = "auto_completed"

	CancelReasonBuyer          = "buyer_cancelled"
	CancelReasonPaymentTimeout = "payment_timeout"

	EventCreated                 = "api_order.created"
	EventPaymentInstructionsRead = "api_order.payment_instructions_read"
	EventPaymentSubmitted        = "api_order.payment_submitted"
	EventPaymentIssueReported    = "api_order.payment_issue_reported"
	EventPaymentConfirmed        = "api_order.payment_confirmed"
	EventDeliverySubmitted       = "api_order.delivery_submitted"
	EventCompleted               = "api_order.completed"
	EventCancelled               = "api_order.cancelled"
	EventPaymentTimeoutCancelled = "api_order.payment_timeout_cancelled"
	EventDisputeOpened           = "api_order.dispute_opened"
	EventDisputeClosed           = "api_order.dispute_closed"
	EventDeliveryReviewReminder  = "api_order.delivery_review_reminder_sent"
	EventAutoCompleted           = "api_order.auto_completed"

	DeliveryKindAPIKeyEndpoint = "api_key_endpoint"
	DeliveryKindLoginAccount   = "login_account"

	PaymentIssueNotReceived    = "not_received"
	PaymentIssueAmountMismatch = "amount_mismatch"
	PaymentIssueRemarkMismatch = "remark_mismatch"

	DeliveryReviewWindow       = 24 * time.Hour
	DeliveryReviewReminderLead = 2 * time.Hour
)

func IsDisputeActive(status string) bool {
	switch status {
	case DisputeStatusNegotiating, DisputeStatusOpen, DisputeStatusAwaitingFulfillment, DisputeStatusFulfillmentConfirmation:
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
	PaymentIssueReason            string
	PaymentIssueNote              string
	PaymentIssueReportedAt        *time.Time
	PaidConfirmedAt               *time.Time
	DeliveryNote                  string
	DeliverySubmittedAt           *time.Time
	DeliveryReviewExpiresAt       *time.Time
	DeliveryReviewRemindedAt      *time.Time
	DeliveryCredential            *DeliveryCredential
	CompletionSource              string
	CompletedAt                   *time.Time
	CancelledAt                   *time.Time
	CancelReason                  string
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	Version                       int64
	BuyerReputation               *reputation.ReputationSnapshot
	SellerReputation              *reputation.ReputationSnapshot
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
	OrderID             string
	ActorUserID         string
	PaymentSummary      string
	PaymentIssueReason  string
	PaymentIssueNote    string
	DeliveryNote        string
	DeliveryCredential  DeliveryCredentialInput
	Reason              string
	IssueCode           string
	RequestedResolution string
	RequestedAmountCNY  string
	ExpectedVersion     int64
	RequestID           string
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
	RequestID           string
	Now                 time.Time
}

type CompletionBuilder func(Order) (idempotency.Completion, *domain.AppError)
