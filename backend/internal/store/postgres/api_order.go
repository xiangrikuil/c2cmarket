package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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
	return s.listAPIOrders(ctx, `WHERE buyer_user_id = $1`, []any{buyerUserID})
}

func (s *Store) GetAPIOrderForBuyer(ctx context.Context, buyerUserID, orderID string, now time.Time) (apiorder.Order, *domain.AppError) {
	if appErr := s.materializeExpiredAPIOrder(ctx, s.pool, orderID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	order, err := s.getAPIOrder(ctx, s.pool, orderID, false)
	if errors.Is(err, pgx.ErrNoRows) || order.BuyerUserID != buyerUserID {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	return order, nil
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
	order, err := s.getAPIOrder(ctx, tx, orderID, true)
	if errors.Is(err, pgx.ErrNoRows) || order.BuyerUserID != buyerUserID {
		return apiorder.PaymentInstructionsView{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.PaymentInstructionsView{}, internalStoreError()
	}
	if order.Status != apiorder.StatusPendingPayment || !now.Before(order.PaymentExpiresAt) {
		return apiorder.PaymentInstructionsView{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前订单不再是有效付款入口。")
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
	return s.listAPIOrders(ctx, `WHERE seller_user_id = $1`, []any{sellerUserID})
}

func (s *Store) ListAdminAPIOrders(ctx context.Context, now time.Time) ([]apiorder.Order, *domain.AppError) {
	if appErr := s.MaterializeExpiredAPIOrders(ctx, now); appErr != nil {
		return nil, appErr
	}
	return s.listAPIOrders(ctx, "", nil)
}

func (s *Store) GetAPIOrderForSeller(ctx context.Context, sellerUserID, orderID string, now time.Time) (apiorder.Order, *domain.AppError) {
	if appErr := s.materializeExpiredAPIOrder(ctx, s.pool, orderID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	order, err := s.getAPIOrder(ctx, s.pool, orderID, false)
	if errors.Is(err, pgx.ErrNoRows) || order.SellerUserID != sellerUserID {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	return order, nil
}

func (s *Store) createAPIOrderInTx(ctx context.Context, tx pgx.Tx, input apiorder.CreateInput, now time.Time) (apiorder.Order, *domain.AppError) {
	intent, err := s.getAPIPurchaseIntent(ctx, tx, input.IntentID, true)
	if errors.Is(err, pgx.ErrNoRows) || intent.BuyerUserID != input.BuyerUserID {
		return apiorder.Order{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
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
	service = apimarket.WithOrderability(service)
	order, appErr := newStoreAPIOrder(input, intent, service, now)
	if appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := reserveAPIOrderInventoryInTx(ctx, tx, order, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := insertAPIOrderInTx(ctx, tx, order); appErr != nil {
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
	return order, nil
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
	order, err := s.getAPIOrder(ctx, tx, input.OrderID, true)
	if errors.Is(err, pgx.ErrNoRows) || !storeCanActorAccessAPIOrder(order, input.ActorUserID, action) {
		return apiorder.Order{}, apiOrderNotFound()
	}
	if err != nil {
		return apiorder.Order{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && order.Version != input.ExpectedVersion {
		return apiorder.Order{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !storeCanTransitionAPIOrder(order, action, now) {
		return apiorder.Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前订单状态不能执行该操作。")
	}
	if action == "submit_delivery" {
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
		if appErr := releaseAPIOrderInventoryInTx(ctx, tx, order, now); appErr != nil {
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
	order = storeApplyAPIOrderAction(order, input, action, now)
	if appErr := updateAPIOrderInTx(ctx, tx, order); appErr != nil {
		return apiorder.Order{}, appErr
	}
	eventType := storeAPIOrderEventType(action)
	if appErr := insertAPIOrderEventInTx(ctx, tx, order, input.ActorUserID, eventType, from, order.Status, storeAPIOrderActionNote(input, action), input.RequestID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, input.ActorUserID, eventType, input.RequestID, now); appErr != nil {
		return apiorder.Order{}, appErr
	}
	if action != "submit_delivery" && order.DeliverySubmittedAt != nil {
		if appErr := s.attachAPIOrderDeliveryCredential(ctx, tx, &order); appErr != nil {
			return apiorder.Order{}, appErr
		}
	}
	return order, nil
}

func (s *Store) MaterializeExpiredAPIOrders(ctx context.Context, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text
		FROM api_orders
		WHERE status = 'pending_payment' AND payment_expires_at <= $1
	`, now)
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

func (s *Store) materializeExpiredAPIOrder(ctx context.Context, q queryer, orderID string, now time.Time) *domain.AppError {
	if tx, ok := q.(pgx.Tx); ok {
		return s.materializeExpiredAPIOrderInTx(ctx, tx, orderID, now)
	}
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := s.materializeExpiredAPIOrderInTx(ctx, tx, orderID, now); appErr != nil {
		return appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) materializeExpiredAPIOrderInTx(ctx context.Context, tx pgx.Tx, orderID string, now time.Time) *domain.AppError {
	order, err := s.getAPIOrder(ctx, tx, orderID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	if order.Status != apiorder.StatusPendingPayment || order.PaymentExpiresAt.After(now) {
		return nil
	}
	if appErr := releaseAPIOrderInventoryInTx(ctx, tx, order, now); appErr != nil {
		return appErr
	}
	order.Status = apiorder.StatusCancelled
	order.CancelReason = apiorder.CancelReasonPaymentTimeout
	order.CancelledAt = &now
	order.PackageStockReserved = false
	order.UpdatedAt = now
	order.Version++
	if appErr := updateAPIOrderInTx(ctx, tx, order); appErr != nil {
		return appErr
	}
	if appErr := insertAPIOrderEventInTx(ctx, tx, order, "", apiorder.EventPaymentTimeoutCancelled, apiorder.StatusPendingPayment, apiorder.StatusCancelled, "", "payment-timeout", now); appErr != nil {
		return appErr
	}
	return insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, "", apiorder.EventPaymentTimeoutCancelled, "payment-timeout", now)
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

func releaseAPIOrderInventoryInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, now time.Time) *domain.AppError {
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

const apiOrderColumns = `
	id::text, api_purchase_intent_id::text, api_service_id::text,
	buyer_user_id::text, seller_user_id::text, status, dispute_status,
	COALESCE(dispute_case_id::text, ''), service_title_snapshot,
	service_version_snapshot, billing_mode_snapshot, COALESCE(selected_package_id::text, ''),
		COALESCE(selected_package_snapshot::text, ''), COALESCE(quote_version_snapshot, 0),
		COALESCE(requested_usd_allowance_snapshot::text, ''), COALESCE(cny_per_usd_allowance_snapshot::text, ''), pricing_snapshot::text,
		package_stock_reserved, package_expires_at,
		amount::text, currency, selected_payment_method,
	payment_window_minutes_snapshot, payment_expires_at, payment_instructions_snapshot,
		COALESCE(payment_qr_code_data_url_snapshot, ''), COALESCE(payment_summary, ''), payment_submitted_at,
		COALESCE(payment_issue_reason, ''), COALESCE(payment_issue_note, ''), payment_issue_reported_at, paid_confirmed_at,
	COALESCE(delivery_note, ''), delivery_submitted_at, completed_at,
	cancelled_at, COALESCE(cancel_reason, ''), created_at, updated_at, version
`

func (s *Store) listAPIOrders(ctx context.Context, whereClause string, args []any) ([]apiorder.Order, *domain.AppError) {
	query := `SELECT ` + apiOrderColumns + ` FROM api_orders `
	if strings.TrimSpace(whereClause) != "" {
		query += whereClause
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
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

func (s *Store) getAPIOrder(ctx context.Context, q queryer, orderID string, forUpdate bool) (apiorder.Order, error) {
	query := `SELECT ` + apiOrderColumns + ` FROM api_orders WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var order apiorder.Order
	err := q.QueryRow(ctx, query, orderID).Scan(apiOrderScanTargets(&order)...)
	if err == nil && !forUpdate {
		if appErr := s.attachAPIOrderDeliveryCredential(ctx, q, &order); appErr != nil {
			return apiorder.Order{}, errors.New(appErr.Detail)
		}
	}
	return order, err
}

func apiOrderScanTargets(order *apiorder.Order) []any {
	return []any{
		&order.ID,
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
		&order.PackageStockReserved,
		&order.PackageExpiresAt,
		&order.Amount,
		&order.Currency,
		&order.SelectedPaymentMethod,
		&order.PaymentWindowMinutesSnapshot,
		&order.PaymentExpiresAt,
		&order.PaymentInstructionsSnapshot,
		&order.PaymentQRCodeDataURLSnapshot,
		&order.PaymentSummary,
		&order.PaymentSubmittedAt,
		&order.PaymentIssueReason,
		&order.PaymentIssueNote,
		&order.PaymentIssueReportedAt,
		&order.PaidConfirmedAt,
		&order.DeliveryNote,
		&order.DeliverySubmittedAt,
		&order.CompletedAt,
		&order.CancelledAt,
		&order.CancelReason,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.Version,
	}
}

func newStoreAPIOrder(input apiorder.CreateInput, intent apiintent.Intent, service apimarket.Service, now time.Time) (apiorder.Order, *domain.AppError) {
	if !apimarket.IsOrderableService(service) {
		return apiorder.Order{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Service not orderable", "当前 API 服务不可下单。")
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

func insertAPIOrderInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO api_orders (
			id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
			status, dispute_status, dispute_case_id, service_title_snapshot,
			service_version_snapshot, billing_mode_snapshot, selected_package_id,
				selected_package_snapshot, quote_version_snapshot,
				requested_usd_allowance_snapshot, cny_per_usd_allowance_snapshot, pricing_snapshot,
				package_stock_reserved, package_expires_at,
				amount, currency,
				selected_payment_method, payment_window_minutes_snapshot, payment_expires_at,
				payment_instructions_snapshot, payment_qr_code_data_url_snapshot, payment_summary, payment_submitted_at,
				payment_issue_reason, payment_issue_note, payment_issue_reported_at,
				paid_confirmed_at, delivery_note, delivery_submitted_at, completed_at,
				cancelled_at, cancel_reason, created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12,
			$13, $14,
			$15, $16, $17,
				$18, $19,
				$20, $21,
				$22, $23, $24,
				$25, $26, $27, $28,
				$29, $30, $31,
				$32, $33, $34, $35,
				$36, $37, $38, $39, $40
		)
	`, order.ID, order.APIPurchaseIntentID, order.APIServiceID, order.BuyerUserID, order.SellerUserID,
		order.Status, order.DisputeStatus, nullUUID(order.DisputeCaseID), order.ServiceTitleSnapshot,
		order.ServiceVersionSnapshot, order.BillingModeSnapshot, nullUUID(order.SelectedPackageID),
		nullJSON(order.SelectedPackageSnapshot), nullInt64(order.QuoteVersionSnapshot),
		nullNumeric(order.RequestedUSDAllowanceSnapshot), nullNumeric(order.CNYPerUSDAllowanceSnapshot), nullJSON(order.PricingSnapshot),
		order.PackageStockReserved, order.PackageExpiresAt,
		order.Amount, order.Currency,
		order.SelectedPaymentMethod, order.PaymentWindowMinutesSnapshot, order.PaymentExpiresAt,
		order.PaymentInstructionsSnapshot, nullText(order.PaymentQRCodeDataURLSnapshot), nullText(order.PaymentSummary), order.PaymentSubmittedAt,
		nullText(order.PaymentIssueReason), nullText(order.PaymentIssueNote), order.PaymentIssueReportedAt,
		order.PaidConfirmedAt, nullText(order.DeliveryNote), order.DeliverySubmittedAt, order.CompletedAt,
		order.CancelledAt, nullText(order.CancelReason), order.CreatedAt, order.UpdatedAt, order.Version)
	if err != nil {
		if isUniqueViolationOnConstraint(err, "ux_api_orders_intent") {
			return domain.NewAPIPurchaseIntentHasOrderError()
		}
		return internalStoreError()
	}
	return nil
}

func updateAPIOrderInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order) *domain.AppError {
	_, err := tx.Exec(ctx, `
		UPDATE api_orders
		SET status = $2,
		    dispute_status = $3,
			    dispute_case_id = $4,
			    payment_summary = $5,
			    payment_submitted_at = $6,
			    payment_issue_reason = $7,
			    payment_issue_note = $8,
			    payment_issue_reported_at = $9,
			    paid_confirmed_at = $10,
			    delivery_note = $11,
			    delivery_submitted_at = $12,
			    completed_at = $13,
			    cancelled_at = $14,
			    cancel_reason = $15,
			    package_stock_reserved = $16,
			    package_expires_at = $17,
			    updated_at = $18,
			    version = $19
		WHERE id = $1
		`, order.ID, order.Status, order.DisputeStatus, nullUUID(order.DisputeCaseID),
		nullText(order.PaymentSummary), order.PaymentSubmittedAt,
		nullText(order.PaymentIssueReason), nullText(order.PaymentIssueNote), order.PaymentIssueReportedAt, order.PaidConfirmedAt,
		nullText(order.DeliveryNote), order.DeliverySubmittedAt, order.CompletedAt,
		order.CancelledAt, nullText(order.CancelReason), order.PackageStockReserved, order.PackageExpiresAt,
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
	if credential.APIKey != "" {
		encoded, err := s.contactCodec.encode(credential.APIKey)
		if err != nil {
			return apiorder.DeliveryCredential{}, internalStoreError()
		}
		apiKeyCiphertext = encoded.Ciphertext
		apiKeyNonce = encoded.Nonce
		keyVersion = encoded.EncryptionKeyVersion
	}
	if credential.Password != "" {
		encoded, err := s.contactCodec.encode(credential.Password)
		if err != nil {
			return apiorder.DeliveryCredential{}, internalStoreError()
		}
		passwordCiphertext = encoded.Ciphertext
		passwordNonce = encoded.Nonce
		keyVersion = encoded.EncryptionKeyVersion
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO api_order_delivery_credentials (
			id, api_order_id, seller_user_id, buyer_user_id, delivery_kind,
			api_base_url, panel_login_url, username, instructions,
			api_key_ciphertext, api_key_nonce, password_ciphertext, password_nonce,
			secret_encryption_key_version, submitted_at, created_at
		)
		VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16
		)
	`, credential.ID, credential.APIOrderID, credential.SellerUserID, credential.BuyerUserID, credential.DeliveryKind,
		nullText(credential.APIBaseURL), nullText(credential.PanelLoginURL), nullText(credential.Username), nullText(credential.Instructions),
		apiKeyCiphertext, apiKeyNonce, passwordCiphertext, passwordNonce, keyVersion, credential.SubmittedAt, credential.CreatedAt)
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
	err := q.QueryRow(ctx, `
		SELECT id::text, api_order_id::text, seller_user_id::text, buyer_user_id::text,
		       delivery_kind, COALESCE(api_base_url, ''), COALESCE(panel_login_url, ''),
		       COALESCE(username, ''), COALESCE(instructions, ''),
		       api_key_ciphertext, api_key_nonce, password_ciphertext, password_nonce,
		       submitted_at, created_at
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
		&credential.SubmittedAt,
		&credential.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.DeliveryCredential{}, false, nil
	}
	if err != nil {
		return apiorder.DeliveryCredential{}, false, internalStoreError()
	}
	if len(apiKeyCiphertext) > 0 {
		apiKey, err := s.contactCodec.decode(apiKeyCiphertext, apiKeyNonce)
		if err != nil {
			return apiorder.DeliveryCredential{}, false, internalStoreError()
		}
		credential.APIKey = apiKey
	}
	if len(passwordCiphertext) > 0 {
		password, err := s.contactCodec.decode(passwordCiphertext, passwordNonce)
		if err != nil {
			return apiorder.DeliveryCredential{}, false, internalStoreError()
		}
		credential.Password = password
	}
	return credential, true, nil
}

func openDisputeFromAPIOrderInTx(ctx context.Context, tx pgx.Tx, order apiorder.Order, input apiorder.ActionInput, now time.Time) (report.DisputeCase, *domain.AppError) {
	counterpartyID := order.SellerUserID
	if input.ActorUserID == order.SellerUserID {
		counterpartyID = order.BuyerUserID
	}
	item, err := scanDispute(ctx, tx, `
		INSERT INTO dispute_cases (
			report_id, target_type, target_id, target_label, primary_user_id, counterparty_user_id,
			status, public_summary, public_result_code, public_result, admin_reason, opened_by_admin_id, opened_at,
			created_at, updated_at, version
		)
		VALUES (NULL, $1, $2, $3, $4, $5, 'open', $6, $7, $8, $9, $10, $11, $11, $11, 1)
		RETURNING `+disputeReturningColumns+`
	`, report.TargetAPIOrder, order.ID, strings.TrimSpace(order.ServiceTitleSnapshot), input.ActorUserID, counterpartyID,
		"API 订单纠纷", report.PublicResultNoAction, "已进入人工处理中", strings.TrimSpace(input.Reason), input.ActorUserID, now)
	if err != nil {
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
	case "submit_payment", "cancel", "confirm_complete":
		return order.BuyerUserID == actorUserID
	case "confirm_payment", "report_payment_issue", "submit_delivery":
		return order.SellerUserID == actorUserID
	case "open_dispute":
		return order.BuyerUserID == actorUserID || order.SellerUserID == actorUserID
	default:
		return false
	}
}

func storeCanTransitionAPIOrder(order apiorder.Order, action string, now time.Time) bool {
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
		return order.Status != apiorder.StatusCancelled && order.Status != apiorder.StatusCompleted && order.DisputeStatus == apiorder.DisputeStatusNone
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
		order.PackageStockReserved = false
	case "submit_delivery":
		order.Status = apiorder.StatusDeliverySubmitted
		order.DeliveryNote = apiorder.DeliverySummary(input.DeliveryCredential.DeliveryKind)
		order.DeliverySubmittedAt = &now
	case "confirm_complete":
		order.Status = apiorder.StatusCompleted
		order.CompletedAt = &now
	case "open_dispute":
		order.DisputeStatus = apiorder.DisputeStatusOpen
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
