package review

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	TransactionCarpoolMembership = "carpool_membership"
	TransactionAPIOrder          = "api_order"

	RoleBuyer  = "buyer"
	RoleSeller = "seller"

	StatusSealed    = "sealed"
	StatusPublished = "published"
	StatusRemoved   = "removed"

	CenterStatusReviewable = "reviewable"
	CenterStatusPaused     = "paused"
	CenterStatusExpired    = "expired"

	DirectionPending  = "pending"
	DirectionSent     = "sent"
	DirectionReceived = "received"

	VisibilityNone      = "none"
	VisibilitySealed    = "sealed"
	VisibilityPublished = "published"
	VisibilityRemoved   = "removed"

	OperationCreate       = "create"
	OperationEdit         = "edit"
	OperationLegacyUpsert = "legacy_upsert"

	ReviewWindow = 14 * 24 * time.Hour

	// SourceCarpoolMembership remains an alias for the retained legacy PUT route.
	SourceCarpoolMembership = TransactionCarpoolMembership
)

type TagDefinition struct {
	Code     string
	Label    string
	Polarity string
}

const (
	APIOrderOutcomeNormalFulfillment    = "normal_fulfillment"
	APIOrderOutcomeFullRefund           = "full_refund"
	APIOrderOutcomePartialRefund        = "partial_refund"
	APIOrderOutcomeContinuedFulfillment = "continued_fulfillment"
	APIOrderOutcomeLegacyFulfillment    = "legacy_fulfillment"
)

func IsReviewableAPIOrderOutcome(value string) bool {
	switch value {
	case APIOrderOutcomeNormalFulfillment, APIOrderOutcomeFullRefund,
		APIOrderOutcomePartialRefund, APIOrderOutcomeContinuedFulfillment:
		return true
	default:
		return false
	}
}

func ReviewDeadlineForAPIOrder(commercialOutcomeAt time.Time) time.Time {
	return commercialOutcomeAt.Add(ReviewWindow)
}

type Transaction struct {
	Type              string
	ID                string
	Target            string
	BuyerUserID       string
	BuyerUsername     string
	BuyerDisplayName  string
	SellerUserID      string
	SellerUsername    string
	SellerDisplayName string
	CompletedAt       time.Time
	ReviewDeadlineAt  time.Time
	CommercialOutcome string
	ReviewPaused      bool
}

type Review struct {
	ID                  string
	TransactionType     string
	CarpoolMembershipID string
	APIOrderID          string
	ReviewerUserID      string
	RevieweeUserID      string
	ReviewerRole        string
	RevieweeRole        string
	Rating              int
	Tags                []string
	Note                string
	Status              string
	ReviewDeadlineAt    time.Time
	CommercialOutcome   string
	VisibleAt           *time.Time
	FrozenAt            *time.Time
	RemovedAt           *time.Time
	RemovedByAdminID    string
	RemovalReason       string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int64
}

type ReviewCenterRow struct {
	ID                    string
	TransactionType       string
	TransactionID         string
	Direction             string
	Target                string
	CounterpartyUsername  string
	CounterpartyName      string
	ReviewerRole          string
	RevieweeRole          string
	Status                string
	Visibility            string
	CounterpartySubmitted bool
	AllowedTags           []TagDefinition
	CanCreate             bool
	CanEdit               bool
	ContentVisible        bool
	Rating                int
	Tags                  []string
	Note                  string
	CompletedAt           time.Time
	ReviewDeadlineAt      time.Time
	CommercialOutcome     string
	ReviewPaused          bool
	SubmittedAt           *time.Time
	VisibleAt             *time.Time
	FrozenAt              *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Version               int64
}

type PublicReview struct {
	ID                string
	ReviewerUsername  string
	Date              time.Time
	ServiceType       string
	TransactionType   string
	ReviewerRole      string
	RevieweeRole      string
	Rating            int
	Tags              []string
	Note              string
	Verified          bool
	CommercialOutcome string
}

type SubmitReviewInput struct {
	TransactionType string
	TransactionID   string
	ReviewerUserID  string
	Operation       string
	Rating          int
	Tags            []string
	Note            string
}

type RemoveReviewInput struct {
	ReviewID        string
	AdminUserID     string
	ExpectedVersion int64
	Reason          string
}

type MutationResult struct {
	Row ReviewCenterRow
}

type CompletionBuilder func(MutationResult) (idempotency.Completion, *domain.AppError)
