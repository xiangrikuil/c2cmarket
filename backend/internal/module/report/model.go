package report

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	TargetContactSnapshot   = "contact_snapshot"
	TargetPublicUser        = "public_user"
	TargetCarpoolMembership = "carpool_membership"
	TargetAPIPurchaseIntent = "api_purchase_intent"
	TargetAPIOrder          = "api_order"

	ReportReasonInvalid       = "invalid"
	ReportReasonUnreachable   = "unreachable"
	ReportReasonImpersonation = "impersonation"
	ReportReasonOther         = "other"

	ReportStatusSubmitted     = "submitted"
	ReportStatusTriaged       = "triaged"
	ReportStatusRejected      = "rejected"
	ReportStatusDisputeOpened = "dispute_opened"

	DisputeStatusOpen        = "open"
	DisputeStatusWaitingInfo = "waiting_info"
	DisputeStatusResolved    = "resolved"
	DisputeStatusClosed      = "closed"

	AppealStatusSubmitted = "submitted"
	AppealStatusApproved  = "approved"
	AppealStatusRejected  = "rejected"
)

type Report struct {
	ID               string
	ReporterUserID   string
	ReporterUsername string
	ReporterName     string
	TargetType       string
	TargetID         string
	TargetLabel      string
	ReportedUsername string
	ReasonCode       string
	Title            string
	Description      string
	Status           string
	AdminReason      string
	HandledByAdminID string
	HandledAt        *time.Time
	DisputeID        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

type DisputeCase struct {
	ID                   string
	ReportID             string
	TargetType           string
	TargetID             string
	TargetLabel          string
	PrimaryUserID        string
	PrimaryUsername      string
	PrimaryDisplayName   string
	CounterpartyUserID   string
	CounterpartyUsername string
	CounterpartyName     string
	Status               string
	PublicSummary        string
	PublicResult         string
	AdminReason          string
	OpenedByAdminID      string
	OpenedAt             time.Time
	ResolvedAt           *time.Time
	ClosedAt             *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int64
}

type Appeal struct {
	ID                string
	AppellantUserID   string
	AppellantUsername string
	AppellantName     string
	ReportID          string
	DisputeID         string
	TargetType        string
	TargetID          string
	Title             string
	Statement         string
	Status            string
	AdminReason       string
	HandledByAdminID  string
	HandledAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Version           int64
}

type Event struct {
	ID         string
	EntityType string
	EntityID   string
	Action     string
	ActorID    string
	ActorRole  string
	Reason     string
	Public     bool
	CreatedAt  time.Time
}

type PublicDispute struct {
	ID         string
	Username   string
	Type       string
	Result     string
	HandledAt  time.Time
	Unresolved bool
}

type PublicStats struct {
	UnresolvedCount    int
	ResolvedLast90Days int
}

type CreateReportInput struct {
	ReporterUserID   string
	ReporterUsername string
	ReporterName     string
	TargetType       string
	TargetID         string
	TargetLabel      string
	ReportedUsername string
	ReasonCode       string
	Title            string
	Description      string
}

type CreateAppealInput struct {
	AppellantUserID   string
	AppellantUsername string
	AppellantName     string
	ReportID          string
	DisputeID         string
	TargetType        string
	TargetID          string
	Title             string
	Statement         string
}

type AdminActionInput struct {
	ID              string
	AdminUserID     string
	Action          string
	Reason          string
	PublicSummary   string
	PublicResult    string
	ExpectedVersion int64
	RequestID       string
}

type MutationResult struct {
	Report  *Report
	Dispute *DisputeCase
	Appeal  *Appeal
}

type ReportCompletionBuilder func(Report) (idempotency.Completion, *domain.AppError)
type AppealCompletionBuilder func(Appeal) (idempotency.Completion, *domain.AppError)
type AdminCompletionBuilder func(MutationResult) (idempotency.Completion, *domain.AppError)
