package catalog

import (
	"context"
	"encoding/base64"
	"math/big"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

type Service struct {
	mu            sync.Mutex
	now           func() time.Time
	repo          Repository
	categories    map[string]ProductCategory
	productPlans  map[string]ProductPlan
	apiProviders  map[string]APIModelProvider
	providerOrder []string
	apiModels     map[string]APIModelCatalog
	apiModelOrder []string
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	service := &Service{
		now:          now,
		repo:         repo,
		categories:   make(map[string]ProductCategory),
		productPlans: make(map[string]ProductPlan),
		apiProviders: make(map[string]APIModelProvider),
		apiModels:    make(map[string]APIModelCatalog),
	}
	service.seedProductCategories()
	service.seedProductPlans()
	service.seedAPIModelProviders()
	service.seedAPIModels()
	return service
}

func (s *Service) ProductCategories(ctx context.Context) ([]ProductCategory, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListProductCategories(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	categories := make([]ProductCategory, 0, len(s.categories))
	for _, category := range s.categories {
		if category.Active {
			categories = append(categories, category)
		}
	}
	sortProductCategories(categories)
	return categories, nil
}

func (s *Service) ProductPlans(ctx context.Context, categoryCode string) ([]ProductPlan, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListProductPlans(ctx, categoryCode)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	categoryCode = strings.ToLower(strings.TrimSpace(categoryCode))
	plans := make([]ProductPlan, 0, len(s.productPlans))
	for _, plan := range s.productPlans {
		category, ok := s.categories[plan.CategoryID]
		if !ok || !category.Active {
			continue
		}
		if !plan.Active {
			continue
		}
		if categoryCode != "" && plan.CategoryCode != categoryCode {
			continue
		}
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(i, j int) bool {
		return productPlanLess(plans[i], plans[j])
	})
	return plans, nil
}

func (s *Service) ProductPlan(ctx context.Context, planID string) (ProductPlan, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetProductPlan(ctx, planID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	plan, ok := s.productPlans[strings.TrimSpace(planID)]
	category, categoryOK := s.categories[plan.CategoryID]
	if !ok || !plan.Active || !categoryOK || !category.Active {
		return ProductPlan{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Product plan not found", "产品套餐不存在。")
	}
	return plan, nil
}

func (s *Service) AdminProductCategories(ctx context.Context, user auth.User) ([]ProductCategory, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.AdminListProductCategories(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	categories := make([]ProductCategory, 0, len(s.categories))
	for _, category := range s.categories {
		categories = append(categories, category)
	}
	sortProductCategories(categories)
	return categories, nil
}

func (s *Service) AdminProductCategory(ctx context.Context, user auth.User, categoryID string) (ProductCategory, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ProductCategory{}, appErr
	}
	if s.repo != nil {
		return s.repo.AdminGetProductCategory(ctx, categoryID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	category, ok := s.categories[strings.TrimSpace(categoryID)]
	if !ok {
		return ProductCategory{}, productCategoryNotFound()
	}
	return category, nil
}

func (s *Service) CreateProductCategory(ctx context.Context, user auth.User, input ProductCategoryInput) (ProductCategory, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ProductCategory{}, appErr
	}
	normalized, appErr := normalizeProductCategoryInput(input)
	if appErr != nil {
		return ProductCategory{}, appErr
	}
	mutation := ProductCategoryMutationInput{OperatorID: user.ID, Form: normalized}
	if s.repo != nil {
		return s.repo.AdminCreateProductCategory(ctx, mutation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.productCategoryCodeExistsLocked("", normalized.Code) {
		return ProductCategory{}, fieldError(http.StatusConflict, "code", "分类 code 已被占用。")
	}
	category := ProductCategory{
		ID:          uuid.NewString(),
		Code:        normalized.Code,
		DisplayName: normalized.DisplayName,
		IconDataURL: normalized.IconDataURL,
		SortOrder:   normalized.SortOrder,
		Active:      normalized.Active,
	}
	s.categories[category.ID] = category
	return category, nil
}

func (s *Service) UpdateProductCategory(ctx context.Context, user auth.User, categoryID string, input ProductCategoryInput) (ProductCategory, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ProductCategory{}, appErr
	}
	normalized, appErr := normalizeProductCategoryInput(input)
	if appErr != nil {
		return ProductCategory{}, appErr
	}
	mutation := ProductCategoryMutationInput{ID: categoryID, OperatorID: user.ID, Form: normalized}
	if s.repo != nil {
		return s.repo.AdminUpdateProductCategory(ctx, mutation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	category, ok := s.categories[strings.TrimSpace(categoryID)]
	if !ok {
		return ProductCategory{}, productCategoryNotFound()
	}
	if s.productCategoryCodeExistsLocked(category.ID, normalized.Code) {
		return ProductCategory{}, fieldError(http.StatusConflict, "code", "分类 code 已被占用。")
	}
	category.Code = normalized.Code
	category.DisplayName = normalized.DisplayName
	category.IconDataURL = normalized.IconDataURL
	category.SortOrder = normalized.SortOrder
	category.Active = normalized.Active
	s.categories[category.ID] = category
	for id, plan := range s.productPlans {
		if plan.CategoryID == category.ID {
			plan.CategoryCode = category.Code
			s.productPlans[id] = plan
		}
	}
	return category, nil
}

func (s *Service) SetProductCategoryActive(ctx context.Context, user auth.User, categoryID string, active bool) (ProductCategory, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ProductCategory{}, appErr
	}
	mutation := ProductCategoryMutationInput{ID: categoryID, OperatorID: user.ID}
	if s.repo != nil {
		return s.repo.AdminSetProductCategoryActive(ctx, mutation, active)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	category, ok := s.categories[strings.TrimSpace(categoryID)]
	if !ok {
		return ProductCategory{}, productCategoryNotFound()
	}
	category.Active = active
	s.categories[category.ID] = category
	return category, nil
}

func (s *Service) AdminProductPlans(ctx context.Context, user auth.User, categoryCode string) ([]ProductPlan, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.AdminListProductPlans(ctx, categoryCode)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	categoryCode = strings.ToLower(strings.TrimSpace(categoryCode))
	plans := make([]ProductPlan, 0, len(s.productPlans))
	for _, plan := range s.productPlans {
		if categoryCode != "" && plan.CategoryCode != categoryCode {
			continue
		}
		plans = append(plans, plan)
	}
	sortProductPlans(plans)
	return plans, nil
}

func (s *Service) AdminProductPlan(ctx context.Context, user auth.User, planID string) (ProductPlan, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ProductPlan{}, appErr
	}
	if s.repo != nil {
		return s.repo.AdminGetProductPlan(ctx, planID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	plan, ok := s.productPlans[strings.TrimSpace(planID)]
	if !ok {
		return ProductPlan{}, productPlanNotFound()
	}
	return plan, nil
}

func (s *Service) CreateProductPlan(ctx context.Context, user auth.User, input ProductPlanInput) (ProductPlan, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ProductPlan{}, appErr
	}
	category, appErr := s.productCategoryForMutation(ctx, input.CategoryID)
	if appErr != nil {
		return ProductPlan{}, appErr
	}
	normalized, appErr := normalizeProductPlanInput(input)
	if appErr != nil {
		return ProductPlan{}, appErr
	}
	mutation := ProductPlanMutationInput{OperatorID: user.ID, Form: normalized}
	if s.repo != nil {
		return s.repo.AdminCreateProductPlan(ctx, mutation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.productPlanSlugExistsLocked("", normalized.Slug) {
		return ProductPlan{}, fieldError(http.StatusConflict, "slug", "套餐 slug 已被占用。")
	}
	now := s.now()
	plan := ProductPlan{
		ID:                   uuid.NewString(),
		CategoryID:           normalized.CategoryID,
		CategoryCode:         category.Code,
		ProviderCode:         normalized.ProviderCode,
		Slug:                 normalized.Slug,
		DisplayName:          normalized.DisplayName,
		Description:          normalized.Description,
		PublishPolicy:        normalized.PublishPolicy,
		AccessMode:           normalized.AccessMode,
		ProviderPolicyStatus: normalized.ProviderPolicyStatus,
		RiskLevel:            normalized.RiskLevel,
		RiskAckRequired:      normalized.RiskAckRequired,
		RiskNoticeCode:       normalized.RiskNoticeCode,
		PolicyVersion:        1,
		PolicyNote:           normalized.PolicyNote,
		QuotaLabel:           normalized.QuotaLabel,
		QuotaUnit:            normalized.QuotaUnit,
		QuotaPeriod:          normalized.QuotaPeriod,
		Active:               normalized.Active,
		AllowCustomVariant:   normalized.AllowCustomVariant,
		SortOrder:            normalized.SortOrder,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	s.productPlans[plan.ID] = plan
	return plan, nil
}

func (s *Service) UpdateProductPlan(ctx context.Context, user auth.User, planID string, input ProductPlanInput) (ProductPlan, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ProductPlan{}, appErr
	}
	category, appErr := s.productCategoryForMutation(ctx, input.CategoryID)
	if appErr != nil {
		return ProductPlan{}, appErr
	}
	normalized, appErr := normalizeProductPlanInput(input)
	if appErr != nil {
		return ProductPlan{}, appErr
	}
	mutation := ProductPlanMutationInput{ID: planID, OperatorID: user.ID, Form: normalized}
	if s.repo != nil {
		return s.repo.AdminUpdateProductPlan(ctx, mutation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	plan, ok := s.productPlans[strings.TrimSpace(planID)]
	if !ok {
		return ProductPlan{}, productPlanNotFound()
	}
	if s.productPlanSlugExistsLocked(plan.ID, normalized.Slug) {
		return ProductPlan{}, fieldError(http.StatusConflict, "slug", "套餐 slug 已被占用。")
	}
	if productPlanPolicyChanged(plan, normalized) {
		plan.PolicyVersion++
	}
	plan.CategoryID = normalized.CategoryID
	plan.CategoryCode = category.Code
	plan.ProviderCode = normalized.ProviderCode
	plan.Slug = normalized.Slug
	plan.DisplayName = normalized.DisplayName
	plan.Description = normalized.Description
	plan.PublishPolicy = normalized.PublishPolicy
	plan.AccessMode = normalized.AccessMode
	plan.ProviderPolicyStatus = normalized.ProviderPolicyStatus
	plan.RiskLevel = normalized.RiskLevel
	plan.RiskAckRequired = normalized.RiskAckRequired
	plan.RiskNoticeCode = normalized.RiskNoticeCode
	plan.PolicyNote = normalized.PolicyNote
	plan.QuotaLabel = normalized.QuotaLabel
	plan.QuotaUnit = normalized.QuotaUnit
	plan.QuotaPeriod = normalized.QuotaPeriod
	plan.Active = normalized.Active
	plan.AllowCustomVariant = normalized.AllowCustomVariant
	plan.SortOrder = normalized.SortOrder
	plan.UpdatedAt = s.now()
	s.productPlans[plan.ID] = plan
	return plan, nil
}

func (s *Service) SetProductPlanActive(ctx context.Context, user auth.User, planID string, active bool) (ProductPlan, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ProductPlan{}, appErr
	}
	mutation := ProductPlanMutationInput{ID: planID, OperatorID: user.ID}
	if s.repo != nil {
		return s.repo.AdminSetProductPlanActive(ctx, mutation, active)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	plan, ok := s.productPlans[strings.TrimSpace(planID)]
	if !ok {
		return ProductPlan{}, productPlanNotFound()
	}
	plan.Active = active
	plan.UpdatedAt = s.now()
	s.productPlans[plan.ID] = plan
	return plan, nil
}

func (s *Service) APIModels(ctx context.Context) ([]APIModelCatalog, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListAPIModels(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	models := make([]APIModelCatalog, 0, len(s.apiModelOrder))
	for _, id := range s.apiModelOrder {
		model := s.apiModels[id]
		provider, providerOK := s.apiProviders[model.ProviderID]
		if model.Active && providerOK && provider.Active {
			model = withAPIModelProvider(model, provider)
			models = append(models, model)
		}
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].SortOrder == models[j].SortOrder {
			return models[i].DisplayName < models[j].DisplayName
		}
		return models[i].SortOrder < models[j].SortOrder
	})
	return models, nil
}

func (s *Service) APIModel(ctx context.Context, modelID string) (APIModelCatalog, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetAPIModel(ctx, modelID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	model, ok := s.apiModels[strings.TrimSpace(modelID)]
	provider, providerOK := s.apiProviders[model.ProviderID]
	if !ok || !model.Active || !providerOK || !provider.Active {
		return APIModelCatalog{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API model not found", "API 模型不存在。")
	}
	return withAPIModelProvider(model, provider), nil
}

func (s *Service) AdminAPIModelProviders(ctx context.Context, user auth.User) ([]APIModelProvider, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.AdminListAPIModelProviders(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	providers := make([]APIModelProvider, 0, len(s.providerOrder))
	for _, id := range s.providerOrder {
		providers = append(providers, s.apiProviders[id])
	}
	sortAPIModelProviders(providers)
	return providers, nil
}

func (s *Service) AdminAPIModelProvider(ctx context.Context, user auth.User, providerID string) (APIModelProvider, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelProvider{}, appErr
	}
	if s.repo != nil {
		return s.repo.AdminGetAPIModelProvider(ctx, providerID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	provider, ok := s.apiProviders[strings.TrimSpace(providerID)]
	if !ok {
		return APIModelProvider{}, apiModelProviderNotFound()
	}
	return provider, nil
}

func (s *Service) CreateAPIModelProvider(ctx context.Context, user auth.User, input APIModelProviderInput) (APIModelProvider, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelProvider{}, appErr
	}
	normalized, appErr := normalizeAPIModelProviderInput(input)
	if appErr != nil {
		return APIModelProvider{}, appErr
	}
	mutation := APIModelProviderMutationInput{OperatorID: user.ID, Form: normalized}
	if s.repo != nil {
		return s.repo.AdminCreateAPIModelProvider(ctx, mutation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.apiModelProviderCodeExistsLocked("", normalized.Code) {
		return APIModelProvider{}, fieldError(http.StatusConflict, "code", "提供商 code 已被占用。")
	}
	now := s.now()
	provider := APIModelProvider{
		ID:               uuid.NewString(),
		ProviderCategory: normalized.ProviderCategory,
		Code:             normalized.Code,
		DisplayName:      normalized.DisplayName,
		Active:           normalized.Active,
		SortOrder:        normalized.SortOrder,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	s.apiProviders[provider.ID] = provider
	s.providerOrder = append(s.providerOrder, provider.ID)
	return provider, nil
}

func (s *Service) UpdateAPIModelProvider(ctx context.Context, user auth.User, providerID string, input APIModelProviderInput) (APIModelProvider, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelProvider{}, appErr
	}
	normalized, appErr := normalizeAPIModelProviderInput(input)
	if appErr != nil {
		return APIModelProvider{}, appErr
	}
	mutation := APIModelProviderMutationInput{ID: providerID, OperatorID: user.ID, Form: normalized}
	if s.repo != nil {
		return s.repo.AdminUpdateAPIModelProvider(ctx, mutation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	provider, ok := s.apiProviders[strings.TrimSpace(providerID)]
	if !ok {
		return APIModelProvider{}, apiModelProviderNotFound()
	}
	if s.apiModelProviderCodeExistsLocked(provider.ID, normalized.Code) {
		return APIModelProvider{}, fieldError(http.StatusConflict, "code", "提供商 code 已被占用。")
	}
	provider.ProviderCategory = normalized.ProviderCategory
	provider.Code = normalized.Code
	provider.DisplayName = normalized.DisplayName
	provider.Active = normalized.Active
	provider.SortOrder = normalized.SortOrder
	provider.UpdatedAt = s.now()
	s.apiProviders[provider.ID] = provider
	for id, model := range s.apiModels {
		if model.ProviderID == provider.ID {
			s.apiModels[id] = withAPIModelProvider(model, provider)
		}
	}
	return provider, nil
}

func (s *Service) SetAPIModelProviderActive(ctx context.Context, user auth.User, providerID string, active bool) (APIModelProvider, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelProvider{}, appErr
	}
	mutation := APIModelProviderMutationInput{ID: providerID, OperatorID: user.ID}
	if s.repo != nil {
		return s.repo.AdminSetAPIModelProviderActive(ctx, mutation, active)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	provider, ok := s.apiProviders[strings.TrimSpace(providerID)]
	if !ok {
		return APIModelProvider{}, apiModelProviderNotFound()
	}
	provider.Active = active
	provider.UpdatedAt = s.now()
	s.apiProviders[provider.ID] = provider
	for id, model := range s.apiModels {
		if model.ProviderID == provider.ID {
			s.apiModels[id] = withAPIModelProvider(model, provider)
		}
	}
	return provider, nil
}

func (s *Service) AdminAPIModels(ctx context.Context, user auth.User) ([]APIModelCatalog, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.AdminListAPIModels(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	models := make([]APIModelCatalog, 0, len(s.apiModelOrder))
	for _, id := range s.apiModelOrder {
		model := s.apiModels[id]
		if provider, ok := s.apiProviders[model.ProviderID]; ok {
			model = withAPIModelProvider(model, provider)
		}
		models = append(models, model)
	}
	sortAPIModels(models)
	return models, nil
}

func (s *Service) AdminAPIModel(ctx context.Context, user auth.User, modelID string) (APIModelCatalog, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelCatalog{}, appErr
	}
	if s.repo != nil {
		return s.repo.AdminGetAPIModel(ctx, modelID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	model, ok := s.apiModels[strings.TrimSpace(modelID)]
	if !ok {
		return APIModelCatalog{}, apiModelNotFound()
	}
	if provider, providerOK := s.apiProviders[model.ProviderID]; providerOK {
		model = withAPIModelProvider(model, provider)
	}
	return model, nil
}

func (s *Service) CreateAPIModel(ctx context.Context, user auth.User, input APIModelInput) (APIModelCatalog, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelCatalog{}, appErr
	}
	provider, appErr := s.apiModelProviderForModelMutation(ctx, input.ProviderID)
	if appErr != nil {
		return APIModelCatalog{}, appErr
	}
	normalized, appErr := normalizeAPIModelInput(input)
	if appErr != nil {
		return APIModelCatalog{}, appErr
	}
	mutation := APIModelMutationInput{OperatorID: user.ID, Form: normalized}
	if s.repo != nil {
		return s.repo.AdminCreateAPIModel(ctx, mutation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.apiModelKeyExistsLocked("", normalized.ModelKey) {
		return APIModelCatalog{}, fieldError(http.StatusConflict, "modelKey", "模型标识已被占用。")
	}
	now := s.now()
	model := APIModelCatalog{
		ID:                         uuid.NewString(),
		ProviderID:                 provider.ID,
		ProviderCategory:           provider.ProviderCategory,
		ProviderCode:               provider.Code,
		Provider:                   provider.DisplayName,
		ProviderActive:             provider.Active,
		ModelKey:                   normalized.ModelKey,
		DisplayName:                normalized.DisplayName,
		Capabilities:               append([]string(nil), normalized.Capabilities...),
		Active:                     normalized.Active,
		SortOrder:                  normalized.SortOrder,
		CurrentPriceSourceURL:      normalized.SourceURL,
		CurrentPriceSourceVersion:  normalized.SourceVersion,
		InputPricePerMillion:       normalized.InputTokenPrice,
		CachedInputPricePerMillion: normalized.CachedInputTokenPrice,
		OutputPricePerMillion:      normalized.OutputTokenPrice,
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}
	if apiModelPriceInputPresent(normalized) {
		model.CurrentPriceVersionID = uuid.NewString()
		model.CurrentPriceValidFrom = &now
	}
	s.apiModels[model.ID] = model
	s.apiModelOrder = append(s.apiModelOrder, model.ID)
	return model, nil
}

func (s *Service) UpdateAPIModel(ctx context.Context, user auth.User, modelID string, input APIModelInput) (APIModelCatalog, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelCatalog{}, appErr
	}
	provider, appErr := s.apiModelProviderForModelMutation(ctx, input.ProviderID)
	if appErr != nil {
		return APIModelCatalog{}, appErr
	}
	normalized, appErr := normalizeAPIModelInput(input)
	if appErr != nil {
		return APIModelCatalog{}, appErr
	}
	mutation := APIModelMutationInput{ID: modelID, OperatorID: user.ID, Form: normalized}
	if s.repo != nil {
		return s.repo.AdminUpdateAPIModel(ctx, mutation)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	model, ok := s.apiModels[strings.TrimSpace(modelID)]
	if !ok {
		return APIModelCatalog{}, apiModelNotFound()
	}
	if s.apiModelKeyExistsLocked(model.ID, normalized.ModelKey) {
		return APIModelCatalog{}, fieldError(http.StatusConflict, "modelKey", "模型标识已被占用。")
	}
	now := s.now()
	priceChanged := apiModelPriceChanged(model, normalized)
	model.ProviderID = provider.ID
	model.ProviderCategory = provider.ProviderCategory
	model.ProviderCode = provider.Code
	model.Provider = provider.DisplayName
	model.ProviderActive = provider.Active
	model.ModelKey = normalized.ModelKey
	model.DisplayName = normalized.DisplayName
	model.Capabilities = append([]string(nil), normalized.Capabilities...)
	model.Active = normalized.Active
	model.SortOrder = normalized.SortOrder
	model.UpdatedAt = now
	if priceChanged {
		model.CurrentPriceVersionID = uuid.NewString()
		model.CurrentPriceSourceURL = normalized.SourceURL
		model.CurrentPriceSourceVersion = normalized.SourceVersion
		model.CurrentPriceValidFrom = &now
		model.InputPricePerMillion = normalized.InputTokenPrice
		model.CachedInputPricePerMillion = normalized.CachedInputTokenPrice
		model.OutputPricePerMillion = normalized.OutputTokenPrice
	}
	s.apiModels[model.ID] = model
	return model, nil
}

func (s *Service) SetAPIModelActive(ctx context.Context, user auth.User, modelID string, active bool) (APIModelCatalog, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return APIModelCatalog{}, appErr
	}
	mutation := APIModelMutationInput{ID: modelID, OperatorID: user.ID}
	if s.repo != nil {
		return s.repo.AdminSetAPIModelActive(ctx, mutation, active)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	model, ok := s.apiModels[strings.TrimSpace(modelID)]
	if !ok {
		return APIModelCatalog{}, apiModelNotFound()
	}
	model.Active = active
	model.UpdatedAt = s.now()
	s.apiModels[model.ID] = model
	return model, nil
}

func (s *Service) seedProductPlans() {
	for _, plan := range SeedProductPlans(s.now()) {
		s.productPlans[plan.ID] = plan
	}
}

func (s *Service) seedProductCategories() {
	for _, category := range SeedProductCategories() {
		s.categories[category.ID] = category
	}
}

func (s *Service) seedAPIModelProviders() {
	for _, provider := range SeedAPIModelProviders(s.now()) {
		s.apiProviders[provider.ID] = provider
		s.providerOrder = append(s.providerOrder, provider.ID)
	}
}

func (s *Service) seedAPIModels() {
	for _, model := range SeedAPIModels(s.now()) {
		if model.ProviderID != "" {
			if provider, ok := s.apiProviders[model.ProviderID]; ok {
				model = withAPIModelProvider(model, provider)
			}
		}
		s.apiModels[model.ID] = model
		s.apiModelOrder = append(s.apiModelOrder, model.ID)
	}
}

func normalizeProductPlanInput(input ProductPlanInput) (ProductPlanInput, *domain.AppError) {
	input.CategoryID = strings.TrimSpace(input.CategoryID)
	input.ProviderCode = strings.ToLower(strings.TrimSpace(input.ProviderCode))
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Description = strings.TrimSpace(input.Description)
	input.PublishPolicy = strings.TrimSpace(input.PublishPolicy)
	input.AccessMode = strings.TrimSpace(input.AccessMode)
	input.ProviderPolicyStatus = strings.TrimSpace(input.ProviderPolicyStatus)
	input.RiskLevel = strings.TrimSpace(input.RiskLevel)
	input.RiskNoticeCode = strings.TrimSpace(input.RiskNoticeCode)
	input.PolicyNote = strings.TrimSpace(input.PolicyNote)
	input.QuotaLabel = strings.TrimSpace(input.QuotaLabel)
	input.QuotaUnit = strings.TrimSpace(input.QuotaUnit)
	input.QuotaPeriod = strings.TrimSpace(input.QuotaPeriod)
	if input.QuotaLabel == "" {
		input.QuotaLabel = "额度"
	}
	if input.QuotaUnit == "" {
		input.QuotaUnit = "USD"
	}
	if input.QuotaPeriod == "" {
		input.QuotaPeriod = "monthly"
	}

	if input.CategoryID == "" {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "categoryId", "必须选择产品分类。")
	}
	if !slugPattern.MatchString(input.ProviderCode) {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "providerCode", "Provider code 只允许小写字母、数字和短横线。")
	}
	if !slugPattern.MatchString(input.Slug) {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "slug", "套餐 slug 只允许小写字母、数字和短横线。")
	}
	if utf8.RuneCountInString(input.DisplayName) < 2 || utf8.RuneCountInString(input.DisplayName) > 80 {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "displayName", "展示名需为 2 至 80 个字符。")
	}
	if utf8.RuneCountInString(input.Description) > 240 {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "description", "描述不能超过 240 个字符。")
	}
	if !validPublishPolicies[input.PublishPolicy] {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "publishPolicy", "发布政策不支持。")
	}
	if !validAccessModes[input.AccessMode] {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "accessMode", "接入方式不支持。")
	}
	if !validProviderPolicyStatuses[input.ProviderPolicyStatus] {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "providerPolicyStatus", "服务商政策状态不支持。")
	}
	if !validRiskLevels[input.RiskLevel] {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "riskLevel", "风险级别不支持。")
	}
	if input.RiskAckRequired && input.RiskNoticeCode == "" {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "riskNoticeCode", "需要风险确认时必须选择风险提示。")
	}
	if input.RiskNoticeCode != "" && !validRiskNoticeCodes[input.RiskNoticeCode] {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "riskNoticeCode", "风险提示 code 不支持。")
	}
	if utf8.RuneCountInString(input.PolicyNote) > 500 {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "policyNote", "政策说明不能超过 500 个字符。")
	}
	if utf8.RuneCountInString(input.QuotaLabel) < 1 || utf8.RuneCountInString(input.QuotaLabel) > 20 {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "quotaLabel", "额度名称需为 1 至 20 个字符。")
	}
	if utf8.RuneCountInString(input.QuotaUnit) < 1 || utf8.RuneCountInString(input.QuotaUnit) > 12 {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "quotaUnit", "额度单位需为 1 至 12 个字符。")
	}
	if input.QuotaPeriod != "monthly" {
		return ProductPlanInput{}, fieldError(http.StatusUnprocessableEntity, "quotaPeriod", "额度周期目前只支持 monthly。")
	}
	return input, nil
}

func normalizeProductCategoryInput(input ProductCategoryInput) (ProductCategoryInput, *domain.AppError) {
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.IconDataURL = strings.TrimSpace(input.IconDataURL)

	if !slugPattern.MatchString(input.Code) {
		return ProductCategoryInput{}, fieldError(http.StatusUnprocessableEntity, "code", "分类 code 只允许小写字母、数字和短横线。")
	}
	if utf8.RuneCountInString(input.DisplayName) < 1 || utf8.RuneCountInString(input.DisplayName) > 40 {
		return ProductCategoryInput{}, fieldError(http.StatusUnprocessableEntity, "displayName", "分类展示名需为 1 至 40 个字符。")
	}
	if appErr := validateProductCategoryIconDataURL(input.IconDataURL); appErr != nil {
		return ProductCategoryInput{}, appErr
	}
	return input, nil
}

func validateProductCategoryIconDataURL(value string) *domain.AppError {
	if value == "" {
		return nil
	}
	validPrefix := strings.HasPrefix(value, "data:image/png;base64,") || strings.HasPrefix(value, "data:image/webp;base64,")
	if !validPrefix {
		return fieldError(http.StatusUnprocessableEntity, "iconDataUrl", "分类图标只支持 PNG 或 WebP。")
	}
	_, encoded, ok := strings.Cut(value, ",")
	if !ok {
		return fieldError(http.StatusUnprocessableEntity, "iconDataUrl", "分类图标格式不正确。")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fieldError(http.StatusUnprocessableEntity, "iconDataUrl", "分类图标格式不正确。")
	}
	if len(decoded) > 256*1024 {
		return fieldError(http.StatusUnprocessableEntity, "iconDataUrl", "分类图标不能超过 256 KB。")
	}
	return nil
}

func normalizeAPIModelProviderInput(input APIModelProviderInput) (APIModelProviderInput, *domain.AppError) {
	input.ProviderCategory = strings.ToLower(strings.TrimSpace(input.ProviderCategory))
	input.Code = strings.ToLower(strings.TrimSpace(input.Code))
	input.DisplayName = strings.TrimSpace(input.DisplayName)

	if !validAPIModelProviderCategories[input.ProviderCategory] {
		return APIModelProviderInput{}, fieldError(http.StatusUnprocessableEntity, "providerCategory", "模型提供商分类不支持。")
	}
	if !slugPattern.MatchString(input.Code) {
		return APIModelProviderInput{}, fieldError(http.StatusUnprocessableEntity, "code", "提供商 code 只允许小写字母、数字和短横线。")
	}
	if utf8.RuneCountInString(input.DisplayName) < 1 || utf8.RuneCountInString(input.DisplayName) > 80 {
		return APIModelProviderInput{}, fieldError(http.StatusUnprocessableEntity, "displayName", "提供商展示名需为 1 至 80 个字符。")
	}
	if domain.LooksLikeSecretContent(input.DisplayName) {
		return APIModelProviderInput{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在提供商目录中保存任何凭据。", "displayName", "secret_content", "不能包含 API Key、密码、Token、Session 或 Cookie。")
	}
	return input, nil
}

func normalizeAPIModelInput(input APIModelInput) (APIModelInput, *domain.AppError) {
	input.ProviderID = strings.TrimSpace(input.ProviderID)
	input.ModelKey = strings.TrimSpace(input.ModelKey)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.SourceURL = strings.TrimSpace(input.SourceURL)
	input.SourceVersion = strings.TrimSpace(input.SourceVersion)

	if input.ProviderID == "" {
		return APIModelInput{}, fieldError(http.StatusUnprocessableEntity, "providerId", "必须选择 API 提供商。")
	}
	if utf8.RuneCountInString(input.ModelKey) < 1 || utf8.RuneCountInString(input.ModelKey) > 120 {
		return APIModelInput{}, fieldError(http.StatusUnprocessableEntity, "modelKey", "模型标识需为 1 至 120 个字符。")
	}
	if utf8.RuneCountInString(input.DisplayName) < 1 || utf8.RuneCountInString(input.DisplayName) > 80 {
		return APIModelInput{}, fieldError(http.StatusUnprocessableEntity, "displayName", "展示名需为 1 至 80 个字符。")
	}
	capabilities, appErr := normalizeAPIModelCapabilities(input.Capabilities)
	if appErr != nil {
		return APIModelInput{}, appErr
	}
	input.Capabilities = capabilities
	if appErr := validateAPIModelSourceText("sourceUrl", input.SourceURL, 500); appErr != nil {
		return APIModelInput{}, appErr
	}
	if appErr := validateAPIModelSourceText("sourceVersion", input.SourceVersion, 120); appErr != nil {
		return APIModelInput{}, appErr
	}
	if input.InputTokenPrice, appErr = normalizeAPIModelPrice("inputTokenPrice", input.InputTokenPrice); appErr != nil {
		return APIModelInput{}, appErr
	}
	if input.CachedInputTokenPrice, appErr = normalizeAPIModelPrice("cachedInputTokenPrice", input.CachedInputTokenPrice); appErr != nil {
		return APIModelInput{}, appErr
	}
	if input.OutputTokenPrice, appErr = normalizeAPIModelPrice("outputTokenPrice", input.OutputTokenPrice); appErr != nil {
		return APIModelInput{}, appErr
	}
	return input, nil
}

func normalizeAPIModelCapabilities(values []string) ([]string, *domain.AppError) {
	seen := map[string]bool{}
	for _, value := range values {
		capability := strings.TrimSpace(value)
		if capability == "" {
			continue
		}
		if !validAPIModelCapabilities[capability] {
			return nil, fieldError(http.StatusUnprocessableEntity, "capabilities", "能力标签不支持。")
		}
		seen[capability] = true
	}
	if len(seen) == 0 {
		return nil, fieldError(http.StatusUnprocessableEntity, "capabilities", "至少选择一种能力标签。")
	}
	capabilities := make([]string, 0, len(seen))
	for _, capability := range apiModelCapabilityOrder {
		if seen[capability] {
			capabilities = append(capabilities, capability)
		}
	}
	return capabilities, nil
}

func normalizeAPIModelPrice(field, value string) (string, *domain.AppError) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.EqualFold(value, "nan") || strings.EqualFold(value, "infinity") || strings.EqualFold(value, "+infinity") || strings.EqualFold(value, "-infinity") || strings.EqualFold(value, "inf") || strings.EqualFold(value, "+inf") || strings.EqualFold(value, "-inf") {
		return "", fieldError(http.StatusUnprocessableEntity, field, "价格必须是非负数字。")
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok || rat.Sign() < 0 {
		return "", fieldError(http.StatusUnprocessableEntity, field, "价格必须是非负数字。")
	}
	return decimalString(rat, 6), nil
}

func validateAPIModelSourceText(field, value string, maxRunes int) *domain.AppError {
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return fieldError(http.StatusUnprocessableEntity, field, "来源信息过长。")
	}
	if strings.ContainsAny(value, "\x00") {
		return fieldError(http.StatusUnprocessableEntity, field, "来源信息包含非法字符。")
	}
	if domain.LooksLikeSecretContent(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在模型目录中保存任何凭据。", field, "secret_content", "不能包含 API Key、密码、Token、Session 或 Cookie。")
	}
	return nil
}

func (s *Service) productCategoryForMutation(ctx context.Context, categoryID string) (ProductCategory, *domain.AppError) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return ProductCategory{}, fieldError(http.StatusUnprocessableEntity, "categoryId", "必须选择产品分类。")
	}
	if s.repo != nil {
		return s.repo.GetProductCategory(ctx, categoryID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	category, ok := s.categories[categoryID]
	if !ok {
		return ProductCategory{}, fieldError(http.StatusUnprocessableEntity, "categoryId", "产品分类不存在。")
	}
	return category, nil
}

func (s *Service) apiModelKeyExistsLocked(currentID, modelKey string) bool {
	for _, model := range s.apiModels {
		if model.ID != currentID && model.ModelKey == modelKey {
			return true
		}
	}
	return false
}

func (s *Service) apiModelProviderCodeExistsLocked(currentID, code string) bool {
	for _, provider := range s.apiProviders {
		if provider.ID != currentID && provider.Code == code {
			return true
		}
	}
	return false
}

func (s *Service) productPlanSlugExistsLocked(currentID, slug string) bool {
	for _, plan := range s.productPlans {
		if plan.ID != currentID && plan.Slug == slug {
			return true
		}
	}
	return false
}

func (s *Service) apiModelProviderForModelMutation(ctx context.Context, providerID string) (APIModelProvider, *domain.AppError) {
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		return APIModelProvider{}, fieldError(http.StatusUnprocessableEntity, "providerId", "必须选择 API 提供商。")
	}
	if s.repo != nil {
		provider, appErr := s.repo.AdminGetAPIModelProvider(ctx, providerID)
		if appErr != nil {
			if appErr.Status == http.StatusNotFound {
				return APIModelProvider{}, fieldError(http.StatusUnprocessableEntity, "providerId", "API 提供商不存在。")
			}
			return APIModelProvider{}, appErr
		}
		if !provider.Active {
			return APIModelProvider{}, fieldError(http.StatusUnprocessableEntity, "providerId", "API 提供商已停用。")
		}
		return provider, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	provider, ok := s.apiProviders[providerID]
	if !ok {
		return APIModelProvider{}, fieldError(http.StatusUnprocessableEntity, "providerId", "API 提供商不存在。")
	}
	if !provider.Active {
		return APIModelProvider{}, fieldError(http.StatusUnprocessableEntity, "providerId", "API 提供商已停用。")
	}
	return provider, nil
}

func withAPIModelProvider(model APIModelCatalog, provider APIModelProvider) APIModelCatalog {
	model.ProviderID = provider.ID
	model.ProviderCategory = provider.ProviderCategory
	model.ProviderCode = provider.Code
	model.Provider = provider.DisplayName
	model.ProviderActive = provider.Active
	return model
}

func (s *Service) productCategoryCodeExistsLocked(currentID, code string) bool {
	for _, category := range s.categories {
		if category.ID != currentID && category.Code == code {
			return true
		}
	}
	return false
}

func productPlanPolicyChanged(current ProductPlan, input ProductPlanInput) bool {
	return current.PublishPolicy != input.PublishPolicy ||
		current.AccessMode != input.AccessMode ||
		current.ProviderPolicyStatus != input.ProviderPolicyStatus ||
		current.RiskLevel != input.RiskLevel ||
		current.RiskAckRequired != input.RiskAckRequired ||
		current.RiskNoticeCode != input.RiskNoticeCode ||
		current.PolicyNote != input.PolicyNote ||
		current.QuotaLabel != input.QuotaLabel ||
		current.QuotaUnit != input.QuotaUnit ||
		current.QuotaPeriod != input.QuotaPeriod
}

func apiModelPriceInputPresent(input APIModelInput) bool {
	return input.SourceURL != "" ||
		input.SourceVersion != "" ||
		input.InputTokenPrice != "" ||
		input.CachedInputTokenPrice != "" ||
		input.OutputTokenPrice != ""
}

func apiModelPriceChanged(current APIModelCatalog, input APIModelInput) bool {
	return current.CurrentPriceSourceURL != input.SourceURL ||
		current.CurrentPriceSourceVersion != input.SourceVersion ||
		current.InputPricePerMillion != input.InputTokenPrice ||
		current.CachedInputPricePerMillion != input.CachedInputTokenPrice ||
		current.OutputPricePerMillion != input.OutputTokenPrice
}

func decimalString(value *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(scale))
	rounded := roundRatHalfUp(scaled)
	intPart := new(big.Int).Quo(rounded, scale)
	frac := new(big.Int).Mod(rounded, scale)
	fracText := frac.String()
	for len(fracText) < places {
		fracText = "0" + fracText
	}
	return intPart.String() + "." + fracText
}

func roundRatHalfUp(value *big.Rat) *big.Int {
	num := new(big.Int).Set(value.Num())
	den := new(big.Int).Set(value.Denom())
	quotient, remainder := new(big.Int).QuoRem(num, den, new(big.Int))
	twice := new(big.Int).Mul(remainder, big.NewInt(2))
	if twice.Cmp(den) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

func requireAdmin(user auth.User) *domain.AppError {
	if !user.IsAdmin {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	return nil
}

func productPlanNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Product plan not found", "产品套餐不存在。")
}

func productCategoryNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Product category not found", "产品分类不存在。")
}

func apiModelNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API model not found", "API 模型不存在。")
}

func apiModelProviderNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API model provider not found", "API 提供商不存在。")
}

func fieldError(status int, field, message string) *domain.AppError {
	return domain.NewFieldError(status, domain.CodeValidationFailed, "Validation failed", message, field, "invalid", message)
}

func sortProductPlans(plans []ProductPlan) {
	sort.Slice(plans, func(i, j int) bool {
		return productPlanLess(plans[i], plans[j])
	})
}

func sortProductCategories(categories []ProductCategory) {
	sort.Slice(categories, func(i, j int) bool {
		if categories[i].SortOrder == categories[j].SortOrder {
			return categories[i].DisplayName < categories[j].DisplayName
		}
		return categories[i].SortOrder < categories[j].SortOrder
	})
}

func sortAPIModels(models []APIModelCatalog) {
	sort.Slice(models, func(i, j int) bool {
		if models[i].ProviderCategory != models[j].ProviderCategory {
			return models[i].ProviderCategory < models[j].ProviderCategory
		}
		if models[i].SortOrder != models[j].SortOrder {
			return models[i].SortOrder < models[j].SortOrder
		}
		if models[i].DisplayName != models[j].DisplayName {
			return models[i].DisplayName < models[j].DisplayName
		}
		return models[i].ID < models[j].ID
	})
}

func sortAPIModelProviders(providers []APIModelProvider) {
	sort.Slice(providers, func(i, j int) bool {
		if providers[i].ProviderCategory != providers[j].ProviderCategory {
			return providers[i].ProviderCategory < providers[j].ProviderCategory
		}
		if providers[i].SortOrder != providers[j].SortOrder {
			return providers[i].SortOrder < providers[j].SortOrder
		}
		if providers[i].DisplayName != providers[j].DisplayName {
			return providers[i].DisplayName < providers[j].DisplayName
		}
		return providers[i].ID < providers[j].ID
	})
}

func productPlanLess(left, right ProductPlan) bool {
	if left.CategoryCode != right.CategoryCode {
		return left.CategoryCode < right.CategoryCode
	}
	if left.SortOrder == right.SortOrder {
		return left.DisplayName < right.DisplayName
	}
	return left.SortOrder < right.SortOrder
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var validPublishPolicies = map[string]bool{
	"allowed":   true,
	"info_only": true,
	"blocked":   true,
}

var validAccessModes = map[string]bool{
	"personal_account_cost_share": true,
	"provider_member_invitation":  true,
	"owner_managed_access":        true,
	"other_off_platform":          true,
	"unsupported":                 true,
}

var validProviderPolicyStatuses = map[string]bool{
	"known_restricted":    true,
	"possibly_restricted": true,
	"unknown":             true,
}

var validRiskLevels = map[string]bool{
	"normal":   true,
	"elevated": true,
	"high":     true,
}

var validRiskNoticeCodes = map[string]bool{
	"openai_subscription_carpool": true,
}

var validAPIModelProviderCategories = map[string]bool{
	"gpt":        true,
	"claude":     true,
	"cursor":     true,
	"gemini":     true,
	"perplexity": true,
	"other":      true,
}

var apiModelCapabilityOrder = []string{
	"text",
	"chat",
	"vision",
	"image_generation",
	"image_edit",
	"reasoning",
}

var validAPIModelCapabilities = map[string]bool{
	"text":             true,
	"chat":             true,
	"vision":           true,
	"image_generation": true,
	"image_edit":       true,
	"reasoning":        true,
}
