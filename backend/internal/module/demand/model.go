package demand

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	StatusPendingReview    = "pending_review"
	StatusActive           = "active"
	StatusChangesRequested = "changes_requested"
	StatusRejected         = "rejected"
	StatusClosed           = "closed"
	StatusTakenDown        = "taken_down"
)

type Demand struct {
	ID                string
	PublisherUserID   string
	PublisherUsername string
	PublisherName     string
	Title             string
	MaxPriceCNY       string
	RegionCode        string
	OwnerPreference   string
	SourceURL         string
	Note              string
	Status            string
	ReviewReason      string
	ReviewedByAdminID string
	ReviewedAt        *time.Time
	ClosedAt          *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Version           int64
}

type CreateInput struct {
	PublisherUserID string
	Title           string
	MaxPriceCNY     string
	RegionCode      string
	OwnerPreference string
	SourceURL       string
	Note            string
}

type OwnerActionInput struct {
	ID              string
	PublisherUserID string
	Action          string
	ExpectedVersion int64
	RequestID       string
}

type AdminActionInput struct {
	ID              string
	AdminUserID     string
	Action          string
	Reason          string
	ExpectedVersion int64
	RequestID       string
}

type CompletionBuilder func(Demand) (idempotency.Completion, *domain.AppError)
