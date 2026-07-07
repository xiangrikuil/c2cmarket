package officialprice

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	LeadStatusPending          = "pending"
	LeadStatusChangesRequested = "changes_requested"
	LeadStatusApproved         = "approved"
	LeadStatusRejected         = "rejected"

	RecordStatusActive     = "active"
	RecordStatusSuperseded = "superseded"
	RecordStatusTakenDown  = "taken_down"
)

type Lead struct {
	ID                   string
	SubmitterUserID      string
	ProductPlanID        string
	ProductText          string
	PlanText             string
	RegionCode           string
	Channel              string
	OpeningMethod        string
	SourceURL            string
	SourceTitle          string
	EvidenceSummary      string
	Note                 string
	Status               string
	ReviewedByAdminID    string
	ReviewedAt           *time.Time
	ReviewReason         string
	ObservedAt           time.Time
	BillingPeriod        string
	CommitmentMonths     *int
	PriceUnit            string
	SeatCount            *int
	Quantity             int
	Currency             string
	OriginalAmount       string
	OriginalPriceText    string
	TaxIncluded          bool
	NormalizedMonthlyCNY string
	FXRate               string
	FXSource             string
	FXObservedAt         *time.Time
	ConversionMode       string
	RoundingRule         string
	Fingerprint          string
	OfferKey             string
	DuplicateOfLeadID    string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int64
}

type Record struct {
	ID                   string
	LeadID               string
	ProductPlanID        string
	RegionCode           string
	Channel              string
	OpeningMethod        string
	SourceURL            string
	ApprovedByAdminID    string
	ApprovedAt           time.Time
	ValidFrom            time.Time
	ValidTo              *time.Time
	Status               string
	ObservedAt           time.Time
	BillingPeriod        string
	CommitmentMonths     *int
	PriceUnit            string
	SeatCount            *int
	Quantity             int
	Currency             string
	OriginalAmount       string
	TaxIncluded          bool
	NormalizedMonthlyCNY string
	FXRate               string
	FXSource             string
	FXObservedAt         time.Time
	ConversionMode       string
	RoundingRule         string
	Fingerprint          string
	OfferKey             string
	IsLowestReference    bool
	CreatedAt            time.Time
	Version              int64
}

type SubmitLeadInput struct {
	ProductPlanID     string
	ProductText       string
	PlanText          string
	RegionCode        string
	Channel           string
	OpeningMethod     string
	SourceURL         string
	SourceTitle       string
	EvidenceSummary   string
	Note              string
	ObservedAt        time.Time
	BillingPeriod     string
	CommitmentMonths  *int
	PriceUnit         string
	SeatCount         *int
	Quantity          int
	Currency          string
	OriginalAmount    string
	OriginalPriceText string
	TaxIncluded       bool
}

type ApproveLeadInput struct {
	LeadID                string
	AdminUserID           string
	ExpectedVersion       int64
	RequestID             string
	Reason                string
	ResolvedProductPlanID string
	ValidFrom             time.Time
	FXRateToCNY           string
	FXSource              string
	FXObservedAt          time.Time
}

type AdminRecordInput struct {
	RecordID        string
	AdminUserID     string
	ExpectedVersion int64
	RequestID       string
	ProductPlanID   string
	ProductText     string
	PlanText        string
	RegionCode      string
	Channel         string
	OpeningMethod   string
	SourceURL       string
	ObservedAt      time.Time
	BillingPeriod   string
	Currency        string
	OriginalAmount  string
	TaxIncluded     bool
	FXRateToCNY     string
	FXSource        string
	FXObservedAt    time.Time
	ValidFrom       time.Time
	Reason          string
}

type AdminRecordActionInput struct {
	RecordID        string
	AdminUserID     string
	ExpectedVersion int64
	RequestID       string
	Reason          string
}

type ApprovalCompletionBuilder func(Lead, Record) (idempotency.Completion, *domain.AppError)
