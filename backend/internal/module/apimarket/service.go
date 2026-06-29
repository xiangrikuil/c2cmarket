package apimarket

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"

	"github.com/google/uuid"
)

type APIModelResolver interface {
	APIModel(ctx context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError)
}

type Manager struct {
	mu           sync.Mutex
	now          func() time.Time
	repo         Repository
	catalog      APIModelResolver
	contact      *contact.Service
	services     map[string]Service
	serviceOrder []string
}

func NewManager(repo Repository, catalogResolver APIModelResolver, contactService *contact.Service, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	if contactService == nil {
		contactService = contact.NewService(nil, now)
	}
	return &Manager{
		now:      now,
		repo:     repo,
		catalog:  catalogResolver,
		contact:  contactService,
		services: make(map[string]Service),
	}
}

func (s *Manager) Create(ctx context.Context, user auth.User, input CreateServiceInput) (Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	service, appErr := s.buildFromInput(ctx, Service{}, input)
	if appErr != nil {
		return Service{}, appErr
	}
	if s.repo != nil {
		if appErr := s.repo.CreateAPIService(ctx, service); appErr != nil {
			return Service{}, appErr
		}
		return WithOrderability(service), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, _, ok := s.contact.VersionForOwner(service.OwnerContactMethodID, user.ID); !ok {
		return Service{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "商户联系方式不可用或不属于当前用户。")
	}
	s.services[service.ID] = service
	s.serviceOrder = append(s.serviceOrder, service.ID)
	return WithOrderability(service), nil
}

func (s *Manager) Update(ctx context.Context, user auth.User, input UpdateServiceInput) (Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ServiceID) == "" {
		return Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}

	var current Service
	var appErr *domain.AppError
	if s.repo != nil {
		current, appErr = s.repo.GetAPIServiceForOwner(ctx, user.ID, input.ServiceID)
		if appErr != nil {
			return Service{}, appErr
		}
	} else {
		s.mu.Lock()
		var ok bool
		current, ok = s.services[input.ServiceID]
		s.mu.Unlock()
		if !ok || current.OwnerUserID != user.ID {
			return Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
		}
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canEditService(current) {
		return Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能直接修改，请先开始修订。")
	}

	service, appErr := s.buildFromInput(ctx, current, CreateServiceInput{
		OwnerUserID:                      user.ID,
		MerchantProfileID:                input.MerchantProfileID,
		MerchantIdentityMode:             input.MerchantIdentityMode,
		OwnerContactMethodID:             input.OwnerContactMethodID,
		Title:                            input.Title,
		ShortDescription:                 input.ShortDescription,
		DistributionSystem:               input.DistributionSystem,
		BillingMode:                      input.BillingMode,
		DeclaredCNYPerUSDAllowance:       input.DeclaredCNYPerUSDAllowance,
		DeclaredMaxUSDAllowancePerIntent: input.DeclaredMaxUSDAllowancePerIntent,
		MinimumIntentCNY:                 input.MinimumIntentCNY,
		MaximumIntentCNY:                 input.MaximumIntentCNY,
		UsageVisibility:                  input.UsageVisibility,
		PublicAccessNote:                 input.PublicAccessNote,
		MerchantNote:                     input.MerchantNote,
		MerchantSupportNote:              input.MerchantSupportNote,
		AccessModes:                      input.AccessModes,
		Models:                           input.Models,
		Packages:                         input.Packages,
	})
	if appErr != nil {
		return Service{}, appErr
	}

	service.ID = current.ID
	service.ReviewStatus = current.ReviewStatus
	service.PublicationStatus = current.PublicationStatus
	service.ModerationStatus = current.ModerationStatus
	service.ApprovedByAdminID = current.ApprovedByAdminID
	service.ApprovedAt = current.ApprovedAt
	service.ModerationReason = current.ModerationReason
	service.CreatedAt = current.CreatedAt
	service.Version = current.Version + 1
	for i := range service.AccessModes {
		service.AccessModes[i].APIServiceID = service.ID
	}
	for i := range service.Models {
		service.Models[i].APIServiceID = service.ID
	}
	for i := range service.Packages {
		service.Packages[i].APIServiceID = service.ID
	}

	if s.repo != nil {
		return s.repo.UpdateAPIService(ctx, input, service, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, _, ok := s.contact.VersionForOwner(service.OwnerContactMethodID, user.ID); !ok {
		return Service{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "商户联系方式不可用或不属于当前用户。")
	}
	s.services[service.ID] = service
	return WithOrderability(service), nil
}

func (s *Manager) PublicServices(ctx context.Context, filter PublicServiceFilter) ([]Service, *domain.AppError) {
	if err := validatePublicServiceFilter(filter); err != nil {
		return nil, err
	}
	if s.repo != nil {
		return s.repo.ListPublicAPIServices(ctx, filter)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	services := []Service{}
	for _, id := range s.serviceOrder {
		service := WithOrderability(s.services[id])
		if IsOrderableService(service) && matchesPaymentMethod(service, filter.PaymentMethod) {
			services = append(services, service)
		}
	}
	return services, nil
}

func (s *Manager) PublicService(ctx context.Context, serviceID string) (Service, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetPublicAPIService(ctx, serviceID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[serviceID]
	if !ok || !IsOrderableService(service) {
		return Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	return WithOrderability(service), nil
}

func (s *Manager) OwnerServices(ctx context.Context, user auth.User) ([]Service, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListAPIServicesByOwner(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	services := []Service{}
	for _, id := range s.serviceOrder {
		service := WithOrderability(s.services[id])
		if service.OwnerUserID == user.ID {
			services = append(services, service)
		}
	}
	return services, nil
}

func (s *Manager) OwnerService(ctx context.Context, user auth.User, serviceID string) (Service, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetAPIServiceForOwner(ctx, user.ID, serviceID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[serviceID]
	if !ok || service.OwnerUserID != user.ID {
		return Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	return WithOrderability(service), nil
}

func (s *Manager) AdminServices(ctx context.Context, user auth.User) ([]Service, *domain.AppError) {
	if !user.IsAdmin {
		return nil, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.ListAdminAPIServices(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	services := make([]Service, 0, len(s.serviceOrder))
	for _, id := range s.serviceOrder {
		services = append(services, WithOrderability(s.services[id]))
	}
	return services, nil
}

func (s *Manager) AdminService(ctx context.Context, user auth.User, serviceID string) (Service, *domain.AppError) {
	if !user.IsAdmin {
		return Service{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.GetAdminAPIService(ctx, serviceID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	service, ok := s.services[serviceID]
	if !ok {
		return Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	return WithOrderability(service), nil
}

func (s *Manager) SubmitForReview(ctx context.Context, user auth.User, input ServiceOwnerActionInput) (Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ServiceID) == "" {
		return Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}
	if s.repo != nil {
		return s.repo.SubmitAPIServiceForReview(ctx, user, input, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[input.ServiceID]
	if !ok || service.OwnerUserID != user.ID {
		return Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if input.ExpectedVersion > 0 && service.Version != input.ExpectedVersion {
		return Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if service.ReviewStatus != ServiceReviewStatusDraft && service.ReviewStatus != ServiceReviewStatusChangesRequested {
		return Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能提交审核。")
	}
	if appErr := requireEarlyAutoApprovalEligibility(user); appErr != nil {
		return Service{}, appErr
	}
	if _, _, ok := s.contact.VersionForOwner(service.OwnerContactMethodID, user.ID); !ok {
		return Service{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "商户联系方式不可用或不属于当前用户。")
	}

	service = applyEarlyAutoApprovalPolicy(service, s.now())
	s.services[service.ID] = service
	return service, nil
}

func (s *Manager) UpdatePublication(ctx context.Context, user auth.User, input ServiceOwnerActionInput, action string) (Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	if s.repo != nil {
		return s.repo.UpdateAPIServicePublication(ctx, input, action, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[input.ServiceID]
	if !ok || service.OwnerUserID != user.ID {
		return Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if input.ExpectedVersion > 0 && service.Version != input.ExpectedVersion {
		return Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdatePublication(service, action) {
		return Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能执行该操作。")
	}
	if action == "publish" || action == "resume" {
		if strings.TrimSpace(service.OwnerContactMethodID) == "" {
			return Service{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeMerchantContactRequired, "Merchant contact required", "上线 API 服务必须配置商户联系方式。")
		}
		if _, _, ok := s.contact.VersionForOwner(service.OwnerContactMethodID, user.ID); !ok {
			return Service{}, domain.NewError(http.StatusConflict, domain.CodeMerchantContactUnavailable, "Merchant contact unavailable", "商户联系方式当前不可用。")
		}
	}

	service = applyPublicationAction(service, action, s.now())
	s.services[service.ID] = service
	return WithOrderability(service), nil
}

func (s *Manager) UpdateAdminStatus(ctx context.Context, user auth.User, input ServiceAdminActionInput) (Service, *domain.AppError) {
	if !user.IsAdmin {
		return Service{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	input.AdminUserID = user.ID
	if err := validateAdminActionInput(input); err != nil {
		return Service{}, err
	}
	if s.repo != nil {
		return s.repo.UpdateAPIServiceModeration(ctx, user, input, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[input.ServiceID]
	if !ok {
		return Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if input.ExpectedVersion > 0 && service.Version != input.ExpectedVersion {
		return Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdateAdminStatus(service, input.Action) {
		return Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能执行该管理动作。")
	}

	service = applyAdminAction(service, input, s.now())
	s.services[service.ID] = service
	return WithOrderability(service), nil
}

func (s *Manager) UpdateOrderSettings(ctx context.Context, user auth.User, input UpdateOrderSettingsInput) (Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ServiceID) == "" {
		return Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}
	if err := validateOrderSettingsInput(input); err != nil {
		return Service{}, err
	}
	if s.repo != nil {
		return s.repo.UpdateAPIServiceOrderSettings(ctx, input, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.services[input.ServiceID]
	if !ok || service.OwnerUserID != user.ID {
		return Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if input.ExpectedVersion > 0 && service.Version != input.ExpectedVersion {
		return Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	service.PaymentWindowMinutes = input.PaymentWindowMinutes
	service.PaymentOptions = buildPaymentOptions(service.ID, service.PaymentOptions, input.PaymentOptions, s.now())
	service.AcceptingOrders = input.AcceptingOrders
	service.UpdatedAt = s.now()
	service.Version++
	if input.AcceptingOrders {
		service = WithOrderability(service)
		if !service.IsOrderable {
			return Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Service not orderable", "当前 API 服务不满足接单条件。", "acceptingOrders", "not_orderable", strings.Join(service.OrderableReasons, "；"))
		}
	}
	service = WithOrderability(service)
	s.services[service.ID] = service
	return service, nil
}

func (s *Manager) buildFromInput(ctx context.Context, current Service, input CreateServiceInput) (Service, *domain.AppError) {
	if err := validateCreateInput(input); err != nil {
		return Service{}, err
	}
	now := s.now()
	serviceID := current.ID
	createdAt := current.CreatedAt
	version := current.Version
	reviewStatus := current.ReviewStatus
	publicationStatus := current.PublicationStatus
	moderationStatus := current.ModerationStatus
	if serviceID == "" {
		serviceID = uuid.NewString()
		createdAt = now
		version = 1
		reviewStatus = ServiceReviewStatusDraft
		publicationStatus = ServicePublicationStatusOffline
		moderationStatus = ServiceModerationStatusClear
	}

	service := Service{
		ID:                               serviceID,
		OwnerUserID:                      input.OwnerUserID,
		MerchantProfileID:                strings.TrimSpace(input.MerchantProfileID),
		MerchantIdentityMode:             strings.TrimSpace(input.MerchantIdentityMode),
		OwnerContactMethodID:             strings.TrimSpace(input.OwnerContactMethodID),
		Title:                            strings.TrimSpace(input.Title),
		ShortDescription:                 strings.TrimSpace(input.ShortDescription),
		DistributionSystem:               strings.TrimSpace(input.DistributionSystem),
		BillingMode:                      strings.TrimSpace(input.BillingMode),
		DeclaredCNYPerUSDAllowance:       strings.TrimSpace(input.DeclaredCNYPerUSDAllowance),
		DeclaredMaxUSDAllowancePerIntent: strings.TrimSpace(input.DeclaredMaxUSDAllowancePerIntent),
		MinimumIntentCNY:                 strings.TrimSpace(input.MinimumIntentCNY),
		MaximumIntentCNY:                 strings.TrimSpace(input.MaximumIntentCNY),
		UsageVisibility:                  strings.TrimSpace(input.UsageVisibility),
		PublicAccessNote:                 strings.TrimSpace(input.PublicAccessNote),
		MerchantNote:                     strings.TrimSpace(input.MerchantNote),
		MerchantSupportNote:              strings.TrimSpace(input.MerchantSupportNote),
		AcceptingOrders:                  current.AcceptingOrders,
		PaymentWindowMinutes:             current.PaymentWindowMinutes,
		ReviewStatus:                     reviewStatus,
		PublicationStatus:                publicationStatus,
		ModerationStatus:                 moderationStatus,
		ApprovedByAdminID:                current.ApprovedByAdminID,
		ApprovedAt:                       current.ApprovedAt,
		ModerationReason:                 current.ModerationReason,
		CreatedAt:                        createdAt,
		UpdatedAt:                        now,
		Version:                          version,
		PaymentOptions:                   append([]PaymentOption(nil), current.PaymentOptions...),
	}
	if service.PaymentWindowMinutes == 0 {
		service.PaymentWindowMinutes = 10
	}
	if service.MerchantIdentityMode == "" {
		service.MerchantIdentityMode = "public_profile"
	}
	for _, modeInput := range input.AccessModes {
		service.AccessModes = append(service.AccessModes, ServiceAccessMode{
			APIServiceID: service.ID,
			AccessMode:   strings.TrimSpace(modeInput.AccessMode),
			PublicNote:   strings.TrimSpace(modeInput.PublicNote),
		})
	}
	for _, modelInput := range input.Models {
		if s.catalog == nil {
			return Service{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 模型目录不可用。")
		}
		model, appErr := s.catalog.APIModel(ctx, modelInput.ModelCatalogID)
		if appErr != nil {
			return Service{}, appErr
		}
		multiplier := strings.TrimSpace(modelInput.MerchantMultiplier)
		if service.DistributionSystem == ServiceDistributionSub2API || multiplier == "" {
			multiplier = "1.0000"
		}
		priceVersionID := strings.TrimSpace(modelInput.ModelPriceVersionID)
		if priceVersionID == "" {
			priceVersionID = model.CurrentPriceVersionID
		}
		service.Models = append(service.Models, ServiceModel{
			ID:                                  uuid.NewString(),
			APIServiceID:                        service.ID,
			DistributionSystem:                  service.DistributionSystem,
			ModelCatalogID:                      model.ID,
			ModelPriceVersionID:                 priceVersionID,
			ModelNameSnapshot:                   model.DisplayName,
			ProviderSnapshot:                    model.Provider,
			CapabilitiesSnapshot:                append([]string(nil), model.Capabilities...),
			MerchantMultiplier:                  normalizeDecimalText(multiplier, 4),
			EffectiveInputPricePerMillion:       multiplyDecimalText(model.InputPricePerMillion, multiplier, 6),
			EffectiveCachedInputPricePerMillion: multiplyDecimalText(model.CachedInputPricePerMillion, multiplier, 6),
			EffectiveOutputPricePerMillion:      multiplyDecimalText(model.OutputPricePerMillion, multiplier, 6),
			Enabled:                             modelInput.Enabled,
			CreatedAt:                           now,
			UpdatedAt:                           now,
		})
	}
	for _, packageInput := range input.Packages {
		service.Packages = append(service.Packages, ServicePackage{
			ID:           uuid.NewString(),
			APIServiceID: service.ID,
			Name:         strings.TrimSpace(packageInput.Name),
			PriceCNY:     strings.TrimSpace(packageInput.PriceCNY),
			DurationDays: packageInput.DurationDays,
			Description:  strings.TrimSpace(packageInput.Description),
			Enabled:      packageInput.Enabled,
			SortOrder:    packageInput.SortOrder,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}
	if service.BillingMode != ServiceBillingModeMetered {
		service.DeclaredCNYPerUSDAllowance = ""
	}
	return service, nil
}

func validateCreateInput(input CreateServiceInput) *domain.AppError {
	if strings.TrimSpace(input.OwnerContactMethodID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeContactMethodRequired, "Contact method required", "发布 API 服务必须选择商户联系方式。", "ownerContactMethodId", "required", "必须选择商户联系方式。")
	}
	if strings.TrimSpace(input.MerchantIdentityMode) == "" {
		input.MerchantIdentityMode = "public_profile"
	}
	switch strings.TrimSpace(input.MerchantIdentityMode) {
	case "public_profile":
	case "store_alias":
		if strings.TrimSpace(input.MerchantProfileID) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Merchant profile required", "使用店铺别名必须选择商户资料。", "merchantProfileId", "required", "必须选择商户资料。")
		}
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Merchant identity mode invalid", "商户展示方式不正确。", "merchantIdentityMode", "invalid", "商户展示方式不正确。")
	}
	if strings.TrimSpace(input.Title) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Title required", "必须填写 API 服务标题。", "title", "required", "必须填写 API 服务标题。")
	}
	if strings.TrimSpace(input.ShortDescription) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Description required", "必须填写 API 服务简介。", "shortDescription", "required", "必须填写 API 服务简介。")
	}
	if err := validateNonSecretText("title", input.Title); err != nil {
		return err
	}
	if err := validateNonSecretText("shortDescription", input.ShortDescription); err != nil {
		return err
	}
	if err := validateOptionalNonSecretText("publicAccessNote", input.PublicAccessNote); err != nil {
		return err
	}
	if err := validateOptionalNonSecretText("merchantNote", input.MerchantNote); err != nil {
		return err
	}
	if err := validateOptionalNonSecretText("merchantSupportNote", input.MerchantSupportNote); err != nil {
		return err
	}
	switch strings.TrimSpace(input.DistributionSystem) {
	case ServiceDistributionSub2API, "new_api_proxy", "other":
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Distribution system invalid", "分发系统不支持。", "distributionSystem", "invalid", "分发系统不支持。")
	}
	switch strings.TrimSpace(input.BillingMode) {
	case ServiceBillingModeMetered:
		if _, ok := parsePositiveDecimal(input.DeclaredCNYPerUSDAllowance); !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance price invalid", "美元额度售价格式不正确。", "declaredCnyPerUsdAllowance", "invalid", "美元额度售价必须为正数。")
		}
	case ServiceBillingModeManual, ServiceBillingModeFixedPackage:
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Billing mode invalid", "计费方式不支持。", "billingMode", "invalid", "计费方式不支持。")
	}
	if strings.TrimSpace(input.DeclaredMaxUSDAllowancePerIntent) != "" {
		if _, ok := parsePositiveDecimal(input.DeclaredMaxUSDAllowancePerIntent); !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance cap invalid", "单次意向美元额度上限格式不正确。", "declaredMaxUsdAllowancePerIntent", "invalid", "额度上限必须为正数。")
		}
	}
	if _, ok := parsePositiveDecimal(input.MinimumIntentCNY); !ok {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Minimum intent amount invalid", "最低意向金额格式不正确。", "minimumIntentCny", "invalid", "最低意向金额必须为正数。")
	}
	if strings.TrimSpace(input.MaximumIntentCNY) != "" {
		minValue, _ := parsePositiveDecimal(input.MinimumIntentCNY)
		maxValue, ok := parsePositiveDecimal(input.MaximumIntentCNY)
		if !ok || maxValue.Cmp(minValue) < 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Maximum intent amount invalid", "最高意向金额必须大于等于最低意向金额。", "maximumIntentCny", "invalid", "最高意向金额必须大于等于最低意向金额。")
		}
	}
	switch strings.TrimSpace(input.UsageVisibility) {
	case "none", "merchant_reported", "offsite_panel_readonly", "fixed_package_only":
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Usage visibility invalid", "用量可见性不支持。", "usageVisibility", "invalid", "用量可见性不支持。")
	}
	if len(input.AccessModes) == 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access mode required", "至少选择一种接入方式。", "accessModes", "required", "至少选择一种接入方式。")
	}
	seenAccessModes := map[string]bool{}
	for i, mode := range input.AccessModes {
		field := fmt.Sprintf("accessModes.%d", i)
		switch strings.TrimSpace(mode.AccessMode) {
		case "merchant_operated_endpoint", "buyer_dedicated_sub_key", "buyer_dedicated_panel_subaccount", "fixed_package_offsite", "manual_offsite_arrangement":
		default:
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access mode invalid", "接入方式不支持。", field+".accessMode", "invalid", "接入方式不支持。")
		}
		if seenAccessModes[strings.TrimSpace(mode.AccessMode)] {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access mode duplicated", "接入方式不能重复。", field+".accessMode", "duplicate", "接入方式不能重复。")
		}
		seenAccessModes[strings.TrimSpace(mode.AccessMode)] = true
		if err := validateOptionalNonSecretText(field+".publicNote", mode.PublicNote); err != nil {
			return err
		}
	}
	if input.BillingMode == ServiceBillingModeFixedPackage {
		if len(input.Packages) == 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package required", "固定套餐计费必须提供套餐。", "packages", "required", "必须提供套餐。")
		}
	} else if len(input.Models) == 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Model required", "该计费方式必须选择支持模型。", "models", "required", "必须选择支持模型。")
	}
	for i, model := range input.Models {
		field := fmt.Sprintf("models.%d", i)
		if strings.TrimSpace(model.ModelCatalogID) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Model catalog required", "模型目录不能为空。", field+".modelCatalogId", "required", "模型目录不能为空。")
		}
		multiplier := strings.TrimSpace(model.MerchantMultiplier)
		if multiplier == "" {
			multiplier = "1.0000"
		}
		if _, ok := parsePositiveDecimal(multiplier); !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Model multiplier invalid", "模型倍率格式不正确。", field+".merchantMultiplier", "invalid", "模型倍率必须为正数。")
		}
		if input.DistributionSystem == ServiceDistributionSub2API && multiplier != "1.0000" && multiplier != "1" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Sub2API multiplier fixed", "Sub2API 模型倍率固定为 1。", field+".merchantMultiplier", "fixed_one", "Sub2API 模型倍率固定为 1。")
		}
	}
	for i, pack := range input.Packages {
		field := fmt.Sprintf("packages.%d", i)
		if strings.TrimSpace(pack.Name) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package name required", "套餐名称不能为空。", field+".name", "required", "套餐名称不能为空。")
		}
		if _, ok := parsePositiveDecimal(pack.PriceCNY); !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package price invalid", "套餐价格格式不正确。", field+".priceCny", "invalid", "套餐价格必须为正数。")
		}
		if pack.DurationDays != nil && *pack.DurationDays <= 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package duration invalid", "套餐有效天数必须大于 0。", field+".durationDays", "invalid", "套餐有效天数必须大于 0。")
		}
		if strings.TrimSpace(pack.Description) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package description required", "套餐说明不能为空。", field+".description", "required", "套餐说明不能为空。")
		}
		if err := validateNonSecretText(field+".name", pack.Name); err != nil {
			return err
		}
		if err := validateNonSecretText(field+".description", pack.Description); err != nil {
			return err
		}
	}
	return nil
}

func validateAdminActionInput(input ServiceAdminActionInput) *domain.AppError {
	if strings.TrimSpace(input.ServiceID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "管理动作必须填写原因。", "reason", "required", "必须填写原因。")
	}
	if err := validateNonSecretText("reason", input.Reason); err != nil {
		return err
	}
	switch input.Action {
	case "approve", "request_changes", "reject", "suspend", "restore", "remove":
		return nil
	default:
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "不支持的 API 服务管理动作。")
	}
}

func canEditService(service Service) bool {
	return service.ReviewStatus == ServiceReviewStatusDraft || service.ReviewStatus == ServiceReviewStatusChangesRequested
}

func requireEarlyAutoApprovalEligibility(user auth.User) *domain.AppError {
	if user.LinuxDoBinding == nil || !user.LinuxDoBinding.Bound {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "linux.do binding required", "提交 API 服务前需要完成 linux.do 身份绑定。", "linuxDoBinding", "required", "需要先完成 linux.do 身份绑定。")
	}
	return nil
}

func applyEarlyAutoApprovalPolicy(service Service, now time.Time) Service {
	service.ReviewStatus = ServiceReviewStatusApproved
	service.PublicationStatus = ServicePublicationStatusOffline
	service.ApprovedByAdminID = ""
	service.ApprovedAt = &now
	service.UpdatedAt = now
	service.Version++
	return WithOrderability(service)
}

func IsPublicService(service Service) bool {
	return service.ReviewStatus == ServiceReviewStatusApproved &&
		service.PublicationStatus == ServicePublicationStatusOnline &&
		service.ModerationStatus == ServiceModerationStatusClear
}

func IsOrderableService(service Service) bool {
	return WithOrderability(service).IsOrderable
}

func WithOrderability(service Service) Service {
	reasons := OrderableReasons(service)
	service.IsOrderable = len(reasons) == 0
	service.OrderableReasons = reasons
	return service
}

func OrderableReasons(service Service) []string {
	reasons := []string{}
	if !service.AcceptingOrders {
		reasons = append(reasons, "not_accepting_orders")
	}
	if service.ReviewStatus != ServiceReviewStatusApproved {
		reasons = append(reasons, "review_not_approved")
	}
	if service.PublicationStatus != ServicePublicationStatusOnline {
		reasons = append(reasons, "not_online")
	}
	if service.ModerationStatus != ServiceModerationStatusClear {
		reasons = append(reasons, "moderation_not_clear")
	}
	if strings.TrimSpace(service.OwnerContactMethodID) == "" {
		reasons = append(reasons, "merchant_contact_unavailable")
	}
	if service.PaymentWindowMinutes < 3 || service.PaymentWindowMinutes > 15 {
		reasons = append(reasons, "payment_window_invalid")
	}
	if enabledPaymentOptionCount(service.PaymentOptions) == 0 {
		reasons = append(reasons, "payment_method_required")
	}
	return reasons
}

func enabledPaymentOptionCount(options []PaymentOption) int {
	count := 0
	for _, option := range options {
		if option.Enabled {
			count++
		}
	}
	return count
}

func matchesPaymentMethod(service Service, paymentMethod string) bool {
	paymentMethod = strings.TrimSpace(paymentMethod)
	if paymentMethod == "" {
		return true
	}
	for _, option := range service.PaymentOptions {
		if option.Enabled && option.PaymentMethod == paymentMethod {
			return true
		}
	}
	return false
}

func HasAccessMode(service Service, accessMode string) bool {
	accessMode = strings.TrimSpace(accessMode)
	if accessMode == "" {
		return true
	}
	for _, mode := range service.AccessModes {
		if strings.TrimSpace(mode.AccessMode) == accessMode {
			return true
		}
	}
	return false
}

func canUpdatePublication(service Service, action string) bool {
	switch action {
	case "publish":
		return service.ReviewStatus == ServiceReviewStatusApproved &&
			service.PublicationStatus == ServicePublicationStatusOffline &&
			service.ModerationStatus == ServiceModerationStatusClear
	case "pause":
		return service.PublicationStatus == ServicePublicationStatusOnline
	case "resume":
		return service.ReviewStatus == ServiceReviewStatusApproved &&
			service.PublicationStatus == ServicePublicationStatusOwnerPaused &&
			service.ModerationStatus == ServiceModerationStatusClear
	case "start_revision":
		return service.PublicationStatus == ServicePublicationStatusOnline ||
			service.PublicationStatus == ServicePublicationStatusOwnerPaused
	default:
		return false
	}
}

func applyPublicationAction(service Service, action string, now time.Time) Service {
	switch action {
	case "publish", "resume":
		service.PublicationStatus = ServicePublicationStatusOnline
	case "pause":
		service.PublicationStatus = ServicePublicationStatusOwnerPaused
		service.AcceptingOrders = false
	case "start_revision":
		service.PublicationStatus = ServicePublicationStatusOffline
		service.ReviewStatus = ServiceReviewStatusChangesRequested
		service.ApprovedByAdminID = ""
		service.ApprovedAt = nil
		service.AcceptingOrders = false
	}
	service.UpdatedAt = now
	service.Version++
	return WithOrderability(service)
}

func canUpdateAdminStatus(service Service, action string) bool {
	switch action {
	case "approve":
		return service.ReviewStatus == ServiceReviewStatusPendingReview &&
			service.ModerationStatus == ServiceModerationStatusClear
	case "request_changes":
		return service.ReviewStatus == ServiceReviewStatusPendingReview
	case "reject":
		return service.ReviewStatus == ServiceReviewStatusPendingReview
	case "suspend":
		return service.ModerationStatus == ServiceModerationStatusClear
	case "restore":
		return service.ModerationStatus == ServiceModerationStatusAdminSuspended
	case "remove":
		return service.ModerationStatus == ServiceModerationStatusClear ||
			service.ModerationStatus == ServiceModerationStatusAdminSuspended
	default:
		return false
	}
}

func applyAdminAction(service Service, input ServiceAdminActionInput, now time.Time) Service {
	switch input.Action {
	case "approve":
		service.ReviewStatus = ServiceReviewStatusApproved
		service.PublicationStatus = ServicePublicationStatusOffline
		service.ApprovedByAdminID = input.AdminUserID
		service.ApprovedAt = &now
	case "request_changes":
		service.ReviewStatus = ServiceReviewStatusChangesRequested
		service.PublicationStatus = ServicePublicationStatusOffline
		service.ApprovedByAdminID = ""
		service.ApprovedAt = nil
		service.AcceptingOrders = false
	case "reject":
		service.ReviewStatus = ServiceReviewStatusRejected
		service.PublicationStatus = ServicePublicationStatusOffline
		service.ApprovedByAdminID = ""
		service.ApprovedAt = nil
		service.AcceptingOrders = false
	case "suspend":
		service.ModerationStatus = ServiceModerationStatusAdminSuspended
		service.ModerationReason = strings.TrimSpace(input.Reason)
		service.AcceptingOrders = false
	case "restore":
		service.ModerationStatus = ServiceModerationStatusClear
		service.ModerationReason = strings.TrimSpace(input.Reason)
	case "remove":
		service.ModerationStatus = ServiceModerationStatusRemoved
		service.PublicationStatus = ServicePublicationStatusArchived
		service.ModerationReason = strings.TrimSpace(input.Reason)
		service.AcceptingOrders = false
	}
	if input.Action == "approve" || input.Action == "request_changes" || input.Action == "reject" {
		service.ModerationReason = strings.TrimSpace(input.Reason)
	}
	service.UpdatedAt = now
	service.Version++
	return WithOrderability(service)
}

func validatePublicServiceFilter(filter PublicServiceFilter) *domain.AppError {
	if strings.TrimSpace(filter.PaymentMethod) == "" {
		return nil
	}
	if !isSupportedPaymentMethod(filter.PaymentMethod) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method invalid", "付款方式不支持。", "paymentMethod", "invalid", "付款方式不支持。")
	}
	return nil
}

func validateOrderSettingsInput(input UpdateOrderSettingsInput) *domain.AppError {
	if input.PaymentWindowMinutes < 3 || input.PaymentWindowMinutes > 15 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment window invalid", "付款窗口必须在 3 到 15 分钟之间。", "paymentWindowMinutes", "range", "付款窗口必须在 3 到 15 分钟之间。")
	}
	if len(input.PaymentOptions) == 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment option required", "至少配置一种收款方式。", "paymentOptions", "required", "至少配置一种收款方式。")
	}
	seen := map[string]bool{}
	enabledCount := 0
	for i, option := range input.PaymentOptions {
		field := fmt.Sprintf("paymentOptions.%d", i)
		method := strings.TrimSpace(option.PaymentMethod)
		if !isSupportedPaymentMethod(method) {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method invalid", "付款方式不支持。", field+".paymentMethod", "invalid", "付款方式不支持。")
		}
		if seen[method] {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method duplicated", "付款方式不能重复。", field+".paymentMethod", "duplicate", "付款方式不能重复。")
		}
		seen[method] = true
		if option.Enabled {
			enabledCount++
			if strings.TrimSpace(option.PaymentInstructions) == "" {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment instructions required", "启用收款方式必须填写收款说明。", field+".paymentInstructions", "required", "必须填写收款说明。")
			}
		}
		if err := validateOptionalNonSecretText(field+".paymentInstructions", option.PaymentInstructions); err != nil {
			return err
		}
	}
	if input.AcceptingOrders && enabledCount == 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method required", "开启接单前至少启用一种收款方式。", "paymentOptions", "required", "至少启用一种收款方式。")
	}
	return nil
}

func isSupportedPaymentMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case PaymentMethodWechat, PaymentMethodAlipay, PaymentMethodUSDT:
		return true
	default:
		return false
	}
}

func buildPaymentOptions(serviceID string, current []PaymentOption, input []PaymentOptionInput, now time.Time) []PaymentOption {
	byMethod := map[string]PaymentOption{}
	for _, option := range current {
		byMethod[option.PaymentMethod] = option
	}
	options := make([]PaymentOption, 0, len(input))
	for _, item := range input {
		method := strings.TrimSpace(item.PaymentMethod)
		option := byMethod[method]
		if option.ID == "" {
			option.ID = uuid.NewString()
			option.APIServiceID = serviceID
			option.PaymentMethod = method
			option.CreatedAt = now
			option.Version = 1
		} else {
			option.Version++
		}
		option.APIServiceID = serviceID
		option.PaymentMethod = method
		option.Enabled = item.Enabled
		option.PaymentInstructions = strings.TrimSpace(item.PaymentInstructions)
		option.UpdatedAt = now
		options = append(options, option)
	}
	return options
}

func validateOptionalNonSecretText(field, value string) *domain.AppError {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return validateNonSecretText(field, value)
}

func validateNonSecretText(field, value string) *domain.AppError {
	value = strings.TrimSpace(value)
	if len(value) > 4000 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Text too long", "文本内容过长。", field, "too_long", "文本内容过长。")
	}
	if strings.ContainsAny(value, "\x00") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Text invalid", "文本内容包含非法字符。", field, "control_character", "文本内容包含非法字符。")
	}
	if looksLikeSecret(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在平台填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含 API Key、密码、Token、Session 或 Cookie。")
	}
	return nil
}

func normalizeDecimalText(value string, places int) string {
	rat, ok := parsePositiveDecimal(value)
	if !ok {
		return strings.TrimSpace(value)
	}
	return decimalString(rat, places)
}

func multiplyDecimalText(value, multiplier string, places int) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	left, ok := parseNonNegativeDecimal(value)
	if !ok {
		return ""
	}
	right, ok := parsePositiveDecimal(multiplier)
	if !ok {
		return ""
	}
	return decimalString(new(big.Rat).Mul(left, right), places)
}

func parseNonNegativeDecimal(value string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() < 0 {
		return nil, false
	}
	return rat, true
}

func parsePositiveDecimal(value string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() <= 0 {
		return nil, false
	}
	return rat, true
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
	return fmt.Sprintf("%s.%s", intPart.String(), fracText)
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

func looksLikeSecret(value string) bool {
	lower := strings.ToLower(value)
	needles := []string{"bearer ", "api_key=", "apikey=", "access_token=", "refresh_token=", "session=", "cookie=", "password=", "api key", "sub2api key", "secret=", "token="}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
