package postgres

import (
	"context"
	"net/http"
	"sync"
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
	if item.Status != report.DisputeStatusPendingSellerResponse || item.NextActor != report.DisputeNextActorRespondent || item.DueAt == nil || item.ApplicantStatement != "服务有效期内无法使用。" || item.FactSnapshotJSON == "" {
		t.Fatalf("unexpected seller-first dispute: %+v", item)
	}
	var messageCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM api_order_dispute_messages
		WHERE dispute_case_id = $1
	`, item.ID).Scan(&messageCount); err != nil {
		t.Fatalf("count opening dispute messages: %v", err)
	}
	if messageCount != 0 {
		t.Fatalf("seller-first handling must not create an opening chat message, got %d", messageCount)
	}
}

func TestPostgresSellerAcceptanceAndVoluntaryRemedyEvidenceBindingsAreAtomic(t *testing.T) {
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
		SET dispute_case_id = $2, latest_dispute_case_id = $2, dispute_status = 'pending_seller_response', version = version + 1
		WHERE id = $1
	`, fixture.OrderID, dispute.ID); err != nil {
		_ = setupTx.Rollback(ctx)
		t.Fatal(err)
	}
	sellerDecisionAssetID := insertReadyEvidenceAsset(t, setupTx, fixture.OrderID, sellerID, now, now.Add(time.Hour))
	claimAssetID := insertReadyEvidenceAsset(t, setupTx, fixture.OrderID, sellerID, now, now.Add(time.Hour))
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupDisputeParticipantEvidenceFixture(t, store, fixture.OrderID, dispute.ID, sellerID, buyerID)
	})

	accepted, _, appErr := store.UpdateDisputeParticipantWithIdempotency(ctx,
		beginDisputeParticipantIdempotency(t, store, sellerID, "seller-decision", now),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeActionSellerDecision,
			Decision: report.SellerDecisionAccepted, Reason: "同意退款申请，将在线下完成退款。",
			RequestID: "postgres-dispute-seller-accept", EvidenceAssetIDs: []string{sellerDecisionAssetID},
		}, now, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("accept after-sales request: %v", appErr)
	}
	if accepted.Status != report.DisputeStatusVoluntaryFulfillment || accepted.SellerDecision != report.SellerDecisionAccepted ||
		accepted.SellerDecidedByUserID != sellerID || accepted.SellerDecidedAt == nil || accepted.SellerResponseLate ||
		accepted.NextActor != report.DisputeNextActorResponsibleParty || accepted.DueAt == nil || len(accepted.Remedies) != 1 ||
		accepted.Remedies[0].Source != report.RemedySourceSellerAcceptance || accepted.Remedies[0].Status != report.RemedyStatusPending {
		t.Fatalf("unexpected persisted seller acceptance: %+v", accepted)
	}
	remedyID := accepted.Remedies[0].ID
	assertDisputeEvidenceBinding(t, store, sellerDecisionAssetID, dispute.ID, evidence.UsageFormalResponse, evidence.SourceDisputeCase, dispute.ID)

	claimedAt := now.Add(time.Minute)
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

	confirmedAt := now.Add(2 * time.Minute)
	confirmed, _, appErr := store.UpdateDisputeParticipantWithIdempotency(ctx,
		beginDisputeParticipantIdempotency(t, store, buyerID, "confirm", confirmedAt),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: buyerID, Action: report.DisputeRemedyActionConfirm,
			Reason: "已经收到退款。", RequestID: "postgres-remedy-confirm",
		}, confirmedAt, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("confirm voluntary remedy: %v", appErr)
	}
	if confirmed.Status != report.DisputeStatusClosed || confirmed.Active || confirmed.FinalReason != "voluntary_fulfillment_confirmed" ||
		confirmed.AppealExpiresAt != nil || len(confirmed.AdverselyAffectedIDs) != 0 ||
		len(confirmed.Remedies) != 1 || confirmed.Remedies[0].Status != report.RemedyStatusConfirmed {
		t.Fatalf("unexpected confirmed voluntary remedy: %+v", confirmed)
	}
	var reputationOutcomeCount int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM dispute_reputation_outcomes WHERE dispute_case_id = $1`, dispute.ID).Scan(&reputationOutcomeCount); err != nil {
		t.Fatalf("count voluntary dispute reputation outcomes: %v", err)
	}
	if reputationOutcomeCount != 0 {
		t.Fatalf("seller acceptance must not create adverse reputation facts, got %d", reputationOutcomeCount)
	}
}

func TestPostgresSellerRejectionAllowsBuyerPlatformIntervention(t *testing.T) {
	store, dispute, orderID, sellerID, buyerID, now := seedPostgresSellerFirstDispute(t)

	rejected, _, appErr := store.UpdateDisputeParticipantWithIdempotency(context.Background(),
		beginDisputeParticipantIdempotency(t, store, sellerID, "seller-reject", now),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeActionSellerDecision,
			Decision: report.SellerDecisionRejected, Reason: "订单交付符合商品说明。", RequestID: "postgres-seller-reject",
		}, now, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("reject after-sales request: %v", appErr)
	}
	if rejected.Status != report.DisputeStatusPendingApplicantDecision || rejected.ApplicantDecisionDueAt == nil || rejected.NextActor != report.DisputeNextActorApplicant {
		t.Fatalf("unexpected seller rejection: %+v", rejected)
	}

	escalatedAt := now.Add(time.Minute)
	escalated, _, appErr := store.UpdateDisputeParticipantWithIdempotency(context.Background(),
		beginDisputeParticipantIdempotency(t, store, buyerID, "platform-intervention", escalatedAt),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: buyerID, Action: report.DisputeActionRequestPlatformIntervention,
			Reason: "不接受卖家的拒绝理由。", RequestID: "postgres-platform-intervention",
		}, escalatedAt, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("request platform intervention: %v", appErr)
	}
	if escalated.Status != report.DisputeStatusOpen || escalated.NextActor != report.DisputeNextActorAdmin || escalated.DueAt != nil || escalated.EscalatedAt == nil ||
		escalated.PlatformInterventionReason != "不接受卖家的拒绝理由。" {
		t.Fatalf("unexpected platform intervention: %+v", escalated)
	}
	adminDetail, appErr := store.GetAdminDispute(context.Background(), dispute.ID)
	if appErr != nil {
		t.Fatalf("read administrator dispute detail: %v", appErr)
	}
	if adminDetail.PlatformInterventionReason != "不接受卖家的拒绝理由。" {
		t.Fatalf("persisted platform intervention reason missing: %+v", adminDetail)
	}
	var orderDisputeStatus string
	if err := store.pool.QueryRow(context.Background(), `SELECT dispute_status FROM api_orders WHERE id = $1`, orderID).Scan(&orderDisputeStatus); err != nil {
		t.Fatalf("read escalated order projection: %v", err)
	}
	if orderDisputeStatus != apiorder.DisputeStatusOpen {
		t.Fatalf("unexpected escalated order dispute status: %s", orderDisputeStatus)
	}
}

func TestPostgresLateSellerDecisionWinsOnlyBeforePlatformIntervention(t *testing.T) {
	store, dispute, _, sellerID, buyerID, now := seedPostgresSellerFirstDispute(t)
	lateAt := now.Add(report.DisputeResponseWindow + time.Minute)

	lateDecision, _, appErr := store.UpdateDisputeParticipantWithIdempotency(context.Background(),
		beginDisputeParticipantIdempotency(t, store, sellerID, "late-seller-decision", lateAt),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeActionSellerDecision,
			Decision: report.SellerDecisionRejected, Reason: "逾期查看后仍拒绝申请。", RequestID: "postgres-late-seller-decision",
		}, lateAt, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("late seller decision before intervention: %v", appErr)
	}
	if !lateDecision.SellerResponseLate || lateDecision.Status != report.DisputeStatusPendingApplicantDecision {
		t.Fatalf("unexpected late seller decision: %+v", lateDecision)
	}

	_, _, appErr = store.UpdateDisputeParticipantWithIdempotency(context.Background(),
		beginDisputeParticipantIdempotency(t, store, buyerID, "late-platform-intervention", lateAt.Add(time.Minute)),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: buyerID, Action: report.DisputeActionRequestPlatformIntervention,
			Reason: "卖家已经逾期且拒绝。", RequestID: "postgres-late-platform-intervention",
		}, lateAt.Add(time.Minute), disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("request intervention after late rejection: %v", appErr)
	}
	_, _, appErr = store.UpdateDisputeParticipantWithIdempotency(context.Background(),
		beginDisputeParticipantIdempotency(t, store, sellerID, "decision-after-intervention", lateAt.Add(2*time.Minute)),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeActionSellerDecision,
			Decision: report.SellerDecisionAccepted, Reason: "平台介入后不能覆盖决定。", RequestID: "postgres-decision-after-intervention",
		}, lateAt.Add(2*time.Minute), disputeParticipantIntegrationCompletion)
	if appErr == nil || appErr.Status != http.StatusConflict || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("seller decision after intervention should conflict, got %v", appErr)
	}
}

func TestPostgresConcurrentSellerDecisionsHaveOneWinner(t *testing.T) {
	store, dispute, _, sellerID, _, now := seedPostgresSellerFirstDispute(t)
	entries := []idempotency.Entry{
		beginDisputeParticipantIdempotency(t, store, sellerID, "concurrent-accept", now),
		beginDisputeParticipantIdempotency(t, store, sellerID, "concurrent-reject", now),
	}
	decisions := []string{report.SellerDecisionAccepted, report.SellerDecisionRejected}
	type result struct {
		item   report.DisputeCase
		appErr *domain.AppError
	}
	results := make(chan result, len(decisions))
	var wait sync.WaitGroup
	for index := range decisions {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			item, _, appErr := store.UpdateDisputeParticipantWithIdempotency(context.Background(), entries[index], report.DisputeParticipantActionInput{
				DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeActionSellerDecision,
				Decision: decisions[index], Reason: "并发处理售后申请。", RequestID: "postgres-concurrent-" + decisions[index],
			}, now, disputeParticipantIntegrationCompletion)
			results <- result{item: item, appErr: appErr}
		}(index)
	}
	wait.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for result := range results {
		if result.appErr == nil {
			successes++
			if result.item.SellerDecision == "" {
				t.Fatalf("winning seller decision was not persisted: %+v", result.item)
			}
			continue
		}
		if result.appErr.Status == http.StatusConflict && result.appErr.Code == domain.CodeInvalidStateTransition {
			conflicts++
			continue
		}
		t.Fatalf("unexpected concurrent seller decision error: %v", result.appErr)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent seller decisions successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestPostgresExpiredApplicantDecisionClosesNeutrally(t *testing.T) {
	store, dispute, orderID, sellerID, _, now := seedPostgresSellerFirstDispute(t)
	_, _, appErr := store.UpdateDisputeParticipantWithIdempotency(context.Background(),
		beginDisputeParticipantIdempotency(t, store, sellerID, "seller-reject-expiry", now),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeActionSellerDecision,
			Decision: report.SellerDecisionRejected, Reason: "订单交付符合商品说明。", RequestID: "postgres-seller-reject-expiry",
		}, now, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("reject request before applicant expiry: %v", appErr)
	}

	expiredAt := now.Add(report.DisputeApplicantDecisionWindow + time.Minute)
	result, appErr := store.RunDataLifecycle(context.Background(), expiredAt, 10, lifecycleCredentialPolicy())
	if appErr != nil {
		t.Fatalf("expire applicant decision: %v", appErr)
	}
	if result.AfterSalesApplicationsExpired != 1 {
		t.Fatalf("expected one expired applicant decision, got %+v", result)
	}
	assertNeutralAfterSalesClosure(t, store, dispute.ID, orderID, "applicant_decision_expired")
}

func TestPostgresExpiredVoluntaryConfirmationClosesNeutrally(t *testing.T) {
	store, dispute, orderID, sellerID, _, now := seedPostgresSellerFirstDispute(t)
	accepted, _, appErr := store.UpdateDisputeParticipantWithIdempotency(context.Background(),
		beginDisputeParticipantIdempotency(t, store, sellerID, "seller-accept-expiry", now),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeActionSellerDecision,
			Decision: report.SellerDecisionAccepted, Reason: "同意申请并在线下退款。", RequestID: "postgres-seller-accept-expiry",
		}, now, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("accept request before confirmation expiry: %v", appErr)
	}
	claimedAt := now.Add(time.Minute)
	_, _, appErr = store.UpdateDisputeParticipantWithIdempotency(context.Background(),
		beginDisputeParticipantIdempotency(t, store, sellerID, "voluntary-claim-expiry", claimedAt),
		report.DisputeParticipantActionInput{
			DisputeID: dispute.ID, ActorUserID: sellerID, Action: report.DisputeRemedyActionClaim,
			Note: "退款已经完成。", RequestID: "postgres-voluntary-claim-expiry",
		}, claimedAt, disputeParticipantIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("claim voluntary remedy before confirmation expiry: %v", appErr)
	}

	expiredAt := claimedAt.Add(report.VoluntaryRemedyConfirmationWindow + time.Minute)
	result, appErr := store.RunDataLifecycle(context.Background(), expiredAt, 10, lifecycleCredentialPolicy())
	if appErr != nil {
		t.Fatalf("expire voluntary confirmation: %v", appErr)
	}
	if result.DisputeRemedyConfirmationsExpired != 1 {
		t.Fatalf("expected one expired voluntary confirmation, got %+v", result)
	}
	assertNeutralAfterSalesClosure(t, store, dispute.ID, orderID, "voluntary_confirmation_no_objection")
	if len(accepted.Remedies) != 1 {
		t.Fatalf("expected one voluntary remedy, got %+v", accepted.Remedies)
	}
	var remedyStatus string
	if err := store.pool.QueryRow(context.Background(), `SELECT status FROM api_order_dispute_remedies WHERE id = $1`, accepted.Remedies[0].ID).Scan(&remedyStatus); err != nil {
		t.Fatalf("read expired voluntary remedy: %v", err)
	}
	if remedyStatus != report.RemedyStatusConfirmationExpired {
		t.Fatalf("unexpected expired voluntary remedy status: %s", remedyStatus)
	}
}

func assertNeutralAfterSalesClosure(t *testing.T, store *Store, disputeID, orderID, finalReason string) {
	t.Helper()
	var status, actualFinalReason, orderDisputeStatus string
	var active, appealExpiresAtIsNull bool
	var adverselyAffectedCount, reputationOutcomeCount int
	if err := store.pool.QueryRow(context.Background(), `
		SELECT status, active, final_reason, appeal_expires_at IS NULL,
		       cardinality(adversely_affected_user_ids)
		FROM dispute_cases
		WHERE id = $1
	`, disputeID).Scan(&status, &active, &actualFinalReason, &appealExpiresAtIsNull, &adverselyAffectedCount); err != nil {
		t.Fatalf("read neutral after-sales closure: %v", err)
	}
	if err := store.pool.QueryRow(context.Background(), `SELECT dispute_status FROM api_orders WHERE id = $1`, orderID).Scan(&orderDisputeStatus); err != nil {
		t.Fatalf("read neutral order projection: %v", err)
	}
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM dispute_reputation_outcomes WHERE dispute_case_id = $1`, disputeID).Scan(&reputationOutcomeCount); err != nil {
		t.Fatalf("count neutral after-sales reputation outcomes: %v", err)
	}
	if status != report.DisputeStatusClosed || active || actualFinalReason != finalReason || !appealExpiresAtIsNull ||
		adverselyAffectedCount != 0 || reputationOutcomeCount != 0 || orderDisputeStatus != apiorder.DisputeStatusNone {
		t.Fatalf("unexpected neutral closure status=%q active=%t reason=%q appeal_null=%t affected=%d reputation=%d order=%q",
			status, active, actualFinalReason, appealExpiresAtIsNull, adverselyAffectedCount, reputationOutcomeCount, orderDisputeStatus)
	}
}

func seedPostgresSellerFirstDispute(t *testing.T) (*Store, report.DisputeCase, string, string, string, time.Time) {
	t.Helper()
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	completedAt := now.Add(-time.Hour)
	validityExpiresAt := now.Add(48 * time.Hour)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, completedAt.Add(-4*time.Hour))
	fixture := insertLifecycleCompletedCredentialOrder(
		t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID,
		completedAt, completedAt.Add(-30*time.Minute), "", nil,
	)

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin seller-first dispute setup: %v", err)
	}
	dispute, appErr := openDisputeFromAPIOrderInTx(ctx, tx, apiorder.Order{
		ID: fixture.OrderID, BuyerUserID: buyerID, SellerUserID: sellerID,
		Status: apiorder.StatusCompleted, DisputeStatus: apiorder.DisputeStatusNone,
		ServiceTitleSnapshot: "Seller-first dispute integration service", PackageExpiresAt: &validityExpiresAt,
	}, apiorder.ActionInput{
		ActorUserID: buyerID, IssueCode: apiorder.DisputeIssueServiceUnavailable,
		RequestedResolution: apiorder.DisputeResolutionFullRefund,
		IssueOccurredAt:     completedAt.Format(time.RFC3339), Reason: "服务有效期内无法使用。",
		RequestID: "seed-seller-first-dispute-" + uuid.NewString(),
	}, now)
	if appErr != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("open seller-first dispute: %v", appErr)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE api_orders
		SET dispute_case_id = $2, latest_dispute_case_id = $2,
		    dispute_status = 'pending_seller_response', version = version + 1
		WHERE id = $1
	`, fixture.OrderID, dispute.ID); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("set seller-first order projection: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit seller-first dispute setup: %v", err)
	}

	t.Cleanup(func() {
		cleanupDisputeParticipantEvidenceFixture(t, store, fixture.OrderID, dispute.ID, sellerID, buyerID)
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	return store, dispute, fixture.OrderID, sellerID, buyerID, now
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
