package navigationbadge

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	NavigationBadgeSummary(ctx context.Context, userID string, isAdmin bool, now time.Time) (Summary, *domain.AppError)
}
