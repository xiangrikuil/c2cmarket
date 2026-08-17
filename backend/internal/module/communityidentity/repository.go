package communityidentity

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	GrantFounding(ctx context.Context, input GrantFoundingInput, now time.Time) (Identity, bool, *domain.AppError)
	GrantAdmin(ctx context.Context, input GrantAdminInput, now time.Time) (Identity, bool, *domain.AppError)
	Revoke(ctx context.Context, input RevokeInput, now time.Time) (Identity, *domain.AppError)
	ListForUser(ctx context.Context, userID string, includeRevoked bool) ([]Identity, *domain.AppError)
	BackfillFounding(ctx context.Context, cutoff, now time.Time) (int, *domain.AppError)
}
