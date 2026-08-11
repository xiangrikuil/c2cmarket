package core

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	contactmodule "c2c-market/backend/internal/module/contact"
)

func TestLinuxDoBindingProjectsOneReadOnlyContactMethod(t *testing.T) {
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	service := NewServiceWithClock(func() time.Time { return now })
	user, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider: "linux_do", Subject: "linuxdo-subject-1", Username: "bound-user",
		LinuxDoUserID: "linuxdo-subject-1", LinuxDoUsername: "bound-user", TrustLevel: 2,
	})
	if appErr != nil {
		t.Fatalf("login linux.do user: %v", appErr)
	}

	methods, appErr := service.ListContactMethods(context.Background(), user.ID)
	if appErr != nil {
		t.Fatalf("list projected contacts: %v", appErr)
	}
	if len(methods) != 1 || methods[0].Type != "linuxdo" || methods[0].DisplayValue != "@bound-user" {
		t.Fatalf("unexpected identity contact projection: %+v", methods)
	}

	if _, appErr := service.CreateContactMethod(context.Background(), ContactMethodInput{
		UserID: user.ID, Type: "linuxdo", Label: "另一个账号", Value: "@forged", Enabled: true,
	}); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("manual linux.do creation must be rejected, got %+v", appErr)
	}
	if _, appErr := service.UpdateContactMethod(context.Background(), contactmodule.UpdateContactMethodInput{
		UserID: user.ID, MethodID: methods[0].ID, Type: "wechat", Label: "伪装微信", Value: "forged", Enabled: true,
	}); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("identity contact update must be rejected, got %+v", appErr)
	}
	if _, appErr := service.DeleteContactMethod(context.Background(), user.ID, methods[0].ID); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("identity contact deletion must be rejected, got %+v", appErr)
	}
}
