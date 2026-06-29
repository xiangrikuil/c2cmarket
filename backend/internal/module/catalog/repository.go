package catalog

import (
	"context"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	ListProductCategories(ctx context.Context) ([]ProductCategory, *domain.AppError)
	GetProductCategory(ctx context.Context, categoryID string) (ProductCategory, *domain.AppError)
	AdminListProductCategories(ctx context.Context) ([]ProductCategory, *domain.AppError)
	AdminGetProductCategory(ctx context.Context, categoryID string) (ProductCategory, *domain.AppError)
	AdminCreateProductCategory(ctx context.Context, input ProductCategoryMutationInput) (ProductCategory, *domain.AppError)
	AdminUpdateProductCategory(ctx context.Context, input ProductCategoryMutationInput) (ProductCategory, *domain.AppError)
	AdminSetProductCategoryActive(ctx context.Context, input ProductCategoryMutationInput, active bool) (ProductCategory, *domain.AppError)
	ListProductPlans(ctx context.Context, categoryCode string) ([]ProductPlan, *domain.AppError)
	GetProductPlan(ctx context.Context, planID string) (ProductPlan, *domain.AppError)
	AdminListProductPlans(ctx context.Context, categoryCode string) ([]ProductPlan, *domain.AppError)
	AdminGetProductPlan(ctx context.Context, planID string) (ProductPlan, *domain.AppError)
	AdminCreateProductPlan(ctx context.Context, input ProductPlanMutationInput) (ProductPlan, *domain.AppError)
	AdminUpdateProductPlan(ctx context.Context, input ProductPlanMutationInput) (ProductPlan, *domain.AppError)
	AdminSetProductPlanActive(ctx context.Context, input ProductPlanMutationInput, active bool) (ProductPlan, *domain.AppError)
	AdminListAPIModelProviders(ctx context.Context) ([]APIModelProvider, *domain.AppError)
	AdminGetAPIModelProvider(ctx context.Context, providerID string) (APIModelProvider, *domain.AppError)
	AdminCreateAPIModelProvider(ctx context.Context, input APIModelProviderMutationInput) (APIModelProvider, *domain.AppError)
	AdminUpdateAPIModelProvider(ctx context.Context, input APIModelProviderMutationInput) (APIModelProvider, *domain.AppError)
	AdminSetAPIModelProviderActive(ctx context.Context, input APIModelProviderMutationInput, active bool) (APIModelProvider, *domain.AppError)
	ListAPIModels(ctx context.Context) ([]APIModelCatalog, *domain.AppError)
	GetAPIModel(ctx context.Context, modelID string) (APIModelCatalog, *domain.AppError)
	AdminListAPIModels(ctx context.Context) ([]APIModelCatalog, *domain.AppError)
	AdminGetAPIModel(ctx context.Context, modelID string) (APIModelCatalog, *domain.AppError)
	AdminCreateAPIModel(ctx context.Context, input APIModelMutationInput) (APIModelCatalog, *domain.AppError)
	AdminUpdateAPIModel(ctx context.Context, input APIModelMutationInput) (APIModelCatalog, *domain.AppError)
	AdminSetAPIModelActive(ctx context.Context, input APIModelMutationInput, active bool) (APIModelCatalog, *domain.AppError)
}
