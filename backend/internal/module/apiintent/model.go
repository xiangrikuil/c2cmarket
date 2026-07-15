package apiintent

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	StatusOpen           = "open"
	StatusContacted      = "contacted"
	StatusOrdered        = "ordered"
	StatusBuyerCancelled = "buyer_cancelled"
	StatusOwnerClosed    = "owner_closed"
)

type Intent struct {
	ID                                       string
	APIServiceID                             string
	APIServiceOwnerUserID                    string
	BuyerUserID                              string
	OwnerUserID                              string
	BuyerContactMethodID                     string
	BuyerContactMethodVersionID              string
	OwnerContactMethodID                     string
	OwnerContactMethodVersionID              string
	Status                                   string
	RequestedCNYAmount                       string
	RequestedUSDAllowance                    string
	SelectedAccessMode                       string
	SelectedPackageID                        string
	SelectedPackageSnapshot                  string
	ServiceVersionSnapshot                   int64
	ServiceTitleSnapshot                     string
	DistributionSystemSnapshot               string
	BillingModeSnapshot                      string
	BuyerContactTypeSnapshot                 string
	BuyerContactLabelSnapshot                string
	OwnerContactTypeSnapshot                 string
	OwnerContactLabelSnapshot                string
	DeclaredCNYPerUSDAllowanceSnapshot       string
	DeclaredMaxUSDAllowancePerIntentSnapshot string
	MinimumIntentCNYSnapshot                 string
	MaximumIntentCNYSnapshot                 string
	PricingSnapshot                          string
	BuyerNote                                string
	ContactedAt                              *time.Time
	BuyerCancelledAt                         *time.Time
	BuyerCancelReason                        string
	OwnerClosedAt                            *time.Time
	OwnerCloseReason                         string
	CreatedAt                                time.Time
	UpdatedAt                                time.Time
	Version                                  int64
	MerchantContact                          *contact.ContactItemView
	BuyerContact                             *contact.ContactItemView
}

type ContactAccessLog struct {
	ID                     string
	APIPurchaseIntentID    string
	ViewerUserID           string
	ViewedContactOwnerSide string
	RequestID              string
	AccessedAt             time.Time
}

type CreateIntentInput struct {
	APIServiceID          string
	BuyerUserID           string
	BuyerContactMethodID  string
	RequestedCNYAmount    string
	RequestedUSDAllowance string
	SelectedAccessMode    string
	SelectedPackageID     string
	BuyerNote             string
	RequestID             string
}

type ActionInput struct {
	IntentID        string
	ActorUserID     string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type CompletionBuilder func(Intent) (idempotency.Completion, *domain.AppError)
