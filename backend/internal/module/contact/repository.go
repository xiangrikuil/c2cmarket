package contact

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateContactMethod(ctx context.Context, input ContactMethodInput, method ContactMethod, version ContactMethodVersion) *domain.AppError
	CreateContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, input ContactMethodInput, method ContactMethod, version ContactMethodVersion, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, *domain.AppError)
	ListContactMethods(ctx context.Context, userID string) ([]ContactMethod, *domain.AppError)
	UpdateContactMethod(ctx context.Context, input UpdateContactMethodInput, method ContactMethod, version ContactMethodVersion) (ContactMethod, *domain.AppError)
	UpdateContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, input UpdateContactMethodInput, method ContactMethod, version ContactMethodVersion, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, *domain.AppError)
	DeleteContactMethod(ctx context.Context, userID, methodID, requestID string, now time.Time) (ContactMethod, *domain.AppError)
	DeleteContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, userID, methodID, requestID string, now time.Time, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, *domain.AppError)
	SetDefaultContactMethod(ctx context.Context, userID, methodID, requestID string, now time.Time) (ContactMethod, *domain.AppError)
	SetDefaultContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, userID, methodID, requestID string, now time.Time, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, *domain.AppError)
	VerifyContactMethod(ctx context.Context, userID, methodID, requestID string, verifiedAt time.Time) (ContactMethod, *domain.AppError)
	VerifyContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, userID, methodID, requestID string, verifiedAt time.Time, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, *domain.AppError)
	CreateContactSession(ctx context.Context, input CreateContactSessionInput, session ContactSession, now time.Time) (ContactSession, *domain.AppError)
	ContactSessionViewerRole(ctx context.Context, sessionID, viewerUserID string) (string, *domain.AppError)
	ReadContactSession(ctx context.Context, sessionID, viewerUserID, requestID string, now time.Time) (ContactSessionView, *domain.AppError)
	ContactAccessLogCount(ctx context.Context, sessionID string) (int, *domain.AppError)
}
