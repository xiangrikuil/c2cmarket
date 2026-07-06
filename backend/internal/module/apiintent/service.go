package apiintent

import (
	"context"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
)

const resourceType = "api_purchase_intent"

type PublicServiceResolver interface {
	PublicService(ctx context.Context, serviceID string) (apimarket.Service, *domain.AppError)
}

type OrderExistenceChecker interface {
	HasOrderForIntent(intentID string) bool
}

type Manager struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	services    PublicServiceResolver
	orders      OrderExistenceChecker
	contact     *contact.Service
	idempotency *idempotency.Service
	intents     map[string]Intent
	accessLogs  []ContactAccessLog
}

func NewManager(repo Repository, serviceResolver PublicServiceResolver, contactService *contact.Service, idempotencyService *idempotency.Service, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	if contactService == nil {
		contactService = contact.NewService(nil, now)
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	return &Manager{
		now:         now,
		repo:        repo,
		services:    serviceResolver,
		contact:     contactService,
		idempotency: idempotencyService,
		intents:     make(map[string]Intent),
	}
}

func (s *Manager) SetOrderExistenceChecker(checker OrderExistenceChecker) {
	s.orders = checker
}

func (s *Manager) CreateWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input CreateIntentInput, buildCompletion CompletionBuilder) (Intent, idempotency.Completion, bool, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return Intent{}, idempotency.Completion{}, false, err
	}
	if buildCompletion == nil {
		return Intent{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	if s.services == nil {
		return Intent{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 服务目录不可用。")
	}
	input.BuyerUserID = userID

	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return Intent{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		if entry.ResourceType == resourceType && entry.ResourceID != "" {
			intent, replayErr := s.buyerIntentWithMerchantContact(ctx, userID, entry.ResourceID, input.RequestID)
			if replayErr != nil {
				return Intent{}, idempotency.Completion{}, false, replayErr
			}
			completion, completionErr := buildCompletion(intent)
			return intent, completion, false, completionErr
		}
		return Intent{}, idempotency.CompletionFromEntry(entry), false, nil
	}

	service, appErr := s.services.PublicService(ctx, input.APIServiceID)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Intent{}, idempotency.Completion{}, false, appErr
	}
	if err := validateCreateInput(input, service); err != nil {
		s.idempotency.Cancel(ctx, entry)
		return Intent{}, idempotency.Completion{}, false, err
	}

	if s.repo != nil {
		intent, completion, appErr := s.repo.CreateAPIPurchaseIntentWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return Intent{}, idempotency.Completion{}, false, appErr
		}
		return intent, completion, true, nil
	}

	intent, appErr := s.createInMemory(input, service)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Intent{}, idempotency.Completion{}, false, appErr
	}
	completion, appErr := buildCompletion(intent)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Intent{}, idempotency.Completion{}, false, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, nil, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return Intent{}, idempotency.Completion{}, false, appErr
	}
	return intent, completion, true, nil
}

func (s *Manager) BuyerIntents(ctx context.Context, user auth.User) ([]Intent, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListAPIPurchaseIntentsByBuyer(ctx, user.ID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	intents := []Intent{}
	for _, intent := range s.intents {
		if intent.BuyerUserID == user.ID {
			intents = append(intents, deriveStatus(intent, s.now()))
		}
	}
	sort.Slice(intents, func(i, j int) bool {
		return intents[i].UpdatedAt.After(intents[j].UpdatedAt)
	})
	return intents, nil
}

func (s *Manager) BuyerIntent(ctx context.Context, user auth.User, intentID, requestID string) (Intent, *domain.AppError) {
	return s.buyerIntentWithMerchantContact(ctx, user.ID, intentID, requestID)
}

func (s *Manager) buyerIntent(ctx context.Context, buyerUserID, intentID string) (Intent, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetAPIPurchaseIntentForBuyer(ctx, buyerUserID, intentID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[strings.TrimSpace(intentID)]
	if !ok || intent.BuyerUserID != buyerUserID {
		return Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	return deriveStatus(intent, s.now()), nil
}

func (s *Manager) buyerIntentWithMerchantContact(ctx context.Context, buyerUserID, intentID, requestID string) (Intent, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetAPIPurchaseIntentForBuyerWithMerchantContact(ctx, buyerUserID, intentID, requestID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[strings.TrimSpace(intentID)]
	if !ok || intent.BuyerUserID != buyerUserID {
		return Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	intent = s.withMerchantContactLocked(deriveStatus(intent, s.now()))
	s.appendContactAccessLogLocked(intent.ID, buyerUserID, "merchant", requestID)
	return intent, nil
}

func (s *Manager) OwnerIntents(ctx context.Context, user auth.User) ([]Intent, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListAPIPurchaseIntentsByOwner(ctx, user.ID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	intents := []Intent{}
	for _, intent := range s.intents {
		if intent.OwnerUserID == user.ID {
			intents = append(intents, deriveStatus(intent, s.now()))
		}
	}
	sort.Slice(intents, func(i, j int) bool {
		return intents[i].UpdatedAt.After(intents[j].UpdatedAt)
	})
	return intents, nil
}

func (s *Manager) OwnerIntent(ctx context.Context, user auth.User, intentID, requestID string) (Intent, *domain.AppError) {
	return s.ownerIntentWithBuyerContact(ctx, user.ID, intentID, requestID)
}

func (s *Manager) ownerIntent(ctx context.Context, ownerUserID, intentID string) (Intent, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetAPIPurchaseIntentForOwner(ctx, ownerUserID, intentID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[strings.TrimSpace(intentID)]
	if !ok || intent.OwnerUserID != ownerUserID {
		return Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	return deriveStatus(intent, s.now()), nil
}

func (s *Manager) ownerIntentWithBuyerContact(ctx context.Context, ownerUserID, intentID, requestID string) (Intent, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetAPIPurchaseIntentForOwnerWithBuyerContact(ctx, ownerUserID, intentID, requestID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[strings.TrimSpace(intentID)]
	if !ok || intent.OwnerUserID != ownerUserID {
		return Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	intent = s.withBuyerContactLocked(deriveStatus(intent, s.now()))
	s.appendContactAccessLogLocked(intent.ID, ownerUserID, "buyer", requestID)
	return intent, nil
}

func (s *Manager) AdminIntents(ctx context.Context, user auth.User) ([]Intent, *domain.AppError) {
	if !user.IsAdmin {
		return nil, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.ListAdminAPIPurchaseIntents(ctx, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	intents := make([]Intent, 0, len(s.intents))
	for _, intent := range s.intents {
		intents = append(intents, deriveStatus(intent, s.now()))
	}
	sort.Slice(intents, func(i, j int) bool {
		return intents[i].UpdatedAt.After(intents[j].UpdatedAt)
	})
	return intents, nil
}

func (s *Manager) AdminIntent(ctx context.Context, user auth.User, intentID string) (Intent, *domain.AppError) {
	if !user.IsAdmin {
		return Intent{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.GetAdminAPIPurchaseIntent(ctx, intentID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[strings.TrimSpace(intentID)]
	if !ok {
		return Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	return deriveStatus(intent, s.now()), nil
}

func (s *Manager) CancelWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, true); err != nil {
		return idempotency.Completion{}, err
	}
	return s.updateWithIdempotency(ctx, routeKey, key, requestHash, input, buildCompletion, "cancel")
}

func (s *Manager) MarkContactedWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, false); err != nil {
		return idempotency.Completion{}, err
	}
	return s.updateWithIdempotency(ctx, routeKey, key, requestHash, input, buildCompletion, "mark_contacted")
}

func (s *Manager) CloseWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, true); err != nil {
		return idempotency.Completion{}, err
	}
	return s.updateWithIdempotency(ctx, routeKey, key, requestHash, input, buildCompletion, "close")
}

func (s *Manager) updateWithIdempotency(ctx context.Context, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder, action string) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}

	entry, appErr := s.idempotency.Begin(ctx, input.ActorUserID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		if entry.ResourceType == resourceType && entry.ResourceID != "" {
			var intent Intent
			var replayErr *domain.AppError
			if action == "cancel" {
				intent, replayErr = s.buyerIntent(ctx, input.ActorUserID, entry.ResourceID)
			} else {
				intent, replayErr = s.ownerIntent(ctx, input.ActorUserID, entry.ResourceID)
			}
			if replayErr != nil {
				return idempotency.Completion{}, replayErr
			}
			return buildCompletion(intent)
		}
		return idempotency.CompletionFromEntry(entry), nil
	}

	if s.repo != nil {
		var completion idempotency.Completion
		switch action {
		case "cancel":
			_, completion, appErr = s.repo.CancelAPIPurchaseIntentWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		case "mark_contacted":
			_, completion, appErr = s.repo.MarkAPIPurchaseIntentContactedWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		case "close":
			_, completion, appErr = s.repo.CloseAPIPurchaseIntentWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		default:
			appErr = domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "未知操作。")
		}
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	intent, appErr := s.updateInMemory(input, action)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(intent)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Manager) createInMemory(input CreateIntentInput, service apimarket.Service) (Intent, *domain.AppError) {
	buyerMethod, buyerVersion, ok := s.contact.VersionForOwner(input.BuyerContactMethodID, input.BuyerUserID)
	if !ok || !buyerMethod.Enabled {
		return Intent{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "买家联系方式不可用或不属于当前用户。")
	}
	ownerMethod, ownerVersion, ok := s.contact.VersionForOwner(service.OwnerContactMethodID, service.OwnerUserID)
	if !ok || !ownerMethod.Enabled {
		return Intent{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "商户联系方式不可用或归属不正确。")
	}

	intent, appErr := NewIntent(input, service, buyerMethod, buyerVersion, ownerMethod, ownerVersion, s.now())
	if appErr != nil {
		return Intent{}, appErr
	}
	intent.MerchantContact = &contact.ContactItemView{
		Side:        "merchant",
		SubjectID:   service.OwnerUserID,
		Type:        ownerMethod.Type,
		Label:       ownerMethod.Label,
		Value:       ownerVersion.Value,
		MaskedValue: ownerVersion.MaskedValue,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.intents[intent.ID] = intent
	s.appendContactAccessLogLocked(intent.ID, input.BuyerUserID, "merchant", input.RequestID)
	return intent, nil
}

func (s *Manager) updateInMemory(input ActionInput, action string) (Intent, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[strings.TrimSpace(input.IntentID)]
	if !ok || !canActorAccess(intent, input.ActorUserID, action) {
		return Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	now := s.now()
	if input.ExpectedVersion > 0 && intent.Version != input.ExpectedVersion {
		return Intent{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdateStatus(intent, action, now) {
		return Intent{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前购买意向状态不能执行该操作。")
	}
	if (action == "cancel" || action == "close") && s.orders != nil && s.orders.HasOrderForIntent(intent.ID) {
		return Intent{}, domain.NewAPIPurchaseIntentHasOrderError()
	}
	intent = applyAction(intent, action, strings.TrimSpace(input.Reason), now)
	s.intents[intent.ID] = intent
	return intent, nil
}

func (s *Manager) withMerchantContactLocked(intent Intent) Intent {
	if version, ok := s.contact.Version(intent.OwnerContactMethodVersionID); ok {
		intent.MerchantContact = &contact.ContactItemView{
			Side:        "merchant",
			Type:        intent.OwnerContactTypeSnapshot,
			Label:       intent.OwnerContactLabelSnapshot,
			Value:       version.Value,
			MaskedValue: version.MaskedValue,
		}
	}
	return intent
}

func (s *Manager) withBuyerContactLocked(intent Intent) Intent {
	if version, ok := s.contact.Version(intent.BuyerContactMethodVersionID); ok {
		intent.BuyerContact = &contact.ContactItemView{
			Side:        "buyer",
			Type:        intent.BuyerContactTypeSnapshot,
			Label:       intent.BuyerContactLabelSnapshot,
			Value:       version.Value,
			MaskedValue: version.MaskedValue,
		}
	}
	return intent
}

func (s *Manager) appendContactAccessLogLocked(intentID, viewerUserID, side, requestID string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	s.accessLogs = append(s.accessLogs, ContactAccessLog{
		ID:                     "api_intent_contact_access_" + intentID + "_" + side + "_" + viewerUserID + "_" + requestID,
		APIPurchaseIntentID:    intentID,
		ViewerUserID:           viewerUserID,
		ViewedContactOwnerSide: side,
		RequestID:              requestID,
		AccessedAt:             s.now(),
	})
}

func validateCreateInput(input CreateIntentInput, service apimarket.Service) *domain.AppError {
	if strings.TrimSpace(input.APIServiceID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service required", "必须提供 API 服务。", "apiServiceId", "required", "必须提供 API 服务。")
	}
	if strings.TrimSpace(input.BuyerContactMethodID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeContactMethodRequired, "Contact method required", "提交购买意向必须选择联系方式。", "buyerContactMethodId", "required", "必须选择联系方式。")
	}
	if input.BuyerUserID == service.OwnerUserID {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "不能向自己的 API 服务提交购买意向。")
	}
	if !apimarket.IsOrderableService(service) {
		return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}

	amount, ok := parsePositiveDecimal(input.RequestedCNYAmount)
	if !ok {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount invalid", "意向金额格式不正确。", "requestedCnyAmount", "invalid", "意向金额必须为正数。")
	}
	minimum, _ := parsePositiveDecimal(service.MinimumIntentCNY)
	if minimum != nil && amount.Cmp(minimum) < 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount too low", "意向金额不能低于服务最低金额。", "requestedCnyAmount", "too_low", "意向金额不能低于服务最低金额。")
	}
	if strings.TrimSpace(service.MaximumIntentCNY) != "" {
		maximum, ok := parsePositiveDecimal(service.MaximumIntentCNY)
		if !ok || amount.Cmp(maximum) > 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount too high", "意向金额不能高于服务最高金额。", "requestedCnyAmount", "too_high", "意向金额不能高于服务最高金额。")
		}
	}
	if err := validateOptionalNonSecretText("buyerNote", input.BuyerNote); err != nil {
		return err
	}
	if strings.TrimSpace(input.SelectedAccessMode) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access mode required", "必须选择接入方式。", "selectedAccessMode", "required", "必须选择接入方式。")
	}
	if !apimarket.HasAccessMode(service, input.SelectedAccessMode) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access mode invalid", "选择的接入方式不属于当前服务。", "selectedAccessMode", "invalid", "选择的接入方式不可用。")
	}

	switch service.BillingMode {
	case apimarket.ServiceBillingModeMetered:
		allowance, ok := parsePositiveDecimal(input.RequestedUSDAllowance)
		if !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance required", "美元额度服务必须填写意向美元额度。", "requestedUsdAllowance", "required", "必须填写意向美元额度。")
		}
		if strings.TrimSpace(service.DeclaredMaxUSDAllowancePerIntent) != "" {
			maxAllowance, ok := parsePositiveDecimal(service.DeclaredMaxUSDAllowancePerIntent)
			if !ok || allowance.Cmp(maxAllowance) > 0 {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance too high", "意向美元额度不能超过商户声明上限。", "requestedUsdAllowance", "too_high", "意向美元额度不能超过商户声明上限。")
			}
		}
		rate, ok := parsePositiveDecimal(service.DeclaredCNYPerUSDAllowance)
		if !ok {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance price invalid", "美元额度售价不可用。", "requestedUsdAllowance", "invalid", "美元额度售价不可用。")
		}
		expectedAmount := new(big.Rat).Mul(allowance, rate)
		if decimalString(expectedAmount, 2) != decimalString(amount, 2) {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount mismatch", "意向金额必须等于美元额度乘以商户声明单价。", "requestedCnyAmount", "mismatch", "意向金额必须匹配意向美元额度。")
		}
		if strings.TrimSpace(input.SelectedPackageID) != "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package not allowed", "美元额度服务不能选择固定套餐。", "selectedPackageId", "not_allowed", "该服务不使用固定套餐。")
		}
	case apimarket.ServiceBillingModeFixedPackage:
		if strings.TrimSpace(input.SelectedPackageID) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package required", "固定套餐服务必须选择套餐。", "selectedPackageId", "required", "必须选择套餐。")
		}
		pack, ok := findServicePackage(service, input.SelectedPackageID)
		if !ok || !pack.Enabled {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package invalid", "选择的套餐不可用。", "selectedPackageId", "invalid", "选择的套餐不可用。")
		}
		packPrice, ok := parsePositiveDecimal(pack.PriceCNY)
		if !ok || amount.Cmp(packPrice) != 0 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Intent amount mismatch", "意向金额必须等于所选套餐价格。", "requestedCnyAmount", "mismatch", "意向金额必须等于所选套餐价格。")
		}
		if strings.TrimSpace(input.RequestedUSDAllowance) != "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance not allowed", "固定套餐服务不能填写美元额度。", "requestedUsdAllowance", "not_allowed", "该服务不使用美元额度。")
		}
	case apimarket.ServiceBillingModeManual:
		if strings.TrimSpace(input.RequestedUSDAllowance) != "" {
			if _, ok := parsePositiveDecimal(input.RequestedUSDAllowance); !ok {
				return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USD allowance invalid", "意向美元额度格式不正确。", "requestedUsdAllowance", "invalid", "意向美元额度必须为正数。")
			}
		}
	default:
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务计费方式不可提交意向。")
	}
	return nil
}

func validateActionInput(input ActionInput, requireReason bool) *domain.AppError {
	if strings.TrimSpace(input.IntentID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API purchase intent required", "必须提供购买意向。", "intentId", "required", "必须提供购买意向。")
	}
	if requireReason && strings.TrimSpace(input.Reason) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "必须填写原因。", "reason", "required", "必须填写原因。")
	}
	if err := validateOptionalNonSecretText("reason", input.Reason); err != nil {
		return err
	}
	return nil
}

func deriveStatus(intent Intent, now time.Time) Intent {
	return intent
}

func canActorAccess(intent Intent, actorUserID, action string) bool {
	switch action {
	case "cancel":
		return intent.BuyerUserID == actorUserID
	case "mark_contacted", "close":
		return intent.OwnerUserID == actorUserID
	default:
		return false
	}
}

func canUpdateStatus(intent Intent, action string, now time.Time) bool {
	switch action {
	case "cancel":
		return intent.Status == StatusOpen || intent.Status == StatusContacted
	case "mark_contacted":
		return intent.Status == StatusOpen
	case "close":
		return intent.Status == StatusOpen || intent.Status == StatusContacted
	default:
		return false
	}
}

func applyAction(intent Intent, action, reason string, now time.Time) Intent {
	switch action {
	case "cancel":
		intent.Status = StatusBuyerCancelled
		intent.BuyerCancelledAt = &now
		intent.BuyerCancelReason = strings.TrimSpace(reason)
	case "mark_contacted":
		intent.Status = StatusContacted
		intent.ContactedAt = &now
	case "close":
		intent.Status = StatusOwnerClosed
		intent.OwnerClosedAt = &now
		intent.OwnerCloseReason = strings.TrimSpace(reason)
	}
	intent.UpdatedAt = now
	intent.Version++
	return intent
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
	if domain.LooksLikeSecretContent(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在平台填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含 API Key、密码、Token、Session 或 Cookie。")
	}
	return nil
}
