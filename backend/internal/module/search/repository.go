package search

import (
	"context"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	Search(ctx context.Context, keyword string, perTypeLimit int) ([]Result, *domain.AppError)
}
