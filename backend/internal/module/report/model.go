package report

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	TargetContactSnapshot    = "contact_snapshot"
	TargetPublicUser         = "public_user"
	TargetCarpoolApplication = "carpool_application"
	TargetCarpoolMembership  = "carpool_membership"
	TargetAPIPurchaseIntent  = "api_purchase_intent"
	TargetAPIOrder           = "api_order"
	TargetAccountGovernance  = "account_governance"

	ReportReasonUnreachable          = "unreachable"
	ReportReasonContactInvalid       = "contact_invalid"
	ReportReasonImpersonation        = "impersonation"
	ReportReasonDescriptionMismatch  = "description_mismatch"
	ReportReasonSeatRuleDispute      = "seat_rule_dispute"
	ReportReasonAPIQuotaDispute      = "api_quota_dispute"
	ReportReasonOrderDeliveryDispute = "order_delivery_dispute"
	ReportReasonOther                = "other"

	ReportStatusSubmitted     = "submitted"
	ReportStatusTriaged       = "triaged"
	ReportStatusNeedsInfo     = "needs_info"
	ReportStatusRejected      = "rejected"
	ReportStatusDisputeOpened = "dispute_opened"
	ReportStatusClosed        = "closed"

	DisputeStatusNegotiating = "negotiating"
	DisputeStatusOpen        = "open"
	DisputeStatusWaitingInfo = "waiting_info"
	DisputeStatusResolved    = "resolved"
	DisputeStatusClosed      = "closed"

	PublicResultNoAction               = "no_action"
	PublicResultContactInvalid         = "contact_invalid"
	PublicResultImpersonationConfirmed = "impersonation_confirmed"
	PublicResultDescriptionMismatch    = "description_mismatch"
	PublicResultRuleOrSeatIssue        = "rule_or_seat_issue"
	PublicResultAPIDeliveryIssue       = "api_delivery_issue"
	PublicResultOtherResolved          = "other_resolved"

	AppealStatusSubmitted = "submitted"
	AppealStatusApproved  = "approved"
	AppealStatusRejected  = "rejected"

	InfoRequestEntityReport   = "report"
	InfoRequestEntityDispute  = "dispute"
	InfoRequestStatusOpen     = "open"
	InfoRequestStatusAnswered = "answered"
	InfoRequestStatusCanceled = "cancelled"

	DisputeMessageActionAppend   = "append_message"
	DisputeMessageActionPropose  = "create_proposal"
	DisputeMessageActionConfirm  = "confirm_proposal"
	DisputeMessageActionReject   = "reject_proposal"
	DisputeMessageActionEscalate = "escalate"
	DisputeRemedyActionClaim     = "claim_remedy"
	DisputeRemedyActionConfirm   = "confirm_remedy"
	DisputeRemedyActionContest   = "contest_remedy"

	SettlementStatusPending    = "pending"
	SettlementStatusAccepted   = "accepted"
	SettlementStatusRejected   = "rejected"
	SettlementStatusSuperseded = "superseded"

	RemedyStatusPending             = "pending"
	RemedyStatusClaimedFulfilled    = "claimed_fulfilled"
	RemedyStatusConfirmed           = "confirmed"
	RemedyStatusContested           = "contested"
	RemedyStatusConfirmationExpired = "confirmation_expired"
	RemedyStatusOverdue             = "overdue"
	RemedyStatusCancelled           = "cancelled"

	RemedyConfirmationWindow = 48 * time.Hour

	RemedyConfirmationExpiredPublicResult = "对方未在期限内反馈，平台未核验到账或履约事实"
	RemedyConfirmationExpiredNote         = "对方未在确认期限内反馈；平台未核验到账或履约事实。"
)

type Report struct {
	ID                  string
	ReporterUserID      string
	ReporterUsername    string
	ReporterName        string
	TargetType          string
	TargetID            string
	CanonicalTargetType string
	CanonicalTargetID   string
	TargetLabel         string
	TargetSnapshotJSON  string
	ReportedUsername    string
	ReasonCode          string
	Title               string
	Description         string
	Status              string
	AdminReason         string
	HandledByAdminID    string
	HandledAt           *time.Time
	DisputeID           string
	OpenInfoRequestID   string
	InfoRequestedFromID string
	Supplements         []InfoSupplement
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int64
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
	SubjectUserID        string
	SubjectUsername      string
	SubjectName          string
	Status               string
	IssueCode            string
	RequestedResolution  string
	RequestedAmountCNY   string
	IssueOccurredAt      *time.Time
	PublicSummary        string
	PublicResultCode     string
	PublicResult         string
	AdminReason          string
	OpenedByAdminID      string
	OpenedAt             time.Time
	ResolvedAt           *time.Time
	ClosedAt             *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int64
	OpenInfoRequestID    string
	InfoRequestedFromID  string
	Supplements          []InfoSupplement
	Messages             []DisputeMessage
	SettlementProposals  []SettlementProposal
	Remedies             []DisputeRemedy
}

type DisputeMessage struct {
	ID            string
	DisputeCaseID string
	SenderUserID  string
	Body          string
	CreatedAt     time.Time
}

type SettlementProposal struct {
	ID               string
	DisputeCaseID    string
	ProposedByUserID string
	Resolution       string
	AmountCNY        string
	Terms            string
	Status           string
	AcceptedByUserID string
	AcceptedAt       *time.Time
	RejectedByUserID string
	RejectedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

type DisputeRemedy struct {
	ID                    string
	DisputeCaseID         string
	Action                string
	AmountCNY             string
	Currency              string
	ResponsibleUserID     string
	BeneficiaryUserID     string
	Instructions          string
	Status                string
	DueAt                 time.Time
	ClaimedAt             *time.Time
	ConfirmationDueAt     *time.Time
	ConfirmedAt           *time.Time
	ContestedAt           *time.Time
	ConfirmationExpiredAt *time.Time
	OverdueAt             *time.Time
	ClaimNote             string
	ResponseNote          string
	CreatedByAdminID      string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Version               int64
}

type DisputeRemedyInput struct {
	Action            string
	AmountCNY         string
	ResponsibleUserID string
	Instructions      string
	DueAt             time.Time
}

type InfoRequest struct {
	ID                 string
	EntityType         string
	EntityID           string
	RequestedFromID    string
	RequestedByAdminID string
	InternalReason     string
	Status             string
	RequestedAt        time.Time
	AnsweredAt         *time.Time
	CancelledAt        *time.Time
}

type InfoSupplement struct {
	ID                  string
	InfoRequestID       string
	SubmittedByUserID   string
	SubmittedByUsername string
	SubmittedByName     string
	Body                string
	CreatedAt           time.Time
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
	Title             string
	Statement         string
}

type CreateAccountGovernanceAppealInput struct {
	AppellantUserID string
	Statement       string
}

type AppealSource struct {
	TargetType string
	TargetID   string
}

type AdminActionInput struct {
	ID               string
	AdminUserID      string
	Action           string
	Reason           string
	PublicSummary    string
	PublicResultCode string
	PublicResult     string
	ExpectedVersion  int64
	RequestID        string
	RequestedFromID  string
	Remedy           *DisputeRemedyInput
}

type SupplementInput struct {
	EntityType             string
	EntityID               string
	InfoRequestID          string
	SubmittingUserID       string
	SubmittingUsername     string
	SubmittingName         string
	Body                   string
	RequestID              string
	ActorAudience          string
	GovernanceActionID     string
	GovernanceVersion      int64
	RestrictionEffectiveAt time.Time
}

type DisputeParticipantActionInput struct {
	DisputeID              string
	ActorUserID            string
	Action                 string
	Body                   string
	Resolution             string
	AmountCNY              string
	Terms                  string
	ProposalID             string
	Note                   string
	Reason                 string
	RequestID              string
	ActorAudience          string
	GovernanceActionID     string
	GovernanceVersion      int64
	RestrictionEffectiveAt time.Time
}

func WithBusinessActor(input DisputeParticipantActionInput, actor auth.BusinessActor) DisputeParticipantActionInput {
	input.ActorUserID = actor.UserID
	input.ActorAudience = actor.Audience
	input.GovernanceActionID = actor.GovernanceActionID
	input.GovernanceVersion = actor.GovernanceVersion
	input.RestrictionEffectiveAt = actor.RestrictionEffectiveAt
	return input
}

func WithSupplementBusinessActor(input SupplementInput, actor auth.BusinessActor) SupplementInput {
	input.SubmittingUserID = actor.UserID
	input.SubmittingUsername = actor.Username
	input.SubmittingName = actor.DisplayName
	input.ActorAudience = actor.Audience
	input.GovernanceActionID = actor.GovernanceActionID
	input.GovernanceVersion = actor.GovernanceVersion
	input.RestrictionEffectiveAt = actor.RestrictionEffectiveAt
	return input
}

type MutationResult struct {
	Report  *Report
	Dispute *DisputeCase
	Appeal  *Appeal
}

type ReportCompletionBuilder func(Report) (idempotency.Completion, *domain.AppError)
type AppealCompletionBuilder func(Appeal) (idempotency.Completion, *domain.AppError)
type AdminCompletionBuilder func(MutationResult) (idempotency.Completion, *domain.AppError)
type SupplementCompletionBuilder func(MutationResult) (idempotency.Completion, *domain.AppError)
type DisputeParticipantCompletionBuilder func(DisputeCase) (idempotency.Completion, *domain.AppError)
