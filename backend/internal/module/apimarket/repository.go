package apimarket

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

type Repository interface {
	CreateAPIService(ctx context.Context, service Service) *domain.AppError
	ListPublicAPIServices(ctx context.Context, filter PublicServiceFilter) ([]Service, *domain.AppError)
	GetPublicAPIService(ctx context.Context, serviceID string) (Service, *domain.AppError)
	ListAPIServicesByOwner(ctx context.Context, ownerUserID string, page domain.PageRequest) (domain.Page[Service], *domain.AppError)
	GetAPIServiceForOwner(ctx context.Context, ownerUserID, serviceID string) (Service, *domain.AppError)
	ListAdminAPIServices(ctx context.Context, page domain.PageRequest) (domain.Page[Service], *domain.AppError)
	GetAdminAPIService(ctx context.Context, serviceID string) (Service, *domain.AppError)
	UpdateAPIService(ctx context.Context, input UpdateServiceInput, service Service, now time.Time) (Service, *domain.AppError)
	UpdateAPIServiceOrderSettings(ctx context.Context, input UpdateOrderSettingsInput, now time.Time) (Service, *domain.AppError)
	SubmitAPIServiceForReview(ctx context.Context, user auth.User, input ServiceOwnerActionInput, now time.Time) (Service, *domain.AppError)
	UpdateAPIServicePublication(ctx context.Context, input ServiceOwnerActionInput, action string, now time.Time) (Service, *domain.AppError)
	UpdateAPIServiceModeration(ctx context.Context, user auth.User, input ServiceAdminActionInput, now time.Time) (Service, *domain.AppError)
}
