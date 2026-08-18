package contact

import (
	"context"
	"net/http"
	"slices"
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
		UserID:      "seller-1",
		Type:        "telegram",
		Label:       "卖家 Telegram",
		Value:       "@seller",
		UsageScopes: []string{UsageScopeCarpoolOwner},
		Enabled:     true,
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
	if !slices.Equal(second.UsageScopes, AllUsageScopes()) {
		t.Fatalf("linux.do mapping scopes = %v, want %v", second.UsageScopes, AllUsageScopes())
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

func TestContactUsageScopesAreCanonicalAndDurableInMemory(t *testing.T) {
	t.Parallel()

	service := NewService(nil, func() time.Time { return time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC) })
	created, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "user-scopes", Type: "email", Label: "邮箱", Value: "scope-user@example.com", Enabled: true,
		UsageScopes: []string{UsageScopeDispute, UsageScopeBuyer, UsageScopeDispute},
	})
	if appErr != nil {
		t.Fatalf("create scoped contact: %v", appErr)
	}
	if want := []string{UsageScopeBuyer, UsageScopeDispute}; !slices.Equal(created.UsageScopes, want) {
		t.Fatalf("created scopes = %v, want %v", created.UsageScopes, want)
	}

	updated, appErr := service.UpdateMethod(context.Background(), UpdateContactMethodInput{
		UserID: "user-scopes", MethodID: created.ID, Type: "email", Label: "工作邮箱", Value: "scope-user-2@example.com", Enabled: true,
		UsageScopes: []string{UsageScopeBuyer, UsageScopeCarpoolOwner, UsageScopeAPIMerchant},
	})
	if appErr != nil {
		t.Fatalf("update scoped contact: %v", appErr)
	}
	wantUpdated := []string{UsageScopeCarpoolOwner, UsageScopeAPIMerchant, UsageScopeBuyer}
	if !slices.Equal(updated.UsageScopes, wantUpdated) {
		t.Fatalf("updated scopes = %v, want %v", updated.UsageScopes, wantUpdated)
	}
	preserved, appErr := service.UpdateMethod(context.Background(), UpdateContactMethodInput{
		UserID: "user-scopes", MethodID: created.ID, Type: "email", Label: "工作邮箱", Value: "scope-user-3@example.com", Enabled: true,
	})
	if appErr != nil || !slices.Equal(preserved.UsageScopes, wantUpdated) {
		t.Fatalf("omitted update scopes must preserve the stored value: method=%+v error=%v", preserved, appErr)
	}

	methods, appErr := service.ListMethods(context.Background(), "user-scopes")
	if appErr != nil || len(methods) != 1 || !slices.Equal(methods[0].UsageScopes, wantUpdated) {
		t.Fatalf("listed methods did not retain scopes: methods=%+v error=%v", methods, appErr)
	}
}

func TestContactUsageScopesDefaultAndValidation(t *testing.T) {
	t.Parallel()

	service := NewService(nil, nil)
	defaulted, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "default-user", Type: "email", Label: "邮箱", Value: "buyer@example.com", Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create default-scoped contact: %v", appErr)
	}
	if !slices.Equal(defaulted.UsageScopes, DefaultUsageScopes()) {
		t.Fatalf("default scopes = %v, want %v", defaulted.UsageScopes, DefaultUsageScopes())
	}

	tests := []struct {
		name   string
		scopes []string
		code   string
	}{
		{name: "empty", scopes: []string{}, code: "required"},
		{name: "unknown", scopes: []string{"seller"}, code: "unsupported"},
		{name: "whitespace is not repaired", scopes: []string{" buyer"}, code: "unsupported"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
				UserID: "invalid-user-" + test.name, Type: "email", Label: "邮箱", Value: "invalid@example.com", Enabled: true, UsageScopes: test.scopes,
			})
			if appErr == nil || appErr.Status != 422 || appErr.Code != domain.CodeValidationFailed {
				t.Fatalf("expected 422 validation error, got %#v", appErr)
			}
			if len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "usageScopes" || appErr.FieldErrors[0].Code != test.code {
				t.Fatalf("unexpected field error: %#v", appErr.FieldErrors)
			}
		})
	}
}

func TestWechatScopesAreAutomaticAndOnlyOneCanBeEnabledInMemory(t *testing.T) {
	t.Parallel()

	service := NewService(nil, nil)
	created, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "wechat-user", Type: MethodTypeWechat, Label: "微信", Value: "wechat-user", Enabled: true,
		UsageScopes: []string{UsageScopeBuyer},
	})
	if appErr != nil {
		t.Fatalf("create WeChat contact: %v", appErr)
	}
	if !slices.Equal(created.UsageScopes, AllUsageScopes()) {
		t.Fatalf("WeChat scopes = %v, want %v", created.UsageScopes, AllUsageScopes())
	}
	if _, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "wechat-user", Type: MethodTypeWechat, Label: "另一个微信", Value: "wechat-user-2", Enabled: true,
	}); appErr == nil || appErr.Status != http.StatusConflict || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Code != "duplicate" {
		t.Fatalf("duplicate enabled WeChat error = %#v", appErr)
	}

	updated, appErr := service.UpdateMethod(context.Background(), UpdateContactMethodInput{
		UserID: "wechat-user", MethodID: created.ID, Type: MethodTypeWechat, Label: "常用微信", Value: "wechat-user-new", Enabled: true,
		UsageScopes: []string{UsageScopeDispute},
	})
	if appErr != nil || !slices.Equal(updated.UsageScopes, AllUsageScopes()) {
		t.Fatalf("updated WeChat scopes = %v error=%v", updated.UsageScopes, appErr)
	}
}
