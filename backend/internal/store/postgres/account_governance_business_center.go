package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/accountgovernance"

	"github.com/jackc/pgx/v5"
)

func (s *Store) BusinessCenter(ctx context.Context, userID string, now time.Time) (accountgovernance.Center, *domain.AppError) {
	if s == nil || s.pool == nil {
		return accountgovernance.Center{}, internalStoreError()
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return accountgovernance.Center{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	center := accountgovernance.Center{GeneratedAt: now.UTC(), Items: []accountgovernance.Disposition{}}
	if err := loadAccountGovernanceCenterHeader(ctx, tx, userID, &center); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return accountgovernance.Center{}, domain.NewError(404, domain.CodeObjectNotFound, "Account not found", "账号不存在。")
		}
		return accountgovernance.Center{}, internalStoreError()
	}
	if err := loadAccountGovernanceCenterItems(ctx, tx, userID, now, &center); err != nil {
		return accountgovernance.Center{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return accountgovernance.Center{}, internalStoreError()
	}
	return center, nil
}

func loadAccountGovernanceCenterHeader(ctx context.Context, q queryer, userID string, center *accountgovernance.Center) error {
	var action accountgovernance.CurrentAction
	var actionType, reasonCode, publicReason *string
	var effectiveAt, expiresAt *time.Time
	var indefinite *bool
	var governanceVersion *int64
	err := q.QueryRow(ctx, `
		SELECT user_account.account_status,
		       CASE
		         WHEN count(job.id) FILTER (WHERE job.status IN ('pending', 'processing', 'failed')) > 0 THEN 'processing'
		         WHEN count(job.id) > 0 THEN 'completed'
		         ELSE 'not_started'
		       END,
		       action.action_type, action.reason_code, action.public_reason,
		       action.effective_at, action.expires_at, action.is_indefinite,
		       action.governance_version
		FROM users user_account
		LEFT JOIN account_governance_actions action ON action.id = user_account.current_governance_action_id
		LEFT JOIN account_governance_jobs job ON job.target_user_id = user_account.id
		WHERE user_account.id = $1
		GROUP BY user_account.account_status, action.action_type, action.reason_code,
		         action.public_reason, action.effective_at, action.expires_at,
		         action.is_indefinite, action.governance_version
	`, userID).Scan(
		&center.AccountStatus, &center.ProcessingStatus,
		&actionType, &reasonCode, &publicReason, &effectiveAt, &expiresAt, &indefinite, &governanceVersion,
	)
	if err != nil {
		return err
	}
	if actionType != nil && effectiveAt != nil && governanceVersion != nil {
		action.ActionType = *actionType
		action.ReasonCode = stringValue(reasonCode)
		action.PublicReason = stringValue(publicReason)
		action.EffectiveAt = effectiveAt.UTC()
		action.ExpiresAt = expiresAt
		action.Indefinite = indefinite != nil && *indefinite
		action.GovernanceVersion = *governanceVersion
		center.CurrentAction = &action
	}
	return nil
}

func loadAccountGovernanceCenterItems(ctx context.Context, q pgx.Tx, userID string, now time.Time, center *accountgovernance.Center) error {
	rows, err := q.Query(ctx, `
		SELECT disposition.id::text, disposition.resource_type, disposition.resource_id::text,
		       CASE disposition.resource_type
		         WHEN 'api_service' THEN COALESCE(api_service.title, 'API 服务')
		         WHEN 'api_quota_batch' THEN COALESCE(api_quota_batch.source_label, 'API 额度批次')
		         WHEN 'api_quota_offer' THEN COALESCE(api_quota_offer.name, 'API 额度')
		         WHEN 'api_service_promotion' THEN 'API 服务推广'
		         WHEN 'api_order' THEN COALESCE(api_order.service_title_snapshot, 'API 订单')
		         WHEN 'api_purchase_intent' THEN COALESCE(api_intent.service_title_snapshot, 'API 购买意向')
		         WHEN 'carpool_listing' THEN COALESCE(carpool_listing.title, '拼车车源')
		         WHEN 'carpool_application' THEN COALESCE(carpool_application.listing_title_snapshot, '拼车申请')
		         WHEN 'carpool_membership' THEN COALESCE(carpool_application_for_membership.listing_title_snapshot, '拼车成员关系')
		         ELSE '业务关系'
		       END,
		       CASE
		         WHEN api_order.buyer_user_id = $1 OR api_intent.buyer_user_id = $1
		           OR carpool_application.buyer_user_id = $1 OR carpool_membership.buyer_user_id = $1
		         THEN 'buyer'
		         ELSE 'seller'
		       END,
		       disposition.result, disposition.reason_code, disposition.trigger_roles,
		       disposition.before_status, disposition.after_status,
		       COALESCE(disposition.released_resource_type, ''),
		       COALESCE(disposition.released_quantity::text, ''),
		       disposition.governance_effective_at, disposition.payment_claim_deadline_at,
		       disposition.updated_at
		FROM account_governance_resource_dispositions disposition
		JOIN account_governance_disposition_actions link ON link.disposition_id = disposition.id
		JOIN account_governance_actions action ON action.id = link.governance_action_id
		LEFT JOIN api_services api_service ON disposition.resource_type = 'api_service' AND api_service.id = disposition.resource_id
		LEFT JOIN api_quota_batches api_quota_batch ON disposition.resource_type = 'api_quota_batch' AND api_quota_batch.id = disposition.resource_id
		LEFT JOIN api_quota_offers api_quota_offer ON disposition.resource_type = 'api_quota_offer' AND api_quota_offer.id = disposition.resource_id
		LEFT JOIN api_orders api_order ON disposition.resource_type = 'api_order' AND api_order.id = disposition.resource_id
		LEFT JOIN api_purchase_intents api_intent ON disposition.resource_type = 'api_purchase_intent' AND api_intent.id = disposition.resource_id
		LEFT JOIN carpool_listings carpool_listing ON disposition.resource_type = 'carpool_listing' AND carpool_listing.id = disposition.resource_id
		LEFT JOIN carpool_applications carpool_application ON disposition.resource_type = 'carpool_application' AND carpool_application.id = disposition.resource_id
		LEFT JOIN carpool_memberships carpool_membership ON disposition.resource_type = 'carpool_membership' AND carpool_membership.id = disposition.resource_id
		LEFT JOIN carpool_applications carpool_application_for_membership ON carpool_application_for_membership.id = carpool_membership.carpool_application_id
		WHERE action.target_user_id = $1
		ORDER BY disposition.updated_at DESC, disposition.id DESC
	`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	seen := make(map[string]struct{})
	for rows.Next() {
		var item accountgovernance.Disposition
		if err := rows.Scan(
			&item.ID, &item.ResourceType, &item.ResourceID, &item.ResourceLabel, &item.ParticipantRole,
			&item.Result, &item.ReasonCode, &item.TriggerRoles, &item.BeforeStatus, &item.AfterStatus,
			&item.ReleasedResourceType, &item.ReleasedQuantity, &item.GovernanceEffectiveAt,
			&item.PaymentClaimDeadlineAt, &item.UpdatedAt,
		); err != nil {
			return err
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		item.ActionCodes = accountGovernanceDispositionActionCodes(item, now)
		item.PaymentClaimEligible = accountGovernanceContainsAction(item.ActionCodes, accountgovernance.ActionPaymentClaim)
		item.TargetURL = accountGovernanceResourceURL(item.ResourceType, item.ResourceID, item.ParticipantRole)
		center.Items = append(center.Items, item)
	}
	return rows.Err()
}

func accountGovernanceDispositionActionCodes(item accountgovernance.Disposition, now time.Time) []string {
	actions := []string{accountgovernance.ActionViewResource}
	if item.Result == "cancelled" && item.PaymentClaimDeadlineAt != nil && now.Before(*item.PaymentClaimDeadlineAt) {
		actions = append(actions, accountgovernance.ActionPaymentClaim)
	}
	return actions
}

func accountGovernanceResourceURL(resourceType, resourceID, participantRole string) string {
	switch resourceType {
	case "api_order":
		if participantRole == "seller" {
			return "/merchant/api-orders/" + resourceID
		}
		return "/my/api-orders/" + resourceID
	case "api_purchase_intent":
		return "/my/api-orders"
	case "carpool_application":
		if participantRole == "seller" {
			return "/merchant/carpool-applications/" + resourceID
		}
		return "/my/rides/" + resourceID
	case "carpool_membership":
		return "/my/rides"
	case "api_service", "api_quota_batch", "api_quota_offer", "api_service_promotion":
		return "/merchant/api-services"
	case "carpool_listing":
		return "/merchant/carpools"
	default:
		return ""
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func accountGovernanceContainsAction(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
