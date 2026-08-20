package postgres

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/module/accountgovernance"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/google/uuid"
)

func TestPostgresRestrictedDisputeRequiresPreservedTargetDisposition(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 14, 0, 0, 0, time.UTC)
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin restricted dispute fixture: %v", err)
	}
	fixture := seedOperationAuditPlanBaseFixture(t, ctx, tx, now.Add(-2*time.Hour))
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit restricted dispute fixture: %v", err)
	}
	buyerID := fixture.actorID
	sellerID := fixture.otherActorID
	orderID := fixture.targetBySource["api_order"]
	disputeID := insertLifecycleDispute(t, store, orderID, sellerID, buyerID, report.DisputeStatusOpen, now.Add(-90*time.Minute))
	unlinkedDisputeID := insertLifecycleDispute(t, store, fixture.otherOrderID, sellerID, buyerID, report.DisputeStatusOpen, now.Add(-80*time.Minute))
	if _, err := store.pool.Exec(ctx, `UPDATE api_orders SET dispute_status = 'open', dispute_case_id = CASE id WHEN $1::uuid THEN $2::uuid ELSE $4::uuid END WHERE id IN ($1::uuid, $3::uuid)`, orderID, disputeID, fixture.otherOrderID, unlinkedDisputeID); err != nil {
		t.Fatalf("link restricted disputes to orders: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE dispute_cases SET next_actor = 'respondent', due_at = $2 WHERE id = $1`, disputeID, now.Add(48*time.Hour)); err != nil {
		t.Fatalf("prepare restricted dispute response window: %v", err)
	}
	actionID := uuid.NewString()
	dispositionID := uuid.NewString()
	effectiveAt := now.Add(-time.Hour)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, is_indefinite,
			request_id, created_at, updated_at
		) VALUES ($1, $2, 'suspend', 'effective', 2, 'POLICY', '继续处理活动纠纷', $3, true, $4, $3, $3)
	`, actionID, buyerID, effectiveAt, "restricted-dispute-"+disputeID); err != nil {
		t.Fatalf("insert restricted dispute action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'suspended', governance_version = 2, current_governance_action_id = $1 WHERE id = $2`, actionID, buyerID); err != nil {
		t.Fatalf("activate restricted dispute action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_resource_dispositions (
			id, resource_type, resource_id, result, reason_code, trigger_roles,
			before_status, after_status, governance_effective_at, created_at, updated_at
		) VALUES ($1, 'api_order', $2, 'preserved', 'ACCOUNT_GOVERNANCE_CANCELLED', ARRAY['buyer'], 'pending_payment', 'pending_payment', $3, $4, $4)
	`, dispositionID, orderID, effectiveAt, now); err != nil {
		t.Fatalf("insert restricted dispute target disposition: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO account_governance_disposition_actions (disposition_id, governance_action_id, trigger_role, linked_at) VALUES ($1, $2, 'buyer', $3)`, dispositionID, actionID, now); err != nil {
		t.Fatalf("link restricted dispute disposition: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_order_dispute_messages WHERE dispute_case_id IN ($1, $2)`, disputeID, unlinkedDisputeID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM dispute_events WHERE entity_id IN ($1, $2)`, disputeID, unlinkedDisputeID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id IN ($1, $2)`, buyerID, sellerID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_disposition_actions WHERE disposition_id = $1`, dispositionID)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE api_orders SET governance_disposition_id = NULL, dispute_case_id = NULL, dispute_status = 'none' WHERE id IN ($1, $2)`, orderID, fixture.otherOrderID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_resource_dispositions WHERE id = $1`, dispositionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM dispute_cases WHERE id IN ($1, $2)`, disputeID, unlinkedDisputeID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_order_events WHERE api_order_id IN ($1, $2)`, orderID, fixture.otherOrderID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_orders WHERE id IN ($1, $2)`, orderID, fixture.otherOrderID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_purchase_intents WHERE id IN ($1, $2)`, fixture.targetBySource["api_purchase_intent_contact_access"], fixture.otherIntentID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_service_access_modes WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, sellerID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_services WHERE owner_user_id = $1`, sellerID)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE users SET current_governance_action_id = NULL WHERE id = $1`, buyerID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_actions WHERE id = $1`, actionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM contact_method_versions WHERE owner_user_id IN ($1, $2)`, buyerID, sellerID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM contact_methods WHERE user_id IN ($1, $2)`, buyerID, sellerID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id IN ($1, $2)`, buyerID, sellerID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id IN ($1, $2)`, buyerID, sellerID)
	})

	actor := auth.BusinessActor{UserID: buyerID, Audience: auth.SessionAudienceRestrictedBusiness, GovernanceActionID: actionID, GovernanceVersion: 2, RestrictionEffectiveAt: effectiveAt}
	disputes, appErr := store.ListDisputesForActor(ctx, actor)
	if appErr != nil || len(disputes) != 1 || disputes[0].ID != disputeID {
		t.Fatalf("restricted disputes=%+v err=%v", disputes, appErr)
	}
	if _, appErr := store.GetDisputeForActor(ctx, actor, unlinkedDisputeID); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("unlinked restricted dispute was visible: %v", appErr)
	}
	entry := idempotency.Entry{UserID: buyerID, RouteKey: "restricted-dispute-response", Key: "restricted-dispute-response", RequestHash: "restricted-dispute-response", State: "processing", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if _, err := store.pool.Exec(ctx, `INSERT INTO idempotency_keys (id, user_id, route_key, idempotency_key, request_hash, status, response_body_cache_allowed, expires_at, created_at) VALUES ($1, $2, $3, $4, $5, 'processing', false, $6, $7)`, uuid.NewString(), entry.UserID, entry.RouteKey, entry.Key, entry.RequestHash, entry.ExpiresAt, now); err != nil {
		t.Fatalf("insert restricted dispute idempotency: %v", err)
	}
	input := report.DisputeParticipantActionInput{DisputeID: disputeID, ActorUserID: buyerID, ActorAudience: auth.SessionAudienceRestrictedBusiness, GovernanceActionID: actionID, GovernanceVersion: 2, RestrictionEffectiveAt: effectiveAt, Action: report.DisputeActionRespond, Body: "受限账号对保留纠纷提交正式答复。", RequestID: "restricted-dispute-response"}
	updated, _, appErr := store.UpdateDisputeParticipantWithIdempotency(ctx, entry, input, now, func(value report.DisputeCase) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`), ResourceType: "dispute", ResourceID: value.ID}, nil
	})
	if appErr != nil || updated.ID != disputeID || updated.RespondedByUserID != buyerID || updated.NextActor != report.DisputeNextActorAdmin || updated.DueAt != nil {
		t.Fatalf("restricted dispute response=%+v err=%v", updated, appErr)
	}
	stale := actor
	stale.GovernanceVersion = 1
	if _, appErr := store.GetDisputeForActor(ctx, stale, disputeID); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("stale governance version read dispute: %v", appErr)
	}
}

func TestPostgresRestrictedCarpoolMembershipRequiresCurrentPreservedDisposition(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	buyer, appErr := store.EnsureUser(ctx, "restricted-carpool-buyer-"+suffix, false, now.Add(-48*time.Hour))
	if appErr != nil {
		t.Fatalf("ensure restricted carpool buyer: %v", appErr)
	}
	owner, appErr := store.EnsureUser(ctx, "restricted-carpool-owner-"+suffix, false, now.Add(-48*time.Hour))
	if appErr != nil {
		t.Fatalf("ensure restricted carpool owner: %v", appErr)
	}
	var productPlanID string
	if err := store.pool.QueryRow(ctx, `SELECT id::text FROM product_plans ORDER BY created_at, id LIMIT 1`).Scan(&productPlanID); err != nil {
		t.Fatalf("read restricted carpool product plan: %v", err)
	}
	buyerContactID := uuid.NewString()
	ownerContactID := uuid.NewString()
	listingID := uuid.NewString()
	applicationID := uuid.NewString()
	membershipID := uuid.NewString()
	unlinkedMembershipID := uuid.NewString()
	actionID := uuid.NewString()
	dispositionID := uuid.NewString()
	effectiveAt := now.Add(-time.Hour)
	joinedAt := now.Add(-24 * time.Hour)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, is_default, enabled, created_at, updated_at)
		VALUES ($1, $2, 'telegram', 'buyer', true, true, $5, $5),
		       ($3, $4, 'telegram', 'owner', true, true, $5, $5)
	`, buyerContactID, buyer.ID, ownerContactID, owner.ID, joinedAt); err != nil {
		t.Fatalf("insert restricted carpool contacts: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO carpool_listings (
			id, owner_user_id, product_plan_id, owner_contact_method_id,
			title, summary, access_arrangement, price_monthly_cny,
			buyer_seat_capacity, active_buyer_members, status, policy_version,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, '受限拼车', '受限业务连续性测试', '双方站外确认', 20,
		          2, 1, 'active', 1, $5, $5)
	`, listingID, owner.ID, productPlanID, ownerContactID, joinedAt.Add(-time.Hour)); err != nil {
		t.Fatalf("insert restricted carpool listing: %v", err)
	}
	insertApplication := `
		INSERT INTO carpool_applications (
			id, carpool_listing_id, buyer_user_id, owner_user_id, product_plan_id,
			buyer_contact_method_id, status, listing_title_snapshot,
			price_monthly_cny_snapshot, policy_version_snapshot,
			conditions_version_snapshot, conditions_snapshot,
			accepted_conditions_version, conditions_accepted_at, joined_at,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'joined', '受限拼车', 20, 1,
		          1, '{}'::jsonb, 1, $7, $7, $7, $7)`
	if _, err := store.pool.Exec(ctx, insertApplication, applicationID, listingID, buyer.ID, owner.ID, productPlanID, buyerContactID, joinedAt); err != nil {
		t.Fatalf("insert preserved carpool application: %v", err)
	}
	secondApplicationID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, insertApplication, secondApplicationID, listingID, buyer.ID, owner.ID, productPlanID, buyerContactID, joinedAt.Add(time.Minute)); err != nil {
		t.Fatalf("insert unlinked carpool application: %v", err)
	}
	insertMembership := `
		INSERT INTO carpool_memberships (
			id, carpool_listing_id, carpool_application_id, buyer_user_id, owner_user_id,
			product_plan_id, status, price_monthly_cny_snapshot,
			policy_version_snapshot, conditions_version_snapshot, conditions_snapshot,
			joined_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'active', 20, 1, 1, '{}'::jsonb, $7, $7, $7)`
	if _, err := store.pool.Exec(ctx, insertMembership, membershipID, listingID, applicationID, buyer.ID, owner.ID, productPlanID, joinedAt); err != nil {
		t.Fatalf("insert preserved carpool membership: %v", err)
	}
	unlinkedInsertMembership := strings.Replace(insertMembership, "status, price_monthly_cny_snapshot,", "status, ended_at, ended_reason, price_monthly_cny_snapshot,", 1)
	unlinkedInsertMembership = strings.Replace(unlinkedInsertMembership, "'active', 20", "'left', $8, 'left', 20", 1)
	if _, err := store.pool.Exec(ctx, unlinkedInsertMembership, unlinkedMembershipID, listingID, secondApplicationID, buyer.ID, owner.ID, productPlanID, joinedAt.Add(time.Minute), joinedAt.Add(2*time.Minute)); err != nil {
		t.Fatalf("insert unlinked carpool membership: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, is_indefinite,
			request_id, created_at, updated_at
		) VALUES ($1, $2, 'suspend', 'effective', 2, 'POLICY', '继续处理已加入拼车', $3, true, $4, $3, $3)
	`, actionID, buyer.ID, effectiveAt, "restricted-carpool-"+suffix); err != nil {
		t.Fatalf("insert restricted carpool action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'suspended', governance_version = 2, current_governance_action_id = $1 WHERE id = $2`, actionID, buyer.ID); err != nil {
		t.Fatalf("activate restricted carpool action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_resource_dispositions (
			id, resource_type, resource_id, result, reason_code, trigger_roles,
			before_status, after_status, governance_effective_at, created_at, updated_at
		) VALUES ($1, 'carpool_membership', $2, 'preserved', 'ACCOUNT_GOVERNANCE_CANCELLED', ARRAY['buyer'], 'active', 'active', $3, $4, $4)
	`, dispositionID, membershipID, effectiveAt, now); err != nil {
		t.Fatalf("insert preserved carpool disposition: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO account_governance_disposition_actions (disposition_id, governance_action_id, trigger_role, linked_at) VALUES ($1, $2, 'buyer', $3)`, dispositionID, actionID, now); err != nil {
		t.Fatalf("link preserved carpool disposition: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM notifications WHERE target_type = 'carpool_membership' AND target_id IN ($1, $2)`, membershipID, unlinkedMembershipID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM domain_events WHERE aggregate_type = 'carpool_membership' AND aggregate_id IN ($1, $2)`, membershipID, unlinkedMembershipID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id = $1`, buyer.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_disposition_actions WHERE disposition_id = $1`, dispositionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_resource_dispositions WHERE id = $1`, dispositionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM carpool_memberships WHERE id IN ($1, $2)`, membershipID, unlinkedMembershipID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM carpool_applications WHERE id IN ($1, $2)`, applicationID, secondApplicationID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM carpool_listings WHERE id = $1`, listingID)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE users SET current_governance_action_id = NULL WHERE id = $1`, buyer.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_actions WHERE id = $1`, actionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM contact_methods WHERE id IN ($1, $2)`, buyerContactID, ownerContactID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id IN ($1, $2)`, buyer.ID, owner.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id IN ($1, $2)`, buyer.ID, owner.ID)
	})

	actor := auth.BusinessActor{UserID: buyer.ID, Audience: auth.SessionAudienceRestrictedBusiness, GovernanceActionID: actionID, GovernanceVersion: 2, RestrictionEffectiveAt: effectiveAt}
	memberships, appErr := store.ListCarpoolMembershipsForActor(ctx, actor, carpool.JoinActorBuyer)
	if appErr != nil || len(memberships) != 1 || memberships[0].ID != membershipID {
		t.Fatalf("restricted carpool memberships=%+v err=%v", memberships, appErr)
	}
	if _, appErr := store.GetCarpoolMembershipForActor(ctx, actor, unlinkedMembershipID, carpool.JoinActorBuyer); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("unlinked restricted membership was visible: %v", appErr)
	}
	entry := idempotency.Entry{UserID: buyer.ID, RouteKey: "restricted-carpool-leave", Key: "restricted-carpool-leave", RequestHash: "restricted-carpool-leave", State: "processing", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if _, err := store.pool.Exec(ctx, `INSERT INTO idempotency_keys (id, user_id, route_key, idempotency_key, request_hash, status, response_body_cache_allowed, expires_at, created_at) VALUES ($1, $2, $3, $4, $5, 'processing', false, $6, $7)`, uuid.NewString(), entry.UserID, entry.RouteKey, entry.Key, entry.RequestHash, entry.ExpiresAt, now); err != nil {
		t.Fatalf("insert restricted carpool idempotency: %v", err)
	}
	input := carpool.EndMembershipInput{MembershipID: membershipID, ActorUserID: buyer.ID, ActorRole: carpool.JoinActorBuyer, ActorAudience: auth.SessionAudienceRestrictedBusiness, GovernanceActionID: actionID, GovernanceVersion: 2, RestrictionEffectiveAt: effectiveAt, TargetStatus: carpool.MembershipStatusLeft, Reason: "受限账号退出拼车", ExpectedVersion: memberships[0].Version, RequestID: "restricted-carpool-leave"}
	ended, _, appErr := store.EndCarpoolMembershipWithIdempotency(ctx, entry, input, now, func(value carpool.Membership) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`), ResourceType: "carpool_membership", ResourceID: value.ID}, nil
	})
	if appErr != nil || ended.Status != carpool.MembershipStatusLeft || ended.EndedAt == nil {
		t.Fatalf("restricted carpool leave=%+v err=%v", ended, appErr)
	}
	stale := actor
	stale.GovernanceVersion = 1
	if _, appErr := store.GetCarpoolMembershipForActor(ctx, stale, membershipID, carpool.JoinActorBuyer); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("stale governance version read membership: %v", appErr)
	}
}

func TestPostgresRestrictedAPIOrderRequiresCurrentPreservedDisposition(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin restricted order fixture: %v", err)
	}
	fixture := seedOperationAuditPlanBaseFixture(t, ctx, tx, now.Add(-2*time.Hour))
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit restricted order fixture: %v", err)
	}
	buyerID := fixture.actorID
	orderID := fixture.targetBySource["api_order"]
	actionID := uuid.NewString()
	dispositionID := uuid.NewString()
	effectiveAt := now.Add(-time.Hour)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, is_indefinite,
			request_id, created_at, updated_at
		) VALUES ($1, $2, 'suspend', 'effective', 2, 'POLICY', '继续处理已付款订单', $3, true, $4, $3, $3)
	`, actionID, buyerID, effectiveAt, "restricted-order-"+orderID); err != nil {
		t.Fatalf("insert restricted order action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'suspended', governance_version = 2, current_governance_action_id = $1 WHERE id = $2`, actionID, buyerID); err != nil {
		t.Fatalf("activate restricted order action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE api_orders SET status = 'completed', payment_summary = '站外付款已提交', payment_submitted_at = $2, paid_confirmed_at = $2, delivery_note = '买家专属接入信息', delivery_submitted_at = $2, delivery_review_expires_at = $3, completion_source = 'seller_delivered', completed_at = $2, commercial_outcome = 'normal_fulfillment', commercial_outcome_updated_at = $2, updated_at = $2 WHERE id = $1`, orderID, now.Add(-30*time.Minute), now.Add(24*time.Hour)); err != nil {
		t.Fatalf("make restricted order actionable: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_resource_dispositions (
			id, resource_type, resource_id, result, reason_code, trigger_roles,
			before_status, after_status, governance_effective_at, created_at, updated_at
		) VALUES ($1, 'api_order', $2, 'preserved', 'ACCOUNT_GOVERNANCE_CANCELLED', ARRAY['buyer'], 'completed', 'completed', $3, $4, $4)
	`, dispositionID, orderID, effectiveAt, now); err != nil {
		t.Fatalf("insert preserved order disposition: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO account_governance_disposition_actions (disposition_id, governance_action_id, trigger_role, linked_at) VALUES ($1, $2, 'buyer', $3)`, dispositionID, actionID, now); err != nil {
		t.Fatalf("link preserved order disposition: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id IN ($1, $2)`, fixture.actorID, fixture.otherActorID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_order_events WHERE api_order_id IN ($1, $2)`, orderID, fixture.otherOrderID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_disposition_actions WHERE disposition_id = $1`, dispositionID)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE api_orders SET governance_disposition_id = NULL WHERE id IN ($1, $2)`, orderID, fixture.otherOrderID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_resource_dispositions WHERE id = $1`, dispositionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_orders WHERE id IN ($1, $2)`, orderID, fixture.otherOrderID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_purchase_intents WHERE id IN ($1, $2)`, fixture.targetBySource["api_purchase_intent_contact_access"], fixture.otherIntentID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_service_access_modes WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, fixture.otherActorID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_services WHERE owner_user_id = $1`, fixture.otherActorID)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE users SET current_governance_action_id = NULL WHERE id = $1`, buyerID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_actions WHERE id = $1`, actionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM contact_method_versions WHERE owner_user_id IN ($1, $2)`, fixture.actorID, fixture.otherActorID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM contact_methods WHERE user_id IN ($1, $2)`, fixture.actorID, fixture.otherActorID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id IN ($1, $2)`, fixture.actorID, fixture.otherActorID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id IN ($1, $2)`, fixture.actorID, fixture.otherActorID)
	})

	actor := auth.BusinessActor{
		UserID:                 buyerID,
		Audience:               auth.SessionAudienceRestrictedBusiness,
		GovernanceActionID:     actionID,
		GovernanceVersion:      2,
		RestrictionEffectiveAt: effectiveAt,
	}
	order, appErr := store.GetAPIOrderForActor(ctx, actor, orderID, "buyer", now)
	if appErr != nil || order.ID != orderID {
		t.Fatalf("read preserved restricted order=%+v err=%v", order, appErr)
	}
	if _, appErr := store.GetAPIOrderForActor(ctx, actor, fixture.otherOrderID, "buyer", now); appErr == nil || appErr.Code != "OBJECT_NOT_FOUND" {
		t.Fatalf("unlinked restricted order was visible: %v", appErr)
	}

	stale := actor
	stale.GovernanceVersion = 1
	if _, appErr := store.GetAPIOrderForActor(ctx, stale, orderID, "buyer", now); appErr == nil || appErr.Code != "OBJECT_NOT_FOUND" {
		t.Fatalf("stale governance version read order: %v", appErr)
	}
}

func TestPostgresAccountGovernanceBusinessCenterProjectionAndClaimDeadline(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	user, appErr := store.EnsureUser(ctx, "governance-center-"+suffix, false, now.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("ensure center user: %v", appErr)
	}
	other, appErr := store.EnsureUser(ctx, "governance-center-other-"+suffix, false, now.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("ensure other user: %v", appErr)
	}
	actionID := uuid.NewString()
	dispositionID := uuid.NewString()
	resourceID := uuid.NewString()
	deadline := now.Add(-time.Hour).Add(7 * 24 * time.Hour)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, expires_at, is_indefinite,
			request_id, created_at, updated_at
		)
		VALUES ($1, $2, 'suspend', 'effective', 2, 'POLICY', '核验在途业务', $3, $4, false, $5, $3, $3)
	`, actionID, user.ID, now.Add(-time.Hour), now.Add(7*24*time.Hour), "center-"+suffix); err != nil {
		t.Fatalf("insert center governance action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'suspended', governance_version = 2, current_governance_action_id = $1 WHERE id = $2`, actionID, user.ID); err != nil {
		t.Fatalf("activate center governance action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_resource_dispositions (
			id, resource_type, resource_id, result, reason_code, trigger_roles,
			before_status, after_status, released_resource_type, released_quantity,
			governance_effective_at, payment_claim_deadline_at, created_at, updated_at
		)
		VALUES ($1, 'api_order', $2, 'cancelled', 'ACCOUNT_GOVERNANCE_CANCELLED', ARRAY['buyer'],
			'pending_payment', 'cancelled', 'package_stock', 1, $3, $4, $5, $5)
	`, dispositionID, resourceID, now.Add(-time.Hour), deadline, now); err != nil {
		t.Fatalf("insert center disposition: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO account_governance_disposition_actions (disposition_id, governance_action_id, trigger_role, linked_at) VALUES ($1, $2, 'buyer', $3)`, dispositionID, actionID, now); err != nil {
		t.Fatalf("link center disposition: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_disposition_actions WHERE disposition_id = $1`, dispositionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_resource_dispositions WHERE id = $1`, dispositionID)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE users SET current_governance_action_id = NULL WHERE id = $1`, user.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_actions WHERE id = $1`, actionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id IN ($1, $2)`, user.ID, other.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id IN ($1, $2)`, user.ID, other.ID)
	})

	center, appErr := store.BusinessCenter(ctx, user.ID, now)
	if appErr != nil {
		t.Fatalf("read business center: %v", appErr)
	}
	if center.AccountStatus != "suspended" || center.CurrentAction == nil || len(center.Items) != 1 {
		t.Fatalf("unexpected center header/items: %+v", center)
	}
	item := center.Items[0]
	if item.ID != dispositionID || item.Result != "cancelled" || !item.PaymentClaimEligible || !accountGovernanceContainsAction(item.ActionCodes, accountgovernance.ActionPaymentClaim) {
		t.Fatalf("unexpected active claim projection: %+v", item)
	}
	lateCenter, appErr := store.BusinessCenter(ctx, user.ID, deadline)
	if appErr != nil || len(lateCenter.Items) != 1 || lateCenter.Items[0].PaymentClaimEligible || accountGovernanceContainsAction(lateCenter.Items[0].ActionCodes, accountgovernance.ActionPaymentClaim) {
		t.Fatalf("expired claim remained actionable center=%+v err=%v", lateCenter, appErr)
	}
	otherCenter, appErr := store.BusinessCenter(ctx, other.ID, now)
	if appErr != nil || len(otherCenter.Items) != 0 {
		t.Fatalf("other user saw disposition center=%+v err=%v", otherCenter, appErr)
	}
}

func TestPostgresAccountGovernanceEmptyDispositionJobCompletesIdempotently(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 14, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	user, appErr := store.EnsureUser(ctx, "governance-empty-job-"+suffix, false, now.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("ensure job user: %v", appErr)
	}
	actionID := uuid.NewString()
	jobID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, is_indefinite,
			request_id, created_at, updated_at
		) VALUES ($1, $2, 'ban', 'effective', 2, 'POLICY', '停止新业务', $3, false, $4, $3, $3)
	`, actionID, user.ID, now.Add(-time.Hour), "empty-job-"+suffix); err != nil {
		t.Fatalf("insert empty disposition job: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'banned', governance_version = 2, current_governance_action_id = $1 WHERE id = $2`, actionID, user.ID); err != nil {
		t.Fatalf("activate empty disposition job: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `INSERT INTO account_governance_jobs (id, target_user_id, governance_action_id, expected_governance_version, available_at, created_at, updated_at) VALUES ($1, $2, $3, 2, $4, $4, $4)`, jobID, user.ID, actionID, now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert empty disposition job record: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_jobs WHERE id = $1`, jobID)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE users SET current_governance_action_id = NULL WHERE id = $1`, user.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_actions WHERE id = $1`, actionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, user.ID)
	})

	policy := lifecycleCredentialPolicy()
	var jobStatus string
	for index := 0; index < 128 && jobStatus != "completed"; index++ {
		result, runErr := store.RunDataLifecycle(ctx, now.Add(time.Duration(index)*time.Second), 10, policy)
		if runErr != nil {
			t.Fatalf("run empty disposition job %d: %v", index, runErr)
		}
		_ = result
		if err := store.pool.QueryRow(ctx, `SELECT status FROM account_governance_jobs WHERE id = $1`, jobID).Scan(&jobStatus); err != nil {
			t.Fatalf("read empty disposition job %d: %v", index, err)
		}
	}
	if jobStatus != "completed" {
		t.Fatalf("empty disposition job did not complete: %s", jobStatus)
	}
	replay, appErr := store.RunDataLifecycle(ctx, now.Add(time.Minute), 10, maintenance.Policy(policy))
	if appErr != nil || replay.GovernanceDispositionResources != 0 || replay.GovernanceDispositionJobsCompleted != 0 {
		t.Fatalf("completed disposition job replayed result=%+v err=%v", replay, appErr)
	}
}

func TestPostgresAccountGovernanceAPIOrderReleasesReservationExactlyOnceForBothParties(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 16, 0, 0, 0, time.UTC)
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin governance order disposition transaction: %v", err)
	}
	defer rollback(ctx, tx)

	fixture := seedOperationAuditPlanBaseFixture(t, ctx, tx, now.Add(-2*time.Hour))
	orderID := fixture.targetBySource["api_order"]
	buyerActionID := uuid.NewString()
	sellerActionID := uuid.NewString()
	effectiveAt := now.Add(-time.Hour)
	if _, err := tx.Exec(ctx, `UPDATE api_services SET available_usd_allowance = 980 WHERE owner_user_id = $1`, fixture.otherActorID); err != nil {
		t.Fatalf("mark fixture allowance reserved: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, is_indefinite,
			request_id, created_at, updated_at
		) VALUES
			($1, $2, 'suspend', 'effective', 2, 'POLICY', '买家限制', $5, true, $6, $5, $5),
			($3, $4, 'ban', 'effective', 2, 'POLICY', '卖家限制', $5, false, $7, $5, $5)
	`, buyerActionID, fixture.actorID, sellerActionID, fixture.otherActorID, effectiveAt, "governance-buyer-"+orderID, "governance-seller-"+orderID); err != nil {
		t.Fatalf("insert both-party governance actions: %v", err)
	}

	buyerJob := accountGovernanceJob{
		TargetUserID: fixture.actorID, GovernanceActionID: buyerActionID, EffectiveAt: effectiveAt,
	}
	if err := store.disposeAccountGovernanceAPIOrderInTx(ctx, tx, buyerJob, orderID, now); err != nil {
		t.Fatalf("dispose buyer order: %v", err)
	}
	sellerJob := accountGovernanceJob{
		TargetUserID: fixture.otherActorID, GovernanceActionID: sellerActionID, EffectiveAt: effectiveAt,
	}
	if err := store.disposeAccountGovernanceAPIOrderInTx(ctx, tx, sellerJob, orderID, now.Add(time.Second)); err != nil {
		t.Fatalf("link seller order disposition: %v", err)
	}

	var status, cancelReason, result, releasedType string
	var allowance, releasedQuantity string
	var dispositionCount, linkCount, eventCount, notificationCount int
	var triggerRoles []string
	if err := tx.QueryRow(ctx, `
		SELECT order_record.status, order_record.cancel_reason,
		       service.available_usd_allowance::text,
		       disposition.result, disposition.released_resource_type,
		       disposition.released_quantity::text, disposition.trigger_roles,
		       (SELECT count(*) FROM account_governance_resource_dispositions item WHERE item.resource_type = 'api_order' AND item.resource_id = order_record.id),
		       (SELECT count(*) FROM account_governance_disposition_actions link WHERE link.disposition_id = disposition.id),
		       (SELECT count(*) FROM domain_events event WHERE event.aggregate_type = 'api_order' AND event.aggregate_id = order_record.id AND event.event_type = 'api_order.governance_cancelled'),
		       (SELECT count(*) FROM notifications notification WHERE notification.target_type = 'api_order' AND notification.target_id = order_record.id AND notification.source_event_type = 'api_order.governance_cancelled')
		FROM api_orders order_record
		JOIN api_services service ON service.id = order_record.api_service_id
		JOIN account_governance_resource_dispositions disposition ON disposition.id = order_record.governance_disposition_id
		WHERE order_record.id = $1
	`, orderID).Scan(&status, &cancelReason, &allowance, &result, &releasedType, &releasedQuantity, &triggerRoles, &dispositionCount, &linkCount, &eventCount, &notificationCount); err != nil {
		t.Fatalf("read both-party disposition result: %v", err)
	}
	if status != apiorder.StatusCancelled || cancelReason != accountGovernanceCancelReason || allowance != "990.000000" {
		t.Fatalf("order cancellation or single release mismatch: status=%s reason=%s allowance=%s", status, cancelReason, allowance)
	}
	if result != "cancelled" || releasedType != "usd_allowance" || releasedQuantity != "10.000000" {
		t.Fatalf("disposition release audit mismatch: result=%s type=%s quantity=%s", result, releasedType, releasedQuantity)
	}
	if strings.Join(triggerRoles, ",") != "buyer,seller" || dispositionCount != 1 || linkCount != 2 || eventCount != 1 || notificationCount != 2 {
		t.Fatalf("both-party dedupe mismatch: roles=%v dispositions=%d links=%d events=%d notifications=%d", triggerRoles, dispositionCount, linkCount, eventCount, notificationCount)
	}
}
