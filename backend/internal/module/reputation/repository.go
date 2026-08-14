package reputation

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	AggregateFacts(ctx context.Context, userIDs []string, now time.Time) (map[string]RawFacts, *domain.AppError)
	ExcludeTransaction(ctx context.Context, input ExclusionMutation, now time.Time) (TransactionExclusion, *domain.AppError)
	RestoreTransaction(ctx context.Context, input ExclusionMutation, now time.Time) (TransactionExclusion, *domain.AppError)
	CreateDisputeOutcomeWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateOutcomeInput, now time.Time, buildCompletion GovernanceCompletionBuilder) (GovernanceMutationResult, idempotency.Completion, *domain.AppError)
	CreateUserRestrictionWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateRestrictionInput, now time.Time, buildCompletion GovernanceCompletionBuilder) (GovernanceMutationResult, idempotency.Completion, *domain.AppError)
	RevokeUserRestrictionWithIdempotency(ctx context.Context, entry idempotency.Entry, input RevokeRestrictionInput, now time.Time, buildCompletion GovernanceCompletionBuilder) (GovernanceMutationResult, idempotency.Completion, *domain.AppError)
	FindActiveRestriction(ctx context.Context, userID, role, action string, now time.Time) (*UserRestriction, *domain.AppError)
}

type EngineRepository interface {
	LoadReputationSnapshots(ctx context.Context, keys []SnapshotKey) (map[SnapshotKey]ReputationSnapshot, *domain.AppError)
	SaveReputationSnapshots(ctx context.Context, snapshots []ReputationSnapshot) *domain.AppError
	ListReputationUserIDs(ctx context.Context) ([]string, *domain.AppError)
	ListReputationHistory(ctx context.Context, userID string, limit int) ([]ReputationHistory, *domain.AppError)
}

type SourceAuthorRepository interface {
	GetSourceAuthorVerificationAudit(ctx context.Context, resourceType, resourceID string, now time.Time) (SourceAuthorVerificationAudit, *domain.AppError)
	UpdateSourceAuthorVerification(ctx context.Context, input UpdateSourceAuthorVerificationInput, now time.Time) (SourceAuthorVerificationAudit, *domain.AppError)
}

type AuditRepository interface {
	LoadAdminReputationEvidence(ctx context.Context, userID string, now time.Time) (AdminReputationEvidence, *domain.AppError)
}

type APIOrderSanctionRepository interface {
	GetAPIOrderSanctionRecommendation(ctx context.Context, disputeCaseID string, now time.Time) (APIOrderSanctionRecommendation, *domain.AppError)
	ApplyAPIOrderSanctionWithIdempotency(ctx context.Context, entry idempotency.Entry, input ApplyAPIOrderSanctionInput, now time.Time, buildCompletion GovernanceCompletionBuilder) (GovernanceMutationResult, idempotency.Completion, *domain.AppError)
}

type ActiveRestrictionRepository interface {
	ListActiveRestrictions(ctx context.Context, userID string, now time.Time) ([]UserRestriction, *domain.AppError)
}
