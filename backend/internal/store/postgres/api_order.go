package postgres

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apiquota"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxAPIOrderNumberInsertAttempts = 8

var errAPIOrderNumberCollision = errors.New("API order number collision")

func (s *Store) CreateAPIOrderWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.CreateInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, input, apiorder.ActionInput{}, now, buildCompletion, "create")
}

func (s *Store) SubmitAPIOrderPaymentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "submit_payment")
}

func (s *Store) CancelAPIOrderWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "cancel")
}

func (s *Store) ConfirmAPIOrderCompleteWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "confirm_complete")
}

func (s *Store) OpenAPIOrderDisputeWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "open_dispute")
}

func (s *Store) ConfirmAPIOrderPaymentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "confirm_payment")
}

func (s *Store) ReportAPIOrderPaymentIssueWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "report_payment_issue")
}

func (s *Store) SubmitAPIOrderDeliveryWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "submit_delivery")
}

func (s *Store) ReportLateAPIOrderPaymentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "report_late_payment")
}

func (s *Store) ResolveLateAPIOrderPaymentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return s.apiOrderWithIdempotency(ctx, entry, apiorder.CreateInput{}, input, now, buildCompletion, "resolve_late_payment")
}

func (s *Store) ResolveAPIOrderCatalogRiskHoldWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiorder.CatalogRiskHoldActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || buildCompletion == nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	order, err := s.getAPIOrder(ctx, tx, input.OrderID, true, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, idempotency.Completion{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	var hold apiorder.CatalogRiskHold
	err = tx.QueryRow(ctx, `
		SELECT id::text, source_type, source_id::text, status, reason, created_at,
		       COALESCE(resolved_by::text, ''), resolved_at, COALESCE(resolution_note, ''), version
		FROM api_order_catalog_risk_holds
		WHERE api_order_id = $1 AND status = 'active'
		FOR UPDATE
	`, order.ID).Scan(&hold.ID, &hold.SourceType, &hold.SourceID, &hold.Status, &hold.Reason, &hold.CreatedAt,
		&hold.ResolvedBy, &hold.ResolvedAt, &hold.ResolutionNote, &hold.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Catalog risk hold unavailable", "该订单没有待处置的目录风险暂停。")
	}
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	if hold.Version != input.ExpectedVersion {
		return apiorder.Order{}, idempotency.Completion{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "风险暂停版本已变化，请刷新后重试。")
	}
	resolvedAt := now.UTC()
	command, err := tx.Exec(ctx, `
		UPDATE api_order_catalog_risk_holds
		SET status = $2, resolved_by = $3, resolved_at = $4, resolution_note = $5, version = version + 1
		WHERE id = $1 AND status = 'active' AND version = $6
	`, hold.ID, input.Resolution, input.AdminUserID, resolvedAt, input.ResolutionNote, input.ExpectedVersion)
	if err != nil || command.RowsAffected() != 1 {
		if err != nil {
			return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
		}
		return apiorder.Order{}, idempotency.Completion{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "风险暂停版本已变化，请刷新后重试。")
	}
	hold.Status = input.Resolution
	hold.ResolvedBy = input.AdminUserID
	hold.ResolvedAt = &resolvedAt
	hold.ResolutionNote = input.ResolutionNote
	hold.Version++
	order.CatalogRiskHold = &hold
	eventType := map[string]string{
		apiorder.CatalogRiskHoldRestored:      apiorder.EventCatalogRiskHoldRestored,
		apiorder.CatalogRiskHoldRefundPending: apiorder.EventCatalogRefundPending,
		apiorder.CatalogRiskHoldDisputeOpened: apiorder.EventCatalogDisputeOpened,
	}[input.Resolution]
	if appErr := insertAPIOrderEventInTx(ctx, tx, order, input.AdminUserID, eventType, order.Status, order.Status, input.ResolutionNote, input.RequestID, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if appErr := insertAPIOrderCatalogRiskNotificationInTx(ctx, tx, order, hold.ID, eventType, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	beforeJSON := `{"status":"active","version":` + strconv.FormatInt(input.ExpectedVersion, 10) + `}`
	afterJSON := `{"status":"` + input.Resolution + `","version":` + strconv.FormatInt(hold.Version, 10) + `}`
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, reason, before_json, after_json, request_id, created_at)
		VALUES ($1, $2, 'api_order', $3, $4, $5::jsonb, $6::jsonb, $7, $8)
	`, input.AdminUserID, eventType, order.ID, input.ResolutionNote, beforeJSON, afterJSON, input.RequestID, now); err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	completion, appErr := buildCompletion(apiorder.WithAfterSalesProjection(order, now))
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	return order, completion, nil
}

func (s *Store) apiOrderWithIdempotency(ctx context.Context, entry idempotency.Entry, createInput apiorder.CreateInput, actionInput apiorder.ActionInput, now time.Time, buildCompletion apiorder.CompletionBuilder, action string) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	if action != "create" {
		// 超时状态、事件和通知必须先独立提交；后续动作返回状态冲突时不能将其回滚。
		if appErr := s.materializeExpiredAPIOrder(ctx, s.pool, actionInput.OrderID, now); appErr != nil {
			return apiorder.Order{}, idempotency.Completion{}, appErr
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	var order apiorder.Order
	if action == "create" {
		order, appErr = s.createAPIOrderInTx(ctx, tx, createInput, now)
	} else {
		order, appErr = s.updateAPIOrderInTx(ctx, tx, actionInput, now, action)
	}
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(order)
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	return order, completion, nil
}

func (s *Store) ListAPIOrdersByBuyer(ctx context.Context, buyerUserID string, now time.Time) ([]apiorder.Order, *domain.AppError) {
	if appErr := s.MaterializeExpiredAPIOrders(ctx, now); appErr != nil {
		return nil, appErr
	}
	return s.listAPIOrders(ctx, `WHERE buyer_user_id = $1`, []any{buyerUserID}, now)
}

func (s *Store) GetAPIOrderForBuyer(ctx context.Context, buyerUserID, orderID string, now time.Time) (apiorder.Order, *domain.AppError) {
	if appErr := s.materializeExpiredAPIOrder(ctx, s.pool, orderID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	order, err := s.getAPIOrderWithCredentialLifecycleLock(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	if order.BuyerUserID != buyerUserID {
		return apiorder.Order{}, apiOrderNotFound()
	}
	return apiorder.WithAfterSalesProjection(order, now), nil
}

func (s *Store) ListAPIOrdersForActor(ctx context.Context, actor auth.BusinessActor, participantRole string, now time.Time) ([]apiorder.Order, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		if participantRole == "seller" {
			return s.ListAPIOrdersBySeller(ctx, actor.UserID, now)
		}
		return s.ListAPIOrdersByBuyer(ctx, actor.UserID, now)
	}
	where, args, ok := restrictedAPIOrderWhere(actor, participantRole, "")
	if !ok {
		return nil, apiOrderNotFound()
	}
	return s.listAPIOrders(ctx, where, args, now)
}

func (s *Store) GetAPIOrderForActor(ctx context.Context, actor auth.BusinessActor, orderID, participantRole string, now time.Time) (apiorder.Order, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		if participantRole == "seller" {
			return s.GetAPIOrderForSeller(ctx, actor.UserID, orderID, now)
		}
		return s.GetAPIOrderForBuyer(ctx, actor.UserID, orderID, now)
	}
	return s.getRestrictedAPIOrderForActor(ctx, actor, orderID, participantRole, now)
}

func (s *Store) getRestrictedAPIOrderForActor(ctx context.Context, actor auth.BusinessActor, orderID, participantRole string, now time.Time) (apiorder.Order, *domain.AppError) {
	if _, _, ok := restrictedAPIOrderWhere(actor, participantRole, orderID); !ok {
		return apiorder.Order{}, apiOrderNotFound()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := lockAccountGovernanceUser(ctx, tx, actor.UserID); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if err := lockAPIOrderCredentialLifecycleInTx(ctx, tx, orderID); err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	order, err := s.getAPIOrder(ctx, tx, orderID, false, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	if appErr := authorizeRestrictedAPIOrderInTx(ctx, tx, actor, order, participantRole); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	return apiorder.WithAfterSalesProjection(order, now), nil
}

func restrictedAPIOrderWhere(actor auth.BusinessActor, participantRole, orderID string) (string, []any, bool) {
	if actor.Audience != auth.SessionAudienceRestrictedBusiness || actor.UserID == "" || actor.GovernanceActionID == "" || actor.GovernanceVersion < 1 || actor.RestrictionEffectiveAt.IsZero() {
		return "", nil, false
	}
	participantColumn := "buyer_user_id"
	if participantRole == "seller" {
		participantColumn = "seller_user_id"
	} else if participantRole != "buyer" {
		return "", nil, false
	}
	where := `WHERE ` + participantColumn + ` = $1
		AND created_at <= $4
		AND EXISTS (
			SELECT 1
			FROM account_governance_resource_dispositions disposition
			JOIN account_governance_disposition_actions link ON link.disposition_id = disposition.id
			JOIN account_governance_actions action ON action.id = link.governance_action_id
			JOIN users user_account ON user_account.id = action.target_user_id
			WHERE disposition.resource_type = 'api_order'
			  AND disposition.resource_id = api_orders.id
			  AND disposition.result = 'preserved'
			  AND link.governance_action_id = $2
			  AND action.target_user_id = $1
			  AND action.governance_version = $3
			  AND action.effective_at = $4
			  AND action.status = 'effective'
			  AND user_account.account_status IN ('suspended', 'banned')
			  AND user_account.security_locked_at IS NULL
			  AND user_account.current_governance_action_id = action.id
			  AND user_account.governance_version = action.governance_version
		)`
	args := []any{actor.UserID, actor.GovernanceActionID, actor.GovernanceVersion, actor.RestrictionEffectiveAt}
	if strings.TrimSpace(orderID) != "" {
		where += ` AND id = $5`
		args = append(args, orderID)
	}
	return where, args, true
}

func (s *Store) ReadAPIOrderPaymentInstructions(ctx context.Context, buyerUserID, orderID, requestID string, now time.Time) (apiorder.PaymentInstructionsView, *domain.AppError) {
	// 付款入口返回状态冲突时，已发生的超时转换仍需保留在独立事务中。
	if appErr := s.materializeExpiredAPIOrder(ctx, s.pool, orderID, now); appErr != nil {
		return apiorder.PaymentInstructionsView{}, appErr
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiorder.PaymentInstructionsView{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	order, err := s.getAPIOrder(ctx, tx, orderID, true, false)
	if errors.Is(err, pgx.ErrNoRows) || order.BuyerUserID != buyerUserID {
		return apiorder.PaymentInstructionsView{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.PaymentInstructionsView{}, internalStoreError()
	}
	if apiorder.IsDisputeActive(order.DisputeStatus) || order.Status != apiorder.StatusPendingPayment || !now.Before(order.PaymentExpiresAt) {
		return apiorder.PaymentInstructionsView{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前订单不再是有效付款入口。")
	}
	if appErr := rejectActiveCatalogRiskHoldInTx(ctx, tx, order.ID, "payment_instructions"); appErr != nil {
		return apiorder.PaymentInstructionsView{}, appErr
	}
	if appErr := insertAPIOrderPaymentInstructionAccessLogInTx(ctx, tx, order.ID, buyerUserID, requestID, now); appErr != nil {
		return apiorder.PaymentInstructionsView{}, appErr
	}
	if appErr := insertAPIOrderEventInTx(ctx, tx, order, buyerUserID, apiorder.EventPaymentInstructionsRead, order.Status, order.Status, "", requestID, now); appErr != nil {
		return apiorder.PaymentInstructionsView{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiorder.PaymentInstructionsView{}, internalStoreError()
	}
	return apiorder.PaymentInstructionsView{
		OrderID:              order.ID,
		PaymentMethod:        order.SelectedPaymentMethod,
		PaymentInstructions:  order.PaymentInstructionsSnapshot,
		PaymentQRCodeDataURL: order.PaymentQRCodeDataURLSnapshot,
		PaymentExpiresAt:     order.PaymentExpiresAt,
	}, nil
}

func (s *Store) ListAPIOrdersBySeller(ctx context.Context, sellerUserID string, now time.Time) ([]apiorder.Order, *domain.AppError) {
	if appErr := s.MaterializeExpiredAPIOrders(ctx, now); appErr != nil {
		return nil, appErr
	}
	return s.listAPIOrders(ctx, `WHERE seller_user_id = $1`, []any{sellerUserID}, now)
}

func (s *Store) HasActiveAPIOrderDisputeForSeller(ctx context.Context, sellerUserID string) (bool, *domain.AppError) {
	if s == nil || s.pool == nil {
		return false, internalStoreError()
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM api_orders
			WHERE seller_user_id = $1
			  AND dispute_status IN ('negotiating', 'open', 'awaiting_fulfillment', 'fulfillment_confirmation')
		)
	`, sellerUserID).Scan(&exists); err != nil {
		return false, internalStoreError()
	}
	return exists, nil
}

func (s *Store) ListAdminAPIOrders(ctx context.Context, filter apiorder.AdminOrderFilter, page domain.PageRequest, now time.Time) (domain.Page[apiorder.Order], *domain.AppError) {
	if appErr := s.MaterializeExpiredAPIOrders(ctx, now); appErr != nil {
		return domain.Page[apiorder.Order]{}, appErr
	}
	return s.listAdminAPIOrdersPage(ctx, filter, page, now)
}

func (s *Store) GetAdminAPIOrder(ctx context.Context, orderID string, now time.Time) (apiorder.Order, *domain.AppError) {
	if appErr := s.materializeExpiredAPIOrder(ctx, s.pool, orderID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	order, err := s.getAPIOrder(ctx, s.pool, orderID, false, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	order.DeliveryCredential = nil
	return apiorder.WithAfterSalesProjection(order, now), nil
}

func (s *Store) GetAPIOrderForSeller(ctx context.Context, sellerUserID, orderID string, now time.Time) (apiorder.Order, *domain.AppError) {
	if appErr := s.materializeExpiredAPIOrder(ctx, s.pool, orderID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	order, err := s.getAPIOrderWithCredentialLifecycleLock(ctx, orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	if order.SellerUserID != sellerUserID {
		return apiorder.Order{}, apiOrderNotFound()
	}
	return apiorder.WithAfterSalesProjection(order, now), nil
}

func (s *Store) getAPIOrderWithCredentialLifecycleLock(ctx context.Context, orderID string) (apiorder.Order, error) {
	if s == nil || s.pool == nil {
		return apiorder.Order{}, errors.New("postgres store is not configured")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiorder.Order{}, err
	}
	defer rollback(ctx, tx)
	if err := lockAPIOrderCredentialLifecycleInTx(ctx, tx, orderID); err != nil {
		return apiorder.Order{}, err
	}
	order, err := s.getAPIOrder(ctx, tx, orderID, false, true)
	if err != nil {
		return apiorder.Order{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return apiorder.Order{}, err
	}
	return order, nil
}

func lockAPIOrderCredentialLifecycleInTx(ctx context.Context, tx pgx.Tx, orderID string) error {
	_, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(
			hashtextextended($1::text || $2::uuid::text, 0)
		)
	`, apiOrderCredentialLifecycleLockPrefix, orderID)
	return err
}

func (s *Store) createAPIOrderInTx(ctx context.Context, tx pgx.Tx, input apiorder.CreateInput, now time.Time) (apiorder.Order, *domain.AppError) {
	intent, err := s.getAPIPurchaseIntent(ctx, tx, input.IntentID, true)
	if errors.Is(err, pgx.ErrNoRows) || intent.BuyerUserID != input.BuyerUserID {
		return apiorder.Order{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	if appErr := ensureActiveBusinessUsersInTx(ctx, tx, intent.BuyerUserID, intent.OwnerUserID); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := ensureNoAPIOrderForIntent(ctx, tx, intent.ID); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if intent.Status != apiintent.StatusOpen && intent.Status != apiintent.StatusContacted {
		return apiorder.Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前购买意向状态不能生成订单。")
	}
	service, err := s.getAPIService(ctx, tx, intent.APIServiceID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	if appErr := ensureAPIServiceCatalogActiveInTx(ctx, tx, service.ID); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := loadReadyProbeTargetInTx(ctx, tx, &service); appErr != nil {
		return apiorder.Order{}, appErr
	}
	service = apimarket.WithOrderabilityAt(service, now)
	if appErr := ensureAPIServicePublishAllowedInTx(ctx, tx, service.OwnerUserID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	order, appErr := newStoreAPIOrder(input, intent, service, now)
	if appErr != nil {
		return apiorder.Order{}, appErr
	}
	productKind := "service"
	productID := order.APIServiceID
	if order.SelectedPackageID != "" {
		productKind = "package"
		productID = order.SelectedPackageID
	}
	if appErr := ensureAPIBuyerPendingCapacityInTx(ctx, tx, order.BuyerUserID, productKind, productID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := reserveAPIOrderInventoryInTx(ctx, tx, order, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := insertAPIOrderInTx(ctx, tx, &order); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := markAPIPurchaseIntentOrderedInTx(ctx, tx, intent.ID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := insertAPIOrderEventInTx(ctx, tx, order, input.BuyerUserID, apiorder.EventCreated, "", order.Status, "", input.RequestID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, input.BuyerUserID, apiorder.EventCreated, input.RequestID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	return apiorder.WithAfterSalesProjection(order, now), nil
}

func ensureAPIBuyerPendingCapacityInTx(ctx context.Context, tx pgx.Tx, buyerUserID, productKind, productID string, now time.Time) *domain.AppError {
	if _, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended('api-order-pending-capacity:' || $1::uuid::text, 0))
	`, buyerUserID); err != nil {
		return internalStoreError()
	}

	var productPending, totalPending int
	err := tx.QueryRow(ctx, `
		SELECT
			count(*) FILTER (
				WHERE ($2 = 'quota' AND api_quota_offer_id = $3::uuid)
				   OR ($2 = 'package' AND selected_package_id = $3::uuid)
				   OR ($2 = 'service' AND purchase_kind = 'api_service' AND api_service_id = $3::uuid AND selected_package_id IS NULL)
			)::integer,
			count(*)::integer
		FROM api_orders
		WHERE buyer_user_id = $1
		  AND status = 'pending_payment'
		  AND payment_expires_at > $4
	`, buyerUserID, productKind, productID, now).Scan(&productPending, &totalPending)
	if err != nil {
		return internalStoreError()
	}
	if productPending >= 1 {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Pending order already exists", "你在该商品下已有待付款订单，请先完成或等待其关闭。")
	}
	if totalPending >= 3 {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Pending order limit reached", "你当前已有 3 个待付款 API 订单，请先处理后再下单。")
	}
	return nil
}

func loadReadyProbeTargetInTx(ctx context.Context, tx pgx.Tx, service *apimarket.Service) *domain.AppError {
	if service == nil || strings.TrimSpace(service.ProbeConnectionID) == "" {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Probe connection required", "当前 API 服务未绑定可用探针连接。")
	}
	err := tx.QueryRow(ctx, `
		SELECT base_url, normalized_base_url
		FROM api_probe_connections
		WHERE id = $1
		  AND owner_user_id = $2
		  AND enabled = true
		  AND verification_status = 'verified'
		FOR SHARE
	`, service.ProbeConnectionID, service.OwnerUserID).Scan(&service.ProbeBaseURL, &service.NormalizedProbeBaseURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Probe connection unavailable", "当前 API 服务绑定的探针连接不可用。")
	}
	if err != nil {
		return internalStoreError()
	}
	service.ProbeReady = true
	return nil
}

func markAPIPurchaseIntentOrderedInTx(ctx context.Context, tx pgx.Tx, intentID string, now time.Time) *domain.AppError {
	commandTag, err := tx.Exec(ctx, `
		UPDATE api_purchase_intents
		SET status = $2,
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
		  AND status IN ($4, $5)
	`, intentID, apiintent.StatusOrdered, now, apiintent.StatusOpen, apiintent.StatusContacted)
	if err != nil {
		return internalStoreError()
	}
	if commandTag.RowsAffected() != 1 {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前购买意向状态不能生成订单。")
	}
	return nil
}

func ensureNoAPIOrderForIntent(ctx context.Context, q queryer, intentID string) *domain.AppError {
	var exists bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM api_orders
			WHERE api_purchase_intent_id = $1
		)
	`, intentID).Scan(&exists); err != nil {
		return internalStoreError()
	}
	if exists {
		return domain.NewAPIPurchaseIntentHasOrderError()
	}
	return nil
}

func (s *Store) updateAPIOrderInTx(ctx context.Context, tx pgx.Tx, input apiorder.ActionInput, now time.Time, action string) (apiorder.Order, *domain.AppError) {
	if input.ActorAudience == auth.SessionAudienceRestrictedBusiness {
		if appErr := lockAccountGovernanceUser(ctx, tx, input.ActorUserID); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	order, err := s.getAPIOrder(ctx, tx, input.OrderID, true, false)
	if errors.Is(err, pgx.ErrNoRows) || !storeCanActorAccessAPIOrder(order, input.ActorUserID, action) {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	if input.ActorAudience == auth.SessionAudienceRestrictedBusiness {
		if !restrictedAPIOrderActionAllowed(action, input.ParticipantRole) {
			return apiorder.Order{}, apiOrderNotFound()
		}
		actor := auth.BusinessActor{
			UserID:                 input.ActorUserID,
			Audience:               input.ActorAudience,
			GovernanceActionID:     input.GovernanceActionID,
			GovernanceVersion:      input.GovernanceVersion,
			RestrictionEffectiveAt: input.RestrictionEffectiveAt,
		}
		if appErr := authorizeRestrictedAPIOrderInTx(ctx, tx, actor, order, input.ParticipantRole); appErr != nil {
			return apiorder.Order{}, appErr
		}
	} else if input.ActorAudience != "" && input.ActorAudience != auth.SessionAudienceNormal {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if action == "submit_payment" {
		if appErr := ensureActiveBusinessUsersInTx(ctx, tx, order.BuyerUserID, order.SellerUserID); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	if input.ExpectedVersion > 0 && order.Version != input.ExpectedVersion {
		return apiorder.Order{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if action != "open_dispute" {
		if appErr := rejectActiveCatalogRiskHoldInTx(ctx, tx, order.ID, action); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	if action == "open_dispute" {
		if _, appErr := apiorder.ValidateDisputeOccurrence(order, input.IssueOccurredAt, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	if !storeCanTransitionAPIOrder(order, action, now) {
		return apiorder.Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前订单状态不能执行该操作。")
	}
	if action == "open_dispute" {
		if appErr := apiorder.ValidateRequestedDisputeAmount(input.RequestedResolution, input.RequestedAmountCNY, order.Amount); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	if action == "submit_delivery" {
		if expiresAt := apiorder.FulfillmentExpiresAt(order); expiresAt != nil && !now.Before(*expiresAt) {
			return apiorder.Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Fulfillment expired", "所购额度已到期，不能再提交正常交付，请退款或进入纠纷处理。")
		}
		expiresAt, appErr := apiorder.PackageExpiryFromSnapshot(order.SelectedPackageSnapshot, now)
		if appErr != nil {
			return apiorder.Order{}, appErr
		}
		order.PackageExpiresAt = expiresAt
		credentialInput, appErr := apiorder.NormalizeDeliveryCredentialForStore(input.DeliveryCredential)
		if appErr != nil {
			return apiorder.Order{}, appErr
		}
		input.DeliveryCredential = credentialInput
		input.DeliveryNote = apiorder.DeliverySummary(credentialInput.DeliveryKind)
	}
	if appErr := storeValidateAPIOrderActionInput(input, action); appErr != nil {
		return apiorder.Order{}, appErr
	}
	from := order.Status
	if action == "cancel" {
		if appErr := releaseAPIOrderReservationInTx(ctx, tx, order, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
		order.PackageStockReserved = false
	}
	if action == "submit_delivery" {
		credential, appErr := s.insertAPIOrderDeliveryCredentialInTx(ctx, tx, order, input.DeliveryCredential, now)
		if appErr != nil {
			return apiorder.Order{}, appErr
		}
		order.DeliveryCredential = &credential
	}
	if action == "open_dispute" {
		dispute, appErr := openDisputeFromAPIOrderInTx(ctx, tx, order, input, now)
		if appErr != nil {
			return apiorder.Order{}, appErr
		}
		order.DisputeCaseID = dispute.ID
		if appErr := insertDisputeEvent(ctx, tx, "dispute", dispute.ID, "opened", input.ActorUserID, "user", input.Reason, true, input.RequestID, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	if action == "confirm_payment" && order.PurchaseKind == apiorder.PurchaseKindLimitedQuotaOffer {
		if appErr := consumeAPIQuotaInventoryInTx(ctx, tx, order, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	order = storeApplyAPIOrderAction(order, input, action, now)
	autoDeliveryKind := ""
	if action == "confirm_payment" && order.PurchaseKind == apiorder.PurchaseKindLimitedQuotaOffer && order.QuotaDeliveryMode == apiquota.DeliveryModePreimported {
		var deliveryErr *domain.AppError
		autoDeliveryKind, deliveryErr = s.deliverPreimportedAPIQuotaCredentialInTx(ctx, tx, order, now)
		if deliveryErr != nil {
			return apiorder.Order{}, deliveryErr
		}
		order.Status = apiorder.StatusDeliverySubmitted
		order.DeliveryNote = apiorder.DeliverySummary(autoDeliveryKind)
		order.DeliverySubmittedAt = &now
		reviewExpiresAt := now.Add(apiorder.DeliveryReviewWindow)
		order.DeliveryReviewExpiresAt = &reviewExpiresAt
		order.Version++
	}
	if appErr := updateAPIOrderInTx(ctx, tx, order); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if autoDeliveryKind != "" {
		confirmedOrder := order
		confirmedOrder.Version--
		confirmedOrder.Status = apiorder.StatusPaidConfirmed
		confirmedOrder.DeliveryNote = ""
		confirmedOrder.DeliverySubmittedAt = nil
		confirmedOrder.DeliveryReviewExpiresAt = nil
		if appErr := insertAPIOrderEventInTx(ctx, tx, confirmedOrder, input.ActorUserID, apiorder.EventPaymentConfirmed, from, apiorder.StatusPaidConfirmed, "", input.RequestID, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
		if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, confirmedOrder, input.ActorUserID, apiorder.EventPaymentConfirmed, input.RequestID, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
		if appErr := insertAPIOrderEventInTx(ctx, tx, order, input.ActorUserID, apiorder.EventDeliverySubmitted, apiorder.StatusPaidConfirmed, apiorder.StatusDeliverySubmitted, apiorder.DeliverySummary(autoDeliveryKind), input.RequestID, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
		if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, input.ActorUserID, apiorder.EventDeliverySubmitted, input.RequestID, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
	} else {
		eventType := storeAPIOrderEventType(action)
		if appErr := insertAPIOrderEventInTx(ctx, tx, order, input.ActorUserID, eventType, from, order.Status, storeAPIOrderActionNote(input, action), input.RequestID, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
		if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, input.ActorUserID, eventType, input.RequestID, now); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	if action != "submit_delivery" && order.DeliverySubmittedAt != nil {
		if appErr := s.attachAPIOrderDeliveryCredential(ctx, tx, &order); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	return apiorder.WithAfterSalesProjection(order, now), nil
}

func restrictedAPIOrderActionAllowed(action, participantRole string) bool {
	switch participantRole {
	case "buyer":
		return action == "confirm_complete" || action == "open_dispute"
	case "seller":
		return action == "confirm_payment" || action == "report_payment_issue" || action == "submit_delivery" || action == "open_dispute"
	default:
		return false
	}
}

func authorizeRestrictedAPIOrderInTx(ctx context.Context, tx pgx.Tx, actor auth.BusinessActor, order apiorder.Order, participantRole string) *domain.AppError {
	if actor.UserID == "" || actor.GovernanceActionID == "" || actor.GovernanceVersion < 1 || actor.RestrictionEffectiveAt.IsZero() || order.CreatedAt.After(actor.RestrictionEffectiveAt) {
		return apiOrderNotFound()
	}
	if participantRole == "buyer" && order.BuyerUserID != actor.UserID {
		return apiOrderNotFound()
	}
	if participantRole == "seller" && order.SellerUserID != actor.UserID {
		return apiOrderNotFound()
	}
	if participantRole != "buyer" && participantRole != "seller" {
		return apiOrderNotFound()
	}
	var authorized bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users user_account
			JOIN account_governance_actions action ON action.id = user_account.current_governance_action_id
			JOIN account_governance_disposition_actions link ON link.governance_action_id = action.id
			JOIN account_governance_resource_dispositions disposition ON disposition.id = link.disposition_id
			WHERE user_account.id = $1
			  AND user_account.account_status IN ('suspended', 'banned')
			  AND user_account.security_locked_at IS NULL
			  AND user_account.current_governance_action_id = $2
			  AND user_account.governance_version = $3
			  AND action.status = 'effective'
			  AND action.effective_at = $4
			  AND disposition.resource_type = 'api_order'
			  AND disposition.resource_id = $5
			  AND disposition.result = 'preserved'
		)
	`, actor.UserID, actor.GovernanceActionID, actor.GovernanceVersion, actor.RestrictionEffectiveAt, order.ID).Scan(&authorized); err != nil {
		return internalStoreError()
	}
	if !authorized {
		return apiOrderNotFound()
	}
	return nil
}

func (s *Store) MaterializeExpiredAPIOrders(ctx context.Context, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text
		FROM api_orders
		WHERE NOT EXISTS (SELECT 1 FROM api_order_catalog_risk_holds hold WHERE hold.api_order_id = api_orders.id AND hold.status = 'active')
		  AND ((status = 'pending_payment' AND dispute_status IN ('none', 'closed') AND payment_expires_at <= $1)
		   OR (
		     status = 'delivery_submitted'
		     AND dispute_status IN ('none', 'closed')
		     AND delivery_review_expires_at <= $2
		   ))
	`, now, now.Add(apiorder.DeliveryReviewReminderLead))
	if err != nil {
		return internalStoreError()
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return internalStoreError()
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return internalStoreError()
	}
	for _, id := range ids {
		if appErr := s.materializeExpiredAPIOrder(ctx, s.pool, id, now); appErr != nil {
			return appErr
		}
	}
	return nil
}

type apiOrderMaterializationResult struct {
	PaymentTimeoutCancelled bool
	DeliveryReviewReminded  bool
	AutoCompleted           bool
}

func (s *Store) materializeExpiredAPIOrder(ctx context.Context, q queryer, orderID string, now time.Time) *domain.AppError {
	if tx, ok := q.(pgx.Tx); ok {
		_, appErr := s.materializeExpiredAPIOrderInTx(ctx, tx, orderID, now)
		return appErr
	}
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	if _, appErr := s.materializeExpiredAPIOrderInTx(ctx, tx, orderID, now); appErr != nil {
		return appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) materializeExpiredAPIOrderInTx(ctx context.Context, tx pgx.Tx, orderID string, now time.Time) (apiOrderMaterializationResult, *domain.AppError) {
	order, err := s.getAPIOrder(ctx, tx, orderID, true, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiOrderMaterializationResult{}, nil
	}
	if err != nil {
		return apiOrderMaterializationResult{}, internalStoreError()
	}
	var activeCatalogHold bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM api_order_catalog_risk_holds WHERE api_order_id = $1 AND status = 'active')`, order.ID).Scan(&activeCatalogHold); err != nil {
		return apiOrderMaterializationResult{}, internalStoreError()
	}
	if activeCatalogHold {
		return apiOrderMaterializationResult{}, nil
	}
	if order.Status == apiorder.StatusPendingPayment && !apiorder.IsDisputeActive(order.DisputeStatus) && !order.PaymentExpiresAt.After(now) {
		if appErr := releaseAPIOrderReservationInTx(ctx, tx, order, now); appErr != nil {
			return apiOrderMaterializationResult{}, appErr
		}
		order.Status = apiorder.StatusCancelled
		order.CancelReason = apiorder.CancelReasonPaymentTimeout
		order.CancelledAt = &now
		order.PackageStockReserved = false
		order.UpdatedAt = now
		order.Version++
		if appErr := updateAPIOrderInTx(ctx, tx, order); appErr != nil {
			return apiOrderMaterializationResult{}, appErr
		}
		if appErr := insertAPIOrderEventInTx(ctx, tx, order, "", apiorder.EventPaymentTimeoutCancelled, apiorder.StatusPendingPayment, apiorder.StatusCancelled, "", "payment-timeout", now); appErr != nil {
			return apiOrderMaterializationResult{}, appErr
		}
		if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, "", apiorder.EventPaymentTimeoutCancelled, "payment-timeout", now); appErr != nil {
			return apiOrderMaterializationResult{}, appErr
		}
		return apiOrderMaterializationResult{PaymentTimeoutCancelled: true}, nil
	}
	if order.Status != apiorder.StatusDeliverySubmitted || apiorder.IsDisputeActive(order.DisputeStatus) || order.DeliveryReviewExpiresAt == nil {
		return apiOrderMaterializationResult{}, nil
	}
	if !now.Before(*order.DeliveryReviewExpiresAt) {
		completedAt := *order.DeliveryReviewExpiresAt
		order.Status = apiorder.StatusCompleted
		order.CompletionSource = apiorder.CompletionSourceAutoCompleted
		order.CompletedAt = &completedAt
		order.UpdatedAt = now
		order.Version++
		if appErr := updateAPIOrderInTx(ctx, tx, order); appErr != nil {
			return apiOrderMaterializationResult{}, appErr
		}
		if appErr := insertAPIOrderEventInTx(ctx, tx, order, "", apiorder.EventAutoCompleted, apiorder.StatusDeliverySubmitted, apiorder.StatusCompleted, "", "delivery-review-auto-complete", now); appErr != nil {
			return apiOrderMaterializationResult{}, appErr
		}
		if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, "", apiorder.EventAutoCompleted, "delivery-review-auto-complete", now); appErr != nil {
			return apiOrderMaterializationResult{}, appErr
		}
		return apiOrderMaterializationResult{AutoCompleted: true}, nil
	}
	reminderAt := order.DeliveryReviewExpiresAt.Add(-apiorder.DeliveryReviewReminderLead)
	if order.DeliveryReviewRemindedAt != nil || now.Before(reminderAt) {
		return apiOrderMaterializationResult{}, nil
	}
	if _, err := tx.Exec(ctx, `
		UPDATE api_orders
		SET delivery_review_reminded_at = $2
		WHERE id = $1 AND delivery_review_reminded_at IS NULL
	`, order.ID, now); err != nil {
		return apiOrderMaterializationResult{}, internalStoreError()
	}
	order.DeliveryReviewRemindedAt = &now
	if appErr := insertAPIOrderEventInTx(ctx, tx, order, "", apiorder.EventDeliveryReviewReminder, order.Status, order.Status, "", "delivery-review-reminder", now); appErr != nil {
		return apiOrderMaterializationResult{}, appErr
	}
	if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, "", apiorder.EventDeliveryReviewReminder, "delivery-review-reminder", now); appErr != nil {
		return apiOrderMaterializationResult{}, appErr
	}
	return apiOrderMaterializationResult{DeliveryReviewReminded: true}, nil
}

func reserveAPIOrderInventoryInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, now time.Time) *domain.AppError {
	if order.BillingModeSnapshot == apimarket.ServiceBillingModeFixedPackage {
		commandTag, err := tx.Exec(ctx, `
			UPDATE api_service_packages
			SET stock_available = stock_available - 1,
			    updated_at = $3
			WHERE id = $1
			  AND api_service_id = $2
			  AND enabled = true
			  AND stock_available > 0
		`, order.SelectedPackageID, order.APIServiceID, now)
		if err != nil {
			return internalStoreError()
		}
		if commandTag.RowsAffected() != 1 {
			return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Package sold out", "套餐库存不足，请刷新后重试。")
		}
		return nil
	}
	if order.BillingModeSnapshot != apimarket.ServiceBillingModeMetered || strings.TrimSpace(order.RequestedUSDAllowanceSnapshot) == "" {
		return nil
	}
	commandTag, err := tx.Exec(ctx, `
		UPDATE api_services
		SET available_usd_allowance = available_usd_allowance - $2::numeric,
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
		  AND available_usd_allowance >= $2::numeric
	`, order.APIServiceID, order.RequestedUSDAllowanceSnapshot, now)
	if err != nil {
		return internalStoreError()
	}
	if commandTag.RowsAffected() != 1 {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "USD allowance unavailable", "商户当前可售美元额度不足，请刷新后重试。")
	}
	return nil
}

func releaseAPIOrderReservationInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, now time.Time) *domain.AppError {
	if order.PurchaseKind == apiorder.PurchaseKindLimitedQuotaOffer {
		return releaseAPIQuotaReservationInTx(ctx, tx, order, now)
	}
	if order.BillingModeSnapshot == apimarket.ServiceBillingModeFixedPackage {
		if !order.PackageStockReserved || strings.TrimSpace(order.SelectedPackageID) == "" {
			return nil
		}
		commandTag, err := tx.Exec(ctx, `
			UPDATE api_service_packages
			SET stock_available = stock_available + 1,
			    updated_at = $3
			WHERE id = $1
			  AND api_service_id = $2
			  AND stock_available < stock_total
		`, order.SelectedPackageID, order.APIServiceID, now)
		if err != nil {
			return internalStoreError()
		}
		if commandTag.RowsAffected() != 1 {
			return internalStoreError()
		}
		return nil
	}
	if order.BillingModeSnapshot != apimarket.ServiceBillingModeMetered || strings.TrimSpace(order.RequestedUSDAllowanceSnapshot) == "" {
		return nil
	}
	commandTag, err := tx.Exec(ctx, `
		UPDATE api_services
		SET available_usd_allowance = available_usd_allowance + $2::numeric,
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
	`, order.APIServiceID, order.RequestedUSDAllowanceSnapshot, now)
	if err != nil {
		return internalStoreError()
	}
	if commandTag.RowsAffected() != 1 {
		return internalStoreError()
	}
	return nil
}

func releaseAPIQuotaReservationInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, now time.Time) *domain.AppError {
	if order.APIQuotaInventoryUnitID == "" || order.APIQuotaAllocationID == "" || order.APIQuotaBatchID == "" {
		return internalStoreError()
	}

	var unitStatus, allocationStatus, saleMode, batchStatus, offerStatus, roundStatus, usdAllowance string
	var saleCutoffAt, expiresAt time.Time
	var roundStartsAt, roundEndsAt *time.Time
	err := tx.QueryRow(ctx, `
		SELECT u.status, u.usd_allowance::text, a.status, a.sale_mode,
		       b.status, o.status, b.sale_cutoff_at, b.expires_at,
		       COALESCE(r.status, ''), r.starts_at, r.ends_at
		FROM api_quota_inventory_units u
		JOIN api_quota_allocations a ON a.id = u.allocation_id AND a.batch_id = u.batch_id AND a.offer_id = u.offer_id
		JOIN api_quota_batches b ON b.id = u.batch_id
		JOIN api_quota_offers o ON o.id = u.offer_id AND o.batch_id = u.batch_id
		LEFT JOIN api_quota_sale_rounds r ON r.id = a.sale_round_id
		WHERE u.id = $1 AND u.reserved_order_id = $2
		FOR UPDATE OF u
	`, order.APIQuotaInventoryUnitID, order.ID).Scan(
		&unitStatus, &usdAllowance, &allocationStatus, &saleMode,
		&batchStatus, &offerStatus, &saleCutoffAt, &expiresAt,
		&roundStatus, &roundStartsAt, &roundEndsAt,
	)
	if err != nil || unitStatus != "reserved" {
		return internalStoreError()
	}

	reusable := allocationStatus == "active" &&
		(batchStatus == apiquota.BatchStatusPublished || batchStatus == apiquota.BatchStatusPaused) &&
		(offerStatus == apiquota.OfferStatusPublished || offerStatus == apiquota.OfferStatusPaused) &&
		now.Before(saleCutoffAt) && now.Before(expiresAt)
	if saleMode == apiquota.SaleModeScheduled {
		reusable = reusable && roundStatus == apiquota.RoundStatusScheduled && roundStartsAt != nil && roundEndsAt != nil &&
			!now.Before(*roundStartsAt) && now.Before(*roundEndsAt)
	}

	nextStatus := "available"
	var retiredAt any
	if !reusable {
		nextStatus = "retired"
		retiredAt = now
	}
	command, err := tx.Exec(ctx, `
		UPDATE api_quota_inventory_units
		SET status = $3, reserved_order_id = NULL, reserved_at = NULL,
		    retired_at = $4, updated_at = $5
		WHERE id = $1 AND reserved_order_id = $2 AND status = 'reserved'
	`, order.APIQuotaInventoryUnitID, order.ID, nextStatus, retiredAt, now)
	if err != nil || command.RowsAffected() != 1 {
		return internalStoreError()
	}

	if !reusable {
		command, err = tx.Exec(ctx, `
			UPDATE api_quota_allocations
			SET returned_usd_allowance = returned_usd_allowance + $2::numeric,
			    updated_at = $3
			WHERE id = $1
			  AND returned_usd_allowance + $2::numeric <= allocated_usd_allowance
		`, order.APIQuotaAllocationID, usdAllowance, now)
		if err != nil || command.RowsAffected() != 1 {
			return internalStoreError()
		}
		command, err = tx.Exec(ctx, `
			UPDATE api_quota_batches
			SET unallocated_usd_allowance = unallocated_usd_allowance + $2::numeric,
			    updated_at = $3, version = version + 1
			WHERE id = $1
			  AND unallocated_usd_allowance + $2::numeric <= declared_total_usd_allowance
		`, order.APIQuotaBatchID, usdAllowance, now)
		if err != nil || command.RowsAffected() != 1 {
			return internalStoreError()
		}
	}

	if order.APIQuotaCredentialID != "" {
		command, err = tx.Exec(ctx, `
			UPDATE api_quota_credentials
			SET status = 'available', reserved_order_id = NULL, reserved_at = NULL, updated_at = $3
			WHERE id = $1 AND reserved_order_id = $2 AND status = 'reserved'
		`, order.APIQuotaCredentialID, order.ID, now)
		if err != nil || command.RowsAffected() != 1 {
			return internalStoreError()
		}
	}
	return nil
}

func consumeAPIQuotaInventoryInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, now time.Time) *domain.AppError {
	if order.APIQuotaInventoryUnitID == "" {
		return internalStoreError()
	}
	command, err := tx.Exec(ctx, `
		UPDATE api_quota_inventory_units
		SET status = 'consumed', consumed_at = $3, updated_at = $3
		WHERE id = $1 AND reserved_order_id = $2 AND status = 'reserved'
	`, order.APIQuotaInventoryUnitID, order.ID, now)
	if err != nil || command.RowsAffected() != 1 {
		return internalStoreError()
	}
	return nil
}

func (s *Store) deliverPreimportedAPIQuotaCredentialInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, now time.Time) (string, *domain.AppError) {
	if s == nil || s.contactCodec == nil || order.APIQuotaCredentialID == "" {
		return "", domain.NewError(http.StatusConflict, domain.CodeAPIQuotaCredentialUnavailable, "Credential inventory unavailable", "订单预留的交付凭据不可用。")
	}
	var deliveryKind, apiBaseURL, panelLoginURL, username, instructions, keyVersion, cipherFormat string
	var apiKeyCiphertext, apiKeyNonce, passwordCiphertext, passwordNonce []byte
	err := tx.QueryRow(ctx, `
		SELECT delivery_kind, COALESCE(api_base_url, ''), COALESCE(panel_login_url, ''),
		       COALESCE(username, ''), COALESCE(instructions, ''),
		       api_key_ciphertext, api_key_nonce, password_ciphertext, password_nonce,
		       secret_encryption_key_version, secret_encryption_format
		FROM api_quota_credentials
		WHERE id = $1 AND api_quota_offer_id = $2 AND seller_user_id = $3
		  AND reserved_order_id = $4 AND status = 'reserved'
		FOR UPDATE
	`, order.APIQuotaCredentialID, order.APIQuotaOfferID, order.SellerUserID, order.ID).Scan(
		&deliveryKind, &apiBaseURL, &panelLoginURL, &username, &instructions,
		&apiKeyCiphertext, &apiKeyNonce, &passwordCiphertext, &passwordNonce, &keyVersion, &cipherFormat,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.NewError(http.StatusConflict, domain.CodeAPIQuotaCredentialUnavailable, "Credential inventory unavailable", "订单预留的交付凭据不可用。")
	}
	if err != nil {
		return "", internalStoreError()
	}
	credentialID := uuid.NewString()
	var encoded encodedContactValue
	switch deliveryKind {
	case apiorder.DeliveryKindAPIKeyEndpoint:
		plaintext, decodeErr := s.contactCodec.decode(
			apiKeyCiphertext, apiKeyNonce, keyVersion, cipherFormat,
			order.APIQuotaCredentialID, contactFieldQuotaAPIKey,
		)
		if decodeErr != nil {
			return "", internalStoreError()
		}
		encoded, err = s.contactCodec.encode(plaintext, credentialID, contactFieldOrderAPIKey)
		if err != nil {
			return "", internalStoreError()
		}
		apiKeyCiphertext = encoded.Ciphertext
		apiKeyNonce = encoded.Nonce
	case apiorder.DeliveryKindLoginAccount:
		plaintext, decodeErr := s.contactCodec.decode(
			passwordCiphertext, passwordNonce, keyVersion, cipherFormat,
			order.APIQuotaCredentialID, contactFieldQuotaPassword,
		)
		if decodeErr != nil {
			return "", internalStoreError()
		}
		encoded, err = s.contactCodec.encode(plaintext, credentialID, contactFieldOrderPassword)
		if err != nil {
			return "", internalStoreError()
		}
		passwordCiphertext = encoded.Ciphertext
		passwordNonce = encoded.Nonce
	default:
		return "", internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_order_delivery_credentials (
			id, api_order_id, seller_user_id, buyer_user_id, delivery_kind,
			api_base_url, panel_login_url, username, instructions,
			api_key_ciphertext, api_key_nonce, password_ciphertext, password_nonce,
			secret_encryption_key_version, secret_encryption_format, submitted_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $16
		)
	`, credentialID, order.ID, order.SellerUserID, order.BuyerUserID, deliveryKind,
		nullText(apiBaseURL), nullText(panelLoginURL), nullText(username), nullText(instructions),
		apiKeyCiphertext, apiKeyNonce, passwordCiphertext, passwordNonce,
		encoded.EncryptionKeyVersion, encoded.CipherFormat, now)
	if err != nil {
		return "", internalStoreError()
	}
	command, err := tx.Exec(ctx, `
		UPDATE api_quota_credentials
		SET status = 'delivered', delivered_at = $3, updated_at = $3
		WHERE id = $1 AND reserved_order_id = $2 AND status = 'reserved'
	`, order.APIQuotaCredentialID, order.ID, now)
	if err != nil || command.RowsAffected() != 1 {
		return "", internalStoreError()
	}
	return deliveryKind, nil
}

const apiOrderColumns = `
	id::text, purchase_kind, api_purchase_intent_id::text, api_service_id::text,
	buyer_user_id::text, seller_user_id::text, status, dispute_status,
	COALESCE(dispute_case_id::text, ''), service_title_snapshot,
	service_version_snapshot, billing_mode_snapshot, COALESCE(selected_package_id::text, ''),
	COALESCE(selected_package_snapshot::text, ''), COALESCE(quote_version_snapshot, 0),
		COALESCE(requested_usd_allowance_snapshot::text, ''), COALESCE(cny_per_usd_allowance_snapshot::text, ''), pricing_snapshot::text,
		COALESCE(probe_connection_id_snapshot::text, ''), COALESCE(api_base_url_snapshot, ''),
		COALESCE(normalized_api_base_url_snapshot, ''),
		five_hour_limit_mode_snapshot, COALESCE(five_hour_limit_usd_snapshot::text, ''),
	daily_limit_mode_snapshot, COALESCE(daily_limit_usd_snapshot::text, ''),
	package_stock_reserved, package_expires_at,
	COALESCE(api_quota_batch_id::text, ''), COALESCE(api_quota_offer_id::text, ''),
	COALESCE(api_quota_sale_round_id::text, ''), COALESCE(api_quota_allocation_id::text, ''),
	COALESCE(api_quota_inventory_unit_id::text, ''), COALESCE(api_quota_credential_id::text, ''),
	COALESCE(quota_offer_snapshot::text, ''), COALESCE(quota_offer_name_snapshot, ''),
	COALESCE(quota_usd_allowance_snapshot::text, ''), COALESCE(quota_price_cny_snapshot::text, ''),
	COALESCE(quota_cny_per_usd_snapshot::text, ''), COALESCE(quota_model_multiplier_snapshot::text, ''),
	quota_sale_cutoff_at_snapshot, quota_expires_at_snapshot, COALESCE(quota_sale_mode_snapshot, ''),
	quota_round_starts_at_snapshot, quota_round_ends_at_snapshot, COALESCE(quota_distribution_system_snapshot, ''),
	COALESCE(quota_ttft_band_snapshot, ''), COALESCE(quota_declared_max_concurrency_snapshot, 0),
	quota_performance_confirmed_at_snapshot, COALESCE(quota_performance_unverified_snapshot, false),
	COALESCE(quota_delivery_eta_minutes_snapshot, 0), COALESCE(quota_delivery_mode_snapshot, ''),
	amount::text, currency, selected_payment_method,
	payment_window_minutes_snapshot, payment_expires_at, payment_instructions_snapshot,
		COALESCE(payment_qr_code_data_url_snapshot, ''), COALESCE(payment_summary, ''), payment_submitted_at,
		merchant_confirm_due_at,
		COALESCE(payment_issue_reason, ''), COALESCE(payment_issue_note, ''), payment_issue_reported_at, paid_confirmed_at,
		delivery_due_at,
	COALESCE(delivery_note, ''), delivery_submitted_at,
	delivery_review_expires_at, delivery_review_reminded_at, COALESCE(completion_source, ''), completed_at,
		cancelled_at, COALESCE(cancel_reason, ''), COALESCE(late_payment_status, ''), late_payment_reported_at,
		COALESCE(late_payment_note, ''), late_payment_resolved_at, created_at, updated_at, version,
	order_no, prompt_audit_enabled_snapshot
`

func (s *Store) listAPIOrders(ctx context.Context, whereClause string, args []any, now time.Time) ([]apiorder.Order, *domain.AppError) {
	query := `SELECT ` + apiOrderColumns + ` FROM api_orders `
	if strings.TrimSpace(whereClause) != "" {
		query += whereClause
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	orders := []apiorder.Order{}
	for rows.Next() {
		var order apiorder.Order
		if err := rows.Scan(apiOrderScanTargets(&order)...); err != nil {
			rows.Close()
			return nil, internalStoreError()
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, internalStoreError()
	}
	rows.Close()
	for index := range orders {
		if appErr := attachAPIOrderCatalogRiskHold(ctx, s.pool, &orders[index]); appErr != nil {
			return nil, appErr
		}
		orders[index] = apiorder.WithAfterSalesProjection(orders[index], now)
	}
	return orders, nil
}

func (s *Store) listAdminAPIOrdersPage(ctx context.Context, filter apiorder.AdminOrderFilter, page domain.PageRequest, now time.Time) (domain.Page[apiorder.Order], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[apiorder.Order]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	sortMode := filter.NormalizedSort()
	position, appErr := apiorder.DecodeAdminOrderCursor(page.Cursor, sortMode)
	if appErr != nil {
		return domain.Page[apiorder.Order]{}, appErr
	}
	var timePosition time.Time
	if page.Cursor != "" {
		if _, err := uuid.Parse(position.ID); err != nil {
			return domain.Page[apiorder.Order]{}, invalidPageCursorError()
		}
		switch sortMode {
		case apiorder.AdminOrderSortAmountAsc, apiorder.AdminOrderSortAmountDesc:
		default:
			parsed, err := time.Parse(time.RFC3339Nano, position.Value)
			if err != nil {
				return domain.Page[apiorder.Order]{}, invalidPageCursorError()
			}
			timePosition = parsed
		}
	}

	conditions := make([]string, 0, 12)
	args := make([]any, 0, 16)
	addArgument := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}
	if len(filter.Statuses) > 0 {
		placeholder := addArgument(filter.Statuses)
		conditions = append(conditions, "status = ANY("+placeholder+"::text[])")
	}
	if createdAfter, ok := filter.CreatedAfter(now); ok {
		placeholder := addArgument(createdAfter)
		conditions = append(conditions, "created_at >= "+placeholder)
	}
	if value := strings.TrimSpace(filter.BuyerUserID); value != "" {
		placeholder := addArgument(value)
		conditions = append(conditions, "buyer_user_id = "+placeholder+"::uuid")
	}
	if value := strings.TrimSpace(filter.SellerUserID); value != "" {
		placeholder := addArgument(value)
		conditions = append(conditions, "seller_user_id = "+placeholder+"::uuid")
	}
	if value := strings.TrimSpace(filter.APIServiceID); value != "" {
		placeholder := addArgument(value)
		conditions = append(conditions, "api_service_id = "+placeholder+"::uuid")
	}
	activeDisputeStatuses := []string{
		apiorder.DisputeStatusNegotiating,
		apiorder.DisputeStatusOpen,
		apiorder.DisputeStatusAwaitingFulfillment,
		apiorder.DisputeStatusFulfillmentConfirmation,
	}
	switch strings.TrimSpace(filter.Dispute) {
	case apiorder.AdminOrderDisputeActive:
		placeholder := addArgument(activeDisputeStatuses)
		conditions = append(conditions, "dispute_status = ANY("+placeholder+"::text[])")
	case apiorder.AdminOrderDisputeNone:
		placeholder := addArgument(activeDisputeStatuses)
		conditions = append(conditions, "NOT (dispute_status = ANY("+placeholder+"::text[]))")
	}
	if value := strings.TrimSpace(filter.MinAmount); value != "" {
		placeholder := addArgument(value)
		conditions = append(conditions, "amount >= "+placeholder+"::numeric")
	}
	if value := strings.TrimSpace(filter.MaxAmount); value != "" {
		placeholder := addArgument(value)
		conditions = append(conditions, "amount <= "+placeholder+"::numeric")
	}
	if value := strings.ToLower(strings.TrimSpace(filter.Query)); value != "" {
		placeholder := addArgument(value)
		conditions = append(conditions, `(
			strpos(lower(id::text), `+placeholder+`) > 0 OR
			strpos(lower(order_no), `+placeholder+`) > 0 OR
			strpos(lower(api_service_id::text), `+placeholder+`) > 0 OR
			strpos(lower(service_title_snapshot), `+placeholder+`) > 0 OR
			strpos(lower(buyer_user_id::text), `+placeholder+`) > 0 OR
			strpos(lower(seller_user_id::text), `+placeholder+`) > 0 OR
			(
				regexp_replace(`+placeholder+`, '[^a-z0-9]', '', 'g') <> '' AND
				strpos(regexp_replace(lower(order_no), '[^a-z0-9]', '', 'g'), regexp_replace(`+placeholder+`, '[^a-z0-9]', '', 'g')) > 0
			)
		)`)
	}
	if page.Cursor != "" {
		valuePlaceholder := ""
		switch sortMode {
		case apiorder.AdminOrderSortAmountAsc, apiorder.AdminOrderSortAmountDesc:
			valuePlaceholder = addArgument(position.Value) + "::numeric"
		default:
			valuePlaceholder = addArgument(timePosition) + "::timestamptz"
		}
		idPlaceholder := addArgument(position.ID) + "::uuid"
		switch sortMode {
		case apiorder.AdminOrderSortCreatedDesc:
			conditions = append(conditions, "(created_at, id) < ("+valuePlaceholder+", "+idPlaceholder+")")
		case apiorder.AdminOrderSortAmountDesc:
			conditions = append(conditions, "(amount, id) < ("+valuePlaceholder+", "+idPlaceholder+")")
		case apiorder.AdminOrderSortAmountAsc:
			conditions = append(conditions, "(amount, id) > ("+valuePlaceholder+", "+idPlaceholder+")")
		default:
			conditions = append(conditions, "(updated_at, id) < ("+valuePlaceholder+", "+idPlaceholder+")")
		}
	}

	query := "SELECT " + apiOrderColumns + " FROM api_orders"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	switch sortMode {
	case apiorder.AdminOrderSortCreatedDesc:
		query += " ORDER BY created_at DESC, id DESC"
	case apiorder.AdminOrderSortAmountDesc:
		query += " ORDER BY amount DESC, id DESC"
	case apiorder.AdminOrderSortAmountAsc:
		query += " ORDER BY amount ASC, id ASC"
	default:
		query += " ORDER BY updated_at DESC, id DESC"
	}
	args = append(args, page.Limit+1)
	query += " LIMIT $" + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[apiorder.Order]{}, internalStoreError()
	}
	orders, appErr := scanAPIOrders(rows)
	if appErr != nil {
		rows.Close()
		return domain.Page[apiorder.Order]{}, appErr
	}
	rows.Close()
	for index := range orders {
		if appErr := attachAPIOrderCatalogRiskHold(ctx, s.pool, &orders[index]); appErr != nil {
			return domain.Page[apiorder.Order]{}, appErr
		}
		orders[index] = apiorder.WithAfterSalesProjection(orders[index], now)
	}
	return apiorder.PageAdminOrderItems(orders, page, sortMode), nil
}

func scanAPIOrders(rows pgx.Rows) ([]apiorder.Order, *domain.AppError) {
	orders := []apiorder.Order{}
	for rows.Next() {
		var order apiorder.Order
		if err := rows.Scan(apiOrderScanTargets(&order)...); err != nil {
			return nil, internalStoreError()
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return orders, nil
}

func (s *Store) getAPIOrder(ctx context.Context, q queryer, orderID string, forUpdate, includeCredential bool) (apiorder.Order, error) {
	query := `SELECT ` + apiOrderColumns + ` FROM api_orders WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var order apiorder.Order
	err := q.QueryRow(ctx, query, orderID).Scan(apiOrderScanTargets(&order)...)
	if err == nil {
		if appErr := attachAPIOrderCatalogRiskHold(ctx, q, &order); appErr != nil {
			return apiorder.Order{}, errors.New(appErr.Detail)
		}
	}
	if err == nil && !forUpdate && includeCredential {
		if appErr := s.attachAPIOrderDeliveryCredential(ctx, q, &order); appErr != nil {
			return apiorder.Order{}, errors.New(appErr.Detail)
		}
	}
	return order, err
}

func attachAPIOrderCatalogRiskHold(ctx context.Context, q queryer, order *apiorder.Order) *domain.AppError {
	if order == nil || strings.TrimSpace(order.ID) == "" {
		return nil
	}
	var hold apiorder.CatalogRiskHold
	err := q.QueryRow(ctx, `
		SELECT id::text, source_type, source_id::text, status, reason, created_at,
		       COALESCE(resolved_by::text, ''), resolved_at, COALESCE(resolution_note, ''), version
		FROM api_order_catalog_risk_holds
		WHERE api_order_id = $1
		ORDER BY (status = 'active') DESC, created_at DESC, id DESC
		LIMIT 1
	`, order.ID).Scan(&hold.ID, &hold.SourceType, &hold.SourceID, &hold.Status, &hold.Reason, &hold.CreatedAt,
		&hold.ResolvedBy, &hold.ResolvedAt, &hold.ResolutionNote, &hold.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		order.CatalogRiskHold = nil
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	order.CatalogRiskHold = &hold
	return nil
}

func rejectActiveCatalogRiskHoldInTx(ctx context.Context, tx pgx.Tx, orderID, action string) *domain.AppError {
	var holdID string
	err := tx.QueryRow(ctx, `
		SELECT id::text
		FROM api_order_catalog_risk_holds
		WHERE api_order_id = $1 AND status = 'active'
		FOR SHARE
	`, orderID).Scan(&holdID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Catalog risk hold active", "订单因关联目录风险被暂停，当前操作不可执行；仍可查看证据或发起纠纷。")
}

func apiOrderScanTargets(order *apiorder.Order) []any {
	return []any{
		&order.ID,
		&order.PurchaseKind,
		&order.APIPurchaseIntentID,
		&order.APIServiceID,
		&order.BuyerUserID,
		&order.SellerUserID,
		&order.Status,
		&order.DisputeStatus,
		&order.DisputeCaseID,
		&order.ServiceTitleSnapshot,
		&order.ServiceVersionSnapshot,
		&order.BillingModeSnapshot,
		&order.SelectedPackageID,
		&order.SelectedPackageSnapshot,
		&order.QuoteVersionSnapshot,
		&order.RequestedUSDAllowanceSnapshot,
		&order.CNYPerUSDAllowanceSnapshot,
		&order.PricingSnapshot,
		&order.ProbeConnectionIDSnapshot,
		&order.APIBaseURLSnapshot,
		&order.NormalizedAPIBaseURLSnapshot,
		&order.QuotaUsagePolicySnapshot.FiveHour.Mode,
		&order.QuotaUsagePolicySnapshot.FiveHour.AmountUSD,
		&order.QuotaUsagePolicySnapshot.Daily.Mode,
		&order.QuotaUsagePolicySnapshot.Daily.AmountUSD,
		&order.PackageStockReserved,
		&order.PackageExpiresAt,
		&order.APIQuotaBatchID,
		&order.APIQuotaOfferID,
		&order.APIQuotaSaleRoundID,
		&order.APIQuotaAllocationID,
		&order.APIQuotaInventoryUnitID,
		&order.APIQuotaCredentialID,
		&order.QuotaOfferSnapshot,
		&order.QuotaOfferNameSnapshot,
		&order.QuotaUSDAllowanceSnapshot,
		&order.QuotaPriceCNYSnapshot,
		&order.QuotaCNYPerUSDSnapshot,
		&order.QuotaModelMultiplierSnapshot,
		&order.QuotaSaleCutoffAtSnapshot,
		&order.QuotaExpiresAtSnapshot,
		&order.QuotaSaleModeSnapshot,
		&order.QuotaRoundStartsAtSnapshot,
		&order.QuotaRoundEndsAtSnapshot,
		&order.QuotaDistributionSnapshot,
		&order.QuotaTTFTBandSnapshot,
		&order.QuotaDeclaredMaxConcurrency,
		&order.QuotaPerformanceConfirmedAt,
		&order.QuotaPerformanceUnverified,
		&order.QuotaDeliveryETAMinutes,
		&order.QuotaDeliveryMode,
		&order.Amount,
		&order.Currency,
		&order.SelectedPaymentMethod,
		&order.PaymentWindowMinutesSnapshot,
		&order.PaymentExpiresAt,
		&order.PaymentInstructionsSnapshot,
		&order.PaymentQRCodeDataURLSnapshot,
		&order.PaymentSummary,
		&order.PaymentSubmittedAt,
		&order.MerchantConfirmDueAt,
		&order.PaymentIssueReason,
		&order.PaymentIssueNote,
		&order.PaymentIssueReportedAt,
		&order.PaidConfirmedAt,
		&order.DeliveryDueAt,
		&order.DeliveryNote,
		&order.DeliverySubmittedAt,
		&order.DeliveryReviewExpiresAt,
		&order.DeliveryReviewRemindedAt,
		&order.CompletionSource,
		&order.CompletedAt,
		&order.CancelledAt,
		&order.CancelReason,
		&order.LatePaymentStatus,
		&order.LatePaymentReportedAt,
		&order.LatePaymentNote,
		&order.LatePaymentResolvedAt,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.Version,
		&order.OrderNo,
		&order.PromptAuditEnabledSnapshot,
	}
}

func newStoreAPIOrder(input apiorder.CreateInput, intent apiintent.Intent, service apimarket.Service, now time.Time) (apiorder.Order, *domain.AppError) {
	if !apimarket.WithOrderabilityAt(service, now).IsOrderable {
		return apiorder.Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Service not orderable", "当前 API 服务不可下单。")
	}
	if strings.TrimSpace(service.ProbeConnectionID) == "" || strings.TrimSpace(service.ProbeBaseURL) == "" || strings.TrimSpace(service.NormalizedProbeBaseURL) == "" {
		return apiorder.Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Probe target unavailable", "当前 API 服务缺少可冻结的探针连接目标。")
	}
	method := strings.TrimSpace(input.PaymentMethod)
	option, ok := storeFindPaymentOption(service, method)
	if !ok {
		return apiorder.Order{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method invalid", "选择的付款方式不可用。", "paymentMethod", "invalid", "选择的付款方式不可用。")
	}
	amount, currency, appErr := storeResolveAPIOrderAmount(intent, service)
	if appErr != nil {
		return apiorder.Order{}, appErr
	}
	return apiorder.Order{
		ID:                            uuid.NewString(),
		PurchaseKind:                  apiorder.PurchaseKindAPIService,
		APIPurchaseIntentID:           intent.ID,
		APIServiceID:                  intent.APIServiceID,
		BuyerUserID:                   input.BuyerUserID,
		SellerUserID:                  intent.OwnerUserID,
		Status:                        apiorder.StatusPendingPayment,
		DisputeStatus:                 apiorder.DisputeStatusNone,
		ServiceTitleSnapshot:          service.Title,
		ServiceVersionSnapshot:        service.Version,
		BillingModeSnapshot:           service.BillingMode,
		SelectedPackageID:             intent.SelectedPackageID,
		SelectedPackageSnapshot:       intent.SelectedPackageSnapshot,
		RequestedUSDAllowanceSnapshot: intent.RequestedUSDAllowance,
		CNYPerUSDAllowanceSnapshot:    intent.DeclaredCNYPerUSDAllowanceSnapshot,
		PricingSnapshot:               intent.PricingSnapshot,
		ProbeConnectionIDSnapshot:     service.ProbeConnectionID,
		APIBaseURLSnapshot:            service.ProbeBaseURL,
		NormalizedAPIBaseURLSnapshot:  service.NormalizedProbeBaseURL,
		QuotaUsagePolicySnapshot:      intent.QuotaUsagePolicySnapshot,
		PromptAuditEnabledSnapshot:    intent.PromptAuditEnabledSnapshot,
		PackageStockReserved:          service.BillingMode == apimarket.ServiceBillingModeFixedPackage,
		Amount:                        amount,
		Currency:                      currency,
		SelectedPaymentMethod:         method,
		PaymentWindowMinutesSnapshot:  service.PaymentWindowMinutes,
		PaymentExpiresAt:              now.Add(time.Duration(service.PaymentWindowMinutes) * time.Minute),
		PaymentInstructionsSnapshot:   option.PaymentInstructions,
		PaymentQRCodeDataURLSnapshot:  option.PaymentQRCodeDataURL,
		CreatedAt:                     now,
		UpdatedAt:                     now,
		Version:                       1,
	}, nil
}

func insertAPIOrderInTx(ctx context.Context, tx pgx.Tx, order *apiorder.Order) *domain.AppError {
	err := insertAPIOrderWithNumberRetry(order, apiorder.GenerateOrderNo, func() error {
		commandTag, insertErr := tx.Exec(ctx, `
		INSERT INTO api_orders (
			id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
			status, dispute_status, dispute_case_id, service_title_snapshot,
			service_version_snapshot, billing_mode_snapshot, selected_package_id,
				selected_package_snapshot, quote_version_snapshot,
					requested_usd_allowance_snapshot, cny_per_usd_allowance_snapshot, pricing_snapshot,
					probe_connection_id_snapshot, api_base_url_snapshot, normalized_api_base_url_snapshot,
				five_hour_limit_mode_snapshot, five_hour_limit_usd_snapshot,
				daily_limit_mode_snapshot, daily_limit_usd_snapshot,
				package_stock_reserved, package_expires_at,
				amount, currency,
				selected_payment_method, payment_window_minutes_snapshot, payment_expires_at,
				payment_instructions_snapshot, payment_qr_code_data_url_snapshot, payment_summary, payment_submitted_at,
				payment_issue_reason, payment_issue_note, payment_issue_reported_at,
				paid_confirmed_at, delivery_note, delivery_submitted_at, completed_at,
				cancelled_at, cancel_reason, created_at, updated_at, version, order_no,
				prompt_audit_enabled_snapshot
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14,
			$15, $16, $17,
					$18, $19, $20,
					$21, $22, $23, $24,
					$25, $26,
					$27, $28,
					$29, $30, $31,
					$32, $33, $34, $35,
					$36, $37, $38,
					$39, $40, $41, $42,
						$43, $44, $45, $46, $47, $48, $49
		)
		ON CONFLICT ON CONSTRAINT ux_api_orders_order_no DO NOTHING
	`, order.ID, order.APIPurchaseIntentID, order.APIServiceID, order.BuyerUserID, order.SellerUserID,
			order.Status, order.DisputeStatus, nullUUID(order.DisputeCaseID), order.ServiceTitleSnapshot,
			order.ServiceVersionSnapshot, order.BillingModeSnapshot, nullUUID(order.SelectedPackageID),
			nullJSON(order.SelectedPackageSnapshot), nullInt64(order.QuoteVersionSnapshot),
			nullNumeric(order.RequestedUSDAllowanceSnapshot), nullNumeric(order.CNYPerUSDAllowanceSnapshot), nullJSON(order.PricingSnapshot),
			nullUUID(order.ProbeConnectionIDSnapshot), nullText(order.APIBaseURLSnapshot), nullText(order.NormalizedAPIBaseURLSnapshot),
			order.QuotaUsagePolicySnapshot.FiveHour.Mode, nullNumeric(order.QuotaUsagePolicySnapshot.FiveHour.AmountUSD),
			order.QuotaUsagePolicySnapshot.Daily.Mode, nullNumeric(order.QuotaUsagePolicySnapshot.Daily.AmountUSD),
			order.PackageStockReserved, order.PackageExpiresAt,
			order.Amount, order.Currency,
			order.SelectedPaymentMethod, order.PaymentWindowMinutesSnapshot, order.PaymentExpiresAt,
			order.PaymentInstructionsSnapshot, nullText(order.PaymentQRCodeDataURLSnapshot), nullText(order.PaymentSummary), order.PaymentSubmittedAt,
			nullText(order.PaymentIssueReason), nullText(order.PaymentIssueNote), order.PaymentIssueReportedAt,
			order.PaidConfirmedAt, nullText(order.DeliveryNote), order.DeliverySubmittedAt, order.CompletedAt,
			order.CancelledAt, nullText(order.CancelReason), order.CreatedAt, order.UpdatedAt, order.Version, order.OrderNo,
			order.PromptAuditEnabledSnapshot)
		if insertErr != nil {
			return insertErr
		}
		if commandTag.RowsAffected() == 0 {
			return errAPIOrderNumberCollision
		}
		return nil
	})
	if err != nil {
		if isUniqueViolationOnConstraint(err, "ux_api_orders_intent") {
			return domain.NewAPIPurchaseIntentHasOrderError()
		}
		return internalStoreError()
	}
	return nil
}

func insertAPIOrderWithNumberRetry(order *apiorder.Order, generate func(time.Time) (string, error), insert func() error) error {
	if order == nil || generate == nil || insert == nil {
		return errors.New("API order number insert dependencies are unavailable")
	}
	for range maxAPIOrderNumberInsertAttempts {
		orderNo, err := generate(order.CreatedAt)
		if err != nil {
			return err
		}
		order.OrderNo = orderNo
		err = insert()
		if errors.Is(err, errAPIOrderNumberCollision) {
			continue
		}
		return err
	}
	return errors.New("API order number collision retry exhausted")
}

func updateAPIOrderInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order) *domain.AppError {
	_, err := tx.Exec(ctx, `
		UPDATE api_orders
		SET status = $2,
		    dispute_status = $3,
			    dispute_case_id = $4,
			    payment_summary = $5,
			    payment_submitted_at = $6,
			    merchant_confirm_due_at = $7,
			    payment_issue_reason = $8,
			    payment_issue_note = $9,
			    payment_issue_reported_at = $10,
			    paid_confirmed_at = $11,
			    delivery_due_at = $12,
			    delivery_note = $13,
			    delivery_submitted_at = $14,
			    delivery_review_expires_at = $15,
			    delivery_review_reminded_at = $16,
			    completion_source = $17,
			    completed_at = $18,
			    cancelled_at = $19,
			    cancel_reason = $20,
			    late_payment_status = $21,
			    late_payment_reported_at = $22,
			    late_payment_note = $23,
			    late_payment_resolved_at = $24,
			    package_stock_reserved = $25,
			    package_expires_at = $26,
			    updated_at = $27,
			    version = $28
		WHERE id = $1
		`, order.ID, order.Status, order.DisputeStatus, nullUUID(order.DisputeCaseID),
		nullText(order.PaymentSummary), order.PaymentSubmittedAt, order.MerchantConfirmDueAt,
		nullText(order.PaymentIssueReason), nullText(order.PaymentIssueNote), order.PaymentIssueReportedAt, order.PaidConfirmedAt, order.DeliveryDueAt,
		nullText(order.DeliveryNote), order.DeliverySubmittedAt,
		order.DeliveryReviewExpiresAt, order.DeliveryReviewRemindedAt, nullText(order.CompletionSource), order.CompletedAt,
		order.CancelledAt, nullText(order.CancelReason), nullText(order.LatePaymentStatus), order.LatePaymentReportedAt,
		nullText(order.LatePaymentNote), order.LatePaymentResolvedAt, order.PackageStockReserved, order.PackageExpiresAt,
		order.UpdatedAt, order.Version)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func insertAPIOrderEventInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, actorUserID, eventType, fromStatus, toStatus, note, requestID string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO api_order_events (
			id, api_order_id, actor_user_id, event_type, from_status,
			to_status, note, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (api_order_id, event_type, request_id) DO NOTHING
	`, uuid.NewString(), order.ID, nullUUID(actorUserID), eventType, nullText(fromStatus),
		nullText(toStatus), nullText(note), requestID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func insertAPIOrderPaymentInstructionAccessLogInTx(ctx context.Context, tx pgx.Tx, orderID, buyerUserID, requestID string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO api_order_payment_instruction_access_logs (
			id, api_order_id, buyer_user_id, request_id, accessed_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.NewString(), orderID, buyerUserID, requestID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) insertAPIOrderDeliveryCredentialInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, input apiorder.DeliveryCredentialInput, now time.Time) (apiorder.DeliveryCredential, *domain.AppError) {
	if s == nil || s.contactCodec == nil {
		return apiorder.DeliveryCredential{}, internalStoreError()
	}
	credential := apiorder.DeliveryCredential{
		ID:            uuid.NewString(),
		APIOrderID:    order.ID,
		SellerUserID:  order.SellerUserID,
		BuyerUserID:   order.BuyerUserID,
		DeliveryKind:  input.DeliveryKind,
		APIBaseURL:    input.APIBaseURL,
		APIKey:        input.APIKey,
		PanelLoginURL: input.PanelLoginURL,
		Username:      input.Username,
		Password:      input.Password,
		Instructions:  input.Instructions,
		SubmittedAt:   now,
		CreatedAt:     now,
	}
	var apiKeyCiphertext []byte
	var apiKeyNonce []byte
	var passwordCiphertext []byte
	var passwordNonce []byte
	keyVersion := s.contactCodec.encryptionKeyVersion
	cipherFormat := contactCipherFormatAADV1
	if credential.APIKey != "" {
		encoded, err := s.contactCodec.encode(credential.APIKey, credential.ID, contactFieldOrderAPIKey)
		if err != nil {
			return apiorder.DeliveryCredential{}, internalStoreError()
		}
		apiKeyCiphertext = encoded.Ciphertext
		apiKeyNonce = encoded.Nonce
		keyVersion = encoded.EncryptionKeyVersion
		cipherFormat = encoded.CipherFormat
	}
	if credential.Password != "" {
		encoded, err := s.contactCodec.encode(credential.Password, credential.ID, contactFieldOrderPassword)
		if err != nil {
			return apiorder.DeliveryCredential{}, internalStoreError()
		}
		passwordCiphertext = encoded.Ciphertext
		passwordNonce = encoded.Nonce
		keyVersion = encoded.EncryptionKeyVersion
		cipherFormat = encoded.CipherFormat
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO api_order_delivery_credentials (
			id, api_order_id, seller_user_id, buyer_user_id, delivery_kind,
			api_base_url, panel_login_url, username, instructions,
			api_key_ciphertext, api_key_nonce, password_ciphertext, password_nonce,
			secret_encryption_key_version, secret_encryption_format, submitted_at, created_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17
		)
	`, credential.ID, credential.APIOrderID, credential.SellerUserID, credential.BuyerUserID, credential.DeliveryKind,
		nullText(credential.APIBaseURL), nullText(credential.PanelLoginURL), nullText(credential.Username), nullText(credential.Instructions),
		apiKeyCiphertext, apiKeyNonce, passwordCiphertext, passwordNonce, keyVersion, cipherFormat,
		credential.SubmittedAt, credential.CreatedAt)
	if err != nil {
		if isUniqueViolationOnConstraint(err, "ux_api_order_delivery_credentials_order") {
			return apiorder.DeliveryCredential{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "交付信息已提交，不能再次修改。")
		}
		return apiorder.DeliveryCredential{}, internalStoreError()
	}
	return credential, nil
}

func (s *Store) attachAPIOrderDeliveryCredential(ctx context.Context, q queryer, order *apiorder.Order) *domain.AppError {
	if order == nil || order.DeliverySubmittedAt == nil {
		return nil
	}
	credential, found, appErr := s.getAPIOrderDeliveryCredential(ctx, q, order.ID)
	if appErr != nil {
		return appErr
	}
	if found {
		order.DeliveryCredential = &credential
	}
	return nil
}

func (s *Store) getAPIOrderDeliveryCredential(ctx context.Context, q queryer, orderID string) (apiorder.DeliveryCredential, bool, *domain.AppError) {
	if s == nil || s.contactCodec == nil {
		return apiorder.DeliveryCredential{}, false, internalStoreError()
	}
	var credential apiorder.DeliveryCredential
	var apiKeyCiphertext []byte
	var apiKeyNonce []byte
	var passwordCiphertext []byte
	var passwordNonce []byte
	var keyVersion, cipherFormat string
	err := q.QueryRow(ctx, `
		SELECT id::text, api_order_id::text, seller_user_id::text, buyer_user_id::text,
		       delivery_kind, COALESCE(api_base_url, ''), COALESCE(panel_login_url, ''),
		       COALESCE(username, ''), COALESCE(instructions, ''),
		       api_key_ciphertext, api_key_nonce, password_ciphertext, password_nonce,
		       secret_encryption_key_version, secret_encryption_format, submitted_at, created_at,
		       destroyed_at, COALESCE(destroy_reason, '')
		FROM api_order_delivery_credentials
		WHERE api_order_id = $1
	`, orderID).Scan(
		&credential.ID,
		&credential.APIOrderID,
		&credential.SellerUserID,
		&credential.BuyerUserID,
		&credential.DeliveryKind,
		&credential.APIBaseURL,
		&credential.PanelLoginURL,
		&credential.Username,
		&credential.Instructions,
		&apiKeyCiphertext,
		&apiKeyNonce,
		&passwordCiphertext,
		&passwordNonce,
		&keyVersion,
		&cipherFormat,
		&credential.SubmittedAt,
		&credential.CreatedAt,
		&credential.DestroyedAt,
		&credential.DestroyReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.DeliveryCredential{}, false, nil
	}
	if err != nil {
		return apiorder.DeliveryCredential{}, false, internalStoreError()
	}
	if credential.DestroyedAt != nil {
		return credential, true, nil
	}
	if len(apiKeyCiphertext) > 0 {
		apiKey, err := s.contactCodec.decode(apiKeyCiphertext, apiKeyNonce, keyVersion, cipherFormat, credential.ID, contactFieldOrderAPIKey)
		if err != nil {
			return apiorder.DeliveryCredential{}, false, internalStoreError()
		}
		credential.APIKey = apiKey
	}
	if len(passwordCiphertext) > 0 {
		password, err := s.contactCodec.decode(passwordCiphertext, passwordNonce, keyVersion, cipherFormat, credential.ID, contactFieldOrderPassword)
		if err != nil {
			return apiorder.DeliveryCredential{}, false, internalStoreError()
		}
		credential.Password = password
	}
	return credential, true, nil
}

func openDisputeFromAPIOrderInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, input apiorder.ActionInput, now time.Time) (report.DisputeCase, *domain.AppError) {
	issueOccurredAt, appErr := apiorder.ValidateDisputeOccurrence(order, input.IssueOccurredAt, now)
	if appErr != nil {
		return report.DisputeCase{}, appErr
	}
	counterpartyID := order.SellerUserID
	if input.ActorUserID == order.SellerUserID {
		counterpartyID = order.BuyerUserID
	}
	item, err := scanDispute(ctx, tx, `
		INSERT INTO dispute_cases (
			report_id, target_type, target_id, target_label, primary_user_id, counterparty_user_id,
			subject_user_id,
			status, issue_code, requested_resolution, requested_amount_cny, issue_occurred_at,
			public_summary, public_result_code, public_result, admin_reason, opened_by_admin_id, opened_at,
			created_at, updated_at, version
		)
		VALUES (NULL, $1, $2, $3, $4, $5, $5, 'negotiating', $6, $7, $8, $9, $10, $11, $12, $13, $4, $14, $14, $14, 1)
		RETURNING `+disputeReturningColumns+`
	`, report.TargetAPIOrder, order.ID, strings.TrimSpace(order.ServiceTitleSnapshot), input.ActorUserID, counterpartyID,
		strings.TrimSpace(input.IssueCode), strings.TrimSpace(input.RequestedResolution), nullNumeric(input.RequestedAmountCNY), issueOccurredAt,
		"API 订单纠纷", report.PublicResultNoAction, "双方协商中", "", now)
	if err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_order_dispute_messages (
			id, dispute_case_id, sender_user_id, body, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, uuid.NewString(), item.ID, input.ActorUserID, strings.TrimSpace(input.Reason), strings.TrimSpace(input.RequestID), now); err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	return item, nil
}

func storeFindPaymentOption(service apimarket.Service, method string) (apimarket.PaymentOption, bool) {
	if !apimarket.IsSupportedPaymentMethod(method) {
		return apimarket.PaymentOption{}, false
	}
	for _, option := range service.PaymentOptions {
		if option.Enabled && apimarket.IsSupportedPaymentMethod(option.PaymentMethod) && option.PaymentMethod == method {
			return option, true
		}
	}
	return apimarket.PaymentOption{}, false
}

func storeResolveAPIOrderAmount(intent apiintent.Intent, service apimarket.Service) (string, string, *domain.AppError) {
	switch service.BillingMode {
	case apimarket.ServiceBillingModeFixedPackage:
		pack, ok := storeFindAPIServicePackage(service, intent.SelectedPackageID)
		if !ok || !pack.Enabled {
			return "", "", domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Package invalid", "选择的套餐不可用。", "selectedPackageId", "invalid", "选择的套餐不可用。")
		}
		return storeDecimalStringOptional(pack.PriceCNY, 2), "CNY", nil
	case apimarket.ServiceBillingModeMetered:
		return storeDecimalStringOptional(intent.RequestedCNYAmount, 2), "CNY", nil
	case apimarket.ServiceBillingModeManual:
		return "", "", domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Seller quote required", "自定义需求必须先由商户给出固定报价。", "intentId", "quote_required", "必须先完成商户报价。")
	default:
		return "", "", domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 服务计费方式不可下单。")
	}
}

func storeCanActorAccessAPIOrder(order apiorder.Order, actorUserID, action string) bool {
	switch action {
	case "submit_payment", "cancel", "confirm_complete", "report_late_payment":
		return order.BuyerUserID == actorUserID
	case "confirm_payment", "report_payment_issue", "submit_delivery", "resolve_late_payment":
		return order.SellerUserID == actorUserID
	case "open_dispute":
		return order.BuyerUserID == actorUserID || order.SellerUserID == actorUserID
	default:
		return false
	}
}

func storeCanTransitionAPIOrder(order apiorder.Order, action string, now time.Time) bool {
	if action != "open_dispute" && apiorder.IsDisputeActive(order.DisputeStatus) {
		return false
	}
	switch action {
	case "submit_payment":
		return (order.Status == apiorder.StatusPendingPayment && now.Before(order.PaymentExpiresAt)) || order.Status == apiorder.StatusPaymentIssue
	case "cancel":
		return order.Status == apiorder.StatusPendingPayment
	case "confirm_payment":
		return order.Status == apiorder.StatusPaymentSubmitted
	case "report_payment_issue":
		return order.Status == apiorder.StatusPaymentSubmitted
	case "submit_delivery":
		return order.Status == apiorder.StatusPaidConfirmed
	case "confirm_complete":
		return order.Status == apiorder.StatusDeliverySubmitted
	case "open_dispute":
		return apiorder.WithAfterSalesProjection(order, now).CanOpenDispute
	case "report_late_payment":
		return apiorder.WithAfterSalesProjection(order, now).CanReportLatePayment
	case "resolve_late_payment":
		return order.Status == apiorder.StatusCancelled && order.CancelReason == "payment_timeout" && order.LatePaymentStatus == apiorder.LatePaymentStatusReported
	default:
		return false
	}
}

func storeValidateAPIOrderActionInput(input apiorder.ActionInput, action string) *domain.AppError {
	switch action {
	case "submit_payment":
		if strings.TrimSpace(input.PaymentSummary) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment summary required", "必须填写付款摘要。", "paymentSummary", "required", "必须填写付款摘要。")
		}
		return storeValidateOptionalNonSecretText("paymentSummary", input.PaymentSummary)
	case "report_payment_issue":
		if !apiorder.IsPaymentIssueReason(input.PaymentIssueReason) {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment issue reason invalid", "请选择有效的付款问题。", "paymentIssueReason", "invalid", "请选择未到账、金额不符或备注不符。")
		}
		return storeValidateOptionalNonSecretText("paymentIssueNote", input.PaymentIssueNote)
	case "report_late_payment":
		return storeValidateOptionalNonSecretText("note", input.LatePaymentNote)
	case "resolve_late_payment":
		if input.LatePaymentStatus != apiorder.LatePaymentStatusNotReceived && input.LatePaymentStatus != apiorder.LatePaymentStatusReceivedRefundPending {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Late payment resolution invalid", "请选择未到账或已到账待退款。", "status", "invalid", "请选择有效的处理结果。")
		}
		return storeValidateOptionalNonSecretText("note", input.LatePaymentNote)
	case "submit_delivery":
		if _, err := apiorder.NormalizeDeliveryCredentialForStore(input.DeliveryCredential); err != nil {
			return err
		}
		return nil
	case "cancel", "open_dispute":
		if strings.TrimSpace(input.Reason) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "必须填写原因。", "reason", "required", "必须填写原因。")
		}
		return storeValidateOptionalNonSecretText("reason", input.Reason)
	default:
		return nil
	}
}

func storeApplyAPIOrderAction(order apiorder.Order, input apiorder.ActionInput, action string, now time.Time) apiorder.Order {
	switch action {
	case "submit_payment":
		order.Status = apiorder.StatusPaymentSubmitted
		order.PaymentSummary = strings.TrimSpace(input.PaymentSummary)
		order.PaymentSubmittedAt = &now
		merchantConfirmDueAt := now.Add(apiorder.MerchantConfirmWindow)
		order.MerchantConfirmDueAt = &merchantConfirmDueAt
		order.PaymentIssueReason = ""
		order.PaymentIssueNote = ""
		order.PaymentIssueReportedAt = nil
	case "report_payment_issue":
		order.Status = apiorder.StatusPaymentIssue
		order.PaymentIssueReason = strings.TrimSpace(input.PaymentIssueReason)
		order.PaymentIssueNote = strings.TrimSpace(input.PaymentIssueNote)
		order.PaymentIssueReportedAt = &now
	case "cancel":
		order.Status = apiorder.StatusCancelled
		order.CancelReason = strings.TrimSpace(input.Reason)
		order.CancelledAt = &now
	case "confirm_payment":
		order.Status = apiorder.StatusPaidConfirmed
		order.PaidConfirmedAt = &now
		deliveryDueAt := now.Add(apiorder.DeliveryWindow(order))
		order.DeliveryDueAt = &deliveryDueAt
		order.PackageStockReserved = false
	case "submit_delivery":
		order.Status = apiorder.StatusDeliverySubmitted
		order.DeliveryNote = apiorder.DeliverySummary(input.DeliveryCredential.DeliveryKind)
		order.DeliverySubmittedAt = &now
		reviewExpiresAt := now.Add(apiorder.DeliveryReviewWindow)
		order.DeliveryReviewExpiresAt = &reviewExpiresAt
	case "confirm_complete":
		order.Status = apiorder.StatusCompleted
		order.CompletionSource = apiorder.CompletionSourceBuyerConfirmed
		order.CompletedAt = &now
	case "open_dispute":
		order.DisputeStatus = apiorder.DisputeStatusNegotiating
	case "report_late_payment":
		order.LatePaymentStatus = apiorder.LatePaymentStatusReported
		order.LatePaymentReportedAt = &now
		order.LatePaymentNote = strings.TrimSpace(input.LatePaymentNote)
	case "resolve_late_payment":
		order.LatePaymentStatus = input.LatePaymentStatus
		order.LatePaymentResolvedAt = &now
		order.LatePaymentNote = strings.TrimSpace(input.LatePaymentNote)
	}
	order.UpdatedAt = now
	order.Version++
	return order
}

func storeAPIOrderEventType(action string) string {
	switch action {
	case "submit_payment":
		return apiorder.EventPaymentSubmitted
	case "cancel":
		return apiorder.EventCancelled
	case "confirm_payment":
		return apiorder.EventPaymentConfirmed
	case "report_payment_issue":
		return apiorder.EventPaymentIssueReported
	case "submit_delivery":
		return apiorder.EventDeliverySubmitted
	case "confirm_complete":
		return apiorder.EventCompleted
	case "open_dispute":
		return apiorder.EventDisputeOpened
	case "report_late_payment":
		return apiorder.EventLatePaymentReported
	case "resolve_late_payment":
		return apiorder.EventLatePaymentResolved
	default:
		return "api_order.updated"
	}
}

func storeAPIOrderActionNote(input apiorder.ActionInput, action string) string {
	switch action {
	case "submit_payment":
		return input.PaymentSummary
	case "submit_delivery":
		return apiorder.DeliverySummary(input.DeliveryCredential.DeliveryKind)
	case "report_payment_issue":
		return apiorder.PaymentIssueLabel(input.PaymentIssueReason) + paymentIssueStoreNoteSuffix(input.PaymentIssueNote)
	case "cancel", "open_dispute":
		return input.Reason
	case "report_late_payment", "resolve_late_payment":
		return input.LatePaymentNote
	default:
		return ""
	}
}

func paymentIssueStoreNoteSuffix(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	return "：" + note
}

func apiOrderNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API order not found", "订单不存在。")
}

func nullInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
