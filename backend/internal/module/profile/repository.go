package profile

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	GetUserProfile(ctx context.Context, userID string, now time.Time) (UserProfile, *domain.AppError)
	UpdateUserProfile(ctx context.Context, input UpdateUserProfileInput, now time.Time) (UserProfile, *domain.AppError)
	CreateEmailVerificationCode(ctx context.Context, input EmailVerificationStartInput, codeHash string, expiresAt, now time.Time) *domain.AppError
	ConfirmEmailVerificationCode(ctx context.Context, input EmailVerificationConfirmInput, codeHash string, now time.Time) (UserProfile, *domain.AppError)
	GetPublicUserProfile(ctx context.Context, username string, now time.Time) (PublicUserProfile, *domain.AppError)
	GetMerchantProfile(ctx context.Context, ownerUserID string, now time.Time) (MerchantProfile, *domain.AppError)
	UpsertMerchantProfile(ctx context.Context, input UpsertMerchantProfileInput, now time.Time) (MerchantProfile, *domain.AppError)
	GetPublicMerchantProfile(ctx context.Context, slug string, now time.Time) (PublicMerchantProfile, *domain.AppError)
}
