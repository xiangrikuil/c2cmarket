package apiorder

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

const resourceType = "api_order"

type BuyerIntentResolver interface {
	BuyerIntent(ctx context.Context, user auth.User, intentID, requestID string) (apiintent.Intent, *domain.AppError)
}

type PublicServiceResolver interface {
	PublicService(ctx context.Context, serviceID string) (apimarket.Service, *domain.AppError)
}

type DisputeCaseCreator interface {
	RegisterAPIOrderDispute(ctx context.Context, input DisputeCaseInput) (string, *domain.AppError)
}

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	intents     BuyerIntentResolver
	services    PublicServiceResolver
	disputes    DisputeCaseCreator
	idempotency *idempotency.Service
	orders      map[string]Order
	events      []Event
	accessLogs  []PaymentInstructionAccessLog
}

func NewService(repo Repository, intentResolver BuyerIntentResolver, serviceResolver PublicServiceResolver, disputeCreator DisputeCaseCreator, idempotencyService *idempotency.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	return &Service{
		now:         now,
		repo:        repo,
		intents:     intentResolver,
		services:    serviceResolver,
		disputes:    disputeCreator,
		idempotency: idempotencyService,
		orders:      make(map[string]Order),
	}
}

func (s *Service) CreateWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input CreateInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.BuyerUserID = userID
	return s.createOrUpdateWithIdempotency(ctx, userID, routeKey, key, requestHash, input, ActionInput{}, buildCompletion, "create")
}

func (s *Service) BuyerOrders(ctx context.Context, user auth.User) ([]Order, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListAPIOrdersByBuyer(ctx, user.ID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	orders := []Order{}
	for id, order := range s.orders {
		if order.BuyerUserID != user.ID {
			continue
		}
		order = s.materializeTimeoutLocked(id)
		orders = append(orders, order)
	}
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].UpdatedAt.After(orders[j].UpdatedAt)
	})
	return orders, nil
}

func (s *Service) HasOrderForIntent(intentID string) bool {
	intentID = strings.TrimSpace(intentID)
	if intentID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, order := range s.orders {
		if order.APIPurchaseIntentID == intentID {
			return true
		}
	}
	return false
}

func (s *Service) BuyerOrder(ctx context.Context, user auth.User, orderID string) (Order, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetAPIOrderForBuyer(ctx, user.ID, orderID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[strings.TrimSpace(orderID)]
	if !ok || order.BuyerUserID != user.ID {
		return Order{}, notFound()
	}
	return s.materializeTimeoutLocked(order.ID), nil
}

func (s *Service) ReadPaymentInstructions(ctx context.Context, user auth.User, orderID, requestID string) (PaymentInstructionsView, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ReadAPIOrderPaymentInstructions(ctx, user.ID, orderID, requestID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[strings.TrimSpace(orderID)]
	if !ok || order.BuyerUserID != user.ID {
		return PaymentInstructionsView{}, notFound()
	}
	order = s.materializeTimeoutLocked(order.ID)
	if order.Status != StatusPendingPayment || !s.now().Before(order.PaymentExpiresAt) {
		return PaymentInstructionsView{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前订单不再是有效付款入口。")
	}
	s.appendAccessLogLocked(order.ID, user.ID, requestID)
	s.appendEventLocked(order, user.ID, EventPaymentInstructionsRead, order.Status, order.Status, "", requestID)
	return PaymentInstructionsView{
		OrderID:             order.ID,
		PaymentMethod:       order.SelectedPaymentMethod,
		PaymentInstructions: order.PaymentInstructionsSnapshot,
		PaymentExpiresAt:    order.PaymentExpiresAt,
	}, nil
}

func (s *Service) SellerOrders(ctx context.Context, user auth.User) ([]Order, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListAPIOrdersBySeller(ctx, user.ID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	orders := []Order{}
	for id, order := range s.orders {
		if order.SellerUserID != user.ID {
			continue
		}
		order = s.materializeTimeoutLocked(id)
		orders = append(orders, order)
	}
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].UpdatedAt.After(orders[j].UpdatedAt)
	})
	return orders, nil
}

func (s *Service) SellerOrder(ctx context.Context, user auth.User, orderID string) (Order, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetAPIOrderForSeller(ctx, user.ID, orderID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[strings.TrimSpace(orderID)]
	if !ok || order.SellerUserID != user.ID {
		return Order{}, notFound()
	}
	return s.materializeTimeoutLocked(order.ID), nil
}

func (s *Service) SubmitPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, "submit_payment"); err != nil {
		return idempotency.Completion{}, err
	}
	return s.createOrUpdateWithIdempotency(ctx, userID, routeKey, key, requestHash, CreateInput{}, input, buildCompletion, "submit_payment")
}

func (s *Service) CancelWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, "cancel"); err != nil {
		return idempotency.Completion{}, err
	}
	return s.createOrUpdateWithIdempotency(ctx, userID, routeKey, key, requestHash, CreateInput{}, input, buildCompletion, "cancel")
}

func (s *Service) ConfirmCompleteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, "confirm_complete"); err != nil {
		return idempotency.Completion{}, err
	}
	return s.createOrUpdateWithIdempotency(ctx, userID, routeKey, key, requestHash, CreateInput{}, input, buildCompletion, "confirm_complete")
}

func (s *Service) OpenDisputeWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, "open_dispute"); err != nil {
		return idempotency.Completion{}, err
	}
	return s.createOrUpdateWithIdempotency(ctx, userID, routeKey, key, requestHash, CreateInput{}, input, buildCompletion, "open_dispute")
}

func (s *Service) ConfirmPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, "confirm_payment"); err != nil {
		return idempotency.Completion{}, err
	}
	return s.createOrUpdateWithIdempotency(ctx, userID, routeKey, key, requestHash, CreateInput{}, input, buildCompletion, "confirm_payment")
}

func (s *Service) SubmitDeliveryWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ActorUserID = userID
	if err := validateActionInput(input, "submit_delivery"); err != nil {
		return idempotency.Completion{}, err
	}
	return s.createOrUpdateWithIdempotency(ctx, userID, routeKey, key, requestHash, CreateInput{}, input, buildCompletion, "submit_delivery")
}

func (s *Service) createOrUpdateWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, createInput CreateInput, actionInput ActionInput, buildCompletion CompletionBuilder, action string) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}

	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		if entry.ResourceType == resourceType && entry.ResourceID != "" {
			order, replayErr := s.orderForReplay(ctx, userID, entry.ResourceID, action)
			if replayErr != nil {
				return idempotency.Completion{}, replayErr
			}
			return buildCompletion(order)
		}
		return idempotency.CompletionFromEntry(entry), nil
	}

	if s.repo != nil {
		var order Order
		var completion idempotency.Completion
		switch action {
		case "create":
			order, completion, appErr = s.repo.CreateAPIOrderWithIdempotency(ctx, *entry, createInput, s.now(), buildCompletion)
		case "submit_payment":
			order, completion, appErr = s.repo.SubmitAPIOrderPaymentWithIdempotency(ctx, *entry, actionInput, s.now(), buildCompletion)
		case "cancel":
			order, completion, appErr = s.repo.CancelAPIOrderWithIdempotency(ctx, *entry, actionInput, s.now(), buildCompletion)
		case "confirm_complete":
			order, completion, appErr = s.repo.ConfirmAPIOrderCompleteWithIdempotency(ctx, *entry, actionInput, s.now(), buildCompletion)
		case "open_dispute":
			order, completion, appErr = s.repo.OpenAPIOrderDisputeWithIdempotency(ctx, *entry, actionInput, s.now(), buildCompletion)
		case "confirm_payment":
			order, completion, appErr = s.repo.ConfirmAPIOrderPaymentWithIdempotency(ctx, *entry, actionInput, s.now(), buildCompletion)
		case "submit_delivery":
			order, completion, appErr = s.repo.SubmitAPIOrderDeliveryWithIdempotency(ctx, *entry, actionInput, s.now(), buildCompletion)
		default:
			appErr = domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "未知订单动作。")
		}
		_ = order
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	var order Order
	if action == "create" {
		order, appErr = s.createInMemory(ctx, createInput)
	} else {
		order, appErr = s.updateInMemory(ctx, actionInput, action)
	}
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(order)
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

func (s *Service) orderForReplay(ctx context.Context, userID, orderID, action string) (Order, *domain.AppError) {
	switch action {
	case "create", "submit_payment", "cancel", "confirm_complete", "open_dispute":
		return s.BuyerOrder(ctx, auth.User{ID: userID}, orderID)
	case "confirm_payment", "submit_delivery":
		return s.SellerOrder(ctx, auth.User{ID: userID}, orderID)
	default:
		return Order{}, notFound()
	}
}

func (s *Service) createInMemory(ctx context.Context, input CreateInput) (Order, *domain.AppError) {
	if s.intents == nil || s.services == nil {
		return Order{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "订单依赖不可用。")
	}
	intent, appErr := s.intents.BuyerIntent(ctx, auth.User{ID: input.BuyerUserID}, input.IntentID, input.RequestID)
	if appErr != nil {
		return Order{}, appErr
	}
	service, appErr := s.services.PublicService(ctx, intent.APIServiceID)
	if appErr != nil {
		return Order{}, appErr
	}
	order, appErr := NewOrder(input, intent, service, s.now())
	if appErr != nil {
		return Order{}, appErr
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.orders {
		if existing.APIPurchaseIntentID == intent.ID {
			return Order{}, domain.NewAPIPurchaseIntentHasOrderError()
		}
	}
	s.orders[order.ID] = order
	s.appendEventLocked(order, input.BuyerUserID, EventCreated, "", order.Status, "", input.RequestID)
	return order, nil
}

func (s *Service) updateInMemory(ctx context.Context, input ActionInput, action string) (Order, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[strings.TrimSpace(input.OrderID)]
	if !ok || !canActorAccess(order, input.ActorUserID, action) {
		return Order{}, notFound()
	}
	order = s.materializeTimeoutLocked(order.ID)
	if input.ExpectedVersion > 0 && order.Version != input.ExpectedVersion {
		return Order{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canTransition(order, action, s.now()) {
		return Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前订单状态不能执行该操作。")
	}
	from := order.Status
	if action == "open_dispute" {
		caseID, appErr := s.registerDisputeCaseLocked(ctx, order, input)
		if appErr != nil {
			return Order{}, appErr
		}
		order.DisputeCaseID = caseID
	}
	order = applyAction(order, input, action, s.now())
	s.orders[order.ID] = order
	s.appendEventLocked(order, input.ActorUserID, eventTypeForAction(action), from, order.Status, noteForAction(input, action), input.RequestID)
	return order, nil
}

func (s *Service) registerDisputeCaseLocked(ctx context.Context, order Order, input ActionInput) (string, *domain.AppError) {
	if s.disputes == nil {
		return "", domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "订单纠纷登记依赖不可用。")
	}
	return s.disputes.RegisterAPIOrderDispute(ctx, DisputeCaseInput{
		OrderID:      order.ID,
		ServiceTitle: order.ServiceTitleSnapshot,
		BuyerUserID:  order.BuyerUserID,
		SellerUserID: order.SellerUserID,
		ActorUserID:  input.ActorUserID,
		Reason:       input.Reason,
		RequestID:    input.RequestID,
		Now:          s.now(),
	})
}

func (s *Service) materializeTimeoutLocked(orderID string) Order {
	order := s.orders[orderID]
	if order.Status != StatusPendingPayment || s.now().Before(order.PaymentExpiresAt) {
		return order
	}
	now := s.now()
	from := order.Status
	order.Status = StatusCancelled
	order.CancelReason = CancelReasonPaymentTimeout
	order.CancelledAt = &now
	order.UpdatedAt = now
	order.Version++
	s.orders[orderID] = order
	s.appendEventLocked(order, "", EventPaymentTimeoutCancelled, from, order.Status, "", "payment-timeout")
	return order
}

func (s *Service) appendEventLocked(order Order, actorUserID, eventType, fromStatus, toStatus, note, requestID string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	for _, event := range s.events {
		if event.APIOrderID == order.ID && event.EventType == eventType && event.RequestID == requestID {
			return
		}
	}
	s.events = append(s.events, Event{
		ID:          uuid.NewString(),
		APIOrderID:  order.ID,
		ActorUserID: actorUserID,
		EventType:   eventType,
		FromStatus:  fromStatus,
		ToStatus:    toStatus,
		Note:        strings.TrimSpace(note),
		RequestID:   requestID,
		CreatedAt:   s.now(),
	})
}

func (s *Service) appendAccessLogLocked(orderID, buyerUserID, requestID string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	s.accessLogs = append(s.accessLogs, PaymentInstructionAccessLog{
		ID:          uuid.NewString(),
		APIOrderID:  orderID,
		BuyerUserID: buyerUserID,
		RequestID:   requestID,
		AccessedAt:  s.now(),
	})
}

func NewOrder(input CreateInput, intent apiintent.Intent, service apimarket.Service, now time.Time) (Order, *domain.AppError) {
	if strings.TrimSpace(input.IntentID) == "" {
		return Order{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API purchase intent required", "必须提供购买意向。", "intentId", "required", "必须提供购买意向。")
	}
	if !apimarket.IsOrderableService(service) {
		return Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Service not orderable", "当前 API 服务不可下单。")
	}
	method := strings.TrimSpace(input.PaymentMethod)
	option, ok := findPaymentOption(service, method)
	if !ok {
		return Order{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method invalid", "选择的付款方式不可用。", "paymentMethod", "invalid", "选择的付款方式不可用。")
	}
	amount, currency, quoteVersion, appErr := resolveOrderAmount(intent, service, method)
	if appErr != nil {
		return Order{}, appErr
	}
	return Order{
		ID:                           uuid.NewString(),
		APIPurchaseIntentID:          intent.ID,
		APIServiceID:                 intent.APIServiceID,
		BuyerUserID:                  input.BuyerUserID,
		SellerUserID:                 intent.OwnerUserID,
		Status:                       StatusPendingPayment,
		DisputeStatus:                DisputeStatusNone,
		ServiceTitleSnapshot:         service.Title,
		ServiceVersionSnapshot:       service.Version,
		BillingModeSnapshot:          service.BillingMode,
		SelectedPackageID:            intent.SelectedPackageID,
		SelectedPackageSnapshot:      intent.SelectedPackageSnapshot,
		QuoteVersionSnapshot:         quoteVersion,
		Amount:                       amount,
		Currency:                     currency,
		SelectedPaymentMethod:        method,
		PaymentWindowMinutesSnapshot: service.PaymentWindowMinutes,
		PaymentExpiresAt:             now.Add(time.Duration(service.PaymentWindowMinutes) * time.Minute),
		PaymentInstructionsSnapshot:  option.PaymentInstructions,
		CreatedAt:                    now,
		UpdatedAt:                    now,
		Version:                      1,
	}, nil
}

func findPaymentOption(service apimarket.Service, method string) (apimarket.PaymentOption, bool) {
	for _, option := range service.PaymentOptions {
		if option.Enabled && option.PaymentMethod == method {
			return option, true
		}
	}
	return apimarket.PaymentOption{}, false
}

func resolveOrderAmount(intent apiintent.Intent, service apimarket.Service, method string) (string, string, int64, *domain.AppError) {
	if method == apimarket.PaymentMethodUSDT {
		return "", "", 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "USDT quote required", "USDT 下单必须先由商户给出固定 USDT 报价。", "paymentMethod", "quote_required", "USDT 下单必须先报价。")
	}
	switch service.BillingMode {
	case apimarket.ServiceBillingModeFixedPackage:
		pack, ok := findServicePackage(service, intent.SelectedPackageID)
		if !ok || !pack.Enabled {
			return "", "", 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package invalid", "选择的套餐不可用。", "selectedPackageId", "invalid", "选择的套餐不可用。")
		}
		return decimalStringOptional(pack.PriceCNY, 2), "CNY", 0, nil
	case apimarket.ServiceBillingModeMetered:
		return decimalStringOptional(intent.RequestedCNYAmount, 2), "CNY", 0, nil
	case apimarket.ServiceBillingModeManual:
		return "", "", 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Seller quote required", "自定义需求必须先由商户给出固定报价。", "intentId", "quote_required", "必须先完成商户报价。")
	default:
		return "", "", 0, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务计费方式不可下单。")
	}
}

func findServicePackage(service apimarket.Service, packageID string) (apimarket.ServicePackage, bool) {
	packageID = strings.TrimSpace(packageID)
	for _, pack := range service.Packages {
		if pack.ID == packageID {
			return pack, true
		}
	}
	return apimarket.ServicePackage{}, false
}

func validateActionInput(input ActionInput, action string) *domain.AppError {
	if strings.TrimSpace(input.OrderID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API order required", "必须提供订单。", "orderId", "required", "必须提供订单。")
	}
	switch action {
	case "submit_payment":
		if strings.TrimSpace(input.PaymentSummary) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment summary required", "必须填写付款摘要。", "paymentSummary", "required", "必须填写付款摘要。")
		}
		return validateNonSecretText("paymentSummary", input.PaymentSummary)
	case "submit_delivery":
		if strings.TrimSpace(input.DeliveryNote) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Off-platform delivery prompt required", "必须填写站外交付提示。", "deliveryNote", "required", "必须填写站外交付提示。")
		}
		return validateNonSecretText("deliveryNote", input.DeliveryNote)
	case "cancel":
		if strings.TrimSpace(input.Reason) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "必须填写取消原因。", "reason", "required", "必须填写取消原因。")
		}
		return validateNonSecretText("reason", input.Reason)
	case "open_dispute":
		if strings.TrimSpace(input.Reason) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "必须填写纠纷说明。", "reason", "required", "必须填写纠纷说明。")
		}
		return validateNonSecretText("reason", input.Reason)
	default:
		return nil
	}
}

func canActorAccess(order Order, actorUserID, action string) bool {
	switch action {
	case "submit_payment", "cancel", "confirm_complete":
		return order.BuyerUserID == actorUserID
	case "confirm_payment", "submit_delivery":
		return order.SellerUserID == actorUserID
	case "open_dispute":
		return order.BuyerUserID == actorUserID || order.SellerUserID == actorUserID
	default:
		return false
	}
}

func canTransition(order Order, action string, now time.Time) bool {
	switch action {
	case "submit_payment":
		return order.Status == StatusPendingPayment && now.Before(order.PaymentExpiresAt)
	case "cancel":
		return order.Status == StatusPendingPayment
	case "confirm_payment":
		return order.Status == StatusPaymentSubmitted
	case "submit_delivery":
		return order.Status == StatusPaidConfirmed
	case "confirm_complete":
		return order.Status == StatusDeliverySubmitted
	case "open_dispute":
		return order.Status != StatusCancelled && order.Status != StatusCompleted && order.DisputeStatus == DisputeStatusNone
	default:
		return false
	}
}

func applyAction(order Order, input ActionInput, action string, now time.Time) Order {
	switch action {
	case "submit_payment":
		order.Status = StatusPaymentSubmitted
		order.PaymentSummary = strings.TrimSpace(input.PaymentSummary)
		order.PaymentSubmittedAt = &now
	case "cancel":
		order.Status = StatusCancelled
		order.CancelReason = CancelReasonBuyer
		order.CancelledAt = &now
	case "confirm_payment":
		order.Status = StatusPaidConfirmed
		order.PaidConfirmedAt = &now
	case "submit_delivery":
		order.Status = StatusDeliverySubmitted
		order.DeliveryNote = strings.TrimSpace(input.DeliveryNote)
		order.DeliverySubmittedAt = &now
	case "confirm_complete":
		order.Status = StatusCompleted
		order.CompletedAt = &now
	case "open_dispute":
		order.DisputeStatus = DisputeStatusOpen
	}
	order.UpdatedAt = now
	order.Version++
	return order
}

func eventTypeForAction(action string) string {
	switch action {
	case "submit_payment":
		return EventPaymentSubmitted
	case "cancel":
		return EventCancelled
	case "confirm_payment":
		return EventPaymentConfirmed
	case "submit_delivery":
		return EventDeliverySubmitted
	case "confirm_complete":
		return EventCompleted
	case "open_dispute":
		return EventDisputeOpened
	default:
		return "api_order.updated"
	}
}

func noteForAction(input ActionInput, action string) string {
	switch action {
	case "submit_payment":
		return input.PaymentSummary
	case "submit_delivery":
		return input.DeliveryNote
	case "cancel", "open_dispute":
		return input.Reason
	default:
		return ""
	}
}

func notFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API order not found", "订单不存在。")
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

func decimalStringOptional(value string, places int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok || rat.Sign() <= 0 {
		return value
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	scaled := new(big.Rat).Mul(rat, new(big.Rat).SetInt(scale))
	num := scaled.Num()
	den := scaled.Denom()
	quotient, remainder := new(big.Int).QuoRem(num, den, new(big.Int))
	doubleRemainder := new(big.Int).Mul(remainder, big.NewInt(2))
	if doubleRemainder.Cmp(den) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	intPart := new(big.Int).Quo(quotient, scale)
	fracPart := new(big.Int).Mod(quotient, scale)
	return fmt.Sprintf("%s.%0*s", intPart.String(), places, fracPart.String())
}

func OrderResponseBody(order Order, mapper func(Order) any) ([]byte, *domain.AppError) {
	body, err := json.Marshal(mapper(order))
	if err != nil {
		return nil, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	return body, nil
}
