package feedback

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateFeedbackTicketWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateInput, now time.Time, buildCompletion CompletionBuilder) (Ticket, idempotency.Completion, *domain.AppError)
	ListFeedbackTicketsBySubmitter(ctx context.Context, submitterUserID string, page domain.PageRequest) (domain.Page[Ticket], *domain.AppError)
	GetFeedbackTicketForSubmitter(ctx context.Context, submitterUserID, id string) (Ticket, *domain.AppError)
	AddFeedbackSupplementWithIdempotency(ctx context.Context, entry idempotency.Entry, input SupplementInput, now time.Time, buildCompletion CompletionBuilder) (Ticket, idempotency.Completion, *domain.AppError)
	MarkFeedbackRead(ctx context.Context, submitterUserID, id string, now time.Time) (Ticket, *domain.AppError)
	UnreadFeedbackCount(ctx context.Context, submitterUserID string) (int, *domain.AppError)
	ListAdminFeedbackTickets(ctx context.Context) ([]Ticket, *domain.AppError)
	GetAdminFeedbackTicket(ctx context.Context, id string) (Ticket, *domain.AppError)
	HandleAdminFeedbackTicketWithIdempotency(ctx context.Context, entry idempotency.Entry, input AdminHandleInput, now time.Time, buildCompletion CompletionBuilder) (Ticket, idempotency.Completion, *domain.AppError)
}
