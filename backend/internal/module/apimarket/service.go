package apimarket

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"

	"github.com/google/uuid"
)

type APIModelResolver interface {
	APIModel(ctx context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError)
}

type Manager struct {
	mu                     sync.Mutex
	now                    func() time.Time
	repo                   Repository
	catalog                APIModelResolver
	contact                *contact.Service
	idempotency            *idempotency.Service
	services               map[string]Service
	serviceOrder           []string
	accountPaymentSettings map[string]AccountPaymentSettings
	serviceAuditEvents     []ServiceAuditEvent
}

func NewManager(repo Repository, catalogResolver APIModelResolver, contactService *contact.Service, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	if contactService == nil {
		contactService = contact.NewService(nil, now)
	}
	var idempotencyRepo idempotency.Repository
	if candidate, ok := repo.(idempotency.Repository); ok {
		idempotencyRepo = candidate
	}
	return &Manager{
		now:                    now,
		repo:                   repo,
		catalog:                catalogResolver,
		contact:                contactService,
		idempotency:            idempotency.NewService(idempotencyRepo, now),
		services:               make(map[string]Service),
		accountPaymentSettings: make(map[string]AccountPaymentSettings),
	}
}

func (s *Manager) beginAPIServiceIdempotency(ctx context.Context, userID, routeKey, key, requestHash string) (*idempotency.Entry, *domain.AppError) {
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return nil, appErr
	}
	return entry, nil
}

func (s *Manager) replayAPIServiceCompletion(ctx context.Context, entry *idempotency.Entry, user auth.User, adminView bool, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, bool, *domain.AppError) {
	if entry == nil || entry.State != "completed" {
		return idempotency.Completion{}, false, nil
	}
	if entry.ResourceType != "api_service" || strings.TrimSpace(entry.ResourceID) == "" {
		return idempotency.CompletionFromEntry(entry), true, nil
	}
	var (
		service Service
		appErr  *domain.AppError
	)
	if adminView {
		service, appErr = s.AdminService(ctx, user, entry.ResourceID)
	} else {
		service, appErr = s.OwnerService(ctx, user, entry.ResourceID)
	}
	if appErr != nil {
		return idempotency.Completion{}, true, appErr
	}
	completion, appErr := buildCompletion(service)
	return completion, true, appErr
}

func (s *Manager) finishMemoryAPIServiceCommand(ctx context.Context, entry *idempotency.Entry, service Service, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	completion, appErr := buildCompletion(service)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	body := completion.Body
	if completion.SkipBodyCache {
		// 内存模式也不得把联系方式或收款设置写进幂等缓存；重放时按资源 ID 重新授权读取。
		body = nil
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, body, completion.ResourceType, completion.ResourceID); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Manager) CreateWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateServiceInput, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	entry, appErr := s.beginAPIServiceIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if replay, completed, replayErr := s.replayAPIServiceCompletion(ctx, entry, user, false, buildCompletion); completed {
		return replay, replayErr
	}
	service, appErr := s.buildFromInput(ctx, Service{}, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.CreateAPIServiceWithIdempotency(ctx, *entry, service, input.RequestID, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
		}
		return completion, appErr
	}
	created, appErr := s.Create(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.finishMemoryAPIServiceCommand(ctx, entry, created, buildCompletion)
}

func (s *Manager) Create(ctx context.Context, user auth.User, input CreateServiceInput) (Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	service, appErr := s.buildFromInput(ctx, Service{}, input)
	if appErr != nil {
		return Service{}, appErr
	}
	if s.repo != nil {
		if appErr := s.repo.CreateAPIService(ctx, service, input.RequestID); appErr != nil {
			return Service{}, appErr
		}
		return s.repo.GetAPIServiceForOwner(ctx, user.ID, service.ID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if appErr := s.validateOwnerContacts(service, user.ID); appErr != nil {
		return Service{}, appErr
	}
	s.services[service.ID] = service
	s.serviceOrder = append(s.serviceOrder, service.ID)
	s.appendServiceAuditEventLocked(service, user.ID, "user", "api_service.created", input.RequestID, nil)
	return WithOrderability(service), nil
}

func (s *Manager) Update(ctx context.Context, user auth.User, input UpdateServiceInput) (Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	current, service, appErr := s.prepareAPIServiceUpdate(ctx, user, input)
	if appErr != nil {
		return Service{}, appErr
	}
	if s.repo != nil {
		return s.repo.UpdateAPIService(ctx, input, service, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if latest, ok := s.services[current.ID]; !ok || latest.Version != current.Version {
		return Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if appErr := s.validateOwnerContacts(service, user.ID); appErr != nil {
		return Service{}, appErr
	}
	s.services[service.ID] = service
	s.appendServiceAuditEventLocked(service, user.ID, "user", "api_service.updated", input.RequestID, []string{"service_configuration"})
	return WithOrderability(service), nil
}

func (s *Manager) UpdateWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input UpdateServiceInput, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	entry, appErr := s.beginAPIServiceIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if replay, completed, replayErr := s.replayAPIServiceCompletion(ctx, entry, user, false, buildCompletion); completed {
		return replay, replayErr
	}
	_, service, appErr := s.prepareAPIServiceUpdate(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateAPIServiceWithIdempotency(ctx, *entry, input, service, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
		}
		return completion, appErr
	}
	updated, appErr := s.Update(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.finishMemoryAPIServiceCommand(ctx, entry, updated, buildCompletion)
}

func (s *Manager) prepareAPIServiceUpdate(ctx context.Context, user auth.User, input UpdateServiceInput) (Service, Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ServiceID) == "" {
		return Service{}, Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}

	var current Service
	var appErr *domain.AppError
	if s.repo != nil {
		current, appErr = s.repo.GetAPIServiceForOwner(ctx, user.ID, input.ServiceID)
		if appErr != nil {
			return Service{}, Service{}, appErr
		}
	} else {
		s.mu.Lock()
		var ok bool
		current, ok = s.services[input.ServiceID]
		s.mu.Unlock()
		if !ok || current.OwnerUserID != user.ID {
			return Service{}, Service{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
		}
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return Service{}, Service{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canEditService(current) {
		return Service{}, Service{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务状态不能直接修改，请先开始修订。")
	}

	service, appErr := s.buildFromInput(ctx, current, CreateServiceInput{
		OwnerUserID:                      user.ID,
		MerchantProfileID:                input.MerchantProfileID,
		MerchantIdentityMode:             input.MerchantIdentityMode,
		OwnerContactMethodID:             input.OwnerContactMethodID,
		OwnerContactMethodIDs:            append([]string(nil), input.OwnerContactMethodIDs...),
		ProbeConnectionID:                input.ProbeConnectionID,
		Title:                            input.Title,
		ShortDescription:                 input.ShortDescription,
		SourceURL:                        input.SourceURL,
		DistributionSystem:               input.DistributionSystem,
		BillingMode:                      input.BillingMode,
		DeclaredCNYPerUSDAllowance:       input.DeclaredCNYPerUSDAllowance,
		DeclaredMaxUSDAllowancePerIntent: input.DeclaredMaxUSDAllowancePerIntent,
		AvailableUSDAllowance:            input.AvailableUSDAllowance,
		QuotaExpiresAt:                   input.QuotaExpiresAt,
		QuotaUsagePolicy:                 input.QuotaUsagePolicy,
		MinimumIntentCNY:                 input.MinimumIntentCNY,
		MaximumIntentCNY:                 input.MaximumIntentCNY,
		UsageVisibility:                  input.UsageVisibility,
		PublicAccessNote:                 input.PublicAccessNote,
		MerchantNote:                     input.MerchantNote,
		AccountPoolType:                  input.AccountPoolType,
		AccountPoolCustomName:            input.AccountPoolCustomName,
		MerchantRefundCommitment:         input.MerchantRefundCommitment,
		DeclaredMaxConcurrency:           input.DeclaredMaxConcurrency,
		PromptAuditEnabled:               input.PromptAuditEnabled,
		AccessModes:                      input.AccessModes,
		Models:                           input.Models,
		Packages:                         input.Packages,
	})
	if appErr != nil {
		return Service{}, Service{}, appErr
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

	return current, service, nil
}

func (s *Manager) UpdateProbeConnection(ctx context.Context, user auth.User, input UpdateProbeConnectionInput) (Service, *domain.AppError) {
	input.OwnerUserID = user.ID
	input.ServiceID = strings.TrimSpace(input.ServiceID)
	input.ProbeConnectionID = strings.TrimSpace(input.ProbeConnectionID)
	if input.ServiceID == "" {
		return Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}
	if s.repo != nil {
		return s.repo.UpdateAPIServiceProbeConnection(ctx, input, s.now())
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
	if strings.TrimSpace(service.ProbeConnectionID) == input.ProbeConnectionID {
		return WithOrderability(service), nil
	}
	service.ProbeConnectionID = input.ProbeConnectionID
	service.ProbeReady = input.ProbeConnectionID != ""
	service.ProbeBaseURL = ""
	service.NormalizedProbeBaseURL = ""
	if service.ProbeReady {
		// 内存运行切片没有探针持久化层，只为本地流程测试提供固定目标快照。
		service.ProbeBaseURL = "https://api.example.com/v1"
		service.NormalizedProbeBaseURL = "https://api.example.com/v1"
	}
	service.UpdatedAt = s.now()
	service.Version++
	service = WithOrderability(service)
	s.services[service.ID] = service
	s.appendServiceAuditEventLocked(service, user.ID, "user", "api_service.probe_binding_changed", input.RequestID, []string{"probe_connection"})
	return service, nil
}

func (s *Manager) UpdateProbeConnectionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input UpdateProbeConnectionInput, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	input.ServiceID = strings.TrimSpace(input.ServiceID)
	input.ProbeConnectionID = strings.TrimSpace(input.ProbeConnectionID)
	if input.ServiceID == "" {
		return idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}
	entry, appErr := s.beginAPIServiceIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if replay, completed, replayErr := s.replayAPIServiceCompletion(ctx, entry, user, false, buildCompletion); completed {
		return replay, replayErr
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateAPIServiceProbeConnectionWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
		}
		return completion, appErr
	}
	updated, appErr := s.UpdateProbeConnection(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.finishMemoryAPIServiceCommand(ctx, entry, updated, buildCompletion)
}

func (s *Manager) PublicServices(ctx context.Context, filter PublicServiceFilter, page domain.PageRequest) (domain.Page[Service], *domain.AppError) {
	if err := validatePublicServiceFilter(filter); err != nil {
		return domain.Page[Service]{}, err
	}
	if s.repo != nil {
		return s.repo.ListPublicAPIServices(ctx, filter, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	services := []Service{}
	now := s.now()
	for _, id := range s.serviceOrder {
		service := WithOrderabilityAt(s.services[id], now)
		if service.IsOrderable && matchesPublicServiceFilter(service, filter) {
			services = append(services, service)
		}
	}
	sortPublicServices(services, filter)
	return domain.PageItems(services, page)
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

func (s *Manager) OwnerServices(ctx context.Context, user auth.User, filter OwnerServiceFilter, page domain.PageRequest) (domain.Page[Service], *domain.AppError) {
	filter, appErr := NormalizeOwnerServiceFilter(filter)
	if appErr != nil {
		return domain.Page[Service]{}, appErr
	}
	if s.repo != nil {
		return s.repo.ListAPIServicesByOwner(ctx, user.ID, filter, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	services := []Service{}
	now := s.now()
	for _, id := range s.serviceOrder {
		service := WithOrderabilityAt(s.services[id], now)
		service.SalesSummary = SalesSummaryForService(service, now)
		if service.OwnerUserID == user.ID && MatchesOwnerSalesView(service.SalesSummary.OverallState, filter.SalesView) {
			services = append(services, service)
		}
	}
	return domain.PageItems(services, page)
}

func NormalizeOwnerServiceFilter(filter OwnerServiceFilter) (OwnerServiceFilter, *domain.AppError) {
	filter.SalesView = strings.TrimSpace(filter.SalesView)
	if filter.SalesView == "" {
		filter.SalesView = OwnerSalesViewActive
	}
	switch filter.SalesView {
	case OwnerSalesViewActive, OwnerSalesViewExpired, OwnerSalesViewPaused, OwnerSalesViewDraft, OwnerSalesViewAll:
		return filter, nil
	default:
		return OwnerServiceFilter{}, domain.NewFieldError(
			http.StatusUnprocessableEntity,
			domain.CodeValidationFailed,
			"Invalid owner sales view",
			"销售状态筛选无效。",
			"salesView",
			"invalid",
			"salesView 必须是 active、expired、paused、draft 或 all。",
		)
	}
}

func MatchesOwnerSalesView(state, salesView string) bool {
	switch salesView {
	case OwnerSalesViewActive:
		return state == ServiceSalesStateSelling || state == ServiceSalesStateUpcoming
	case OwnerSalesViewExpired:
		return state == ServiceSalesStateExpired
	case OwnerSalesViewPaused:
		return state == ServiceSalesStatePaused
	case OwnerSalesViewDraft:
		return state == ServiceSalesStateDraft || state == ServiceSalesStateOffline
	case OwnerSalesViewAll:
		return true
	default:
		return false
	}
}

func SalesSummaryForService(service Service, now time.Time) ServiceSalesSummary {
	channels := []ServiceSalesChannel{}
	if service.BillingMode == ServiceBillingModeMetered {
		available := strings.TrimSpace(service.AvailableUSDAllowance)
		if available == "" {
			available = strings.TrimSpace(service.DeclaredMaxUSDAllowancePerIntent)
		}
		channels = append(channels, ServiceSalesChannel{
			Kind:                  ServiceSalesChannelFlexibleQuota,
			State:                 flexibleQuotaSalesState(service, now),
			AvailableUSDAllowance: available,
			ExpiresAt:             service.QuotaExpiresAt,
		})
	}
	overall := serviceFallbackSalesState(service, now)
	if len(channels) > 0 {
		overall = HighestPrioritySalesState(channels)
	}
	return ServiceSalesSummary{OverallState: overall, Channels: channels}
}

func HighestPrioritySalesState(channels []ServiceSalesChannel) string {
	bestState := ServiceSalesStateArchived
	bestPriority := salesStatePriority(bestState)
	for _, channel := range channels {
		priority := salesStatePriority(channel.State)
		if priority < bestPriority {
			bestPriority = priority
			bestState = channel.State
		}
	}
	return bestState
}

func salesStatePriority(state string) int {
	switch state {
	case ServiceSalesStateSelling:
		return 0
	case ServiceSalesStateUpcoming:
		return 1
	case ServiceSalesStatePaused:
		return 2
	case ServiceSalesStateSoldOut:
		return 3
	case ServiceSalesStateExpired:
		return 4
	case ServiceSalesStateDraft:
		return 5
	case ServiceSalesStateOffline:
		return 6
	case ServiceSalesStateArchived:
		return 7
	default:
		return 8
	}
}

func flexibleQuotaSalesState(service Service, now time.Time) string {
	if state := serviceLifecycleSalesState(service); state != "" {
		return state
	}
	if service.QuotaExpiresAt == nil {
		return ServiceSalesStateOffline
	}
	if !service.QuotaExpiresAt.After(now.Add(24 * time.Hour)) {
		return ServiceSalesStateExpired
	}
	available := strings.TrimSpace(service.AvailableUSDAllowance)
	if available == "" {
		available = strings.TrimSpace(service.DeclaredMaxUSDAllowancePerIntent)
	}
	if amount, ok := parseNonNegativeDecimal(available); !ok || amount.Sign() == 0 {
		return ServiceSalesStateSoldOut
	}
	if WithOrderabilityAt(service, now).IsOrderable {
		return ServiceSalesStateSelling
	}
	return ServiceSalesStateOffline
}

func serviceFallbackSalesState(service Service, now time.Time) string {
	if state := serviceLifecycleSalesState(service); state != "" {
		return state
	}
	orderable := WithOrderabilityAt(service, now)
	if orderable.IsOrderable {
		return ServiceSalesStateSelling
	}
	for _, reason := range orderable.OrderableReasons {
		if reason == "quota_sold_out" || reason == "package_sold_out" {
			return ServiceSalesStateSoldOut
		}
		if reason == "quota_expired" {
			return ServiceSalesStateExpired
		}
	}
	return ServiceSalesStateOffline
}

func serviceLifecycleSalesState(service Service) string {
	switch {
	case service.PublicationStatus == ServicePublicationStatusArchived || service.ModerationStatus == ServiceModerationStatusRemoved:
		return ServiceSalesStateArchived
	case service.PublicationStatus == ServicePublicationStatusOwnerPaused || service.ModerationStatus == ServiceModerationStatusAdminSuspended:
		return ServiceSalesStatePaused
	case service.ReviewStatus != ServiceReviewStatusApproved:
		return ServiceSalesStateDraft
	case service.PublicationStatus != ServicePublicationStatusOnline:
		return ServiceSalesStateOffline
	default:
		return ""
	}
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

func (s *Manager) AdminServices(ctx context.Context, user auth.User, filter AdminServiceFilter, page domain.PageRequest) (domain.Page[Service], *domain.AppError) {
	if !user.IsAdmin {
		return domain.Page[Service]{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.ListAdminAPIServices(ctx, filter, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	services := make([]Service, 0, len(s.serviceOrder))
	for _, id := range s.serviceOrder {
		services = append(services, WithOrderability(s.services[id]))
	}
	return domain.PageItems(filterAdminServices(services, filter), page)
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
	if appErr := s.validateOwnerContacts(service, user.ID); appErr != nil {
		return Service{}, appErr
	}

	service = applyEarlyAutoApprovalPolicy(service, s.now())
	s.services[service.ID] = service
	s.appendServiceAuditEventLocked(service, user.ID, "user", "api_service.review_submitted", input.RequestID, nil)
	return service, nil
}

func (s *Manager) SubmitForReviewWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input ServiceOwnerActionInput, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ServiceID) == "" {
		return idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}
	entry, appErr := s.beginAPIServiceIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if replay, completed, replayErr := s.replayAPIServiceCompletion(ctx, entry, user, false, buildCompletion); completed {
		return replay, replayErr
	}
	if appErr := requireEarlyAutoApprovalEligibility(user); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.SubmitAPIServiceForReviewWithIdempotency(ctx, *entry, user, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
		}
		return completion, appErr
	}
	updated, appErr := s.SubmitForReview(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.finishMemoryAPIServiceCommand(ctx, entry, updated, buildCompletion)
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
		if strings.TrimSpace(service.ProbeConnectionID) == "" || !service.ProbeReady {
			return Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe connection required", "上线 API 服务前必须绑定已启用且验证通过的探针连接。", "probeConnectionId", "not_ready", "请选择已启用且验证通过的探针连接。")
		}
		if len(service.OwnerContactMethodIDs) == 0 {
			return Service{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeMerchantContactRequired, "Merchant contact required", "上线 API 服务必须配置商户联系方式。")
		}
		if appErr := s.validateOwnerContacts(service, user.ID); appErr != nil {
			return Service{}, domain.NewError(http.StatusConflict, domain.CodeMerchantContactUnavailable, "Merchant contact unavailable", "商户联系方式当前不可用。")
		}
	}

	service = applyPublicationAction(service, action, s.now())
	s.services[service.ID] = service
	s.appendServiceAuditEventLocked(service, user.ID, "user", apiServicePublicationEventType(action), input.RequestID, nil)
	return WithOrderability(service), nil
}

func (s *Manager) UpdatePublicationWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input ServiceOwnerActionInput, action string, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	if action != "publish" && action != "pause" && action != "resume" && action != "start_revision" {
		return idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid action", "API 服务操作无效。", "action", "invalid", "API 服务操作无效。")
	}
	input.OwnerUserID = user.ID
	entry, appErr := s.beginAPIServiceIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if replay, completed, replayErr := s.replayAPIServiceCompletion(ctx, entry, user, false, buildCompletion); completed {
		return replay, replayErr
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateAPIServicePublicationWithIdempotency(ctx, *entry, input, action, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
		}
		return completion, appErr
	}
	updated, appErr := s.UpdatePublication(ctx, user, input, action)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.finishMemoryAPIServiceCommand(ctx, entry, updated, buildCompletion)
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
	s.appendServiceAuditEventLocked(service, user.ID, "admin", apiServiceAdminEventType(input.Action), input.RequestID, nil)
	return WithOrderability(service), nil
}

func (s *Manager) UpdateAdminStatusWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input ServiceAdminActionInput, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if !user.IsAdmin {
		return idempotency.Completion{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.AdminUserID = user.ID
	if appErr := validateAdminActionInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.beginAPIServiceIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if replay, completed, replayErr := s.replayAPIServiceCompletion(ctx, entry, user, true, buildCompletion); completed {
		return replay, replayErr
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateAPIServiceModerationWithIdempotency(ctx, *entry, user, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
		}
		return completion, appErr
	}
	updated, appErr := s.UpdateAdminStatus(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.finishMemoryAPIServiceCommand(ctx, entry, updated, buildCompletion)
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
	if service.AcceptingOrders == input.AcceptingOrders &&
		service.PaymentWindowMinutes == input.PaymentWindowMinutes &&
		PaymentOptionsMatchInput(service.PaymentOptions, input.PaymentOptions) {
		return WithOrderability(service), nil
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
	s.appendServiceAuditEventLocked(service, user.ID, "user", "api_service.order_settings_changed", input.RequestID, []string{"order_settings"})
	return service, nil
}

func apiServicePublicationEventType(action string) string {
	switch strings.TrimSpace(action) {
	case "publish":
		return "api_service.published"
	case "pause":
		return "api_service.paused"
	case "resume":
		return "api_service.resumed"
	case "start_revision":
		return "api_service.revision_started"
	default:
		return ""
	}
}

func apiServiceAdminEventType(action string) string {
	switch strings.TrimSpace(action) {
	case "approve":
		return "api_service.approved"
	case "request_changes":
		return "api_service.changes_requested"
	case "reject":
		return "api_service.rejected"
	case "suspend":
		return "api_service.suspended"
	case "restore":
		return "api_service.restored"
	case "remove":
		return "api_service.removed"
	default:
		return ""
	}
}

// appendServiceAuditEventLocked 只记录允许进入操作审计的结构化事实。
// 调用方必须持有 s.mu；空动作代表业务未发生真实迁移，因此不写事件。
func (s *Manager) appendServiceAuditEventLocked(service Service, actorUserID, actorKind, eventType, requestID string, changedFields []string) {
	if strings.TrimSpace(eventType) == "" {
		return
	}
	s.serviceAuditEvents = append(s.serviceAuditEvents, ServiceAuditEvent{
		EventType:        eventType,
		ActorUserID:      strings.TrimSpace(actorUserID),
		ActorKind:        strings.TrimSpace(actorKind),
		RequestID:        strings.TrimSpace(requestID),
		AggregateID:      service.ID,
		AggregateVersion: service.Version,
		ChangedFields:    append([]string(nil), changedFields...),
		CreatedAt:        s.now(),
	})
}

// ServiceAuditEvents 返回审计事实的深拷贝，避免测试或调用方修改内部状态。
func (s *Manager) ServiceAuditEvents() []ServiceAuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]ServiceAuditEvent, len(s.serviceAuditEvents))
	for i, event := range s.serviceAuditEvents {
		events[i] = event
		events[i].ChangedFields = append([]string(nil), event.ChangedFields...)
	}
	return events
}

func (s *Manager) UpdateOrderSettingsWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input UpdateOrderSettingsInput, buildCompletion ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	if strings.TrimSpace(input.ServiceID) == "" {
		return idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "serviceId", "required", "必须提供 API 服务。")
	}
	if appErr := validateOrderSettingsInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.beginAPIServiceIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if replay, completed, replayErr := s.replayAPIServiceCompletion(ctx, entry, user, false, buildCompletion); completed {
		return replay, replayErr
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateAPIServiceOrderSettingsWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
		}
		return completion, appErr
	}
	updated, appErr := s.UpdateOrderSettings(ctx, user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.finishMemoryAPIServiceCommand(ctx, entry, updated, buildCompletion)
}

func (s *Manager) buildFromInput(ctx context.Context, current Service, input CreateServiceInput) (Service, *domain.AppError) {
	now := s.now()
	isCreating := current.ID == ""
	ownerContactMethodIDs, appErr := normalizeOwnerContactMethodIDs(input.OwnerContactMethodID, input.OwnerContactMethodIDs)
	if appErr != nil {
		return Service{}, appErr
	}
	input.OwnerContactMethodIDs = ownerContactMethodIDs
	input.OwnerContactMethodID = ownerContactMethodIDs[0]
	if strings.TrimSpace(input.BillingMode) == ServiceBillingModeMetered && strings.TrimSpace(input.AvailableUSDAllowance) == "" {
		// 兼容迁移期客户端：旧字段只提供单笔上限时，以该值初始化真实可售额度。
		input.AvailableUSDAllowance = input.DeclaredMaxUSDAllowancePerIntent
	}
	if err := validateCreateInput(input, now); err != nil {
		return Service{}, err
	}
	quotaExpiresAt, _ := parseQuotaExpiresAt(input.QuotaExpiresAt)
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

	quotaUsagePolicy := UnspecifiedQuotaUsagePolicy()
	if strings.TrimSpace(input.BillingMode) == ServiceBillingModeMetered {
		quotaUsagePolicy = NormalizeQuotaUsagePolicy(input.QuotaUsagePolicy)
	}
	probeConnectionID := strings.TrimSpace(input.ProbeConnectionID)
	probeReady := current.ProbeReady && current.ProbeConnectionID == probeConnectionID
	probeBaseURL := current.ProbeBaseURL
	normalizedProbeBaseURL := current.NormalizedProbeBaseURL
	if s.repo == nil && probeConnectionID != "" {
		probeReady = true
		// 内存运行切片没有探针持久化层，只为本地流程测试提供固定目标快照。
		probeBaseURL = "https://api.example.com/v1"
		normalizedProbeBaseURL = "https://api.example.com/v1"
	}
	if current.ProbeConnectionID != probeConnectionID {
		probeBaseURL = ""
		normalizedProbeBaseURL = ""
		if s.repo == nil && probeConnectionID != "" {
			probeBaseURL = "https://api.example.com/v1"
			normalizedProbeBaseURL = "https://api.example.com/v1"
		}
	}
	service := Service{
		ID:                               serviceID,
		OwnerUserID:                      input.OwnerUserID,
		MerchantProfileID:                strings.TrimSpace(input.MerchantProfileID),
		MerchantIdentityMode:             strings.TrimSpace(input.MerchantIdentityMode),
		OwnerContactMethodID:             strings.TrimSpace(input.OwnerContactMethodID),
		OwnerContactMethodIDs:            append([]string(nil), input.OwnerContactMethodIDs...),
		ProbeConnectionID:                probeConnectionID,
		ProbeReady:                       probeReady,
		ProbeBaseURL:                     probeBaseURL,
		NormalizedProbeBaseURL:           normalizedProbeBaseURL,
		Title:                            strings.TrimSpace(input.Title),
		ShortDescription:                 strings.TrimSpace(input.ShortDescription),
		SourceURL:                        strings.TrimSpace(input.SourceURL),
		DistributionSystem:               strings.TrimSpace(input.DistributionSystem),
		BillingMode:                      strings.TrimSpace(input.BillingMode),
		DeclaredCNYPerUSDAllowance:       strings.TrimSpace(input.DeclaredCNYPerUSDAllowance),
		DeclaredMaxUSDAllowancePerIntent: strings.TrimSpace(input.DeclaredMaxUSDAllowancePerIntent),
		AvailableUSDAllowance:            strings.TrimSpace(input.AvailableUSDAllowance),
		QuotaExpiresAt:                   quotaExpiresAt,
		QuotaUsagePolicy:                 quotaUsagePolicy,
		MinimumIntentCNY:                 strings.TrimSpace(input.MinimumIntentCNY),
		MaximumIntentCNY:                 strings.TrimSpace(input.MaximumIntentCNY),
		UsageVisibility:                  strings.TrimSpace(input.UsageVisibility),
		PublicAccessNote:                 strings.TrimSpace(input.PublicAccessNote),
		MerchantNote:                     strings.TrimSpace(input.MerchantNote),
		MerchantSupportNote:              MerchantSupportNote(*input.MerchantRefundCommitment),
		AccountPoolType:                  strings.TrimSpace(input.AccountPoolType),
		AccountPoolCustomName:            strings.TrimSpace(input.AccountPoolCustomName),
		MerchantRefundCommitment:         *input.MerchantRefundCommitment,
		DeclaredTTFTBand:                 current.DeclaredTTFTBand,
		DeclaredMaxConcurrency:           input.DeclaredMaxConcurrency,
		PerformanceConfirmedAt:           current.PerformanceConfirmedAt,
		PromptAuditEnabled:               input.PromptAuditEnabled,
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
	currentModels := make(map[string]ServiceModel, len(current.Models))
	for _, model := range current.Models {
		currentModels[model.ModelCatalogID] = model
	}
	retainedModelIDs := make(map[string]bool, len(input.Models))
	for _, modelInput := range input.Models {
		if s.catalog == nil {
			return Service{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 模型目录不可用。")
		}
		model, appErr := s.catalog.APIModel(ctx, modelInput.ModelCatalogID)
		if appErr != nil {
			return Service{}, appErr
		}
		if model.EffectiveStatus != "" && !model.IsEffectiveActive() {
			return Service{}, domain.NewFieldError(
				http.StatusUnprocessableEntity,
				domain.CodeInvalidStateTransition,
				"API model catalog unavailable",
				"所选模型目录已退役或被阻断，不能用于新发布。",
				"models",
				"catalog_unavailable",
				"请移除不可用模型后重新发布。",
			)
		}
		multiplier := strings.TrimSpace(modelInput.MerchantMultiplier)
		if multiplier == "" {
			multiplier = "1.0000"
		}
		priceVersionID := strings.TrimSpace(modelInput.ModelPriceVersionID)
		if priceVersionID == "" {
			priceVersionID = model.CurrentPriceVersionID
		}
		modelID := uuid.NewString()
		modelCreatedAt := now
		if existing, ok := currentModels[model.ID]; ok {
			modelID = existing.ID
			modelCreatedAt = existing.CreatedAt
		}
		retainedModelIDs[model.ID] = true
		service.Models = append(service.Models, ServiceModel{
			ID:                                  modelID,
			APIServiceID:                        service.ID,
			DistributionSystem:                  service.DistributionSystem,
			ModelCatalogID:                      model.ID,
			ModelPriceVersionID:                 priceVersionID,
			ModelKey:                            model.ModelKey,
			ProviderSnapshot:                    model.Provider,
			CapabilitiesSnapshot:                append([]string(nil), model.Capabilities...),
			MerchantMultiplier:                  normalizeDecimalText(multiplier, 4),
			EffectiveInputPricePerMillion:       multiplyDecimalText(model.InputPricePerMillion, multiplier, 6),
			EffectiveCachedInputPricePerMillion: multiplyDecimalText(model.CachedInputPricePerMillion, multiplier, 6),
			EffectiveOutputPricePerMillion:      multiplyDecimalText(model.OutputPricePerMillion, multiplier, 6),
			Enabled:                             modelInput.Enabled,
			CreatedAt:                           modelCreatedAt,
			UpdatedAt:                           now,
		})
	}
	for _, existing := range current.Models {
		if retainedModelIDs[existing.ModelCatalogID] {
			continue
		}
		existing.Enabled = false
		existing.UpdatedAt = now
		service.Models = append(service.Models, existing)
	}
	packageModels := make(map[string]ServiceModel, len(service.Models))
	for _, model := range service.Models {
		if model.Enabled {
			packageModels[model.ModelCatalogID] = model
		}
	}
	currentPackages := make(map[string]ServicePackage, len(current.Packages))
	for _, pack := range current.Packages {
		currentPackages[pack.ID] = pack
	}
	retainedPackageIDs := make(map[string]bool, len(input.Packages))
	for _, packageInput := range input.Packages {
		pack := ServicePackage{
			ID:               uuid.NewString(),
			APIServiceID:     service.ID,
			Name:             strings.TrimSpace(packageInput.Name),
			PriceCNY:         strings.TrimSpace(packageInput.PriceCNY),
			PanelAllowance:   strings.TrimSpace(packageInput.PanelAllowance),
			QuotaUsagePolicy: NormalizeQuotaUsagePolicy(packageInput.QuotaUsagePolicy),
			DurationDays:     packageInput.DurationDays,
			StockTotal:       packageInput.StockTotal,
			StockAvailable:   packageInput.StockTotal,
			Description:      strings.TrimSpace(packageInput.Description),
			Enabled:          packageInput.Enabled,
			SortOrder:        packageInput.SortOrder,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if packageID := strings.TrimSpace(packageInput.ID); !isCreating && packageID != "" {
			existing, ok := currentPackages[packageID]
			if !ok {
				return Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package invalid", "套餐不属于当前 API 服务。", "packages", "invalid_id", "套餐不属于当前 API 服务。")
			}
			available := existing.StockAvailable + packageInput.StockTotal - existing.StockTotal
			if available < 0 {
				return Service{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package stock invalid", "套餐总库存不能低于已预占或已售数量。", "packages", "stock_below_committed", "套餐总库存不能低于已预占或已售数量。")
			}
			pack.ID = existing.ID
			pack.StockAvailable = available
			pack.CreatedAt = existing.CreatedAt
		}
		retainedPackageIDs[pack.ID] = true
		for _, modelCatalogID := range packageInput.ModelCatalogIDs {
			model := packageModels[strings.TrimSpace(modelCatalogID)]
			pack.Models = append(pack.Models, servicePackageModelFromServiceModel(model))
		}
		service.Packages = append(service.Packages, pack)
	}
	for _, existing := range current.Packages {
		if retainedPackageIDs[existing.ID] {
			continue
		}
		existing.Enabled = false
		existing.UpdatedAt = now
		service.Packages = append(service.Packages, existing)
	}
	if service.BillingMode != ServiceBillingModeMetered {
		service.DeclaredCNYPerUSDAllowance = ""
	}
	return service, nil
}

func validateCreateInput(input CreateServiceInput, now time.Time) *domain.AppError {
	if _, appErr := normalizeOwnerContactMethodIDs(input.OwnerContactMethodID, input.OwnerContactMethodIDs); appErr != nil {
		return appErr
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
	if err := validateOptionalLinuxDoTopicURL(input.SourceURL); err != nil {
		return err
	}
	if err := validateOptionalNonSecretText("publicAccessNote", input.PublicAccessNote); err != nil {
		return err
	}
	if err := validateOptionalNonSecretText("merchantNote", input.MerchantNote); err != nil {
		return err
	}
	if appErr := validateAccountPool(input.AccountPoolType, input.AccountPoolCustomName); appErr != nil {
		return appErr
	}
	if input.MerchantRefundCommitment == nil {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Merchant refund commitment required", "请选择是否提供商户退款承诺。", "merchantRefundCommitment", "required", "请选择无额外退款承诺或商户全额退款承诺。")
	}
	if appErr := validateServiceDeclaration(input); appErr != nil {
		return appErr
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
		if appErr := validateQuotaExpiresAt(input.QuotaExpiresAt, now, true); appErr != nil {
			return appErr
		}
		if available, ok := parseNonNegativeDecimal(input.AvailableUSDAllowance); !ok || available.Sign() < 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Available USD allowance invalid", "可售美元额度格式不正确。", "availableUsdAllowance", "invalid", "可售美元额度必须是大于等于 0 的数字。")
		}
		if appErr := ValidateQuotaUsagePolicy(input.QuotaUsagePolicy, "quotaUsagePolicy", false); appErr != nil {
			return appErr
		}
	case ServiceBillingModeManual:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Billing mode unsupported", "当前版本暂不支持商户手工核对计费。", "billingMode", "unsupported", "当前版本暂不支持商户手工核对计费，请使用美元额度或固定套餐。")
	case ServiceBillingModeFixedPackage:
		if appErr := validateQuotaExpiresAt(input.QuotaExpiresAt, now, false); appErr != nil {
			return appErr
		}
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
	seenModels := map[string]bool{}
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
		if seenModels[strings.TrimSpace(model.ModelCatalogID)] {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Model duplicated", "支持模型不能重复。", field+".modelCatalogId", "duplicate", "支持模型不能重复。")
		}
		seenModels[strings.TrimSpace(model.ModelCatalogID)] = true
	}
	if input.BillingMode == ServiceBillingModeFixedPackage && len(input.Models) == 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Model required", "限时套餐必须选择支持模型。", "models", "required", "必须选择支持模型。")
	}
	enabledModels := map[string]bool{}
	for _, model := range input.Models {
		if model.Enabled {
			enabledModels[strings.TrimSpace(model.ModelCatalogID)] = true
		}
	}
	seenPackageIDs := map[string]bool{}
	for i, pack := range input.Packages {
		field := fmt.Sprintf("packages.%d", i)
		if appErr := ValidateQuotaUsagePolicy(pack.QuotaUsagePolicy, field+".quotaUsagePolicy", false); appErr != nil {
			return appErr
		}
		if strings.TrimSpace(pack.Name) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package name required", "套餐名称不能为空。", field+".name", "required", "套餐名称不能为空。")
		}
		if _, ok := parsePositiveDecimal(pack.PriceCNY); !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package price invalid", "套餐价格格式不正确。", field+".priceCny", "invalid", "套餐价格必须为正数。")
		}
		if packID := strings.TrimSpace(pack.ID); packID != "" {
			if seenPackageIDs[packID] {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package duplicated", "套餐不能重复。", field+".id", "duplicate", "套餐不能重复。")
			}
			seenPackageIDs[packID] = true
		}
		if _, ok := parsePositiveDecimal(pack.PanelAllowance); !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package allowance invalid", "面板额度格式不正确。", field+".panelAllowance", "invalid", "面板额度必须为正数。")
		}
		if pack.DurationDays == nil || !isLimitedPackageDuration(*pack.DurationDays) {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package duration invalid", "套餐有效期只能选择 1、3、7 或 30 天。", field+".durationDays", "invalid", "套餐有效期只能选择 1、3、7 或 30 天。")
		}
		if pack.StockTotal < 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package stock invalid", "套餐库存不能小于 0。", field+".stockTotal", "invalid", "套餐库存不能小于 0。")
		}
		if len(pack.ModelCatalogIDs) == 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package models required", "每个套餐至少选择一个支持模型。", field+".modelCatalogIds", "required", "每个套餐至少选择一个支持模型。")
		}
		seenPackageModels := map[string]bool{}
		for modelIndex, modelCatalogID := range pack.ModelCatalogIDs {
			modelCatalogID = strings.TrimSpace(modelCatalogID)
			modelField := fmt.Sprintf("%s.modelCatalogIds.%d", field, modelIndex)
			if !enabledModels[modelCatalogID] {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package model invalid", "套餐模型必须来自当前服务已启用的模型。", modelField, "invalid", "套餐模型必须来自当前服务已启用的模型。")
			}
			if seenPackageModels[modelCatalogID] {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package model duplicated", "套餐模型不能重复。", modelField, "duplicate", "套餐模型不能重复。")
			}
			seenPackageModels[modelCatalogID] = true
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

func normalizeOwnerContactMethodIDs(primary string, values []string) ([]string, *domain.AppError) {
	if len(values) == 0 && strings.TrimSpace(primary) != "" {
		values = []string{primary}
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, contact.WechatRequiredError("ownerContactMethodIds", "发布 API 服务前必须先配置微信联系方式。")
		}
		if _, exists := seen[value]; exists {
			return nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Contact method duplicated", "商户联系方式不能重复选择。", "ownerContactMethodIds", "duplicate", "联系方式不能重复。")
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, contact.WechatRequiredError("ownerContactMethodIds", "发布 API 服务前必须先配置微信联系方式。")
	}
	if len(result) != 1 {
		return nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "One WeChat contact required", "API 服务只能使用当前账号唯一的微信联系方式。", "ownerContactMethodIds", "invalid_count", "只能提交一个微信联系方式。")
	}
	return result, nil
}

func (s *Manager) validateOwnerContacts(service Service, ownerUserID string) *domain.AppError {
	for _, methodID := range service.OwnerContactMethodIDs {
		method, _, ok := s.contact.WechatVersionForOwnerAndScope(methodID, ownerUserID, contact.UsageScopeAPIMerchant)
		if !ok || !method.Enabled {
			return contact.WechatRequiredError("ownerContactMethodIds", "发布 API 服务前必须先配置微信联系方式。")
		}
	}
	return nil
}

func isLimitedPackageDuration(days int) bool {
	switch days {
	case 1, 3, 7, 30:
		return true
	default:
		return false
	}
}

func servicePackageModelFromServiceModel(model ServiceModel) ServicePackageModel {
	return ServicePackageModel{
		ServiceModelID:      model.ID,
		ModelCatalogID:      model.ModelCatalogID,
		ModelPriceVersionID: model.ModelPriceVersionID,
		ModelKey:            model.ModelKey,
		ProviderSnapshot:    model.ProviderSnapshot,
		MerchantMultiplier:  model.MerchantMultiplier,
	}
}

func validateQuotaExpiresAt(value string, now time.Time, required bool) *domain.AppError {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if !required {
			return nil
		}
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Quota expiration required", "美元额度服务必须填写固定有效时间。", "quotaExpiresAt", "required", "必须填写有效至时间。")
	}
	expiresAt, ok := parseQuotaExpiresAt(trimmed)
	if !ok {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Quota expiration invalid", "额度有效时间格式不正确。", "quotaExpiresAt", "invalid", "有效时间格式不正确。")
	}
	if !expiresAt.After(now) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Quota expiration expired", "额度有效时间必须晚于当前时间。", "quotaExpiresAt", "expired", "有效时间必须晚于当前时间。")
	}
	return nil
}

func parseQuotaExpiresAt(value string) (*time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, true
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return nil, false
	}
	expiresAt = expiresAt.UTC()
	return &expiresAt, true
}

func validateServiceDeclaration(input CreateServiceInput) *domain.AppError {
	if input.DeclaredMaxConcurrency < 1 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Maximum concurrency required", "必须填写商户声明最大并发。", "declaredMaxConcurrency", "required", "请输入大于 0 的最大并发。")
	}
	if input.PromptAuditEnabled == nil {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Prompt audit selection required", "必须声明是否开启提示词审计。", "promptAuditEnabled", "required", "请选择是否开启提示词审计。")
	}
	return nil
}

func validateAccountPool(poolType, customName string) *domain.AppError {
	poolType = strings.TrimSpace(poolType)
	customName = strings.TrimSpace(customName)
	switch poolType {
	case AccountPoolGPTPro20x, AccountPoolGPTPro5x, AccountPoolGPTPlus:
		if customName != "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Account pool custom name invalid", "预设号池不能填写自定义名称。", "accountPoolCustomName", "invalid", "只有选择其他号池时才能填写自定义名称。")
		}
	case AccountPoolCustom:
		length := len([]rune(customName))
		if length < 2 || length > 40 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Account pool custom name invalid", "自定义号池名称长度必须为 2 到 40 个字符。", "accountPoolCustomName", "invalid", "请输入 2 到 40 个字符的号池名称。")
		}
		if err := validateNonSecretText("accountPoolCustomName", customName); err != nil {
			return err
		}
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Account pool invalid", "请选择有效的号池类型。", "accountPoolType", "invalid", "请选择 GPT Pro 20x、GPT Pro 5x、GPT Plus 或其他。")
	}
	return nil
}

func AccountPoolLabel(service Service) string {
	switch service.AccountPoolType {
	case AccountPoolGPTPro20x:
		return "GPT Pro 20x"
	case AccountPoolGPTPro5x:
		return "GPT Pro 5x"
	case AccountPoolGPTPlus:
		return "GPT Plus"
	case AccountPoolCustom:
		return strings.TrimSpace(service.AccountPoolCustomName)
	default:
		return ""
	}
}

func MerchantSupportNote(enabled bool) string {
	if !enabled {
		return "无额外退款承诺，具体问题由双方站外协商。平台不托管、不垫付、不代赔。"
	}
	return "商户退款承诺：订单服务有效期内，如未交付、实际号池/模型/额度与订单快照不符，或交付后连续不可用超过 1 小时且不属于排除情形，商户承诺退还订单全部实付金额。买家违规、超出商户声明最大并发、额度正常耗尽、正常上游限流或买家自身网络问题不适用。平台不托管、不垫付、不代赔。"
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
	return WithOrderabilityAt(service, time.Now())
}

func WithOrderabilityAt(service Service, now time.Time) Service {
	reasons := OrderableReasonsAt(service, now)
	service.IsOrderable = len(reasons) == 0
	service.OrderableReasons = reasons
	return service
}

func OrderableReasons(service Service) []string {
	return OrderableReasonsAt(service, time.Now())
}

func OrderableReasonsAt(service Service, now time.Time) []string {
	reasons := []string{}
	if !service.AcceptingOrders {
		reasons = append(reasons, "not_accepting_orders")
	}
	if strings.TrimSpace(service.ProbeConnectionID) == "" {
		reasons = append(reasons, "probe_connection_required")
	} else if !service.ProbeReady {
		reasons = append(reasons, "probe_connection_not_ready")
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
	switch service.BillingMode {
	case ServiceBillingModeMetered:
		availableText := strings.TrimSpace(service.AvailableUSDAllowance)
		if availableText == "" {
			availableText = strings.TrimSpace(service.DeclaredMaxUSDAllowancePerIntent)
		}
		if available, ok := parseNonNegativeDecimal(availableText); !ok || available.Sign() == 0 {
			reasons = append(reasons, "quota_sold_out")
		}
		if service.QuotaExpiresAt == nil {
			reasons = append(reasons, "quota_expiration_required")
		} else if !service.QuotaExpiresAt.After(now.Add(24 * time.Hour)) {
			reasons = append(reasons, "quota_expired")
		}
	case ServiceBillingModeFixedPackage:
		available := false
		for _, pack := range service.Packages {
			if pack.Enabled && pack.StockAvailable > 0 && len(pack.Models) > 0 {
				available = true
				break
			}
		}
		if !available {
			reasons = append(reasons, "package_sold_out")
		}
	default:
		reasons = append(reasons, "billing_mode_unsupported")
	}
	return reasons
}

func enabledPaymentOptionCount(options []PaymentOption) int {
	count := 0
	for _, option := range options {
		if option.Enabled && IsSupportedPaymentMethod(option.PaymentMethod) {
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
	if !IsSupportedPaymentMethod(paymentMethod) {
		return false
	}
	for _, option := range service.PaymentOptions {
		if option.Enabled && IsSupportedPaymentMethod(option.PaymentMethod) && option.PaymentMethod == paymentMethod {
			return true
		}
	}
	return false
}

func matchesPublicServiceFilter(service Service, filter PublicServiceFilter) bool {
	if !matchesPaymentMethod(service, filter.PaymentMethod) {
		return false
	}
	billingMode := strings.TrimSpace(filter.BillingMode)
	if billingMode != "" && service.BillingMode != billingMode {
		return false
	}
	distributionSystem := strings.TrimSpace(filter.DistributionSystem)
	if distributionSystem != "" && service.DistributionSystem != distributionSystem {
		return false
	}
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	if search != "" {
		values := []string{service.Title, service.ShortDescription, service.MerchantDisplayName}
		for _, model := range service.Models {
			values = append(values, model.ModelKey, model.ProviderSnapshot)
		}
		matched := false
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), search) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	modelCatalogID := strings.TrimSpace(filter.ModelCatalogID)
	if modelCatalogID != "" {
		matched := false
		for _, model := range service.Models {
			if model.Enabled && model.ModelCatalogID == modelCatalogID {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if !decimalFilterAtMost(service.DeclaredCNYPerUSDAllowance, filter.MaxCNYPerUSD) ||
		!decimalFilterAtMost(service.MinimumIntentCNY, filter.MinimumIntentCNYMax) {
		return false
	}
	packageModelCatalogID := strings.TrimSpace(filter.PackageModelCatalogID)
	if packageModelCatalogID == "" && filter.PackageDurationDays == 0 &&
		strings.TrimSpace(filter.PackagePriceCNYMax) == "" && strings.TrimSpace(filter.PackageMultiplierMax) == "" {
		return true
	}
	for _, item := range service.Packages {
		if !item.Enabled || item.StockAvailable <= 0 {
			continue
		}
		if filter.PackageDurationDays > 0 && (item.DurationDays == nil || *item.DurationDays != filter.PackageDurationDays) {
			continue
		}
		if !decimalFilterAtMost(item.PriceCNY, filter.PackagePriceCNYMax) {
			continue
		}
		if packageModelCatalogID == "" && strings.TrimSpace(filter.PackageMultiplierMax) == "" {
			return true
		}
		for _, model := range item.Models {
			if (packageModelCatalogID == "" || model.ModelCatalogID == packageModelCatalogID) &&
				decimalFilterAtMost(model.MerchantMultiplier, filter.PackageMultiplierMax) {
				return true
			}
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
	if paymentMethod := strings.TrimSpace(filter.PaymentMethod); paymentMethod != "" && !IsSupportedPaymentMethod(paymentMethod) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method invalid", "付款方式不支持。", "paymentMethod", "invalid", "付款方式不支持。")
	}
	if billingMode := strings.TrimSpace(filter.BillingMode); billingMode != "" && billingMode != ServiceBillingModeMetered && billingMode != ServiceBillingModeFixedPackage {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Billing mode invalid", "计费模式筛选无效。", "billingMode", "invalid", "计费模式筛选无效。")
	}
	if filter.PackageDurationDays != 0 && filter.PackageDurationDays != 1 && filter.PackageDurationDays != 3 && filter.PackageDurationDays != 7 && filter.PackageDurationDays != 30 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package duration invalid", "套餐有效期筛选无效。", "packageDurationDays", "invalid", "套餐有效期仅支持 1、3、7 或 30 天。")
	}
	if distributionSystem := strings.TrimSpace(filter.DistributionSystem); distributionSystem != "" && distributionSystem != ServiceDistributionSub2API && distributionSystem != "new_api_proxy" && distributionSystem != "other" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Distribution system invalid", "接入系统筛选无效。", "distributionSystem", "invalid", "接入系统筛选无效。")
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 100 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Search query too long", "搜索关键词不能超过 100 个字符。", "search", "max_length", "搜索关键词不能超过 100 个字符。")
	}
	decimalFields := []struct {
		field string
		value string
	}{
		{field: "maxCnyPerUsd", value: filter.MaxCNYPerUSD},
		{field: "minimumIntentCnyMax", value: filter.MinimumIntentCNYMax},
		{field: "packagePriceCnyMax", value: filter.PackagePriceCNYMax},
		{field: "packageMultiplierMax", value: filter.PackageMultiplierMax},
	}
	for _, item := range decimalFields {
		if strings.TrimSpace(item.value) == "" {
			continue
		}
		if _, ok := parseNonNegativeDecimal(item.value); !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Decimal filter invalid", "数值筛选必须是非负数字。", item.field, "invalid", "请输入非负数字。")
		}
	}
	if sortMode := strings.TrimSpace(filter.Sort); sortMode != "" && sortMode != PublicServiceSortUpdatedDesc &&
		sortMode != PublicServiceSortRecommended && sortMode != PublicServiceSortReputationDesc &&
		sortMode != PublicServiceSortCompletedDesc && sortMode != PublicServiceSortResponseFast &&
		sortMode != PublicServiceSortPriceAsc && sortMode != PublicServiceSortMinimumPurchaseAsc && sortMode != PublicServiceSortPackagePriceAsc {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Sort invalid", "排序方式无效。", "sort", "invalid", "排序方式无效。")
	}
	if strings.TrimSpace(filter.Sort) == PublicServiceSortPriceAsc && strings.TrimSpace(filter.BillingMode) != ServiceBillingModeMetered {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Sort incompatible", "单价排序只适用于自选额度。", "sort", "incompatible", "单价排序只适用于自选额度。")
	}
	if strings.TrimSpace(filter.Sort) == PublicServiceSortPackagePriceAsc && strings.TrimSpace(filter.BillingMode) != ServiceBillingModeFixedPackage {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Sort incompatible", "套餐价格排序只适用于短期流量包。", "sort", "incompatible", "套餐价格排序只适用于短期流量包。")
	}
	return nil
}

func (filter PublicServiceFilter) NormalizedSort() string {
	switch strings.TrimSpace(filter.Sort) {
	case PublicServiceSortRecommended, PublicServiceSortReputationDesc, PublicServiceSortCompletedDesc,
		PublicServiceSortResponseFast, PublicServiceSortPriceAsc, PublicServiceSortMinimumPurchaseAsc, PublicServiceSortPackagePriceAsc:
		return strings.TrimSpace(filter.Sort)
	default:
		return PublicServiceSortUpdatedDesc
	}
}

func sortPublicServices(services []Service, filter PublicServiceFilter) {
	sortMode := filter.NormalizedSort()
	sort.Slice(services, func(i, j int) bool {
		left := services[i]
		right := services[j]
		if sortMode == PublicServiceSortUpdatedDesc {
			if left.UpdatedAt.Equal(right.UpdatedAt) {
				return left.ID > right.ID
			}
			return left.UpdatedAt.After(right.UpdatedAt)
		}
		if sortMode == PublicServiceSortRecommended {
			if comparison := comparePublicServiceRecommendation(left, right); comparison != 0 {
				return comparison < 0
			}
			return left.ID < right.ID
		}
		if sortMode == PublicServiceSortReputationDesc {
			leftValue, leftOK := publicServiceReputationValue(left)
			rightValue, rightOK := publicServiceReputationValue(right)
			if leftOK != rightOK {
				return leftOK
			}
			if leftOK {
				if comparison := leftValue.Cmp(rightValue); comparison != 0 {
					return comparison > 0
				}
			}
			return left.ID < right.ID
		}
		if sortMode == PublicServiceSortCompletedDesc {
			if left.Completed30d != right.Completed30d {
				return left.Completed30d > right.Completed30d
			}
			return left.ID < right.ID
		}
		if sortMode == PublicServiceSortResponseFast {
			if comparison := compareNullableResponse(left.ResponseMedianMinutes, right.ResponseMedianMinutes); comparison != 0 {
				return comparison < 0
			}
			return left.ID < right.ID
		}

		leftValue, leftOK := publicServiceSortValue(left, filter)
		rightValue, rightOK := publicServiceSortValue(right, filter)
		if leftOK != rightOK {
			return leftOK
		}
		if leftOK {
			if comparison := leftValue.Cmp(rightValue); comparison != 0 {
				return comparison < 0
			}
		}
		return left.ID < right.ID
	})
}

func publicServiceSortValue(service Service, filter PublicServiceFilter) (*big.Rat, bool) {
	switch filter.NormalizedSort() {
	case PublicServiceSortPriceAsc:
		return parseNonNegativeDecimal(service.DeclaredCNYPerUSDAllowance)
	case PublicServiceSortMinimumPurchaseAsc:
		return parseNonNegativeDecimal(service.MinimumIntentCNY)
	case PublicServiceSortPackagePriceAsc:
		return minimumPackagePriceForPublicFilter(service, filter)
	default:
		return nil, false
	}
}

func compareNullableResponse(left, right *float64) int {
	if left == nil {
		if right == nil {
			return 0
		}
		return 1
	}
	if right == nil {
		return -1
	}
	if *left < *right {
		return -1
	}
	if *left > *right {
		return 1
	}
	return 0
}

func publicServiceReputationValue(service Service) (*big.Rat, bool) {
	if service.SellerReputation == nil {
		return nil, false
	}
	tierScore := map[string]int{
		reputation.TierInsufficient: 1,
		reputation.TierNormal:       2,
		reputation.TierReliable:     3,
		reputation.TierHighTrust:    4,
	}[service.SellerReputation.Tier]
	rating := service.SellerReputation.Metrics.WeightedRating
	if rating == nil {
		rating = new(float64)
	}
	return new(big.Rat).SetFloat64(float64(tierScore)*1000000 + *rating*1000 + float64(service.SellerReputation.Metrics.VerifiedReviewCount)), true
}

func comparePublicServiceRecommendation(left, right Service) int {
	leftReputation, leftHasReputation := publicServiceReputationValue(left)
	rightReputation, rightHasReputation := publicServiceReputationValue(right)
	if leftHasReputation != rightHasReputation {
		if leftHasReputation {
			return -1
		}
		return 1
	}
	if leftHasReputation {
		if comparison := leftReputation.Cmp(rightReputation); comparison != 0 {
			return -comparison
		}
	}
	if comparison := compareNullableResponse(left.ResponseMedianMinutes, right.ResponseMedianMinutes); comparison != 0 {
		return comparison
	}
	if left.Completed30d != right.Completed30d {
		if left.Completed30d > right.Completed30d {
			return -1
		}
		return 1
	}
	if left.UnresolvedDisputes != right.UnresolvedDisputes {
		if left.UnresolvedDisputes < right.UnresolvedDisputes {
			return -1
		}
		return 1
	}
	if left.UpdatedAt.Equal(right.UpdatedAt) {
		return 0
	}
	if left.UpdatedAt.After(right.UpdatedAt) {
		return -1
	}
	return 1
}

func minimumPackagePriceForPublicFilter(service Service, filter PublicServiceFilter) (*big.Rat, bool) {
	modelCatalogID := strings.TrimSpace(filter.PackageModelCatalogID)
	var minimum *big.Rat
	for _, item := range service.Packages {
		if !item.Enabled || item.StockAvailable <= 0 ||
			(filter.PackageDurationDays > 0 && (item.DurationDays == nil || *item.DurationDays != filter.PackageDurationDays)) ||
			!decimalFilterAtMost(item.PriceCNY, filter.PackagePriceCNYMax) {
			continue
		}
		price, ok := parseNonNegativeDecimal(item.PriceCNY)
		if !ok {
			continue
		}
		matchesModel := false
		for _, model := range item.Models {
			if (modelCatalogID == "" || model.ModelCatalogID == modelCatalogID) &&
				decimalFilterAtMost(model.MerchantMultiplier, filter.PackageMultiplierMax) {
				matchesModel = true
				break
			}
		}
		if matchesModel && (minimum == nil || price.Cmp(minimum) < 0) {
			minimum = price
		}
	}
	return minimum, minimum != nil
}

func decimalFilterAtMost(actual, maximum string) bool {
	maximum = strings.TrimSpace(maximum)
	if maximum == "" {
		return true
	}
	actualValue, actualOK := parseNonNegativeDecimal(actual)
	maximumValue, maximumOK := parseNonNegativeDecimal(maximum)
	return actualOK && maximumOK && actualValue.Cmp(maximumValue) <= 0
}

func validateOrderSettingsInput(input UpdateOrderSettingsInput) *domain.AppError {
	if input.PaymentWindowMinutes < 3 || input.PaymentWindowMinutes > 15 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment window invalid", "付款窗口必须在 3 到 15 分钟之间。", "paymentWindowMinutes", "range", "付款窗口必须在 3 到 15 分钟之间。")
	}
	enabledCount, appErr := validatePaymentOptionInputs(input.PaymentOptions)
	if appErr != nil {
		return appErr
	}
	if input.AcceptingOrders && enabledCount == 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method required", "开启接单前至少启用一种收款方式。", "paymentOptions", "required", "至少启用一种收款方式。")
	}
	return nil
}

func IsSupportedPaymentMethod(method string) bool {
	switch strings.TrimSpace(method) {
	case PaymentMethodWechat, PaymentMethodAlipay:
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
		if !shouldPersistPaymentOption(item) {
			continue
		}
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
		option.PaymentQRCodeDataURL = strings.TrimSpace(item.PaymentQRCodeDataURL)
		option.UpdatedAt = now
		options = append(options, option)
	}
	return options
}

func shouldPersistPaymentOption(input PaymentOptionInput) bool {
	return input.Enabled || strings.TrimSpace(input.PaymentInstructions) != "" || strings.TrimSpace(input.PaymentQRCodeDataURL) != ""
}

// PaymentOptionsMatchInput compares settings without exposing the sensitive
// instruction or QR-code values to operation audit metadata.
func PaymentOptionsMatchInput(current []PaymentOption, input []PaymentOptionInput) bool {
	persisted := make(map[string]PaymentOption, len(current))
	for _, option := range current {
		persisted[strings.TrimSpace(option.PaymentMethod)] = option
	}
	wanted := make(map[string]PaymentOptionInput, len(input))
	for _, option := range input {
		if !shouldPersistPaymentOption(option) {
			continue
		}
		wanted[strings.TrimSpace(option.PaymentMethod)] = option
	}
	if len(persisted) != len(wanted) {
		return false
	}
	for method, requested := range wanted {
		stored, ok := persisted[method]
		if !ok || stored.Enabled != requested.Enabled ||
			strings.TrimSpace(stored.PaymentInstructions) != strings.TrimSpace(requested.PaymentInstructions) ||
			strings.TrimSpace(stored.PaymentQRCodeDataURL) != strings.TrimSpace(requested.PaymentQRCodeDataURL) {
			return false
		}
	}
	return true
}

func requiresPaymentQRCode(method string) bool {
	switch strings.TrimSpace(method) {
	case PaymentMethodWechat, PaymentMethodAlipay:
		return true
	default:
		return false
	}
}

func validateOptionalPaymentQRCodeDataURL(field, value string) *domain.AppError {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 2*1024*1024 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "QR code too large", "收款码图片过大。", field, "too_large", "收款码图片过大。")
	}
	if strings.ContainsAny(value, "\x00\r\n\t") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "QR code invalid", "收款码数据格式不正确。", field, "invalid", "收款码数据格式不正确。")
	}
	if !strings.HasPrefix(value, "data:image/") || !strings.Contains(value, ";base64,") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "QR code invalid", "收款码必须是图片 data URL。", field, "invalid", "收款码必须是图片 data URL。")
	}
	return nil
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

func validateOptionalLinuxDoTopicURL(value string) *domain.AppError {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 2048 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "linux.do 原帖链接过长。", "sourceUrl", "too_long", "原帖链接过长。")
	}
	if strings.ContainsAny(value, "\x00\r\n\t") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "linux.do 原帖链接包含非法字符。", "sourceUrl", "control_character", "原帖链接包含非法字符。")
	}
	if !strings.HasPrefix(value, "https://linux.do/t/") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "linux.do 原帖链接必须是 https://linux.do/t/*。", "sourceUrl", "invalid", "必须填写 https://linux.do/t/* 原帖链接。")
	}
	if looksLikeSecret(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "linux.do 原帖链接不能包含认证秘密。", "sourceUrl", "secret_content", "原帖链接不能包含 API Key、Token、Session 或 Cookie。")
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
