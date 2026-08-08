package apimodeltest

import (
	"context"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	ListAPIModelTestOrderSources(ctx context.Context, buyerUserID string) ([]OrderSource, *domain.AppError)
	GetAPIModelTestOrderCredential(ctx context.Context, buyerUserID, orderID string) (OrderCredential, *domain.AppError)
}
