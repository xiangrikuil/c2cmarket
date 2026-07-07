package modelaudit

import (
	"context"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	CreateModelAuditTarget(ctx context.Context, target Target, apiKey string) (Target, *domain.AppError)
	ListModelAuditTargets(ctx context.Context) ([]Target, *domain.AppError)
	GetModelAuditTarget(ctx context.Context, targetID string) (Target, *domain.AppError)
	GetModelAuditTargetSecret(ctx context.Context, targetID string) (Target, string, *domain.AppError)
	UpdateModelAuditTarget(ctx context.Context, target Target, apiKey *string) (Target, *domain.AppError)
	DeleteModelAuditTarget(ctx context.Context, targetID string) *domain.AppError

	CreateModelAuditBaseline(ctx context.Context, baseline Baseline) (Baseline, *domain.AppError)
	ListModelAuditBaselines(ctx context.Context) ([]Baseline, *domain.AppError)
	GetModelAuditBaseline(ctx context.Context, baselineID string) (Baseline, *domain.AppError)

	CreateModelAuditRun(ctx context.Context, run Run) (Run, *domain.AppError)
	ListModelAuditRuns(ctx context.Context) ([]Run, *domain.AppError)
	GetModelAuditRun(ctx context.Context, runID string) (Run, *domain.AppError)
	UpdateModelAuditRun(ctx context.Context, run Run) (Run, *domain.AppError)
	CancelModelAuditRun(ctx context.Context, runID string) (Run, *domain.AppError)

	CreateModelAuditSample(ctx context.Context, sample Sample) (Sample, *domain.AppError)
	ListModelAuditSamples(ctx context.Context, runID string) ([]Sample, *domain.AppError)
	CreateModelAuditProbeScore(ctx context.Context, runID string, score ProbeScore) *domain.AppError
	ListModelAuditProbeScores(ctx context.Context, runID string) ([]ProbeScore, *domain.AppError)

	CreateModelAuditMonitor(ctx context.Context, monitor Monitor) (Monitor, *domain.AppError)
	ListModelAuditMonitors(ctx context.Context) ([]Monitor, *domain.AppError)
}
