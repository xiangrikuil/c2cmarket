package review

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	SourceCarpoolMembership = "carpool_membership"
	ReviewerRoleBuyer       = "buyer"
	RevieweeRoleOwner       = "owner"
	StatusReviewable        = "reviewable"
	StatusReviewed          = "reviewed"
)

type Review struct {
	ID             string
	SourceType     string
	SourceID       string
	ReviewerUserID string
	RevieweeUserID string
	ReviewerRole   string
	RevieweeRole   string
	Rating         int
	Tags           []string
	Note           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ReviewCenterRow struct {
	ID                   string
	SourceType           string
	SourceID             string
	Target               string
	CounterpartyUsername string
	CounterpartyName     string
	Status               string
	Rating               int
	Tags                 []string
	Note                 string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type PublicReview struct {
	ID          string
	Username    string
	Date        time.Time
	ServiceType string
	Rating      int
	Tags        []string
	Note        string
	Verified    bool
}

type SubmitReviewInput struct {
	SourceType     string
	SourceID       string
	ReviewerUserID string
	Rating         int
	Tags           []string
	Note           string
}

type MutationResult struct {
	Row ReviewCenterRow
}

type CompletionBuilder func(MutationResult) (idempotency.Completion, *domain.AppError)
