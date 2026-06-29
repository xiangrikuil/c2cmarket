package demand

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateDemand(ctx context.Context, item Demand) *domain.AppError
	ListPublicDemands(ctx context.Context) ([]Demand, *domain.AppError)
	GetPublicDemand(ctx context.Context, id string) (Demand, *domain.AppError)
	ListDemandsByPublisher(ctx context.Context, publisherUserID string) ([]Demand, *domain.AppError)
	GetDemandForPublisher(ctx context.Context, publisherUserID, id string) (Demand, *domain.AppError)
	ListAdminDemands(ctx context.Context) ([]Demand, *domain.AppError)
	GetAdminDemand(ctx context.Context, id string) (Demand, *domain.AppError)
	UpdateDemandOwnerStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, input OwnerActionInput, now time.Time, buildCompletion CompletionBuilder) (Demand, idempotency.Completion, *domain.AppError)
	UpdateDemandAdminStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, input AdminActionInput, now time.Time, buildCompletion CompletionBuilder) (Demand, idempotency.Completion, *domain.AppError)
}
