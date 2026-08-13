package accountgovernance

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	BusinessCenter(ctx context.Context, userID string, now time.Time) (Center, *domain.AppError)
}
