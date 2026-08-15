package postgres

import (
	"context"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/evidence"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/google/uuid"
)

func TestPostgresOpenAPIOrderDisputePersistsTypedOrderReference(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	completedAt := now.Add(-time.Hour)
	validityExpiresAt := now.Add(48 * time.Hour)

	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	t.Cleanup(func() {
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, completedAt.Add(-4*time.Hour))
	fixture := insertLifecycleCompletedCredentialOrder(
		t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID,
		completedAt, completedAt.Add(-30*time.Minute), "", nil,
	)

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin API order dispute transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	item, appErr := openDisputeFromAPIOrderInTx(ctx, tx, apiorder.Order{
		ID:                   fixture.OrderID,
		BuyerUserID:          buyerID,
		SellerUserID:         sellerID,
		Status:               apiorder.StatusCompleted,
		DisputeStatus:        apiorder.DisputeStatusNone,
		ServiceTitleSnapshot: "API order dispute integration service",
		PackageExpiresAt:     &validityExpiresAt,
	}, apiorder.ActionInput{
		ActorUserID:         buyerID,
		IssueCode:           apiorder.DisputeIssueServiceUnavailable,
		RequestedResolution: apiorder.DisputeResolutionFullRefund,
		IssueOccurredAt:     completedAt.Format(time.RFC3339),
		Reason:              "服务有效期内无法使用。",
		RequestID:           "api-order-dispute-typed-reference",
	}, now)
	if appErr != nil {
		t.Fatalf("open API order dispute: %v", appErr)
	}
	if item.APIOrderID != fixture.OrderID || item.TargetID != fixture.OrderID || !item.Active {
		t.Fatalf("unexpected API order dispute reference: %+v", item)
	}
	if item.Status != report.DisputeStatusNegotiating {
		t.Fatalf("unexpected API order dispute status: %q", item.Status)
	}
	var messageCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM api_order_dispute_messages
		WHERE dispute_case_id = $1
	`, item.ID).Scan(&messageCount); err != nil {
		t.Fatalf("count opening dispute messages: %v", err)
	}
	if messageCount != 1 {
		t.Fatalf("expected one opening dispute message, got %d", messageCount)
	}
}

func TestPostgresDisputeEscalationAndRemedyEvidenceBindingsAreAtomic(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 15, 13, 0, 0, 0, time.UTC)
	completedAt := now.Add(-time.Hour)
	validityExpiresAt := now.Add(48 * time.Hour)

	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	t.Cleanup(func() {
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, completedAt.Add(-4*time.Hour))
	fixture := insertLifecycleCompletedCredentialOrder(
		t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID,
		completedAt, completedAt.Add(-30*time.Minute), "", nil,
	)

	setupTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dispute, appErr := openDisputeFromAPIOrderInTx(ctx, setupTx, apiorder.Order{
		ID: fixture.OrderID, BuyerUserID: buyerID, SellerUserID: sellerID,
		Status: apiorder.StatusCompleted, DisputeStatus: apiorder.DisputeStatusNone,
		ServiceTitleSnapshot: "API order evidence transaction service", PackageExpiresAt: &validityExpiresAt,
	}, apiorder.ActionInput{
		ActorUserID: buyerID, IssueCode: apiorder.DisputeIssueServiceUnavailable,
		RequestedResolution: apiorder.DisputeResolutionFullRefund,
		IssueOccurredAt:     completedAt.Format(time.RFC3339), Reason: "服务有效期内无法使用。",
		RequestID: "api-order-dispute-evidence-open",
	}, now)
	if appErr != nil {
		_ = setupTx.Rollback(ctx)
		t.Fatalf("open API order dispute: %v", appErr)
	}
	if _, err := setupTx.Exec(ctx, `
		UPDATE api_orders
		SET dispute_case_id = $2, latest_dispute_case_id = $2, dispute_status = 'negotiating', version = version + 1
		WHERE id = $1
	`, fixture.OrderID, dispute.ID); err != nil {
		_ = setupTx.Rollback(ctx)
		t.Fatal(err)
	}
	proposalID := uuid.NewString()
	if _, err := setupTx.Exec(ctx, `
		INSERT INTO api_order_dispute_settlement_proposals (
			id, dispute_case_id, proposed_by_user_id, resolution, terms,
			fulfillment_required, status, request_id, created_at, updated_at, version
		) VALUES ($1, $2, $3, 'other', '双方未达成一致。', false, 'pending', $4, $5, $5, 1)
	`, proposalID, dispute.ID, buyerID, "proposal-before-platform-escalation", now); err != nil {
		_ = setupTx.Rollback(ctx)
		t.Fatal(err)
	}
	escalationAssetID := insertReadyEvidenceAsset(t, setupTx, fixture.OrderID, sellerID, now, now.Add(time.Hour))
	claimAssetID := insertReadyEvidenceAsset(t, setupTx, fixture.OrderID, sellerID, now, now.Add(time.Hour))
	contestAssetID := insertReadyEvidenceAsset(t, setupTx, fixture.OrderID, buyerID, now, now.Add(time.Hour))
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupDisputeParticipantEvidenceFixture(t, store, fixture.OrderID, dispute.ID, sellerID, buyerID)
	})

	escalated, _, appErr := store.UpdateDisputeParticipantWithIdempotency(ctx,
		beginDisputeParticipantIdempotency(t, store, sellerID, "escalate", now),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeMessageActionEscalate,
			NegotiationChannels:       []string{report.NegotiationChannelWeChat, report.NegotiationChannelEmail},
			NegotiationEndedConfirmed: true, NegotiationSummary: "双方对退款责任无法达成一致。",
			RequestedPlatformAction: "请平台判断退款责任。", RequestID: "postgres-dispute-escalate",
			EvidenceAssetIDs: []string{escalationAssetID},
		}, now, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("escalate dispute: %v", appErr)
	}
	if escalated.Status != report.DisputeStatusOpen || !escalated.NegotiationEndedConfirmed ||
		escalated.EscalatedByUserID != sellerID || escalated.EscalatedAt == nil ||
		len(escalated.NegotiationChannels) != 2 || escalated.SettlementProposals[0].SupersededReason != "platform_escalation" {
		t.Fatalf("unexpected persisted escalation: %+v", escalated)
	}
	assertDisputeEvidenceBinding(t, store, escalationAssetID, dispute.ID, evidence.UsagePlatformEscalation, evidence.SourceDisputeCase, dispute.ID)

	remedyID := uuid.NewString()
	remedySetupAt := now.Add(time.Minute)
	if _, err := store.pool.Exec(ctx, `UPDATE dispute_cases SET status = 'resolved', resolved_at = $2, updated_at = $2 WHERE id = $1`, dispute.ID, remedySetupAt); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE api_orders SET dispute_status = 'awaiting_fulfillment', active_remedy_action = 'full_refund', updated_at = $2 WHERE id = $1`, fixture.OrderID, remedySetupAt); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_order_dispute_remedies (
			id, dispute_case_id, action, currency, responsible_user_id, beneficiary_user_id,
			instructions, status, due_at, lateness_status, source, created_by_admin_id,
			created_request_id, created_at, updated_at, version
		) VALUES ($1, $2, 'full_refund', 'CNY', $3, $4, '请在站外完成退款。', 'pending', $5, 'not_due', 'admin_decision', $3, $6, $7, $7, 1)
	`, remedyID, dispute.ID, sellerID, buyerID, now.Add(time.Hour), "postgres-remedy-create", remedySetupAt); err != nil {
		t.Fatal(err)
	}

	claimedAt := now.Add(2 * time.Minute)
	claimed, _, appErr := store.UpdateDisputeParticipantWithIdempotency(ctx,
		beginDisputeParticipantIdempotency(t, store, sellerID, "claim", claimedAt),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeRemedyActionClaim,
			Note: "已在站外完成退款。", RequestID: "postgres-remedy-claim", EvidenceAssetIDs: []string{claimAssetID},
		}, claimedAt, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("claim remedy: %v", appErr)
	}
	if len(claimed.Remedies) != 1 || claimed.Remedies[0].Status != report.RemedyStatusClaimedFulfilled {
		t.Fatalf("unexpected claimed remedy: %+v", claimed.Remedies)
	}
	assertDisputeEvidenceBinding(t, store, claimAssetID, dispute.ID, evidence.UsageRemedyClaim, evidence.SourceDisputeRemedy, remedyID)

	contestedAt := now.Add(3 * time.Minute)
	contested, _, appErr := store.UpdateDisputeParticipantWithIdempotency(ctx,
		beginDisputeParticipantIdempotency(t, store, buyerID, "contest", contestedAt),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: buyerID, Action: report.DisputeRemedyActionContest,
			Reason: "仍未收到退款。", RequestID: "postgres-remedy-contest", EvidenceAssetIDs: []string{contestAssetID},
		}, contestedAt, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("contest remedy: %v", appErr)
	}
	if contested.Status != report.DisputeStatusOpen || len(contested.Remedies) != 1 || contested.Remedies[0].Status != report.RemedyStatusContested {
		t.Fatalf("unexpected contested remedy: %+v", contested)
	}
	assertDisputeEvidenceBinding(t, store, contestAssetID, dispute.ID, evidence.UsageRemedyContest, evidence.SourceDisputeRemedy, remedyID)
}

func beginDisputeParticipantIdempotency(t *testing.T, store *Store, userID, action string, now time.Time) idempotency.Entry {
	t.Helper()
	entry := idempotency.Entry{
		UserID: userID, RouteKey: "/api/v1/me/disputes/{id}/" + action,
		Key: "postgres-dispute-" + action + "-" + uuid.NewString(), RequestHash: "hash-" + action + "-" + uuid.NewString(),
		CreatedAt: now, ExpiresAt: now.Add(idempotency.ProcessingLifetime),
	}
	created, appErr := store.BeginIdempotency(context.Background(), entry)
	if appErr != nil {
		t.Fatalf("begin %s idempotency: %v", action, appErr)
	}
	return *created
}

func disputeParticipantIntegrationCompletion(item report.DisputeCase) (idempotency.Completion, *domain.AppError) {
	return idempotency.Completion{
		Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`),
		ResourceType: "dispute", ResourceID: item.ID,
	}, nil
}

func assertDisputeEvidenceBinding(t *testing.T, store *Store, assetID, disputeID, usage, sourceType, sourceID string) {
	t.Helper()
	var actualDisputeID, actualUsage, actualSourceType, actualSourceID string
	if err := store.pool.QueryRow(context.Background(), `
		SELECT dispute_case_id::text, usage, source_type, source_id::text
		FROM api_order_evidence_bindings WHERE asset_id = $1
	`, assetID).Scan(&actualDisputeID, &actualUsage, &actualSourceType, &actualSourceID); err != nil {
		t.Fatalf("read evidence binding for %s: %v", assetID, err)
	}
	if actualDisputeID != disputeID || actualUsage != usage || actualSourceType != sourceType || actualSourceID != sourceID {
		t.Fatalf("unexpected evidence binding dispute=%s usage=%s source=%s/%s", actualDisputeID, actualUsage, actualSourceType, actualSourceID)
	}
}

func cleanupDisputeParticipantEvidenceFixture(t *testing.T, store *Store, orderID, disputeID, sellerID, buyerID string) {
	t.Helper()
	ctx := context.Background()
	deleteEvidenceBindingsForTest(t, store.pool, `DELETE FROM api_order_evidence_bindings WHERE dispute_case_id = $1`, disputeID)
	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM idempotency_keys WHERE user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM notifications WHERE user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`UPDATE api_orders SET dispute_case_id = NULL, latest_dispute_case_id = NULL, dispute_status = 'none', active_remedy_action = '' WHERE id = $1`, []any{orderID}},
		{`DELETE FROM dispute_events WHERE entity_id = $1`, []any{disputeID}},
		{`DELETE FROM dispute_cases WHERE id = $1`, []any{disputeID}},
		{`DELETE FROM api_order_evidence_assets WHERE api_order_id = $1`, []any{orderID}},
	}
	for _, statement := range statements {
		if _, err := store.pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Errorf("cleanup dispute participant evidence fixture: %v", err)
		}
	}
}
