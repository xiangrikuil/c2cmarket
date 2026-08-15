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
		FulfillmentRequired: true, ResponsibleUserID: seller.ID, BeneficiaryUserID: buyer.ID, DueAt: now.Add(time.Hour),
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
	if closed.Status != DisputeStatusResolved || len(closed.Remedies) != 1 || closed.SettlementProposals[0].AcceptedByUserID != seller.ID {
		t.Fatalf("counterparty confirmation must create a pending remedy: %+v", closed)
	}
	if len(projection.statuses) != 1 || projection.statuses[0] != apiorder.DisputeStatusAwaitingFulfillment {
		t.Fatalf("expected awaiting fulfillment projection, got %+v", projection.statuses)
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
		FulfillmentRequired: true, ResponsibleUserID: seller.ID, BeneficiaryUserID: buyer.ID, DueAt: now.Add(time.Hour),
	})
	proposalID := created.SettlementProposals[0].ID
	rejected := runNegotiationAction(t, service, seller, "proposal-reject", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionReject, ProposalID: proposalID, Reason: "无法按该时间履行。",
	})
	if rejected.Status != DisputeStatusNegotiating || rejected.SettlementProposals[0].Status != SettlementStatusRejected {
		t.Fatalf("seller rejection must not close or invalidate claim: %+v", rejected)
	}
	pending := runNegotiationAction(t, service, buyer, "proposal-create-before-escalation", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionPropose,
		Resolution: apiorder.DisputeResolutionOther, Terms: "双方无法继续达成一致，请平台介入。",
	})
	pendingProposalID := ""
	for _, proposal := range pending.SettlementProposals {
		if proposal.Status == SettlementStatusPending {
			pendingProposalID = proposal.ID
			break
		}
	}
	if pendingProposalID == "" {
		t.Fatalf("expected pending proposal before escalation: %+v", pending.SettlementProposals)
	}

	escalated := runNegotiationAction(t, service, seller, "proposal-escalate", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionEscalate,
		NegotiationChannels: []string{NegotiationChannelWeChat}, NegotiationEndedConfirmed: true,
		NegotiationSummary: "双方无法就履行时间达成一致。", RequestedPlatformAction: "请平台审核交付责任。",
	})
	if escalated.Status != DisputeStatusOpen || escalated.PublicResult != "平台审核中" ||
		!escalated.NegotiationEndedConfirmed || len(escalated.NegotiationChannels) != 1 || escalated.NegotiationChannels[0] != NegotiationChannelWeChat ||
		escalated.NegotiationSummary != "双方无法就履行时间达成一致。" || escalated.RequestedPlatformAction != "请平台审核交付责任。" ||
		escalated.EscalatedByUserID != seller.ID || escalated.EscalatedAt == nil {
		t.Fatalf("unexpected escalation result: %+v", escalated)
	}
	for _, proposal := range escalated.SettlementProposals {
		if proposal.ID == pendingProposalID && (proposal.Status != SettlementStatusSuperseded || proposal.SupersededReason != "platform_escalation") {
			t.Fatalf("pending proposal must record platform escalation supersession: %+v", proposal)
		}
	}
	if len(projection.statuses) != 1 || projection.statuses[0] != apiorder.DisputeStatusOpen {
		t.Fatalf("expected open projection, got %+v", projection.statuses)
	}

	lockedActions := []struct {
		name  string
		input DisputeParticipantActionInput
	}{
		{name: "message", input: DisputeParticipantActionInput{DisputeID: disputeID, Action: DisputeMessageActionAppend, Body: "介入后不能继续留言。"}},
		{name: "proposal", input: DisputeParticipantActionInput{DisputeID: disputeID, Action: DisputeMessageActionPropose, Resolution: apiorder.DisputeResolutionOther, Terms: "介入后不能新增方案。"}},
		{name: "confirm", input: DisputeParticipantActionInput{DisputeID: disputeID, Action: DisputeMessageActionConfirm, ProposalID: pendingProposalID}},
		{name: "reject", input: DisputeParticipantActionInput{DisputeID: disputeID, Action: DisputeMessageActionReject, ProposalID: pendingProposalID}},
	}
	for _, action := range lockedActions {
		t.Run("open rejects "+action.name, func(t *testing.T) {
			assertNegotiationActionConflict(t, service, buyer, "open-"+action.name, action.input)
		})
	}

	service.mu.Lock()
	waiting := service.disputes[disputeID]
	waiting.Status = DisputeStatusWaitingInfo
	service.disputes[disputeID] = waiting
	service.mu.Unlock()
	for _, action := range lockedActions {
		t.Run("waiting info rejects "+action.name, func(t *testing.T) {
			assertNegotiationActionConflict(t, service, buyer, "waiting-"+action.name, action.input)
		})
	}
}

func TestDisputeEscalationRequiresCompletedNegotiationContext(t *testing.T) {
	now := time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	disputeID := registerNegotiationDispute(t, service, now)
	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	valid := DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionEscalate,
		NegotiationChannels: []string{NegotiationChannelWeChat}, NegotiationEndedConfirmed: true,
		NegotiationSummary: "双方无法就履行时间达成一致。", RequestedPlatformAction: "请平台审核交付责任。",
	}
	tests := []struct {
		name  string
		field string
		edit  func(*DisputeParticipantActionInput)
	}{
		{name: "confirmation required", field: "negotiationEndedConfirmed", edit: func(input *DisputeParticipantActionInput) { input.NegotiationEndedConfirmed = false }},
		{name: "channel required", field: "negotiationChannels", edit: func(input *DisputeParticipantActionInput) { input.NegotiationChannels = nil }},
		{name: "duplicate channel", field: "negotiationChannels", edit: func(input *DisputeParticipantActionInput) {
			input.NegotiationChannels = []string{NegotiationChannelWeChat, NegotiationChannelWeChat}
		}},
		{name: "unsupported channel", field: "negotiationChannels", edit: func(input *DisputeParticipantActionInput) { input.NegotiationChannels = []string{"phone"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			input.NegotiationChannels = append([]string(nil), valid.NegotiationChannels...)
			test.edit(&input)
			_, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), buyer, test.name, test.name, test.name, input, negotiationCompletion)
			if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || appErr.Code != domain.CodeValidationFailed || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != test.field {
				t.Fatalf("expected validation error on %s, got %+v", test.field, appErr)
			}
		})
	}
}

func assertNegotiationActionConflict(t *testing.T, service *Service, user auth.User, key string, input DisputeParticipantActionInput) {
	t.Helper()
	_, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), user, key, key, key, input, negotiationCompletion)
	if appErr == nil || appErr.Status != http.StatusConflict || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected participant negotiation action conflict, got %+v", appErr)
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
