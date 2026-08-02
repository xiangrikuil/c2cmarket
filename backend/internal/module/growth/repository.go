package growth

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	RecordActivity(ctx context.Context, userID string, activityDate string, seenAt time.Time) *domain.AppError
	GrowthOverview(ctx context.Context, asOf time.Time, windowDays int) (Overview, *domain.AppError)
}
