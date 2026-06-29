package carpool

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	ListingStatusDraft            = "draft"
	ListingStatusPendingReview    = "pending_review"
	ListingStatusChangesRequested = "changes_requested"
	ListingStatusActive           = "active"
	ListingStatusPaused           = "paused"
	ListingStatusRejected         = "rejected"
	ListingStatusRemoved          = "removed"

	ApplicationStatusPendingOwner     = "pending_owner"
	ApplicationStatusAcceptedReserved = "accepted_reserved"
	ApplicationStatusJoined           = "joined"
	ApplicationStatusRejected         = "rejected"
	ApplicationStatusCancelledByBuyer = "cancelled_by_buyer"
	ApplicationStatusCancelledByOwner = "cancelled_by_owner"
	ApplicationStatusExpired          = "expired"

	JoinActorBuyer = "buyer"
	JoinActorOwner = "owner"

	MembershipStatusActive    = "active"
	MembershipStatusCompleted = "completed"
	MembershipStatusLeft      = "left"
	MembershipStatusRemoved   = "removed"
)

const (
	ContactWindowDuration    = 30 * time.Minute
	JoinConfirmationDuration = ContactWindowDuration
)

type RiskAcknowledgement struct {
	RiskNoticeCode string
	PolicyVersion  int64
	AcknowledgedAt time.Time
}

type Listing struct {
	ID                   string
	OwnerUserID          string
	ProductPlanID        string
	OwnerContactMethodID string
	CycleTerm            *CycleTerm
	Title                string
	Summary              string
	AccessArrangement    string
	SourceURL            string
	PriceMonthlyCNY      string
	ServiceMultiplier    string
	MonthlyQuotaAmount   string
	QuotaLabel           string
	QuotaUnit            string
	QuotaPeriod          string
	BuyerSeatCapacity    int
	ActiveBuyerMembers   int
	Status               string
	ReviewedByAdminID    string
	ReviewedAt           *time.Time
	ReviewReason         string
	PolicyVersion        int64
	RiskNoticeCode       string
	RiskAckRequired      bool
	ReservedSeats        int
	AvailableSeats       int
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int64
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
	ID                       string
	CarpoolListingID         string
	BuyerUserID              string
	OwnerUserID              string
	ProductPlanID            string
	BuyerContactMethodID     string
	Status                   string
	SeatCount                int
	ListingTitleSnapshot     string
	PriceMonthlyCNY          string
	PolicyVersionSnapshot    int64
	RiskNoticeCode           string
	ContactSessionID         string
	ReservationExpiresAt     *time.Time
	JoinConfirmationDeadline *time.Time
	BuyerConfirmedAt         *time.Time
	OwnerConfirmedAt         *time.Time
	JoinedAt                 *time.Time
	DecisionReason           string
	DecidedAt                *time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
	Version                  int64
}

type Membership struct {
	ID                    string
	CarpoolListingID      string
	CarpoolApplicationID  string
	CycleTermID           string
	BuyerUserID           string
	OwnerUserID           string
	ProductPlanID         string
	Status                string
	SeatCount             int
	PriceMonthlyCNY       string
	PolicyVersionSnapshot int64
	RiskNoticeCode        string
	JoinedAt              time.Time
	BuyerCompletedAt      *time.Time
	OwnerCompletedAt      *time.Time
	CompletedAt           *time.Time
	EndedAt               *time.Time
	EndedReason           string
	EndedByUserID         string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Version               int64
}

type CreateListingInput struct {
	OwnerUserID          string
	ProductPlanID        string
	OwnerContactMethodID string
	CycleTerm            CycleTermInput
	Title                string
	Summary              string
	AccessArrangement    string
	SourceURL            string
	PriceMonthlyCNY      string
	ServiceMultiplier    string
	MonthlyQuotaAmount   string
	BuyerSeatCapacity    int
	ActiveBuyerMembers   int
	RiskAcknowledgement  *RiskAcknowledgement
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
	ListingID            string
	OwnerUserID          string
	ProductPlanID        string
	OwnerContactMethodID string
	CycleTerm            CycleTermInput
	Title                string
	Summary              string
	AccessArrangement    string
	SourceURL            string
	PriceMonthlyCNY      string
	ServiceMultiplier    string
	MonthlyQuotaAmount   string
	BuyerSeatCapacity    int
	ActiveBuyerMembers   int
	RiskAcknowledgement  *RiskAcknowledgement
	ExpectedVersion      int64
	RequestID            string
}

type SubmitListingReviewInput struct {
	ListingID       string
	OwnerUserID     string
	ExpectedVersion int64
	RequestID       string
}

type CreateApplicationInput struct {
	ListingID            string
	BuyerUserID          string
	BuyerContactMethodID string
	RiskAcknowledgement  *RiskAcknowledgement
}

type AcceptApplicationInput struct {
	ApplicationID   string
	OwnerUserID     string
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

type WithdrawAcceptanceInput struct {
	ApplicationID   string
	OwnerUserID     string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type ConfirmApplicationJoinInput struct {
	ApplicationID   string
	ActorUserID     string
	ActorRole       string
	ExpectedVersion int64
	RequestID       string
}

type ApplicationCompletionBuilder func(Application) (idempotency.Completion, *domain.AppError)

type ConfirmMembershipCompleteInput struct {
	MembershipID    string
	ActorUserID     string
	ActorRole       string
	ExpectedVersion int64
	RequestID       string
}

type EndMembershipInput struct {
	MembershipID    string
	ActorUserID     string
	ActorRole       string
	TargetStatus    string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type MembershipCompletionBuilder func(Membership) (idempotency.Completion, *domain.AppError)
