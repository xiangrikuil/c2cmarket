package apipromotion

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	PlacementAPIMarketTop = "api_market_top"
	APIMarketTopCapacity  = 3

	StatusStopped    = "stopped"
	StatusScheduled  = "scheduled"
	StatusFinished   = "finished"
	StatusSuppressed = "suppressed"
	StatusServing    = "serving"
)

type Eligibility struct {
	Configurable       bool
	Displayable        bool
	HardBlockReasons   []string
	WarningReasons     []string
	SuppressionReasons []string
}

type Availability struct {
	Eligibility          Eligibility
	OverlappingCampaigns int
	Capacity             int
	RemainingCapacity    int
	SameServiceOverlap   bool
}

type Promotion struct {
	ID                   string
	APIServiceID         string
	Placement            string
	StartsAt             time.Time
	EndsAt               time.Time
	CreatedReason        string
	CreatedByAdminID     string
	StoppedAt            *time.Time
	StoppedByAdminID     string
	StoppedReason        string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int64
	Status               string
	Service              apimarket.Service
	Eligibility          Eligibility
	OverlappingCampaigns int
	Capacity             int
}

type CreateInput struct {
	APIServiceID string
	Placement    string
	StartsAt     time.Time
	EndsAt       time.Time
	Reason       string
	AdminUserID  string
	RequestID    string
}

type AvailabilityInput struct {
	APIServiceID string
	Placement    string
	StartsAt     time.Time
	EndsAt       time.Time
}

type StopInput struct {
	PromotionID     string
	Reason          string
	AdminUserID     string
	ExpectedVersion int64
	RequestID       string
}

type CompletionBuilder func(Promotion) (idempotency.Completion, *domain.AppError)
