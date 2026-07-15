package auth

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	EnsureUser(ctx context.Context, username string, isAdmin bool, now time.Time) (User, *domain.AppError)
	UserByID(ctx context.Context, userID string) (User, *domain.AppError)
	ListAdminUsers(ctx context.Context) ([]AdminUser, *domain.AppError)
	UpsertOAuthUser(ctx context.Context, profile OAuthProfile, now time.Time) (OAuthUserResult, *domain.AppError)
	BootstrapAdminPassword(ctx context.Context, credential PasswordCredential, now time.Time) (BootstrapAdminResult, *domain.AppError)
	PasswordCredential(ctx context.Context, username string) (PasswordCredential, *domain.AppError)
	PasswordCredentialByUserID(ctx context.Context, userID string) (PasswordCredential, *domain.AppError)
	UpsertPasswordCredential(ctx context.Context, credential PasswordCredential, now time.Time) *domain.AppError
	CreateEmailRegistrationCode(ctx context.Context, input EmailRegistrationStartInput, codeHash string, expiresAt, now time.Time) *domain.AppError
	ConfirmEmailRegistration(ctx context.Context, input EmailRegistrationConfirmInput, codeHash, sessionTokenHash, csrfTokenHash string, sessionExpiresAt, now time.Time) (User, *domain.AppError)
	CreateSession(ctx context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, now time.Time) *domain.AppError
	GetSession(ctx context.Context, sessionTokenHash string, now time.Time) (User, Session, *domain.AppError)
	GetSessionWithCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (User, Session, *domain.AppError)
	RefreshSessionCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) *domain.AppError
	RevokeSession(ctx context.Context, sessionTokenHash string, revokedAt time.Time) *domain.AppError
}
