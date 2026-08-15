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
	return p.SetDisputeProjection(ctx, apiorder.DisputeProjection{CaseID: disputeCaseID, Status: apiorder.DisputeStatusClosed}, actorUserID, requestID)
}

func (p *negotiationProjection) SetDisputeProjection(_ context.Context, projection apiorder.DisputeProjection, _, _ string) *domain.AppError {
	p.statuses = append(p.statuses, projection.Status)
	return nil
}

func (p *negotiationProjection) ValidateDisputeProposalAmount(_ context.Context, _, resolution, amount string) *domain.AppError {
	return apiorder.ValidateRequestedDisputeAmount(resolution, amount, "100.00")
}

func TestNewAPIOrderDisputeStartsInPlatformHandling(t *testing.T) {
	now := time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	disputeID := registerNegotiationDispute(t, service, now)

	item, appErr := service.MyDispute(context.Background(), auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}, disputeID)
	if appErr != nil {
		t.Fatalf("read dispute: %+v", appErr)
	}
	if item.Status != DisputeStatusOpen || item.NextActor != DisputeNextActorRespondent || item.DueAt == nil {
		t.Fatalf("new dispute must wait for respondent directly: %+v", item)
	}
	if item.ApplicantStatement != "付款后尚未收到交付信息。" || len(item.Messages) != 0 || len(item.SettlementProposals) != 0 {
		t.Fatalf("new dispute must store the application without creating negotiation records: %+v", item)
	}
}

func TestRespondentFormalAnswerIsSingleAndImmutable(t *testing.T) {
	now := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	disputeID := registerNegotiationDispute(t, service, now)
	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	seller := auth.User{ID: "seller-1", Status: auth.AccountStatusActive}

	_, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), buyer, "buyer-answer", "buyer-answer", "buyer-answer", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeActionRespond, Body: "申请人不能代替被申请方答复。",
	}, negotiationCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("applicant answer must be rejected, got %+v", appErr)
	}

	answered := runNegotiationAction(t, service, seller, "seller-answer", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeActionRespond, Body: "交付延迟属实，已提交相关事实材料。",
	})
	if answered.RespondentResponse == "" || answered.RespondedAt == nil || answered.NextActor != DisputeNextActorAdmin || answered.DueAt != nil {
		t.Fatalf("formal answer must move the case to administrator review: %+v", answered)
	}

	_, appErr = service.DisputeParticipantActionWithIdempotency(context.Background(), seller, "seller-answer-again", "seller-answer-again", "seller-answer-again", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeActionRespond, Body: "不能覆盖第一次正式答复。",
	}, negotiationCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("second formal answer must be rejected, got %+v", appErr)
	}
}

func TestRespondentFormalAnswerExpiresAtDeadline(t *testing.T) {
	now := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	disputeID := registerNegotiationDispute(t, service, now)
	now = now.Add(DisputeResponseWindow)

	_, _, _, appErr := service.updateDisputeParticipantMemory(DisputeParticipantActionInput{
		DisputeID: disputeID, ActorUserID: "seller-1", Action: DisputeActionRespond, Body: "截止时间后的答复。",
	})
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected response deadline conflict, got %v", appErr)
	}
	item := service.disputes[disputeID]
	if item.RespondedAt != nil || item.NextActor != DisputeNextActorRespondent {
		t.Fatalf("expired response changed the case: %+v", item)
	}
}

func TestApplicantCannotWithdrawAfterPlatformRemedy(t *testing.T) {
	now := time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	disputeID := registerNegotiationDispute(t, service, now)
	item := service.disputes[disputeID]
	item.Status = DisputeStatusOpen
	item.NextActor = DisputeNextActorAdmin
	item.DueAt = nil
	service.disputes[disputeID] = item
	service.disputeRemedies[disputeID] = []DisputeRemedy{{
		ID: "remedy-1", DisputeCaseID: disputeID, Status: RemedyStatusContested,
		ResponsibleUserID: "seller-1", BeneficiaryUserID: "buyer-1", CreatedAt: now, UpdatedAt: now,
	}}

	_, _, _, appErr := service.updateDisputeParticipantMemory(DisputeParticipantActionInput{
		DisputeID: disputeID, ActorUserID: "buyer-1", Action: DisputeActionWithdraw, Reason: "尝试绕过平台裁决。",
	})
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected post-ruling withdrawal conflict, got %v", appErr)
	}
	if current := service.disputes[disputeID]; !current.Active || current.Status != DisputeStatusOpen {
		t.Fatalf("post-ruling withdrawal changed the case: %+v", current)
	}
}

func TestApplicantCanWithdrawOrConfirmOfflineResolution(t *testing.T) {
	for _, test := range []struct {
		name       string
		action     string
		wantStatus string
	}{
		{name: "withdraw", action: DisputeActionWithdraw, wantStatus: DisputeStatusWithdrawn},
		{name: "self resolve", action: DisputeActionSelfResolve, wantStatus: DisputeStatusSelfResolved},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
			service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
			projection := &negotiationProjection{}
			service.SetDisputeProjectionCloser(projection)
			disputeID := registerNegotiationDispute(t, service, now)
			closed := runNegotiationAction(t, service, auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}, test.name, DisputeParticipantActionInput{
				DisputeID: disputeID, Action: test.action, Reason: "双方已经在线下处理。",
			})
			if closed.Status != test.wantStatus || closed.Active || closed.NextActor != DisputeNextActorNone || closed.ClosedAt == nil {
				t.Fatalf("unexpected applicant terminal state: %+v", closed)
			}
			if len(projection.statuses) != 1 || projection.statuses[0] != apiorder.DisputeStatusClosed {
				t.Fatalf("order projection must close: %+v", projection.statuses)
			}
		})
	}
}

func TestNewFlowRejectsLegacyNegotiationWrites(t *testing.T) {
	now := time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	disputeID := registerNegotiationDispute(t, service, now)
	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	for name, input := range map[string]DisputeParticipantActionInput{
		"message":  {DisputeID: disputeID, Action: "append_message", Body: "旧留言动作不可用。"},
		"proposal": {DisputeID: disputeID, Action: "create_proposal"},
		"escalate": {DisputeID: disputeID, Action: "escalate"},
	} {
		t.Run(name, func(t *testing.T) {
			_, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), buyer, name, name, name, input, negotiationCompletion)
			if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
				t.Fatalf("legacy action must be rejected, got %+v", appErr)
			}
		})
	}
}

func registerNegotiationDispute(t *testing.T, service *Service, now time.Time) string {
	t.Helper()
	projection, appErr := service.RegisterAPIOrderDispute(context.Background(), apiorder.DisputeCaseInput{
		OrderID: "order-1", ServiceTitle: "测试 API", BuyerUserID: "buyer-1", SellerUserID: "seller-1",
		ActorUserID: "buyer-1", IssueCode: apiorder.DisputeIssueNotDelivered,
		RequestedResolution: apiorder.DisputeResolutionContinueFulfillment,
		Reason:              "付款后尚未收到交付信息。", RequestID: "open-dispute", Now: now,
	})
	if appErr != nil {
		t.Fatalf("register dispute: %+v", appErr)
	}
	return projection.CaseID
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
