package apiorder

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateAPIOrderWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateInput, now time.Time, buildCompletion CompletionBuilder) (Order, idempotency.Completion, *domain.AppError)
	ListAPIOrdersByBuyer(ctx context.Context, buyerUserID string, now time.Time) ([]Order, *domain.AppError)
	GetAPIOrderForBuyer(ctx context.Context, buyerUserID, orderID string, now time.Time) (Order, *domain.AppError)
	ReadAPIOrderPaymentInstructions(ctx context.Context, buyerUserID, orderID, requestID string, now time.Time) (PaymentInstructionsView, *domain.AppError)
	ListAPIOrdersBySeller(ctx context.Context, sellerUserID string, now time.Time) ([]Order, *domain.AppError)
	ListAdminAPIOrders(ctx context.Context, now time.Time) ([]Order, *domain.AppError)
	GetAPIOrderForSeller(ctx context.Context, sellerUserID, orderID string, now time.Time) (Order, *domain.AppError)
	SubmitAPIOrderPaymentWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Order, idempotency.Completion, *domain.AppError)
	CancelAPIOrderWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Order, idempotency.Completion, *domain.AppError)
	ConfirmAPIOrderCompleteWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Order, idempotency.Completion, *domain.AppError)
	OpenAPIOrderDisputeWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Order, idempotency.Completion, *domain.AppError)
	ConfirmAPIOrderPaymentWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Order, idempotency.Completion, *domain.AppError)
	ReportAPIOrderPaymentIssueWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Order, idempotency.Completion, *domain.AppError)
	SubmitAPIOrderDeliveryWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Order, idempotency.Completion, *domain.AppError)
}
