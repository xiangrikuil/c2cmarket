package contact

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/reputation"
)

type actionCheckCall struct {
	userID string
	role   string
	action string
}

type recordingActionChecker struct {
	calls []actionCheckCall
	err   *domain.AppError
}

func (c *recordingActionChecker) CheckActionAllowed(_ context.Context, userID, role, action string) *domain.AppError {
	c.calls = append(c.calls, actionCheckCall{userID: userID, role: role, action: action})
	return c.err
}

func TestReadSessionChecksRoleBeforeWritingAccessLog(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	buyerMethod, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID:  "buyer-1",
		Type:    "telegram",
		Label:   "买家 Telegram",
		Value:   "@buyer",
		Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create buyer contact: %v", appErr)
	}
	sellerMethod, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID:  "seller-1",
		Type:    "telegram",
		Label:   "卖家 Telegram",
		Value:   "@seller",
		Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create seller contact: %v", appErr)
	}
	session, appErr := service.CreateSession(context.Background(), CreateContactSessionInput{
		BuyerUserID:           "buyer-1",
		SellerUserID:          "seller-1",
		BuyerContactMethodID:  buyerMethod.ID,
		SellerContactMethodID: sellerMethod.ID,
		Duration:              time.Hour,
	})
	if appErr != nil {
		t.Fatalf("create contact session: %v", appErr)
	}

	checker := &recordingActionChecker{
		err: domain.NewError(403, domain.CodeReputationActionRestricted, "Reputation action restricted", "暂不可查看联系方式。"),
	}
	service.SetActionChecker(checker)
	if _, appErr := service.ReadSession(context.Background(), session.ID, "buyer-1", "request-buyer"); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected buyer contact restriction, got %#v", appErr)
	}
	if count := service.AccessLogCount(context.Background(), session.ID); count != 0 {
		t.Fatalf("rejected read must not write access log, got %d", count)
	}
	if len(checker.calls) != 1 || checker.calls[0].role != reputation.RoleBuyer || checker.calls[0].action != reputation.ActionContactView {
		t.Fatalf("unexpected buyer action check: %#v", checker.calls)
	}

	checker.err = nil
	view, appErr := service.ReadSession(context.Background(), session.ID, "seller-1", "request-seller")
	if appErr != nil {
		t.Fatalf("read seller contact session: %v", appErr)
	}
	if len(view.Items) != 2 || view.Items[0].Value == "" || view.Items[1].Value == "" {
		t.Fatalf("expected two disclosed contact values, got %#v", view.Items)
	}
	if count := service.AccessLogCount(context.Background(), session.ID); count != 1 {
		t.Fatalf("allowed read must write one access log, got %d", count)
	}
	if len(checker.calls) != 2 || checker.calls[1].userID != "seller-1" || checker.calls[1].role != reputation.RoleSeller || checker.calls[1].action != reputation.ActionContactView {
		t.Fatalf("unexpected seller action check: %#v", checker.calls)
	}
}

func TestEnsureLinuxDoMethodReusesIdentityManagedMapping(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	if _, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "user-1", Type: "wechat", Label: "微信", Value: "wechat-user-1", Enabled: true, IsDefault: true,
	}); appErr != nil {
		t.Fatalf("create manual contact: %v", appErr)
	}

	first, appErr := service.EnsureLinuxDoMethod(context.Background(), "user-1", "bound-user")
	if appErr != nil {
		t.Fatalf("ensure linux.do mapping: %v", appErr)
	}
	second, appErr := service.EnsureLinuxDoMethod(context.Background(), "user-1", "@renamed-user")
	if appErr != nil {
		t.Fatalf("refresh linux.do mapping: %v", appErr)
	}
	if first.ID != second.ID || second.DisplayValue != "@renamed-user" || second.Type != "linuxdo" {
		t.Fatalf("identity mapping was not reused: first=%+v second=%+v", first, second)
	}

	methods, appErr := service.ListMethods(context.Background(), "user-1")
	if appErr != nil {
		t.Fatalf("list contacts: %v", appErr)
	}
	linuxDoCount := 0
	for _, method := range methods {
		if method.Type == "linuxdo" && method.Enabled {
			linuxDoCount++
		}
	}
	if linuxDoCount != 1 {
		t.Fatalf("expected one enabled linux.do mapping, got %d methods=%+v", linuxDoCount, methods)
	}
}
