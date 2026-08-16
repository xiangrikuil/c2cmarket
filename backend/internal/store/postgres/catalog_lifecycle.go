package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/jackc/pgx/v5"
)

type catalogLifecycleTarget struct {
	table         string
	aggregateType string
	columns       string
	sourceType    string
	parentCheck   string
	notFound      func() *domain.AppError
}

func (s *Store) AdminApplyLifecycleWithIdempotency(
	ctx context.Context,
	entry idempotency.Entry,
	input catalog.LifecycleActionInput,
	now time.Time,
	buildCompletion catalog.LifecycleCompletionBuilder,
) (catalog.LifecycleMutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || buildCompletion == nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	target, ok := catalogLifecycleTargetFor(input.ResourceType)
	if !ok {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	lockedEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, appErr
	}
	var currentStatus string
	var currentVersion int64
	err = tx.QueryRow(ctx, "SELECT status, version FROM "+target.table+" WHERE id = $1 FOR UPDATE", input.ResourceID).Scan(&currentStatus, &currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, target.notFound()
	}
	if err != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if currentVersion != input.ExpectedVersion {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, catalogLifecycleVersionConflict()
	}
	if appErr := validateCatalogLifecycleTransition(currentStatus, input.Action); appErr != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, appErr
	}
	targetStatus := catalogLifecycleTargetStatus(input.Action, input.TargetStatus)
	if targetStatus == catalog.StatusActive && target.parentCheck != "" {
		var parentActive bool
		if err := tx.QueryRow(ctx, target.parentCheck, input.ResourceID).Scan(&parentActive); err != nil || !parentActive {
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return catalog.LifecycleMutationResult{}, idempotency.Completion{}, internalStoreError()
			}
			return catalog.LifecycleMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Catalog parent unavailable", "父级目录当前不可用，不能恢复该记录。")
		}
	}
	if input.Action == catalog.LifecycleActionUnblock && target.sourceType != "" {
		var activeHolds int
		predicate := "model.model_catalog_id = $1"
		if input.ResourceType == catalog.ResourceAPIProvider {
			predicate = "model_catalog.provider_id = $1"
		}
		if err := tx.QueryRow(ctx, `
			SELECT count(DISTINCT hold.id)
			FROM api_order_catalog_risk_holds hold
			JOIN api_orders orders ON orders.id = hold.api_order_id
			JOIN api_service_models model ON model.api_service_id = orders.api_service_id
			JOIN api_model_catalog model_catalog ON model_catalog.id = model.model_catalog_id
			WHERE `+predicate+` AND hold.status = 'active'
		`, input.ResourceID).Scan(&activeHolds); err != nil {
			return catalog.LifecycleMutationResult{}, idempotency.Completion{}, internalStoreError()
		}
		if activeHolds > 0 {
			return catalog.LifecycleMutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Catalog holds unresolved", "仍有订单风险暂停未逐单处置，不能解除阻断。")
		}
	}

	_, err = tx.Exec(ctx, "UPDATE "+target.table+` SET status = $2, status_changed_at = $3,
		status_reason = $4, status_changed_by = $5, version = version + 1, updated_at = $3 WHERE id = $1`,
		input.ResourceID, targetStatus, now, input.Reason, nullUUID(input.OperatorID))
	if err != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if targetStatus == catalog.StatusDeprecated || targetStatus == catalog.StatusBlocked {
		if appErr := closeCatalogPendingObjectsInTx(ctx, tx, s, input, now); appErr != nil {
			return catalog.LifecycleMutationResult{}, idempotency.Completion{}, appErr
		}
	}
	if targetStatus == catalog.StatusBlocked && target.sourceType != "" {
		if appErr := createCatalogRiskHoldsInTx(ctx, tx, s, input, target.sourceType, now); appErr != nil {
			return catalog.LifecycleMutationResult{}, idempotency.Completion{}, appErr
		}
	}

	result, appErr := readCatalogLifecycleResultInTx(ctx, tx, input.ResourceType, input.ResourceID)
	if appErr != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := insertCatalogLifecycleAuditInTx(ctx, tx, input, currentStatus, targetStatus, result, now); appErr != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, lockedEntry, completion, now); appErr != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.LifecycleMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func catalogLifecycleTargetFor(resourceType string) (catalogLifecycleTarget, bool) {
	switch resourceType {
	case catalog.ResourceProductCategory:
		return catalogLifecycleTarget{table: "product_categories", aggregateType: resourceType, notFound: productCategoryNotFound}, true
	case catalog.ResourceProductPlan:
		return catalogLifecycleTarget{table: "product_plans", aggregateType: resourceType, parentCheck: `SELECT category.status = 'active' FROM product_plans item JOIN product_categories category ON category.id = item.category_id WHERE item.id = $1`, notFound: productPlanNotFound}, true
	case catalog.ResourceAPIProvider:
		return catalogLifecycleTarget{table: "api_model_providers", aggregateType: resourceType, sourceType: catalog.ResourceAPIProvider, notFound: apiModelProviderNotFound}, true
	case catalog.ResourceAPIModel:
		return catalogLifecycleTarget{table: "api_model_catalog", aggregateType: resourceType, sourceType: catalog.ResourceAPIModel, parentCheck: `SELECT provider.status = 'active' FROM api_model_catalog item JOIN api_model_providers provider ON provider.id = item.provider_id WHERE item.id = $1`, notFound: apiModelNotFound}, true
	default:
		return catalogLifecycleTarget{}, false
	}
}

func validateCatalogLifecycleTransition(current, action string) *domain.AppError {
	allowed := (action == catalog.LifecycleActionDeprecate && current == catalog.StatusActive) ||
		(action == catalog.LifecycleActionBlock && (current == catalog.StatusActive || current == catalog.StatusDeprecated)) ||
		(action == catalog.LifecycleActionReactivate && current == catalog.StatusDeprecated) ||
		(action == catalog.LifecycleActionUnblock && current == catalog.StatusBlocked)
	if !allowed {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Catalog lifecycle transition invalid", "当前目录状态不允许执行该动作。")
	}
	return nil
}

func catalogLifecycleTargetStatus(action, unblockTarget string) string {
	switch action {
	case catalog.LifecycleActionDeprecate:
		return catalog.StatusDeprecated
	case catalog.LifecycleActionBlock:
		return catalog.StatusBlocked
	case catalog.LifecycleActionReactivate:
		return catalog.StatusActive
	case catalog.LifecycleActionUnblock:
		return unblockTarget
	default:
		return ""
	}
}

func closeCatalogPendingObjectsInTx(ctx context.Context, tx pgx.Tx, store *Store, input catalog.LifecycleActionInput, now time.Time) *domain.AppError {
	reason := "关联目录已不可用：" + input.Reason
	if input.ResourceType == catalog.ResourceProductCategory || input.ResourceType == catalog.ResourceProductPlan {
		predicate := "listing.product_plan_id = $1"
		if input.ResourceType == catalog.ResourceProductCategory {
			predicate = "plan.category_id = $1"
		}
		rows, err := tx.Query(ctx, `
			UPDATE carpool_listings listing
			SET governance_status = 'removed', review_reason = $3,
			    updated_at = $2, version = version + 1
			FROM product_plans plan
			WHERE plan.id = listing.product_plan_id AND `+predicate+`
			RETURNING listing.id::text
		`, input.ResourceID, now, reason)
		if err != nil {
			return internalStoreError()
		}
		listingIDs := make([]string, 0)
		for rows.Next() {
			var listingID string
			if err := rows.Scan(&listingID); err != nil {
				rows.Close()
				return internalStoreError()
			}
			listingIDs = append(listingIDs, listingID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return internalStoreError()
		}
		rows.Close()
		for _, listingID := range listingIDs {
			listing, err := store.getCarpoolListing(ctx, tx, listingID, false, false)
			if err != nil {
				return internalStoreError()
			}
			if appErr := insertCarpoolListingEvent(ctx, tx, listing, input.OperatorID, "admin", "carpool_listing.catalog_removed", input.RequestID, now); appErr != nil {
				return appErr
			}
		}
		return nil
	}

	predicate := "model.model_catalog_id = $1"
	if input.ResourceType == catalog.ResourceAPIProvider {
		predicate = "catalog.provider_id = $1"
	}
	rows, err := tx.Query(ctx, `
		UPDATE api_purchase_intents intent
		SET status = 'owner_closed', owner_closed_at = $2, owner_close_reason = $3,
		    updated_at = $2, version = version + 1
		WHERE intent.status IN ('open', 'contacted') AND EXISTS (
		  SELECT 1 FROM api_service_models model
		  JOIN api_model_catalog catalog ON catalog.id = model.model_catalog_id
			  WHERE model.api_service_id = intent.api_service_id AND `+predicate+`
			)
		RETURNING intent.id::text
	`, input.ResourceID, now, reason)
	if err != nil {
		return internalStoreError()
	}
	intentIDs := make([]string, 0)
	for rows.Next() {
		var intentID string
		if err := rows.Scan(&intentID); err != nil {
			rows.Close()
			return internalStoreError()
		}
		intentIDs = append(intentIDs, intentID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return internalStoreError()
	}
	rows.Close()
	for _, intentID := range intentIDs {
		intent, err := store.getAPIPurchaseIntent(ctx, tx, intentID, false)
		if err != nil {
			return internalStoreError()
		}
		if appErr := insertAPIPurchaseIntentEventAndTargetNotification(
			ctx, tx, intent, input.OperatorID, intent.BuyerUserID,
			"api_purchase_intent.catalog_closed", "目录变更，购买意向已关闭", reason,
			input.RequestID, now,
		); appErr != nil {
			return appErr
		}
	}
	return nil
}

func createCatalogRiskHoldsInTx(ctx context.Context, tx pgx.Tx, store *Store, input catalog.LifecycleActionInput, sourceType string, now time.Time) *domain.AppError {
	predicate := "model.model_catalog_id = $1"
	if input.ResourceType == catalog.ResourceAPIProvider {
		predicate = "catalog.provider_id = $1"
	}
	rows, err := tx.Query(ctx, `
		SELECT orders.id::text
		FROM api_orders orders
		JOIN api_service_models model ON model.api_service_id = orders.api_service_id
		JOIN api_model_catalog catalog ON catalog.id = model.model_catalog_id
		WHERE `+predicate+` AND orders.status NOT IN ('completed', 'cancelled')
		ORDER BY orders.id
		FOR UPDATE OF orders
	`, input.ResourceID)
	if err != nil {
		return internalStoreError()
	}
	orderIDs := make([]string, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var orderID string
		if err := rows.Scan(&orderID); err != nil {
			rows.Close()
			return internalStoreError()
		}
		if _, ok := seen[orderID]; ok {
			continue
		}
		seen[orderID] = struct{}{}
		orderIDs = append(orderIDs, orderID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return internalStoreError()
	}
	rows.Close()
	for _, orderID := range orderIDs {
		var holdID string
		err := tx.QueryRow(ctx, `
			INSERT INTO api_order_catalog_risk_holds (
			  api_order_id, source_type, source_id, reason, created_by, created_at
			) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (api_order_id) WHERE status = 'active' DO NOTHING
			RETURNING id::text
		`, orderID, sourceType, input.ResourceID, input.Reason, nullUUID(input.OperatorID), now).Scan(&holdID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return internalStoreError()
		}
		order, err := store.getAPIOrder(ctx, tx, orderID, false, false)
		if err != nil {
			return internalStoreError()
		}
		requestID := "catalog-risk-hold:" + holdID
		if appErr := insertAPIOrderEventInTx(ctx, tx, order, input.OperatorID, apiorder.EventCatalogRiskHoldCreated, order.Status, order.Status, "", requestID, now); appErr != nil {
			return appErr
		}
		if appErr := insertAPIOrderCatalogRiskNotificationInTx(ctx, tx, order, holdID, apiorder.EventCatalogRiskHoldCreated, now); appErr != nil {
			return appErr
		}
	}
	return nil
}

func readCatalogLifecycleResultInTx(ctx context.Context, tx pgx.Tx, resourceType, resourceID string) (catalog.LifecycleMutationResult, *domain.AppError) {
	result := catalog.LifecycleMutationResult{ResourceType: resourceType}
	switch resourceType {
	case catalog.ResourceProductCategory:
		var item catalog.ProductCategory
		if err := scanProductCategory(tx.QueryRow(ctx, `SELECT `+productCategoryColumns+` FROM product_categories WHERE id = $1`, resourceID), &item); err != nil {
			return result, internalStoreError()
		}
		result.Category = &item
	case catalog.ResourceProductPlan:
		var item catalog.ProductPlan
		if err := scanProductPlan(tx.QueryRow(ctx, `SELECT `+productPlanColumns+` FROM product_plans p JOIN product_categories c ON c.id = p.category_id WHERE p.id = $1`, resourceID), &item); err != nil {
			return result, internalStoreError()
		}
		result.Plan = &item
	case catalog.ResourceAPIProvider:
		var item catalog.APIModelProvider
		if err := scanAPIModelProvider(tx.QueryRow(ctx, `SELECT `+apiModelProviderColumns+` FROM api_model_providers WHERE id = $1`, resourceID), &item); err != nil {
			return result, internalStoreError()
		}
		result.Provider = &item
	case catalog.ResourceAPIModel:
		item, appErr := getAPIModelInTx(ctx, tx, resourceID)
		if appErr != nil {
			return result, appErr
		}
		result.Model = &item
	default:
		return result, internalStoreError()
	}
	return result, nil
}

func insertCatalogLifecycleAuditInTx(ctx context.Context, tx pgx.Tx, input catalog.LifecycleActionInput, beforeStatus, afterStatus string, result catalog.LifecycleMutationResult, now time.Time) *domain.AppError {
	metadata, err := json.Marshal(map[string]any{"action": input.Action, "beforeStatus": beforeStatus, "afterStatus": afterStatus, "reason": input.Reason})
	if err != nil {
		return internalStoreError()
	}
	version := input.ExpectedVersion + 1
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind, aggregate_version, request_id, metadata_json, created_at)
		VALUES ($1, $2, $3, $4, 'admin', $5, $6, $7::jsonb, $8)
	`, input.ResourceType, input.ResourceID, "catalog.lifecycle."+input.Action, input.OperatorID, version, input.RequestID, string(metadata), now)
	if err != nil {
		return internalStoreError()
	}
	beforeJSON, _ := json.Marshal(map[string]any{"status": beforeStatus, "version": input.ExpectedVersion})
	afterJSON, _ := json.Marshal(map[string]any{"status": afterStatus, "version": version})
	_, err = tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, reason, before_json, after_json, request_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9)
	`, input.OperatorID, "catalog.lifecycle."+input.Action, input.ResourceType, result.ResourceID(), input.Reason, string(beforeJSON), string(afterJSON), input.RequestID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func catalogLifecycleVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "目录版本已变化，请刷新后重试。")
}
