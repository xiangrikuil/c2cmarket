package apiorder

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	StatusPendingPayment    = "pending_payment"
	StatusPaymentSubmitted  = "payment_submitted"
	StatusPaidConfirmed     = "paid_confirmed"
	StatusDeliverySubmitted = "delivery_submitted"
	StatusCompleted         = "completed"
	StatusCancelled         = "cancelled"

	DisputeStatusNone   = "none"
	DisputeStatusOpen   = "open"
	DisputeStatusClosed = "closed"

	CancelReasonBuyer          = "buyer_cancelled"
	CancelReasonPaymentTimeout = "payment_timeout"

	EventCreated                 = "api_order.created"
	EventPaymentInstructionsRead = "api_order.payment_instructions_read"
	EventPaymentSubmitted        = "api_order.payment_submitted"
	EventPaymentConfirmed        = "api_order.payment_confirmed"
	EventDeliverySubmitted       = "api_order.delivery_submitted"
	EventCompleted               = "api_order.completed"
	EventCancelled               = "api_order.cancelled"
	EventPaymentTimeoutCancelled = "api_order.payment_timeout_cancelled"
	EventDisputeOpened           = "api_order.dispute_opened"
)

type Order struct {
	ID                           string
	APIPurchaseIntentID          string
	APIServiceID                 string
	BuyerUserID                  string
	SellerUserID                 string
	Status                       string
	DisputeStatus                string
	DisputeCaseID                string
	ServiceTitleSnapshot         string
	ServiceVersionSnapshot       int64
	BillingModeSnapshot          string
	SelectedPackageID            string
	SelectedPackageSnapshot      string
	QuoteVersionSnapshot         int64
	Amount                       string
	Currency                     string
	SelectedPaymentMethod        string
	PaymentWindowMinutesSnapshot int
	PaymentExpiresAt             time.Time
	PaymentInstructionsSnapshot  string
	PaymentSummary               string
	PaymentSubmittedAt           *time.Time
	PaidConfirmedAt              *time.Time
	DeliveryNote                 string
	DeliverySubmittedAt          *time.Time
	CompletedAt                  *time.Time
	CancelledAt                  *time.Time
	CancelReason                 string
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
	Version                      int64
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

type CreateInput struct {
	IntentID      string
	BuyerUserID   string
	PaymentMethod string
	RequestID     string
}

type ActionInput struct {
	OrderID         string
	ActorUserID     string
	PaymentSummary  string
	DeliveryNote    string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type PaymentInstructionsView struct {
	OrderID             string
	PaymentMethod       string
	PaymentInstructions string
	PaymentExpiresAt    time.Time
}

type DisputeCaseInput struct {
	OrderID      string
	ServiceTitle string
	BuyerUserID  string
	SellerUserID string
	ActorUserID  string
	Reason       string
	RequestID    string
	Now          time.Time
}

type CompletionBuilder func(Order) (idempotency.Completion, *domain.AppError)
