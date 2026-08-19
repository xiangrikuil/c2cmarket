package apimarket

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	GetAPIAccountPaymentSettings(ctx context.Context, userID string) (AccountPaymentSettings, *domain.AppError)
	UpdateAPIAccountPaymentSettings(ctx context.Context, input UpdateAccountPaymentSettingsInput, now time.Time) (AccountPaymentSettings, *domain.AppError)
	CreateAPIService(ctx context.Context, service Service, requestID string) *domain.AppError
	CreateAPIServiceWithIdempotency(ctx context.Context, entry idempotency.Entry, service Service, requestID string, now time.Time, buildCompletion ServiceCompletionBuilder) (Service, idempotency.Completion, *domain.AppError)
	ListPublicAPIServices(ctx context.Context, filter PublicServiceFilter, page domain.PageRequest) (domain.Page[Service], *domain.AppError)
	ListPublicAPIPackageFilterAvailability(ctx context.Context) (PublicPackageFilterAvailability, *domain.AppError)
	GetPublicAPIService(ctx context.Context, serviceID string) (Service, *domain.AppError)
	ListAPIServicesByOwner(ctx context.Context, ownerUserID string, filter OwnerServiceFilter, page domain.PageRequest) (domain.Page[Service], *domain.AppError)
	GetAPIServiceForOwner(ctx context.Context, ownerUserID, serviceID string) (Service, *domain.AppError)
	ListAdminAPIServices(ctx context.Context, filter AdminServiceFilter, page domain.PageRequest) (domain.Page[Service], *domain.AppError)
	GetAdminAPIService(ctx context.Context, serviceID string) (Service, *domain.AppError)
	UpdateAPIService(ctx context.Context, input UpdateServiceInput, service Service, now time.Time) (Service, *domain.AppError)
	UpdateAPIServiceWithIdempotency(ctx context.Context, entry idempotency.Entry, input UpdateServiceInput, service Service, now time.Time, buildCompletion ServiceCompletionBuilder) (Service, idempotency.Completion, *domain.AppError)
	UpdateAPIServiceProbeConnection(ctx context.Context, input UpdateProbeConnectionInput, now time.Time) (Service, *domain.AppError)
	UpdateAPIServiceProbeConnectionWithIdempotency(ctx context.Context, entry idempotency.Entry, input UpdateProbeConnectionInput, now time.Time, buildCompletion ServiceCompletionBuilder) (Service, idempotency.Completion, *domain.AppError)
	UpdateAPIServiceOrderSettings(ctx context.Context, input UpdateOrderSettingsInput, now time.Time) (Service, *domain.AppError)
	UpdateAPIServiceOrderSettingsWithIdempotency(ctx context.Context, entry idempotency.Entry, input UpdateOrderSettingsInput, now time.Time, buildCompletion ServiceCompletionBuilder) (Service, idempotency.Completion, *domain.AppError)
	SubmitAPIServiceForReview(ctx context.Context, user auth.User, input ServiceOwnerActionInput, now time.Time) (Service, *domain.AppError)
	SubmitAPIServiceForReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, user auth.User, input ServiceOwnerActionInput, now time.Time, buildCompletion ServiceCompletionBuilder) (Service, idempotency.Completion, *domain.AppError)
	UpdateAPIServicePublication(ctx context.Context, input ServiceOwnerActionInput, action string, now time.Time) (Service, *domain.AppError)
	UpdateAPIServicePublicationWithIdempotency(ctx context.Context, entry idempotency.Entry, input ServiceOwnerActionInput, action string, now time.Time, buildCompletion ServiceCompletionBuilder) (Service, idempotency.Completion, *domain.AppError)
	UpdateAPIServiceModeration(ctx context.Context, user auth.User, input ServiceAdminActionInput, now time.Time) (Service, *domain.AppError)
	UpdateAPIServiceModerationWithIdempotency(ctx context.Context, entry idempotency.Entry, user auth.User, input ServiceAdminActionInput, now time.Time, buildCompletion ServiceCompletionBuilder) (Service, idempotency.Completion, *domain.AppError)
}
