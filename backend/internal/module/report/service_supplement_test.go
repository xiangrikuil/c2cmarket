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
	"c2c-market/backend/internal/module/notification"
)

type failingSupplementProjection struct{}

func (failingSupplementProjection) CloseDisputeProjection(context.Context, string, string, string) *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "projection failed")
}

func (failingSupplementProjection) SetDisputeProjection(context.Context, apiorder.DisputeProjection, string, string) *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "projection failed")
}

func (failingSupplementProjection) ValidateDisputeProposalAmount(context.Context, string, string, string) *domain.AppError {
	return nil
}

func TestInfoSupplementRequiresDesignatedParticipantAndReplaysIdempotently(t *testing.T) {
	now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	notifications := notification.NewService(nil, func() time.Time { return now })
	service := NewServiceWithNotifications(nil, idempotency.NewService(nil, func() time.Time { return now }), notifications, func() time.Time { return now })
	admin := auth.User{ID: "10000000-0000-4000-8000-000000000001", IsAdmin: true, Status: auth.AccountStatusActive}
	primary := auth.User{ID: "20000000-0000-4000-8000-000000000001", Username: "seller", DisplayName: "Seller", Status: auth.AccountStatusActive}
	counterparty := auth.User{ID: "30000000-0000-4000-8000-000000000001", Username: "buyer", DisplayName: "Buyer", Status: auth.AccountStatusActive}
	disputeID := "40000000-0000-4000-8000-000000000001"
	service.disputes[disputeID] = DisputeCase{
		ID: disputeID, PrimaryUserID: primary.ID, CounterpartyUserID: counterparty.ID,
		Status: DisputeStatusOpen, PublicSummary: "订单说明争议", PublicResultCode: PublicResultNoAction,
		OpenedAt: now, CreatedAt: now, UpdatedAt: now, Version: 1,
	}

	completion, appErr := service.AdminDisputeActionWithIdempotency(context.Background(), admin,
		"POST /api/v1/admin/disputes/{id}/request_info:"+disputeID, "request-info-1", "hash-1",
		AdminActionInput{ID: disputeID, Action: "request_info", Reason: "请买家补充脱敏事实", RequestedFromID: counterparty.ID, ExpectedVersion: 1},
		mutationCompletionForSupplementTest)
	if appErr != nil || completion.Status != http.StatusOK {
		t.Fatalf("request info: completion=%+v err=%v", completion, appErr)
	}
	requested := service.disputes[disputeID]
	if requested.Status != DisputeStatusWaitingInfo || requested.OpenInfoRequestID == "" || requested.InfoRequestedFromID != counterparty.ID || requested.NextActor != DisputeNextActorRespondent || requested.DueAt == nil || !requested.DueAt.Equal(now.Add(DisputeInfoRequestWindow)) {
		t.Fatalf("missing designated open request: %+v", requested)
	}

	_, appErr = service.SubmitInfoSupplementWithIdempotency(context.Background(), primary,
		"POST /api/v1/me/disputes/{id}/supplements:"+disputeID, "wrong-user-1", "wrong-user-hash",
		SupplementInput{EntityType: InfoRequestEntityDispute, EntityID: disputeID, InfoRequestID: requested.OpenInfoRequestID, Body: "这是未被指定用户提交的说明。"},
		mutationCompletionForSupplementTest)
	if appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("expected non-designated participant to be hidden as not found, got %v", appErr)
	}

	_, appErr = service.SubmitInfoSupplementWithIdempotency(context.Background(), counterparty,
		"POST /api/v1/me/disputes/{id}/supplements:"+disputeID, "secret-1", "secret-hash",
		SupplementInput{EntityType: InfoRequestEntityDispute, EntityID: disputeID, InfoRequestID: requested.OpenInfoRequestID, Body: "api_key=secret-value"},
		mutationCompletionForSupplementTest)
	if appErr == nil || appErr.Code != domain.CodeSecretContentDetected {
		t.Fatalf("expected credential-like text to be rejected, got %v", appErr)
	}

	routeKey := "POST /api/v1/me/disputes/{id}/supplements:" + disputeID
	input := SupplementInput{EntityType: InfoRequestEntityDispute, EntityID: disputeID, InfoRequestID: requested.OpenInfoRequestID, Body: "订单页面显示的状态与付款记录时间不一致，请复核。"}
	first, appErr := service.SubmitInfoSupplementWithIdempotency(context.Background(), counterparty, routeKey, "supplement-1", "supplement-hash", input, mutationCompletionForSupplementTest)
	if appErr != nil {
		t.Fatalf("submit supplement: %v", appErr)
	}
	replay, appErr := service.SubmitInfoSupplementWithIdempotency(context.Background(), counterparty, routeKey, "supplement-1", "supplement-hash", input, mutationCompletionForSupplementTest)
	if appErr != nil || string(replay.Body) != string(first.Body) {
		t.Fatalf("idempotent replay mismatch: first=%+v replay=%+v err=%v", first, replay, appErr)
	}
	updated := service.disputes[disputeID]
	if updated.Status != DisputeStatusOpen || updated.NextActor != DisputeNextActorAdmin || updated.DueAt != nil || updated.OpenInfoRequestID != "" || updated.Version != 3 {
		t.Fatalf("supplement must answer request without resolving case: %+v", updated)
	}
	adminDetail, appErr := service.AdminDispute(context.Background(), admin, disputeID)
	if appErr != nil || len(adminDetail.Supplements) != 1 {
		t.Fatalf("admin detail supplement missing: detail=%+v err=%v", adminDetail, appErr)
	}
	supplement := adminDetail.Supplements[0]
	if supplement.Body != input.Body || supplement.SubmittedByUserID != counterparty.ID || supplement.SubmittedByUsername != counterparty.Username || supplement.CreatedAt != now {
		t.Fatalf("admin detail supplement incomplete: %+v", supplement)
	}
	adminNotifications, notifyErr := notifications.List(context.Background(), admin.ID, domain.PageRequest{Limit: 10})
	if notifyErr != nil || len(adminNotifications.Items) != 1 || adminNotifications.Items[0].SourceEventType != "moderation.info_supplemented" {
		t.Fatalf("requesting admin notification missing: page=%+v err=%v", adminNotifications, notifyErr)
	}
}

func mutationCompletionForSupplementTest(result MutationResult) (idempotency.Completion, *domain.AppError) {
	resourceID := ""
	if result.Report != nil {
		resourceID = result.Report.ID
	}
	if result.Dispute != nil {
		resourceID = result.Dispute.ID
	}
	return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{"ok":true}`), ResourceType: "moderation_case", ResourceID: resourceID}, nil
}

func TestInfoSupplementRollsBackMemoryWhenOrderProjectionFails(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	participant := auth.User{ID: "30000000-0000-4000-8000-000000000001", Status: auth.AccountStatusActive}
	disputeID := "40000000-0000-4000-8000-000000000001"
	requestID := "50000000-0000-4000-8000-000000000001"
	service.disputes[disputeID] = DisputeCase{
		ID: disputeID, TargetType: TargetAPIOrder, PrimaryUserID: "20000000-0000-4000-8000-000000000001", CounterpartyUserID: participant.ID,
		Status: DisputeStatusWaitingInfo, Active: true, NextActor: DisputeNextActorRespondent, OpenInfoRequestID: requestID,
		InfoRequestedFromID: participant.ID, OpenedAt: now, CreatedAt: now, UpdatedAt: now, Version: 2,
	}
	service.infoRequests[requestID] = InfoRequest{
		ID: requestID, EntityType: InfoRequestEntityDispute, EntityID: disputeID, RequestedFromID: participant.ID,
		RequestedByAdminID: "10000000-0000-4000-8000-000000000001", Status: InfoRequestStatusOpen, RequestedAt: now,
	}
	service.SetDisputeProjectionCloser(failingSupplementProjection{})

	_, appErr := service.SubmitInfoSupplementWithIdempotency(context.Background(), participant,
		"POST /api/v1/me/disputes/{id}/supplements:"+disputeID, "rollback-1", "rollback-hash",
		SupplementInput{EntityType: InfoRequestEntityDispute, EntityID: disputeID, InfoRequestID: requestID, Body: "补充脱敏订单事实。"},
		mutationCompletionForSupplementTest)
	if appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected projection failure, got %v", appErr)
	}
	item := service.disputes[disputeID]
	request := service.infoRequests[requestID]
	if item.Status != DisputeStatusWaitingInfo || item.NextActor != DisputeNextActorRespondent || item.OpenInfoRequestID != requestID || item.Version != 2 {
		t.Fatalf("dispute mutation was not rolled back: %+v", item)
	}
	if request.Status != InfoRequestStatusOpen || request.AnsweredAt != nil {
		t.Fatalf("info request mutation was not rolled back: %+v", request)
	}
	if supplements := service.infoSupplements[infoSupplementEntityKey(InfoRequestEntityDispute, disputeID)]; len(supplements) != 0 {
		t.Fatalf("supplement append was not rolled back: %+v", supplements)
	}
}

func TestWithSupplementBusinessActorPreservesDisplayIdentity(t *testing.T) {
	actor := auth.BusinessActor{
		UserID: "30000000-0000-4000-8000-000000000019", Username: "buyer-19", DisplayName: "Buyer Nineteen",
		Audience: auth.SessionAudienceNormal,
	}
	input := WithSupplementBusinessActor(SupplementInput{}, actor)
	if input.SubmittingUserID != actor.UserID || input.SubmittingUsername != actor.Username || input.SubmittingName != actor.DisplayName {
		t.Fatalf("supplement display identity lost: %+v", input)
	}
}
