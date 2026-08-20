package contact

import (
	"context"
	"net/http"
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
		UserID: "buyer-1", Type: MethodTypeWechat, Label: "买家微信", Value: "buyer-wechat", Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create buyer contact: %v", appErr)
	}
	sellerMethod, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "seller-1", Type: MethodTypeWechat, Label: "卖家微信", Value: "seller-wechat", Enabled: true,
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
		UserID: "user-1", Type: MethodTypeWechat, Label: "微信", Value: "wechat-user-1", Enabled: true, IsDefault: true,
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
	if _, _, ok := service.TransactionVersionForOwner(second.ID, "user-1"); ok {
		t.Fatal("linux.do identity mapping must not be transaction eligible")
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

func TestTransactionContactEligibilityInMemory(t *testing.T) {
	t.Parallel()

	service := NewService(nil, func() time.Time { return time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC) })
	email, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "user-contact", Type: MethodTypeEmail, Label: "邮箱", Value: "user@example.com", Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create email contact: %v", appErr)
	}
	if _, _, ok := service.TransactionVersionForOwner(email.ID, "user-contact"); ok {
		t.Fatal("unverified email must not be transaction eligible")
	}
	verifiedAt := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	email, appErr = service.UpdateMethod(context.Background(), UpdateContactMethodInput{
		UserID: "user-contact", MethodID: email.ID, Type: MethodTypeEmail, Label: "邮箱", Value: "user@example.com", Enabled: true, VerifiedAt: &verifiedAt,
	})
	if appErr != nil {
		t.Fatalf("mark account email verified: %v", appErr)
	}
	if _, _, ok := service.TransactionVersionForOwner(email.ID, "user-contact"); !ok {
		t.Fatal("verified email must be transaction eligible")
	}
	wechat, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "wechat-contact", Type: MethodTypeWechat, Label: "微信", Value: "wechat-user", Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create WeChat contact: %v", appErr)
	}
	if _, _, ok := service.TransactionVersionForOwner(wechat.ID, "wechat-contact"); !ok {
		t.Fatal("enabled WeChat must be transaction eligible")
	}
}

func TestOptionalWechatCanBeUpdatedDisabledDeletedAndOnlyOneEnabled(t *testing.T) {
	t.Parallel()

	service := NewService(nil, func() time.Time { return time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC) })
	created, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "optional-wechat", Type: MethodTypeWechat, Label: "微信", Value: "wechat-id", Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create optional WeChat: %v", appErr)
	}
	if _, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "optional-wechat", Type: MethodTypeWechat, Label: "另一个微信", Value: "wechat-2", Enabled: true,
	}); appErr == nil || appErr.Status != http.StatusConflict || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Code != "duplicate" {
		t.Fatalf("duplicate enabled WeChat error = %#v", appErr)
	}
	updated, appErr := service.UpdateMethod(context.Background(), UpdateContactMethodInput{
		UserID: "optional-wechat", MethodID: created.ID, Type: MethodTypeWechat, Label: "常用微信", Value: "wechat-updated", Enabled: false,
	})
	if appErr != nil || updated.Enabled || updated.DisplayValue != "wechat-updated" {
		t.Fatalf("disable optional WeChat = %+v error=%v", updated, appErr)
	}
	if _, _, ok := service.TransactionVersionForOwner(created.ID, "optional-wechat"); ok {
		t.Fatal("disabled WeChat must not be transaction eligible")
	}
	replacement, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "optional-wechat", Type: MethodTypeWechat, Label: "备用微信", Value: "wechat-2", Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create replacement WeChat: %v", appErr)
	}
	if _, appErr := service.DeleteMethod(context.Background(), "optional-wechat", replacement.ID); appErr != nil {
		t.Fatalf("delete optional WeChat: %v", appErr)
	}
}
