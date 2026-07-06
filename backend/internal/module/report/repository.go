package report

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateReportWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateReportInput, now time.Time, buildCompletion ReportCompletionBuilder) (Report, idempotency.Completion, *domain.AppError)
	ListReportsByUser(ctx context.Context, userID string) ([]Report, *domain.AppError)
	ListAdminReports(ctx context.Context, page domain.PageRequest) (domain.Page[Report], *domain.AppError)
	GetAdminReport(ctx context.Context, id string) (Report, *domain.AppError)
	UpdateReportAdminWithIdempotency(ctx context.Context, entry idempotency.Entry, input AdminActionInput, now time.Time, buildCompletion AdminCompletionBuilder) (MutationResult, idempotency.Completion, *domain.AppError)

	CreateAppealWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateAppealInput, now time.Time, buildCompletion AppealCompletionBuilder) (Appeal, idempotency.Completion, *domain.AppError)
	ListAppealsByUser(ctx context.Context, userID string) ([]Appeal, *domain.AppError)
	ListAdminAppeals(ctx context.Context) ([]Appeal, *domain.AppError)
	GetAdminAppeal(ctx context.Context, id string) (Appeal, *domain.AppError)
	UpdateAppealAdminWithIdempotency(ctx context.Context, entry idempotency.Entry, input AdminActionInput, now time.Time, buildCompletion AdminCompletionBuilder) (MutationResult, idempotency.Completion, *domain.AppError)

	ListAdminDisputes(ctx context.Context) ([]DisputeCase, *domain.AppError)
	GetAdminDispute(ctx context.Context, id string) (DisputeCase, *domain.AppError)
	UpdateDisputeAdminWithIdempotency(ctx context.Context, entry idempotency.Entry, input AdminActionInput, now time.Time, buildCompletion AdminCompletionBuilder) (MutationResult, idempotency.Completion, *domain.AppError)
	ListPublicUserDisputes(ctx context.Context, username string) ([]PublicDispute, *domain.AppError)
	PublicUserDisputeStats(ctx context.Context, username string, now time.Time) (PublicStats, *domain.AppError)
}
