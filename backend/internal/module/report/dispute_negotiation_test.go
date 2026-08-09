package report

import (
	"context"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

type negotiationProjection struct {
	statuses []string
}

func (p *negotiationProjection) CloseDisputeProjection(ctx context.Context, disputeCaseID, actorUserID, requestID string) *domain.AppError {
	return p.SetDisputeProjection(ctx, disputeCaseID, apiorder.DisputeStatusClosed, actorUserID, requestID)
}

func (p *negotiationProjection) SetDisputeProjection(_ context.Context, _, status, _, _ string) *domain.AppError {
	p.statuses = append(p.statuses, status)
	return nil
}

func (p *negotiationProjection) ValidateDisputeProposalAmount(_ context.Context, _, resolution, amount string) *domain.AppError {
	return apiorder.ValidateRequestedDisputeAmount(resolution, amount, "100.00")
}

func TestDisputeNegotiationRequiresCounterpartyConfirmation(t *testing.T) {
	now := time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	projection := &negotiationProjection{}
	service.SetDisputeProjectionCloser(projection)
	disputeID := registerNegotiationDispute(t, service, now)

	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	seller := auth.User{ID: "seller-1", Status: auth.AccountStatusActive}
	created := runNegotiationAction(t, service, buyer, "proposal-create", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionPropose,
		Resolution: apiorder.DisputeResolutionPartialRefund, AmountCNY: "25.00", Terms: "退还 25 元，剩余额度继续使用。",
	})
	if created.Status != DisputeStatusNegotiating || len(created.SettlementProposals) != 1 {
		t.Fatalf("unexpected proposal result: %+v", created)
	}
	proposalID := created.SettlementProposals[0].ID

	_, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), buyer, "confirm", "proposal-self-confirm", "proposal-self-confirm", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionConfirm, ProposalID: proposalID,
	}, negotiationCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("proposer must not confirm own proposal, got %+v", appErr)
	}

	closed := runNegotiationAction(t, service, seller, "proposal-confirm", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionConfirm, ProposalID: proposalID,
	})
	if closed.Status != DisputeStatusClosed || closed.SettlementProposals[0].AcceptedByUserID != seller.ID {
		t.Fatalf("counterparty confirmation must close dispute: %+v", closed)
	}
	if len(projection.statuses) != 1 || projection.statuses[0] != apiorder.DisputeStatusClosed {
		t.Fatalf("expected one closed projection, got %+v", projection.statuses)
	}
}

func TestDisputeRejectionStaysNegotiatingAndEscalationLocksSettlement(t *testing.T) {
	now := time.Date(2026, 8, 9, 9, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	projection := &negotiationProjection{}
	service.SetDisputeProjectionCloser(projection)
	disputeID := registerNegotiationDispute(t, service, now)
	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	seller := auth.User{ID: "seller-1", Status: auth.AccountStatusActive}

	created := runNegotiationAction(t, service, buyer, "proposal-create-reject", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionPropose,
		Resolution: apiorder.DisputeResolutionContinueFulfillment, Terms: "卖家在今天内继续交付。",
	})
	proposalID := created.SettlementProposals[0].ID
	rejected := runNegotiationAction(t, service, seller, "proposal-reject", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionReject, ProposalID: proposalID, Reason: "无法按该时间履行。",
	})
	if rejected.Status != DisputeStatusNegotiating || rejected.SettlementProposals[0].Status != SettlementStatusRejected {
		t.Fatalf("seller rejection must not close or invalidate claim: %+v", rejected)
	}

	escalated := runNegotiationAction(t, service, seller, "proposal-escalate", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionEscalate, Reason: "双方无法就履行时间达成一致，请平台审核。",
	})
	if escalated.Status != DisputeStatusOpen || escalated.PublicResult != "平台审核中" {
		t.Fatalf("unexpected escalation result: %+v", escalated)
	}
	if len(projection.statuses) != 1 || projection.statuses[0] != apiorder.DisputeStatusOpen {
		t.Fatalf("expected open projection, got %+v", projection.statuses)
	}

	_, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), buyer, "confirm-after-escalation", "confirm-after-escalation", "confirm-after-escalation", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionConfirm, ProposalID: proposalID,
	}, negotiationCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("participant settlement must be locked after escalation, got %+v", appErr)
	}
}

func registerNegotiationDispute(t *testing.T, service *Service, now time.Time) string {
	t.Helper()
	id, appErr := service.RegisterAPIOrderDispute(context.Background(), apiorder.DisputeCaseInput{
		OrderID: "order-1", ServiceTitle: "测试 API", BuyerUserID: "buyer-1", SellerUserID: "seller-1",
		ActorUserID: "buyer-1", IssueCode: apiorder.DisputeIssueNotDelivered,
		RequestedResolution: apiorder.DisputeResolutionContinueFulfillment,
		Reason:              "付款后尚未收到交付信息。", RequestID: "open-dispute", Now: now,
	})
	if appErr != nil {
		t.Fatalf("register dispute: %+v", appErr)
	}
	return id
}

func runNegotiationAction(t *testing.T, service *Service, user auth.User, key string, input DisputeParticipantActionInput) DisputeCase {
	t.Helper()
	var result DisputeCase
	completion, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), user, key, key, key, input, func(item DisputeCase) (idempotency.Completion, *domain.AppError) {
		result = item
		return negotiationCompletion(item)
	})
	if appErr != nil {
		t.Fatalf("action %s: %+v", key, appErr)
	}
	if completion.Status != http.StatusOK {
		t.Fatalf("action %s status = %d", key, completion.Status)
	}
	return result
}

func negotiationCompletion(DisputeCase) (idempotency.Completion, *domain.AppError) {
	return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`)}, nil
}
