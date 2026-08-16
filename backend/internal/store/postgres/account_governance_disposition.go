package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	accountGovernanceCancelReason = "ACCOUNT_GOVERNANCE_CANCELLED"
	accountGovernanceJobLease     = 2 * time.Minute
)

var accountGovernancePhaseResources = map[string][]string{
	"sales":       {"api_service", "api_quota_batch", "api_quota_offer", "api_service_promotion", "carpool_listing"},
	"api_orders":  {"api_order"},
	"api_intents": {"api_purchase_intent"},
	"carpool":     {"carpool_application", "carpool_membership"},
}

type accountGovernanceJob struct {
	ID                 string
	TargetUserID       string
	GovernanceActionID string
	Phase              string
	CursorResourceType string
	CursorID           string
	EffectiveAt        time.Time
}

type accountGovernanceDispositionInput struct {
	ResourceType         string
	ResourceID           string
	Result               string
	TriggerRole          string
	BeforeStatus         string
	AfterStatus          string
	ReleasedResourceType string
	ReleasedQuantity     any
	PaymentClaimDeadline *time.Time
}

func (s *Store) processAccountGovernanceDispositionJobInTx(ctx context.Context, tx pgx.Tx, now time.Time, batchSize int) (int64, int64, error) {
	job, found, err := claimAccountGovernanceJobInTx(ctx, tx, now)
	if err != nil || !found {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT account_governance_disposition_batch`); err != nil {
		return 0, 0, err
	}

	processed, completed, processErr := s.processClaimedAccountGovernanceJobInTx(ctx, tx, job, now, batchSize)
	if processErr == nil {
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT account_governance_disposition_batch`); err != nil {
			return 0, 0, err
		}
		return processed, completed, nil
	}

	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT account_governance_disposition_batch`); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE account_governance_jobs
		SET status = 'failed', available_at = $2::timestamptz + interval '1 minute',
		    locked_at = NULL, lease_expires_at = NULL,
		    last_error_code = 'DISPOSITION_BATCH_FAILED',
		    last_error_summary = left($3, 1000), updated_at = $2
		WHERE id = $1
	`, job.ID, now, processErr.Error()); err != nil {
		return 0, 0, err
	}
	return 0, 0, nil
}

func claimAccountGovernanceJobInTx(ctx context.Context, tx pgx.Tx, now time.Time) (accountGovernanceJob, bool, error) {
	var job accountGovernanceJob
	err := tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT job.id
			FROM account_governance_jobs job
			WHERE job.available_at <= $1
			  AND (
			    job.status IN ('pending', 'failed')
			    OR (job.status = 'processing' AND job.lease_expires_at <= $1)
			  )
			ORDER BY job.available_at, job.id
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE account_governance_jobs job
		SET status = 'processing', attempts = attempts + 1,
		    locked_at = $1, lease_expires_at = $1 + make_interval(secs => $2),
		    last_error_code = NULL, last_error_summary = NULL, updated_at = $1
		FROM candidate, account_governance_actions action
		WHERE job.id = candidate.id
		  AND action.id = job.governance_action_id
		RETURNING job.id::text, job.target_user_id::text, job.governance_action_id::text,
		          job.phase, COALESCE(job.cursor_resource_type, ''), COALESCE(job.cursor_id::text, ''),
		          action.effective_at
	`, now, int(accountGovernanceJobLease/time.Second)).Scan(
		&job.ID,
		&job.TargetUserID,
		&job.GovernanceActionID,
		&job.Phase,
		&job.CursorResourceType,
		&job.CursorID,
		&job.EffectiveAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return accountGovernanceJob{}, false, nil
	}
	if err != nil {
		return accountGovernanceJob{}, false, err
	}
	return job, true, nil
}

func (s *Store) processClaimedAccountGovernanceJobInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, now time.Time, batchSize int) (int64, int64, error) {
	resourceTypes, ok := accountGovernancePhaseResources[job.Phase]
	if !ok {
		return 0, 0, fmt.Errorf("unsupported account governance job phase %q", job.Phase)
	}
	resourceType := currentAccountGovernanceResourceType(resourceTypes, job.CursorResourceType)
	if resourceType == "" {
		return advanceAccountGovernanceJobPhaseInTx(ctx, tx, job, now)
	}

	ids, err := accountGovernanceResourceIDs(ctx, tx, job, resourceType, batchSize)
	if err != nil {
		return 0, 0, err
	}
	for _, resourceID := range ids {
		if err := s.disposeAccountGovernanceResourceInTx(ctx, tx, job, resourceType, resourceID, now); err != nil {
			return 0, 0, fmt.Errorf("dispose %s %s: %w", resourceType, resourceID, err)
		}
	}
	if len(ids) == batchSize {
		_, err := tx.Exec(ctx, `
			UPDATE account_governance_jobs
			SET status = 'pending', cursor_resource_type = $2, cursor_id = $3,
			    available_at = $4, locked_at = NULL, lease_expires_at = NULL, updated_at = $4
			WHERE id = $1
		`, job.ID, resourceType, ids[len(ids)-1], now)
		return int64(len(ids)), 0, err
	}

	nextResourceType := nextAccountGovernanceResourceType(resourceTypes, resourceType)
	if nextResourceType != "" {
		_, err := tx.Exec(ctx, `
			UPDATE account_governance_jobs
			SET status = 'pending', cursor_resource_type = $2, cursor_id = NULL,
			    available_at = $3, locked_at = NULL, lease_expires_at = NULL, updated_at = $3
			WHERE id = $1
		`, job.ID, nextResourceType, now)
		return int64(len(ids)), 0, err
	}
	processed, completed, err := advanceAccountGovernanceJobPhaseInTx(ctx, tx, job, now)
	return int64(len(ids)) + processed, completed, err
}

func advanceAccountGovernanceJobPhaseInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, now time.Time) (int64, int64, error) {
	nextPhase := map[string]string{
		"sales":       "api_orders",
		"api_orders":  "api_intents",
		"api_intents": "carpool",
		"carpool":     "completed",
	}[job.Phase]
	if nextPhase == "" {
		return 0, 0, fmt.Errorf("cannot advance account governance phase %q", job.Phase)
	}
	if nextPhase == "completed" {
		_, err := tx.Exec(ctx, `
			UPDATE account_governance_jobs
			SET status = 'completed', phase = 'completed', cursor_resource_type = NULL,
			    cursor_id = NULL, locked_at = NULL, lease_expires_at = NULL,
			    completed_at = $2, updated_at = $2
			WHERE id = $1
		`, job.ID, now)
		return 0, 1, err
	}
	firstResource := accountGovernancePhaseResources[nextPhase][0]
	_, err := tx.Exec(ctx, `
		UPDATE account_governance_jobs
		SET status = 'pending', phase = $2, cursor_resource_type = $3, cursor_id = NULL,
		    available_at = $4, locked_at = NULL, lease_expires_at = NULL, updated_at = $4
		WHERE id = $1
	`, job.ID, nextPhase, firstResource, now)
	return 0, 0, err
}

func nextAccountGovernanceResourceType(resourceTypes []string, current string) string {
	current = strings.TrimSpace(current)
	if current == "" {
		if len(resourceTypes) == 0 {
			return ""
		}
		return resourceTypes[0]
	}
	for index, resourceType := range resourceTypes {
		if resourceType == current && index+1 < len(resourceTypes) {
			return resourceTypes[index+1]
		}
	}
	return ""
}

func currentAccountGovernanceResourceType(resourceTypes []string, current string) string {
	current = strings.TrimSpace(current)
	if current != "" {
		for _, resourceType := range resourceTypes {
			if resourceType == current {
				return current
			}
		}
		return ""
	}
	if len(resourceTypes) == 0 {
		return ""
	}
	return resourceTypes[0]
}

func accountGovernanceResourceIDs(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, resourceType string, batchSize int) ([]string, error) {
	table, participantPredicate, statePredicate, err := accountGovernanceResourceQuery(resourceType)
	if err != nil {
		return nil, err
	}
	query := `SELECT id::text FROM ` + table + ` WHERE ` + participantPredicate
	args := []any{job.TargetUserID}
	nextPlaceholder := 2
	if !accountGovernanceIsSalesResource(resourceType) {
		query += fmt.Sprintf(` AND created_at <= $%d`, nextPlaceholder)
		args = append(args, job.EffectiveAt)
		nextPlaceholder++
	} else if resourceType == "api_service_promotion" {
		args = append(args, job.EffectiveAt)
		nextPlaceholder++
	}
	if statePredicate != "" {
		query += ` AND (` + statePredicate + `)`
	}
	query += fmt.Sprintf(` AND ($%d::uuid IS NULL OR id > $%d::uuid) ORDER BY id LIMIT $%d`, nextPlaceholder, nextPlaceholder, nextPlaceholder+1)
	args = append(args, nullUUID(job.CursorID), batchSize)
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0, batchSize)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func accountGovernanceIsSalesResource(resourceType string) bool {
	switch resourceType {
	case "api_service", "api_quota_batch", "api_quota_offer", "api_service_promotion", "carpool_listing":
		return true
	default:
		return false
	}
}

func accountGovernanceResourceQuery(resourceType string) (string, string, string, error) {
	switch resourceType {
	case "api_service":
		return "api_services", "owner_user_id = $1", "publication_status = 'online' OR governance_disposition_id IS NOT NULL", nil
	case "api_quota_batch":
		return "api_quota_batches", "owner_user_id = $1", "status = 'published' OR governance_disposition_id IS NOT NULL", nil
	case "api_quota_offer":
		return "api_quota_offers", "owner_user_id = $1", "status = 'published' OR governance_disposition_id IS NOT NULL", nil
	case "api_service_promotion":
		return "api_service_promotions", "EXISTS (SELECT 1 FROM api_services service WHERE service.id = api_service_id AND service.owner_user_id = $1)", "(stopped_at IS NULL AND ends_at > $2) OR governance_disposition_id IS NOT NULL", nil
	case "carpool_listing":
		return "carpool_listings", "owner_user_id = $1", "status = 'active' OR governance_disposition_id IS NOT NULL", nil
	case "api_order":
		return "api_orders", "(buyer_user_id = $1 OR seller_user_id = $1)", "", nil
	case "api_purchase_intent":
		return "api_purchase_intents", "(buyer_user_id = $1 OR owner_user_id = $1)", "", nil
	case "carpool_application":
		return "carpool_applications", "(buyer_user_id = $1 OR owner_user_id = $1)", "", nil
	case "carpool_membership":
		return "carpool_memberships", "(buyer_user_id = $1 OR owner_user_id = $1)", "", nil
	default:
		return "", "", "", fmt.Errorf("unsupported account governance resource type %q", resourceType)
	}
}

func (s *Store) disposeAccountGovernanceResourceInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, resourceType, resourceID string, now time.Time) error {
	switch resourceType {
	case "api_service", "api_quota_batch", "api_quota_offer", "api_service_promotion", "carpool_listing":
		return disposeAccountGovernanceSalesResourceInTx(ctx, tx, job, resourceType, resourceID, now)
	case "api_order":
		return s.disposeAccountGovernanceAPIOrderInTx(ctx, tx, job, resourceID, now)
	case "api_purchase_intent":
		return disposeAccountGovernanceAPIIntentInTx(ctx, tx, job, resourceID, now)
	case "carpool_application":
		return disposeAccountGovernanceCarpoolApplicationInTx(ctx, tx, job, resourceID, now)
	case "carpool_membership":
		return disposeAccountGovernanceCarpoolMembershipInTx(ctx, tx, job, resourceID, now)
	default:
		return fmt.Errorf("unsupported account governance resource type %q", resourceType)
	}
}

func loadExistingAccountGovernanceDispositionInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, resourceType, resourceID, triggerRole string, now time.Time) (string, bool, error) {
	var dispositionID string
	err := tx.QueryRow(ctx, `
		SELECT id::text
		FROM account_governance_resource_dispositions
		WHERE resource_type = $1 AND resource_id = $2
		FOR UPDATE
	`, resourceType, resourceID).Scan(&dispositionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if err := linkAccountGovernanceDispositionActionInTx(ctx, tx, dispositionID, job.GovernanceActionID, triggerRole, now); err != nil {
		return "", false, err
	}
	return dispositionID, true, nil
}

func createAccountGovernanceDispositionInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, input accountGovernanceDispositionInput, now time.Time) (string, error) {
	dispositionID := uuid.NewString()
	_, err := tx.Exec(ctx, `
		INSERT INTO account_governance_resource_dispositions (
			id, resource_type, resource_id, result, reason_code, trigger_roles,
			before_status, after_status, released_resource_type, released_quantity,
			governance_effective_at, payment_claim_deadline_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, ARRAY[$6]::text[],
		        $7, $8, $9, $10, $11, $12, $13, $13)
	`, dispositionID, input.ResourceType, input.ResourceID, input.Result, accountGovernanceCancelReason,
		input.TriggerRole, input.BeforeStatus, input.AfterStatus, nullText(input.ReleasedResourceType), input.ReleasedQuantity,
		job.EffectiveAt, input.PaymentClaimDeadline, now)
	if err != nil {
		return "", err
	}
	if err := linkAccountGovernanceDispositionActionInTx(ctx, tx, dispositionID, job.GovernanceActionID, input.TriggerRole, now); err != nil {
		return "", err
	}
	return dispositionID, nil
}

func linkAccountGovernanceDispositionActionInTx(ctx context.Context, tx pgx.Tx, dispositionID, governanceActionID, triggerRole string, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		UPDATE account_governance_resource_dispositions
		SET trigger_roles = ARRAY(
		      SELECT DISTINCT role_name
		      FROM unnest(trigger_roles || ARRAY[$2]::text[]) role_name
		      ORDER BY role_name
		    ),
		    updated_at = GREATEST(updated_at, $3)
		WHERE id = $1
	`, dispositionID, triggerRole, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO account_governance_disposition_actions (
			disposition_id, governance_action_id, trigger_role, linked_at
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (disposition_id, governance_action_id) DO NOTHING
	`, dispositionID, governanceActionID, triggerRole, now)
	return err
}

func accountGovernanceTriggerRole(targetUserID, buyerUserID string) string {
	if targetUserID == buyerUserID {
		return "buyer"
	}
	return "seller"
}

func accountGovernanceHasPreservationFact(ctx context.Context, tx pgx.Tx, resourceType, resourceID string) (bool, error) {
	var preserved bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM dispute_cases dispute
		  WHERE dispute.target_type = $1
		    AND dispute.target_id = $2::uuid::text
		    AND dispute.status IN ('negotiating', 'open', 'waiting_info')
		  UNION ALL
		  SELECT 1
		  FROM moderation_info_requests info_request
		  LEFT JOIN dispute_cases dispute ON dispute.id = info_request.dispute_case_id
		  LEFT JOIN reports report ON report.id = info_request.report_id
		  WHERE info_request.status = 'open'
		    AND (
		      (dispute.target_type = $1 AND dispute.target_id = $2::uuid::text)
		      OR (report.canonical_target_type = $1 AND report.canonical_target_id = $2::uuid::text)
		    )
		  UNION ALL
		  SELECT 1
		  FROM api_order_dispute_remedies remedy
		  JOIN dispute_cases dispute ON dispute.id = remedy.dispute_case_id
		  WHERE remedy.status IN ('pending', 'claimed_fulfilled')
		    AND dispute.target_type = $1
		    AND dispute.target_id = $2::uuid::text
		  UNION ALL
		  SELECT 1
		  FROM appeals appeal
		  LEFT JOIN dispute_cases dispute ON dispute.id = appeal.dispute_case_id
		  LEFT JOIN reports report ON report.id = appeal.report_id
		  WHERE appeal.status = 'submitted'
		    AND (
		      (appeal.target_type = $1 AND appeal.target_id = $2::uuid::text)
		      OR (dispute.target_type = $1 AND dispute.target_id = $2::uuid::text)
		      OR (report.canonical_target_type = $1 AND report.canonical_target_id = $2::uuid::text)
		    )
		)
	`, resourceType, resourceID).Scan(&preserved)
	return preserved, err
}

func disposeAccountGovernanceSalesResourceInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, resourceType, resourceID string, now time.Time) error {
	if _, exists, err := loadExistingAccountGovernanceDispositionInTx(ctx, tx, job, resourceType, resourceID, "seller", now); err != nil || exists {
		return err
	}
	var beforeStatus string
	switch resourceType {
	case "api_service":
		if err := tx.QueryRow(ctx, `SELECT publication_status FROM api_services WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`, resourceID, job.TargetUserID).Scan(&beforeStatus); err != nil {
			return err
		}
	case "api_quota_batch":
		if err := tx.QueryRow(ctx, `SELECT status FROM api_quota_batches WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`, resourceID, job.TargetUserID).Scan(&beforeStatus); err != nil {
			return err
		}
	case "api_quota_offer":
		if err := tx.QueryRow(ctx, `SELECT status FROM api_quota_offers WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`, resourceID, job.TargetUserID).Scan(&beforeStatus); err != nil {
			return err
		}
	case "api_service_promotion":
		if err := tx.QueryRow(ctx, `SELECT CASE WHEN stopped_at IS NULL AND ends_at > $2 THEN 'active' ELSE 'stopped' END FROM api_service_promotions WHERE id = $1 FOR UPDATE`, resourceID, now).Scan(&beforeStatus); err != nil {
			return err
		}
	case "carpool_listing":
		if err := tx.QueryRow(ctx, `SELECT governance_status FROM carpool_listings WHERE id = $1 AND owner_user_id = $2 FOR UPDATE`, resourceID, job.TargetUserID).Scan(&beforeStatus); err != nil {
			return err
		}
	}
	afterStatus := map[string]string{
		"api_service": "owner_paused", "api_quota_batch": "paused", "api_quota_offer": "paused",
		"api_service_promotion": "stopped", "carpool_listing": "removed",
	}[resourceType]
	dispositionID, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{
		ResourceType: resourceType, ResourceID: resourceID, Result: "sales_stopped", TriggerRole: "seller",
		BeforeStatus: beforeStatus, AfterStatus: afterStatus,
	}, now)
	if err != nil {
		return err
	}
	switch resourceType {
	case "api_service":
		_, err = tx.Exec(ctx, `UPDATE api_services SET publication_status = 'owner_paused', governance_disposition_id = $2, updated_at = $3, version = version + 1 WHERE id = $1 AND publication_status = 'online'`, resourceID, dispositionID, now)
	case "api_quota_batch":
		_, err = tx.Exec(ctx, `UPDATE api_quota_batches SET status = 'paused', governance_disposition_id = $2, updated_at = $3, version = version + 1 WHERE id = $1 AND status = 'published'`, resourceID, dispositionID, now)
	case "api_quota_offer":
		_, err = tx.Exec(ctx, `UPDATE api_quota_offers SET status = 'paused', governance_disposition_id = $2, updated_at = $3, version = version + 1 WHERE id = $1 AND status = 'published'`, resourceID, dispositionID, now)
	case "api_service_promotion":
		_, err = tx.Exec(ctx, `UPDATE api_service_promotions SET stopped_at = $3, stopped_by_governance_action_id = $4, stopped_reason = $5, governance_disposition_id = $2, updated_at = $3, version = version + 1 WHERE id = $1 AND stopped_at IS NULL AND ends_at > $3`, resourceID, dispositionID, now, job.GovernanceActionID, accountGovernanceCancelReason)
	case "carpool_listing":
		_, err = tx.Exec(ctx, `UPDATE carpool_listings SET governance_status = 'removed', governance_disposition_id = $2, updated_at = $3, version = version + 1 WHERE id = $1 AND governance_status = 'clear'`, resourceID, dispositionID, now)
	}
	return err
}

func (s *Store) disposeAccountGovernanceAPIOrderInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, orderID string, now time.Time) error {
	order, err := s.getAPIOrder(ctx, tx, orderID, true, false)
	if err != nil {
		return err
	}
	triggerRole := accountGovernanceTriggerRole(job.TargetUserID, order.BuyerUserID)
	if _, exists, err := loadExistingAccountGovernanceDispositionInTx(ctx, tx, job, "api_order", order.ID, triggerRole, now); err != nil || exists {
		return err
	}
	preserved, err := accountGovernanceHasPreservationFact(ctx, tx, "api_order", order.ID)
	if err != nil {
		return err
	}
	result := "preserved"
	if order.Status == apiorder.StatusCancelled || order.Status == apiorder.StatusCompleted {
		result = "already_terminal"
	}
	if order.Status != apiorder.StatusPendingPayment || preserved || result == "already_terminal" {
		_, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{
			ResourceType: "api_order", ResourceID: order.ID, Result: result, TriggerRole: triggerRole,
			BeforeStatus: order.Status, AfterStatus: order.Status,
		}, now)
		return err
	}

	releasedType, releasedQuantity := apiOrderGovernanceRelease(order)
	if appErr := releaseAPIOrderReservationInTx(ctx, tx, order, now); appErr != nil {
		return errors.New(appErr.Detail)
	}
	claimDeadline := job.EffectiveAt.Add(7 * 24 * time.Hour)
	dispositionID, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{
		ResourceType: "api_order", ResourceID: order.ID, Result: "cancelled", TriggerRole: triggerRole,
		BeforeStatus: order.Status, AfterStatus: apiorder.StatusCancelled,
		ReleasedResourceType: releasedType, ReleasedQuantity: releasedQuantity,
		PaymentClaimDeadline: &claimDeadline,
	}, now)
	if err != nil {
		return err
	}
	fromStatus := order.Status
	order.Status = apiorder.StatusCancelled
	order.CancelReason = accountGovernanceCancelReason
	order.CancelledAt = &now
	order.PackageStockReserved = false
	order.UpdatedAt = now
	order.Version++
	if _, err := tx.Exec(ctx, `
		UPDATE api_orders
		SET status = 'cancelled', cancelled_at = $2, cancel_reason = $3,
		    package_stock_reserved = false, governance_disposition_id = $4,
		    governance_cancelled_at = $2, updated_at = $2, version = $5
		WHERE id = $1 AND status = 'pending_payment'
	`, order.ID, now, accountGovernanceCancelReason, dispositionID, order.Version); err != nil {
		return err
	}
	requestID := "account-governance-disposition:" + dispositionID
	if appErr := insertAPIOrderEventInTx(ctx, tx, order, "", apiorder.EventGovernanceCancelled, fromStatus, order.Status, accountGovernanceCancelReason, requestID, now); appErr != nil {
		return errors.New(appErr.Detail)
	}
	return insertAccountGovernanceRelationshipEventAndNotificationsInTx(ctx, tx, dispositionID, "api_order", order.ID, order.Version,
		order.BuyerUserID, order.SellerUserID, "api_order.governance_cancelled", "/my/api-orders/"+order.ID, "/merchant/api-orders/"+order.ID, now)
}

func apiOrderGovernanceRelease(order apiorder.Order) (string, any) {
	if order.PurchaseKind == apiorder.PurchaseKindLimitedQuotaOffer {
		return "quota_inventory_unit", 1
	}
	if order.BillingModeSnapshot == apimarket.ServiceBillingModeFixedPackage && order.PackageStockReserved {
		return "package_stock", 1
	}
	if order.BillingModeSnapshot == apimarket.ServiceBillingModeMetered && strings.TrimSpace(order.RequestedUSDAllowanceSnapshot) != "" {
		return "usd_allowance", order.RequestedUSDAllowanceSnapshot
	}
	return "", nil
}

func disposeAccountGovernanceAPIIntentInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, intentID string, now time.Time) error {
	var buyerID, ownerID, status string
	var version int64
	if err := tx.QueryRow(ctx, `SELECT buyer_user_id::text, owner_user_id::text, status, version FROM api_purchase_intents WHERE id = $1 FOR UPDATE`, intentID).Scan(&buyerID, &ownerID, &status, &version); err != nil {
		return err
	}
	triggerRole := accountGovernanceTriggerRole(job.TargetUserID, buyerID)
	if _, exists, err := loadExistingAccountGovernanceDispositionInTx(ctx, tx, job, "api_purchase_intent", intentID, triggerRole, now); err != nil || exists {
		return err
	}
	var hasOrder bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM api_orders WHERE api_purchase_intent_id = $1)`, intentID).Scan(&hasOrder); err != nil {
		return err
	}
	preserved, err := accountGovernanceHasPreservationFact(ctx, tx, "api_purchase_intent", intentID)
	if err != nil {
		return err
	}
	if hasOrder || preserved || status == "ordered" {
		_, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{ResourceType: "api_purchase_intent", ResourceID: intentID, Result: "preserved", TriggerRole: triggerRole, BeforeStatus: status, AfterStatus: status}, now)
		return err
	}
	if status != "open" && status != "contacted" {
		_, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{ResourceType: "api_purchase_intent", ResourceID: intentID, Result: "already_terminal", TriggerRole: triggerRole, BeforeStatus: status, AfterStatus: status}, now)
		return err
	}
	afterStatus := "owner_closed"
	if triggerRole == "buyer" {
		afterStatus = "buyer_cancelled"
	}
	dispositionID, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{ResourceType: "api_purchase_intent", ResourceID: intentID, Result: "cancelled", TriggerRole: triggerRole, BeforeStatus: status, AfterStatus: afterStatus}, now)
	if err != nil {
		return err
	}
	version++
	if afterStatus == "buyer_cancelled" {
		_, err = tx.Exec(ctx, `UPDATE api_purchase_intents SET status = $2, buyer_cancelled_at = $3, buyer_cancel_reason = $4, governance_disposition_id = $5, governance_closed_at = $3, updated_at = $3, version = $6 WHERE id = $1`, intentID, afterStatus, now, accountGovernanceCancelReason, dispositionID, version)
	} else {
		_, err = tx.Exec(ctx, `UPDATE api_purchase_intents SET status = $2, owner_closed_at = $3, owner_close_reason = $4, governance_disposition_id = $5, governance_closed_at = $3, updated_at = $3, version = $6 WHERE id = $1`, intentID, afterStatus, now, accountGovernanceCancelReason, dispositionID, version)
	}
	if err != nil {
		return err
	}
	return insertAccountGovernanceRelationshipEventAndNotificationsInTx(ctx, tx, dispositionID, "api_purchase_intent", intentID, version, buyerID, ownerID,
		"api_purchase_intent.governance_cancelled", "/my/api-orders", "/merchant/api-orders", now)
}

func disposeAccountGovernanceCarpoolApplicationInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, applicationID string, now time.Time) error {
	var buyerID, ownerID, status string
	var version int64
	if err := tx.QueryRow(ctx, `SELECT buyer_user_id::text, owner_user_id::text, status, version FROM carpool_applications WHERE id = $1 FOR UPDATE`, applicationID).Scan(&buyerID, &ownerID, &status, &version); err != nil {
		return err
	}
	triggerRole := accountGovernanceTriggerRole(job.TargetUserID, buyerID)
	if _, exists, err := loadExistingAccountGovernanceDispositionInTx(ctx, tx, job, "carpool_application", applicationID, triggerRole, now); err != nil || exists {
		return err
	}
	preserved, err := accountGovernanceHasPreservationFact(ctx, tx, "carpool_application", applicationID)
	if err != nil {
		return err
	}
	var hasActiveMembership bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM carpool_memberships WHERE carpool_application_id = $1 AND status = 'active')`, applicationID).Scan(&hasActiveMembership); err != nil {
		return err
	}
	if status == "joined" || hasActiveMembership || preserved {
		_, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{ResourceType: "carpool_application", ResourceID: applicationID, Result: "preserved", TriggerRole: triggerRole, BeforeStatus: status, AfterStatus: status}, now)
		return err
	}
	if status != "pending_owner" {
		_, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{ResourceType: "carpool_application", ResourceID: applicationID, Result: "already_terminal", TriggerRole: triggerRole, BeforeStatus: status, AfterStatus: status}, now)
		return err
	}
	afterStatus := "cancelled_by_owner"
	if triggerRole == "buyer" {
		afterStatus = "cancelled_by_buyer"
	}
	claimDeadline := job.EffectiveAt.Add(7 * 24 * time.Hour)
	dispositionID, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{
		ResourceType: "carpool_application", ResourceID: applicationID, Result: "cancelled", TriggerRole: triggerRole,
		BeforeStatus: status, AfterStatus: afterStatus,
		PaymentClaimDeadline: &claimDeadline,
	}, now)
	if err != nil {
		return err
	}
	version++
	if _, err := tx.Exec(ctx, `
		UPDATE carpool_applications
		SET status = $2, decision_reason = $3, decided_at = $4,
		    governance_disposition_id = $5, governance_cancelled_at = $4,
		    updated_at = $4, version = $6
		WHERE id = $1
	`, applicationID, afterStatus, accountGovernanceCancelReason, now, dispositionID, version); err != nil {
		return err
	}
	return insertAccountGovernanceRelationshipEventAndNotificationsInTx(ctx, tx, dispositionID, "carpool_application", applicationID, version, buyerID, ownerID,
		"carpool_application.governance_cancelled", "/my/rides/"+applicationID, "/merchant/carpool-applications/"+applicationID, now)
}

func disposeAccountGovernanceCarpoolMembershipInTx(ctx context.Context, tx pgx.Tx, job accountGovernanceJob, membershipID string, now time.Time) error {
	var buyerID, status string
	if err := tx.QueryRow(ctx, `SELECT buyer_user_id::text, status FROM carpool_memberships WHERE id = $1 FOR UPDATE`, membershipID).Scan(&buyerID, &status); err != nil {
		return err
	}
	triggerRole := accountGovernanceTriggerRole(job.TargetUserID, buyerID)
	if _, exists, err := loadExistingAccountGovernanceDispositionInTx(ctx, tx, job, "carpool_membership", membershipID, triggerRole, now); err != nil || exists {
		return err
	}
	result := "already_terminal"
	if status == "active" {
		result = "preserved"
	}
	_, err := createAccountGovernanceDispositionInTx(ctx, tx, job, accountGovernanceDispositionInput{ResourceType: "carpool_membership", ResourceID: membershipID, Result: result, TriggerRole: triggerRole, BeforeStatus: status, AfterStatus: status}, now)
	return err
}

func insertAccountGovernanceRelationshipEventAndNotificationsInTx(ctx context.Context, tx pgx.Tx, dispositionID, resourceType, resourceID string, aggregateVersion int64, buyerID, sellerID, eventType, buyerURL, sellerURL string, now time.Time) error {
	eventID := uuid.NewString()
	requestID := "account-governance-disposition:" + dispositionID
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, $2, $3, $4, NULL, 'system', $5, $6,
		        jsonb_build_object('reasonCode', $7::text, 'dispositionId', $8::text), $9)
	`, eventID, resourceType, resourceID, eventType, aggregateVersion, requestID, accountGovernanceCancelReason, dispositionID, now); err != nil {
		return err
	}
	for _, notification := range []struct {
		userID string
		url    string
	}{
		{userID: buyerID, url: buyerURL},
		{userID: sellerID, url: sellerURL},
	} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO notifications (
				user_id, type, title, body, target_type, target_id, target_url,
				source_event_type, source_event_id, dedupe_key, created_at
			)
			VALUES ($1, 'account_governance', '业务关系已因账号治理关闭',
			        '该关系已停止后续交易；如限制生效前已发生站外付款，请在申报期限内处理。',
			        $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
		`, notification.userID, resourceType, resourceID, notification.url, eventType, eventID,
			"account_governance_disposition:"+dispositionID+":"+notification.userID, now); err != nil {
			return err
		}
	}
	return nil
}

func ensureActiveBusinessUsersInTx(ctx context.Context, tx pgx.Tx, userIDs ...string) *domain.AppError {
	unique := make(map[string]struct{}, len(userIDs))
	ordered := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return internalStoreError()
		}
		if _, exists := unique[userID]; exists {
			continue
		}
		unique[userID] = struct{}{}
		ordered = append(ordered, userID)
	}
	sort.Strings(ordered)
	for _, userID := range ordered {
		if appErr := lockAccountGovernanceUser(ctx, tx, userID); appErr != nil {
			return appErr
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT account_status = 'active' FROM users WHERE id = $1 FOR SHARE`, userID).Scan(&active); err != nil {
			return internalStoreError()
		}
		if !active {
			return domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Account restricted", "当前账号不可执行新的交易操作。")
		}
	}
	return nil
}
