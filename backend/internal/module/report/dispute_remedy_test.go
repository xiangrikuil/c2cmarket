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

func TestDisputeRemedyRequiresResponsibleClaimAndBeneficiaryConfirmation(t *testing.T) {
	now := time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)
	notifications := notification.NewService(nil, func() time.Time { return now })
	service := NewServiceWithNotifications(nil, idempotency.NewService(nil, func() time.Time { return now }), notifications, func() time.Time { return now })
	projection := &negotiationProjection{}
	service.SetDisputeProjectionCloser(projection)
	disputeID := registerNegotiationDispute(t, service, now)
	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	seller := auth.User{ID: "seller-1", Status: auth.AccountStatusActive}
	admin := auth.User{ID: "admin-1", IsAdmin: true, Status: auth.AccountStatusActive}

	escalated := runNegotiationAction(t, service, buyer, "remedy-escalate", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionEscalate,
		NegotiationChannels: []string{NegotiationChannelEmail}, NegotiationEndedConfirmed: true,
		NegotiationSummary: "双方无法就继续履行达成一致。", RequestedPlatformAction: "请平台裁决继续履行事项。",
	})
	projection.statuses = nil
	resolved := runRemedyAdminAction(t, service, admin, "remedy-resolve", AdminActionInput{
		ID: disputeID, Action: "resolve", ExpectedVersion: escalated.Version,
		Reason: "平台认定卖家需要继续履行。", PublicResultCode: PublicResultAPIDeliveryIssue,
		PublicResult: "卖家应按裁决继续履行",
		Remedy: &DisputeRemedyInput{
			Action: apiorder.DisputeResolutionContinueFulfillment, ResponsibleUserID: seller.ID,
			Instructions: "请在期限内继续完成本订单交付。", DueAt: now.Add(24 * time.Hour),
		},
	})
	if resolved.Status != DisputeStatusResolved || len(resolved.Remedies) != 1 || resolved.Remedies[0].Status != RemedyStatusPending {
		t.Fatalf("unexpected remedy resolution: %+v", resolved)
	}
	if len(projection.statuses) != 1 || projection.statuses[0] != apiorder.DisputeStatusAwaitingFulfillment {
		t.Fatalf("resolution must enter awaiting fulfillment: %+v", projection.statuses)
	}

	_, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), buyer, "buyer-claim", "buyer-claim", "buyer-claim", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeRemedyActionClaim, Note: "买家不能代替责任方声明履行。",
	}, negotiationCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("beneficiary must not claim fulfillment, got %+v", appErr)
	}

	claimed := runNegotiationAction(t, service, seller, "seller-remedy-claim", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeRemedyActionClaim, Note: "已按裁决补充交付，请买家核对。",
	})
	if claimed.Status != DisputeStatusResolved || claimed.Remedies[0].Status != RemedyStatusClaimedFulfilled || claimed.ClosedAt != nil {
		t.Fatalf("claim must wait for beneficiary instead of closing: %+v", claimed)
	}
	if projection.statuses[len(projection.statuses)-1] != apiorder.DisputeStatusFulfillmentConfirmation {
		t.Fatalf("claim must enter fulfillment confirmation: %+v", projection.statuses)
	}

	confirmed := runNegotiationAction(t, service, buyer, "buyer-remedy-confirm", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeRemedyActionConfirm,
	})
	if confirmed.Status != DisputeStatusClosed || confirmed.Remedies[0].Status != RemedyStatusConfirmed {
		t.Fatalf("beneficiary confirmation must close remedy and dispute: %+v", confirmed)
	}
	if projection.statuses[len(projection.statuses)-1] != apiorder.DisputeStatusClosed {
		t.Fatalf("beneficiary confirmation must close order projection: %+v", projection.statuses)
	}
	buyerNotifications, appErr := notifications.List(context.Background(), buyer.ID, domain.PageRequest{Limit: 20})
	if appErr != nil || len(buyerNotifications.Items) != 2 {
		t.Fatalf("buyer must receive ruling and claim notifications: items=%+v err=%+v", buyerNotifications.Items, appErr)
	}
	sellerNotifications, appErr := notifications.List(context.Background(), seller.ID, domain.PageRequest{Limit: 20})
	if appErr != nil || len(sellerNotifications.Items) != 2 {
		t.Fatalf("seller must receive ruling and confirmation notifications: items=%+v err=%+v", sellerNotifications.Items, appErr)
	}
}

func TestDisputeRemedyConfirmValidationReportsReasonField(t *testing.T) {
	t.Parallel()

	appErr := validateDisputeParticipantAction(DisputeParticipantActionInput{
		DisputeID: "dispute-1",
		Action:    DisputeRemedyActionConfirm,
		Reason:    "x",
	})
	if appErr == nil || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "reason" {
		t.Fatalf("confirmation validation must report the request reason field, got %+v", appErr)
	}
}

func TestDisputeRemedyContestReturnsToPlatformReview(t *testing.T) {
	now := time.Date(2026, 8, 9, 11, 0, 0, 0, time.UTC)
	service, projection, disputeID, buyer, seller, _ := setupClaimedRemedy(t, &now)

	contested := runNegotiationAction(t, service, buyer, "buyer-remedy-contest", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeRemedyActionContest, Reason: "仍未收到裁决要求的交付内容。",
	})
	if contested.Status != DisputeStatusOpen || contested.Remedies[0].Status != RemedyStatusContested {
		t.Fatalf("contest must return dispute to platform review: %+v", contested)
	}
	if contested.Remedies[0].ResponseNote == "" || projection.statuses[len(projection.statuses)-1] != apiorder.DisputeStatusOpen {
		t.Fatalf("contest reason and open projection must be retained: %+v %+v", contested.Remedies[0], projection.statuses)
	}

	_, appErr := service.DisputeParticipantActionWithIdempotency(context.Background(), seller, "seller-contest", "seller-contest", "seller-contest", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeRemedyActionContest, Reason: "责任方不能否认自己的履行声明。",
	}, negotiationCompletion)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("responsible party must not contest its own claim, got %+v", appErr)
	}
}

func TestDisputeRemedyConfirmationDeadlineClosesNeutrally(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	service, projection, disputeID, buyer, _, _ := setupClaimedRemedy(t, &now)
	now = now.Add(RemedyConfirmationWindow)

	expired := runNegotiationAction(t, service, buyer, "buyer-confirm-at-deadline", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeRemedyActionConfirm,
	})
	if expired.Status != DisputeStatusClosed || expired.PublicResult != RemedyConfirmationExpiredPublicResult {
		t.Fatalf("deadline must close with neutral result: %+v", expired)
	}
	if expired.Remedies[0].Status != RemedyStatusConfirmationExpired || expired.Remedies[0].ConfirmedAt != nil {
		t.Fatalf("deadline must not be recorded as beneficiary confirmation: %+v", expired.Remedies[0])
	}
	if projection.statuses[len(projection.statuses)-1] != apiorder.DisputeStatusClosed {
		t.Fatalf("deadline must close order projection: %+v", projection.statuses)
	}
}

func TestDisputeRemedyDetailReadNormalizesExpiredConfirmation(t *testing.T) {
	now := time.Date(2026, 8, 9, 13, 0, 0, 0, time.UTC)
	service, projection, disputeID, buyer, _, _ := setupClaimedRemedy(t, &now)
	now = now.Add(RemedyConfirmationWindow + time.Second)

	detail, appErr := service.MyDispute(context.Background(), buyer, disputeID)
	if appErr != nil {
		t.Fatalf("read expired remedy: %+v", appErr)
	}
	if detail.Status != DisputeStatusClosed || detail.Remedies[0].Status != RemedyStatusConfirmationExpired {
		t.Fatalf("detail read must normalize expiration in memory mode: %+v", detail)
	}
	if projection.statuses[len(projection.statuses)-1] != apiorder.DisputeStatusClosed {
		t.Fatalf("detail normalization must close order projection: %+v", projection.statuses)
	}
}

func TestActiveDisputeRemedyLatenessDecisionPreservesFulfillmentProgress(t *testing.T) {
	now := time.Date(2026, 8, 9, 14, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	projection := &negotiationProjection{}
	service.SetDisputeProjectionCloser(projection)
	disputeID := registerNegotiationDispute(t, service, now)
	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	admin := auth.User{ID: "admin-1", IsAdmin: true, Status: auth.AccountStatusActive}
	escalated := runNegotiationAction(t, service, buyer, "overdue-escalate", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionEscalate,
		NegotiationChannels: []string{NegotiationChannelInSite}, NegotiationEndedConfirmed: true,
		NegotiationSummary: "双方无法就整改期限达成一致。", RequestedPlatformAction: "请平台确认整改期限。",
	})
	dueAt := now.Add(2 * time.Hour)
	resolved := runRemedyAdminAction(t, service, admin, "overdue-resolve", AdminActionInput{
		ID: disputeID, Action: "resolve", ExpectedVersion: escalated.Version,
		Reason: "平台裁决卖家退款。", PublicResultCode: PublicResultAPIDeliveryIssue,
		PublicResult: "卖家应按裁决退款",
		Remedy: &DisputeRemedyInput{
			Action: apiorder.DisputeResolutionFullRefund, ResponsibleUserID: "seller-1",
			Instructions: "请按原站外支付方式完成退款。", DueAt: dueAt,
		},
	})

	for key, action := range map[string]string{"close-active-remedy": "close", "overdue-before-deadline": "confirm_lateness"} {
		_, appErr := service.AdminDisputeActionWithIdempotency(context.Background(), admin, key, key, key, AdminActionInput{
			ID: disputeID, Action: action, ExpectedVersion: resolved.Version, Reason: "管理员执行整改状态检查。",
		}, adminMutationCompletion)
		if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
			t.Fatalf("%s must be rejected, got %+v", action, appErr)
		}
	}

	now = dueAt
	overdue := runRemedyAdminAction(t, service, admin, "confirm-lateness-at-deadline", AdminActionInput{
		ID: disputeID, Action: "confirm_lateness", ExpectedVersion: resolved.Version,
		Reason: "责任方未在裁决期限内履行。",
	})
	if overdue.Status != DisputeStatusResolved || overdue.Remedies[0].Status != RemedyStatusPending || overdue.Remedies[0].LatenessStatus != RemedyLatenessLateConfirmed {
		t.Fatalf("administrator lateness confirmation must preserve fulfillment progress: %+v", overdue)
	}
	if !overdue.Active || overdue.ClosedAt != nil || overdue.FinalReason != "" {
		t.Fatalf("lateness confirmation must not finalize the dispute: %+v", overdue)
	}

	claimed := runNegotiationAction(t, service, auth.User{ID: "seller-1", Status: auth.AccountStatusActive}, "claim-after-confirmed-lateness", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeRemedyActionClaim, Note: "已在逾期裁定后补充履行。",
	})
	if claimed.Remedies[0].Status != RemedyStatusClaimedFulfilled || claimed.Remedies[0].LatenessStatus != RemedyLatenessLateConfirmed {
		t.Fatalf("late fulfillment claim must preserve the independent lateness decision: %+v", claimed.Remedies[0])
	}
}

func TestActiveDisputeRemedyLatenessExcusePreservesPendingProgress(t *testing.T) {
	now := time.Date(2026, 8, 9, 14, 0, 0, 0, time.UTC)
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	disputeID := registerNegotiationDispute(t, service, now)
	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	admin := auth.User{ID: "admin-1", IsAdmin: true, Status: auth.AccountStatusActive}
	escalated := runNegotiationAction(t, service, buyer, "excuse-escalate", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionEscalate,
		NegotiationChannels: []string{NegotiationChannelLinuxDO}, NegotiationEndedConfirmed: true,
		NegotiationSummary: "双方无法就整改期限达成一致。", RequestedPlatformAction: "请平台确认整改期限。",
	})
	dueAt := now.Add(time.Hour)
	resolved := runRemedyAdminAction(t, service, admin, "excuse-resolve", AdminActionInput{
		ID: disputeID, Action: "resolve", ExpectedVersion: escalated.Version,
		Reason: "平台裁决卖家退款。", PublicResultCode: PublicResultAPIDeliveryIssue,
		PublicResult: "卖家应按裁决退款",
		Remedy: &DisputeRemedyInput{
			Action: apiorder.DisputeResolutionFullRefund, ResponsibleUserID: "seller-1",
			Instructions: "请按原站外支付方式完成退款。", DueAt: dueAt,
		},
	})
	now = dueAt
	excused := runRemedyAdminAction(t, service, admin, "excuse-lateness-at-deadline", AdminActionInput{
		ID: disputeID, Action: "excuse_lateness", ExpectedVersion: resolved.Version,
		Reason: "责任方提供了可核实的客观延期原因。",
	})
	if excused.Status != DisputeStatusResolved || excused.Remedies[0].Status != RemedyStatusPending || excused.Remedies[0].LatenessStatus != RemedyLatenessLateExcused {
		t.Fatalf("administrator lateness excuse must preserve pending progress: %+v", excused)
	}
}

func setupClaimedRemedy(t *testing.T, now *time.Time) (*Service, *negotiationProjection, string, auth.User, auth.User, auth.User) {
	t.Helper()
	service := NewService(nil, idempotency.NewService(nil, func() time.Time { return *now }), func() time.Time { return *now })
	projection := &negotiationProjection{}
	service.SetDisputeProjectionCloser(projection)
	disputeID := registerNegotiationDispute(t, service, *now)
	buyer := auth.User{ID: "buyer-1", Status: auth.AccountStatusActive}
	seller := auth.User{ID: "seller-1", Status: auth.AccountStatusActive}
	admin := auth.User{ID: "admin-1", IsAdmin: true, Status: auth.AccountStatusActive}
	escalated := runNegotiationAction(t, service, buyer, "setup-remedy-escalate", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeMessageActionEscalate,
		NegotiationChannels: []string{NegotiationChannelWeChat, NegotiationChannelEmail}, NegotiationEndedConfirmed: true,
		NegotiationSummary: "双方无法自行解决履行争议。", RequestedPlatformAction: "请平台裁决。",
	})
	runRemedyAdminAction(t, service, admin, "setup-remedy-resolve", AdminActionInput{
		ID: disputeID, Action: "resolve", ExpectedVersion: escalated.Version,
		Reason: "平台认定卖家需要继续履行。", PublicResultCode: PublicResultAPIDeliveryIssue,
		PublicResult: "卖家应按裁决继续履行",
		Remedy: &DisputeRemedyInput{
			Action: apiorder.DisputeResolutionContinueFulfillment, ResponsibleUserID: seller.ID,
			Instructions: "请继续完成本订单交付。", DueAt: now.Add(24 * time.Hour),
		},
	})
	runNegotiationAction(t, service, seller, "setup-remedy-claim", DisputeParticipantActionInput{
		DisputeID: disputeID, Action: DisputeRemedyActionClaim, Note: "已继续履行，请买家确认。",
	})
	return service, projection, disputeID, buyer, seller, admin
}

func runRemedyAdminAction(t *testing.T, service *Service, user auth.User, key string, input AdminActionInput) DisputeCase {
	t.Helper()
	input.RequestID = key
	var result DisputeCase
	completion, appErr := service.AdminDisputeActionWithIdempotency(context.Background(), user, key, key, key, input, func(item MutationResult) (idempotency.Completion, *domain.AppError) {
		if item.Dispute != nil {
			result = *item.Dispute
		}
		return adminMutationCompletion(item)
	})
	if appErr != nil {
		t.Fatalf("admin action %s: %+v", key, appErr)
	}
	if completion.Status != http.StatusOK || result.ID == "" {
		t.Fatalf("admin action %s returned status=%d result=%+v", key, completion.Status, result)
	}
	return result
}

func adminMutationCompletion(MutationResult) (idempotency.Completion, *domain.AppError) {
	return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`)}, nil
}
