package favorite

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type CompletionBuilder func(MutationResult) (idempotency.Completion, *domain.AppError)

type Repository interface {
	ListFavorites(ctx context.Context, userID string) ([]ListItem, *domain.AppError)
	IsFavorite(ctx context.Context, userID, targetType, targetID string) (bool, *domain.AppError)
	CreateFavoriteWithIdempotency(ctx context.Context, entry idempotency.Entry, userID, targetType, targetID string, now time.Time, buildCompletion CompletionBuilder) (MutationResult, idempotency.Completion, *domain.AppError)
	DeleteFavorite(ctx context.Context, userID, targetType, targetID string) (MutationResult, *domain.AppError)
}
