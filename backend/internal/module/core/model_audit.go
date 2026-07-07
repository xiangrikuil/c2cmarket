package core

import (
	"context"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

func (s *Service) AdminModelAuditTargets(ctx context.Context, user auth.User) ([]ModelAuditTarget, *domain.AppError) {
	return s.modelAudit.AdminTargets(ctx, user)
}

func (s *Service) AdminModelAuditTarget(ctx context.Context, user auth.User, targetID string) (ModelAuditTarget, *domain.AppError) {
	return s.modelAudit.AdminTarget(ctx, user, targetID)
}

func (s *Service) CreateModelAuditTarget(ctx context.Context, user auth.User, input ModelAuditTargetInput) (ModelAuditTarget, *domain.AppError) {
	return s.modelAudit.CreateTarget(ctx, user, input)
}

func (s *Service) UpdateModelAuditTarget(ctx context.Context, user auth.User, targetID string, input ModelAuditTargetInput) (ModelAuditTarget, *domain.AppError) {
	return s.modelAudit.UpdateTarget(ctx, user, targetID, input)
}

func (s *Service) DeleteModelAuditTarget(ctx context.Context, user auth.User, targetID string) *domain.AppError {
	return s.modelAudit.DeleteTarget(ctx, user, targetID)
}

func (s *Service) AdminModelAuditBaselines(ctx context.Context, user auth.User) ([]ModelAuditBaseline, *domain.AppError) {
	return s.modelAudit.AdminBaselines(ctx, user)
}

func (s *Service) AdminModelAuditBaseline(ctx context.Context, user auth.User, baselineID string) (ModelAuditBaseline, *domain.AppError) {
	return s.modelAudit.AdminBaseline(ctx, user, baselineID)
}

func (s *Service) CreateModelAuditBaseline(ctx context.Context, user auth.User, input ModelAuditBaselineInput) (ModelAuditBaseline, *domain.AppError) {
	return s.modelAudit.CreateBaseline(ctx, user, input)
}

func (s *Service) AdminModelAuditRuns(ctx context.Context, user auth.User) ([]ModelAuditRun, *domain.AppError) {
	return s.modelAudit.AdminRuns(ctx, user)
}

func (s *Service) AdminModelAuditRun(ctx context.Context, user auth.User, runID string) (ModelAuditRun, *domain.AppError) {
	return s.modelAudit.AdminRun(ctx, user, runID)
}

func (s *Service) CreateModelAuditRun(ctx context.Context, user auth.User, input ModelAuditRunInput) (ModelAuditRun, *domain.AppError) {
	return s.modelAudit.CreateRun(ctx, user, input)
}

func (s *Service) CancelModelAuditRun(ctx context.Context, user auth.User, runID string) (ModelAuditRun, *domain.AppError) {
	return s.modelAudit.CancelRun(ctx, user, runID)
}

func (s *Service) AdminModelAuditReport(ctx context.Context, user auth.User, runID string) (ModelAuditReport, *domain.AppError) {
	return s.modelAudit.AdminReport(ctx, user, runID)
}

func (s *Service) AdminModelAuditMonitors(ctx context.Context, user auth.User) ([]ModelAuditMonitor, *domain.AppError) {
	return s.modelAudit.AdminMonitors(ctx, user)
}

func (s *Service) CreateModelAuditMonitor(ctx context.Context, user auth.User, input ModelAuditMonitorInput) (ModelAuditMonitor, *domain.AppError) {
	return s.modelAudit.CreateMonitor(ctx, user, input)
}
