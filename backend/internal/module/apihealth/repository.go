package apihealth

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	GetOwnerProbeConfig(ctx context.Context, ownerUserID, serviceID string) (Config, bool, *domain.AppError)
	UpsertOwnerProbeConfig(ctx context.Context, mutation ConfigMutation, credential *string, expectedVersion int64) (Config, *domain.AppError)
	DeleteOwnerProbeConfig(ctx context.Context, ownerUserID, serviceID string, expectedVersion int64, now time.Time) *domain.AppError
	CreateProbeChallenge(ctx context.Context, ownerUserID, serviceID, method string, tokenHash []byte, expiresAt time.Time, expectedVersion int64, now time.Time) (Config, *domain.AppError)
	GetProbeChallenge(ctx context.Context, ownerUserID, serviceID string) (StoredChallenge, *domain.AppError)
	CompleteProbeVerification(ctx context.Context, ownerUserID, serviceID, method string, expectedVersion int64, succeeded bool, reason string, now time.Time) (Config, *domain.AppError)
	ListAdminProbeConfigs(ctx context.Context, status string, page domain.PageRequest) (domain.Page[Config], *domain.AppError)
	AdminDecideProbeConfig(ctx context.Context, adminUserID, configID string, expectedVersion int64, approve bool, reason string, now time.Time) (Config, *domain.AppError)
	LoadProbeSummaryInputs(ctx context.Context, serviceIDs []string, since time.Time) (map[string]SummaryInput, *domain.AppError)
	ClaimDueProbes(ctx context.Context, slotStartedAt, now time.Time, limit int, runningTimeout time.Duration) ([]ProbeJob, *domain.AppError)
	FinalizeProbe(ctx context.Context, sampleID string, result ProbeResult, finishedAt time.Time) (bool, *domain.AppError)
	DeleteFinalProbeSamplesBefore(ctx context.Context, cutoff time.Time, limit int) (int, *domain.AppError)
}
