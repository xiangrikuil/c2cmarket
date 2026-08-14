package operationaudit

import (
	"context"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	ListOperationAudit(ctx context.Context, query Query) ([]Entry, *domain.AppError)
}
