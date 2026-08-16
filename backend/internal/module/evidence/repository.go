package evidence

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateReadyAssets(ctx context.Context, assets []Asset) *domain.AppError
	AuthorizedAsset(ctx context.Context, assetID, viewerUserID string, admin bool) (Asset, *domain.AppError)
	QuarantineAssetWithIdempotency(ctx context.Context, entry idempotency.Entry, input AdminQuarantineInput, now time.Time, buildCompletion AdminQuarantineCompletionBuilder) (AdminQuarantineResult, idempotency.Completion, *domain.AppError)
	ClaimDestroyCandidates(ctx context.Context, now time.Time, batchSize int) ([]DestroyCandidate, *domain.AppError)
	MarkDestroyed(ctx context.Context, assetID string, now time.Time) *domain.AppError
}
