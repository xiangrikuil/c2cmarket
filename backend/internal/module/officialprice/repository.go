package officialprice

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	FindDuplicateOfficialPriceLeadID(ctx context.Context, fingerprint string) (string, *domain.AppError)
	CreateOfficialPriceLead(ctx context.Context, lead Lead) *domain.AppError
	GetOfficialPriceLead(ctx context.Context, leadID string) (Lead, *domain.AppError)
	ListOfficialPriceLeadsBySubmitter(ctx context.Context, submitterUserID string) ([]Lead, *domain.AppError)
	ListOfficialPriceLeads(ctx context.Context) ([]Lead, *domain.AppError)
	ApproveOfficialPriceLead(ctx context.Context, input ApproveLeadInput, normalizedMonthlyCNY, offerKey string, now time.Time) (Lead, Record, *domain.AppError)
	ApproveOfficialPriceLeadWithIdempotency(ctx context.Context, entry idempotency.Entry, input ApproveLeadInput, normalizedMonthlyCNY, offerKey string, now time.Time, buildCompletion ApprovalCompletionBuilder) (Lead, Record, idempotency.Completion, *domain.AppError)
	UpdateLeadReviewStatus(ctx context.Context, user auth.User, leadID, status, reason string, ifMatchVersion int64, now time.Time) (Lead, *domain.AppError)
	ListOfficialPriceRecords(ctx context.Context) ([]Record, *domain.AppError)
	GetOfficialPriceRecord(ctx context.Context, recordID string) (Record, *domain.AppError)
}
