package contact

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	CreateContactMethod(ctx context.Context, input ContactMethodInput, method ContactMethod, version ContactMethodVersion) *domain.AppError
	ListContactMethods(ctx context.Context, userID string) ([]ContactMethod, *domain.AppError)
	UpdateContactMethod(ctx context.Context, input UpdateContactMethodInput, method ContactMethod, version ContactMethodVersion) (ContactMethod, *domain.AppError)
	DeleteContactMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError)
	SetDefaultContactMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError)
	VerifyContactMethod(ctx context.Context, userID, methodID string, verifiedAt time.Time) (ContactMethod, *domain.AppError)
	CreateContactSession(ctx context.Context, input CreateContactSessionInput, session ContactSession, now time.Time) (ContactSession, *domain.AppError)
	ReadContactSession(ctx context.Context, sessionID, viewerUserID, requestID string, now time.Time) (ContactSessionView, *domain.AppError)
	ContactAccessLogCount(ctx context.Context, sessionID string) (int, *domain.AppError)
}
