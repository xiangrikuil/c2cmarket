package apihealth

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	ListOwnerProbeConnections(ctx context.Context, ownerUserID string) ([]Connection, *domain.AppError)
	GetOwnerProbeConnection(ctx context.Context, ownerUserID, connectionID string) (Connection, bool, *domain.AppError)
	GetOwnerProbeConnectionCredential(ctx context.Context, ownerUserID, connectionID string) (Connection, string, bool, *domain.AppError)
	CreateOwnerProbeConnection(ctx context.Context, connection Connection, credential string, audit ProbeAuditMutation) (Connection, *domain.AppError)
	UpdateOwnerProbeConnection(ctx context.Context, connection Connection, credential *string, expectedVersion int64, audit ProbeAuditMutation) (Connection, *domain.AppError)
	DeleteOwnerProbeConnection(ctx context.Context, ownerUserID, connectionID string, expectedVersion int64, audit ProbeAuditMutation) *domain.AppError
	LookupProbeModelPrice(ctx context.Context, model string) (PriceSnapshot, bool, *domain.AppError)
	LoadOwnerProbeConnectionSamples(ctx context.Context, ownerUserID string, connectionIDs []string, since time.Time) (map[string][]Sample, *domain.AppError)
	LoadProbeSummaryInputs(ctx context.Context, serviceIDs []string, since time.Time) (map[string]SummaryInput, *domain.AppError)
	ClaimDueProbes(ctx context.Context, slotStartedAt, now time.Time, limit int, runningTimeout time.Duration) ([]ProbeJob, *domain.AppError)
	FinalizeProbe(ctx context.Context, sampleID string, result ProbeResult, finishedAt time.Time) (bool, *domain.AppError)
	DeleteFinalProbeSamplesBefore(ctx context.Context, cutoff time.Time, limit int) (int, *domain.AppError)
}

// AtomicMutationRepository 将探针写入、审计事件和幂等完成记录提交在同一事务中。
type AtomicMutationRepository interface {
	CreateOwnerProbeConnectionWithIdempotency(ctx context.Context, entry idempotency.Entry, connection Connection, credential string, audit ProbeAuditMutation, buildCompletion MutationCompletionBuilder) (Connection, idempotency.Completion, *domain.AppError)
	UpdateOwnerProbeConnectionWithIdempotency(ctx context.Context, entry idempotency.Entry, connection Connection, credential *string, expectedVersion int64, audit ProbeAuditMutation, buildCompletion MutationCompletionBuilder) (Connection, idempotency.Completion, *domain.AppError)
	DeleteOwnerProbeConnectionWithIdempotency(ctx context.Context, entry idempotency.Entry, ownerUserID, connectionID string, expectedVersion int64, audit ProbeAuditMutation, buildCompletion MutationCompletionBuilder) (idempotency.Completion, *domain.AppError)
}

type CalibrationRepository interface {
	LoadProbeCalibration(ctx context.Context, model, protocol, environment string, now time.Time) (Calibration, *domain.AppError)
	PreviewProbeLatencyRule(ctx context.Context, calibration Calibration, slowTTFTMS, hardTimeoutMS int) (LatencyRulePreview, *domain.AppError)
	PublishProbeLatencyRule(ctx context.Context, rule LatencyRule) (LatencyRule, *domain.AppError)
	ListProbeLatencyRules(ctx context.Context) ([]LatencyRule, *domain.AppError)
}
