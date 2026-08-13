package auth

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestStudentLinuxDoLinkRequiresRecentPasswordAndRotatesSession(t *testing.T) {
	ctx := context.Background()
	current := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	service := NewServiceWithRegistrationEmailSender(nil, func() time.Time { return current }, &studentRegistrationTestSender{})
	student, session, password := createLinkableStudent(t, ctx, service)

	if _, appErr := service.StartLinuxDoLink(ctx, session.ID); appErr == nil || appErr.Code != domain.CodeRecentReauthenticationRequired {
		t.Fatalf("link started without recent password authentication: %v", appErr)
	}
	if appErr := service.ReauthenticatePassword(ctx, session.ID, session.CSRFToken, "wrong-password"); appErr == nil || appErr.Code != domain.CodeInvalidCredentials {
		t.Fatalf("wrong reauthentication password accepted: %v", appErr)
	}
	if appErr := service.ReauthenticatePassword(ctx, session.ID, session.CSRFToken, password); appErr != nil {
		t.Fatalf("ReauthenticatePassword() error = %v", appErr)
	}
	current = current.Add(RecentPasswordReauthenticationWindow + time.Second)
	if _, appErr := service.StartLinuxDoLink(ctx, session.ID); appErr == nil || appErr.Code != domain.CodeRecentReauthenticationRequired {
		t.Fatalf("expired recent authentication accepted: %v", appErr)
	}
	current = current.Add(time.Second)
	if appErr := service.ReauthenticatePassword(ctx, session.ID, session.CSRFToken, password); appErr != nil {
		t.Fatalf("second ReauthenticatePassword() error = %v", appErr)
	}

	owner, _, appErr := service.LoginWithOAuthProfile(ctx, OAuthProfile{Provider: "linux_do", Subject: "owned-linuxdo", Username: "owned-linuxdo", LinuxDoUserID: "owned-linuxdo", TrustLevel: 2})
	if appErr != nil || owner.ID == "" {
		t.Fatalf("create conflicting OAuth owner: user=%+v error=%v", owner, appErr)
	}
	conflictState, appErr := service.StartLinuxDoLink(ctx, session.ID)
	if appErr != nil {
		t.Fatalf("StartLinuxDoLink(conflict) error = %v", appErr)
	}
	conflictProfile := OAuthProfile{Provider: "linux_do", Subject: "owned-linuxdo", Username: "owned-linuxdo", LinuxDoUserID: "owned-linuxdo", TrustLevel: 2}
	if _, _, appErr := service.CompleteLinuxDoLink(ctx, session.ID, conflictState, conflictProfile); appErr == nil || appErr.Code != domain.CodeOAuthIdentityConflict {
		t.Fatalf("expected OAuth ownership conflict, got %v", appErr)
	}
	if _, _, appErr := service.CompleteLinuxDoLink(ctx, session.ID, conflictState, conflictProfile); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("conflicting state was not consumed exactly once: %v", appErr)
	}
	if read, _, appErr := service.GetSession(ctx, session.ID); appErr != nil || read.ID != student.ID {
		t.Fatalf("OAuth conflict revoked the original session: user=%+v error=%v", read, appErr)
	}

	state, appErr := service.StartLinuxDoLink(ctx, session.ID)
	if appErr != nil {
		t.Fatalf("StartLinuxDoLink(success) error = %v", appErr)
	}
	linked, replacement, appErr := service.CompleteLinuxDoLink(ctx, session.ID, state, OAuthProfile{
		Provider: "linux_do", Subject: "new-linuxdo", Username: "linked-name",
		LinuxDoUserID: "new-linuxdo", LinuxDoUsername: "linked-name", TrustLevel: 3,
	})
	if appErr != nil {
		t.Fatalf("CompleteLinuxDoLink() error = %v", appErr)
	}
	if linked.ID != student.ID || linked.StudentClaim == nil || linked.LinuxDoBinding == nil || !linked.LinuxDoBinding.Bound {
		t.Fatalf("link did not upgrade the original student in place: before=%+v after=%+v", student, linked)
	}
	wantCapabilities := []string{
		CapabilityAPIOrderCreate,
		CapabilityAPIProbeManage,
		CapabilityAPIQuotaPublish,
		CapabilityAPIServicePublish,
		CapabilityCarpoolApply,
		CapabilityCarpoolPublish,
	}
	if got := ProjectCapabilities(linked); !reflect.DeepEqual(got, wantCapabilities) {
		t.Fatalf("linked capabilities = %v, want %v", got, wantCapabilities)
	}
	if replacement.ID == "" || replacement.ID == session.ID || replacement.CSRFToken == "" {
		t.Fatalf("link did not rotate session and CSRF: old=%+v new=%+v", session, replacement)
	}
	if _, _, appErr := service.GetSession(ctx, session.ID); appErr == nil || appErr.Code != domain.CodeSessionRevoked {
		t.Fatalf("old link session remained usable: %v", appErr)
	}
	if read, _, appErr := service.GetSessionWithCSRF(ctx, replacement.ID, replacement.CSRFToken); appErr != nil || read.ID != student.ID || read.StudentClaim == nil || read.LinuxDoBinding == nil {
		t.Fatalf("rotated session did not retain both identities: user=%+v error=%v", read, appErr)
	}
}

func createLinkableStudent(t *testing.T, ctx context.Context, service *Service) (User, Session, string) {
	t.Helper()
	admin, _, appErr := service.CreateDevSession(ctx, "link-admin", true)
	if appErr != nil {
		t.Fatalf("create link admin: %v", appErr)
	}
	if _, appErr := service.CreateStudentInstitutionDomainWithIdempotency(ctx, admin, "create-link-domain", "create-link-domain", "create-link-domain-hash", StudentInstitutionDomainCreateInput{Domain: "link.edu", InstitutionName: "Link University", Enabled: true, Reason: "配置链接测试域名"}, studentInstitutionTestCompletion(http.StatusCreated)); appErr != nil {
		t.Fatalf("create link domain: %v", appErr)
	}
	if _, appErr := service.UpdateAdminStudentRegistrationWithIdempotency(ctx, admin, "enable-link-registration", "enable-link-registration", "enable-link-registration-hash", StudentRegistrationSettingUpdate{Enabled: true, ExpectedVersion: 1, Reason: "开启链接测试"}, studentRegistrationTestCompletion); appErr != nil {
		t.Fatalf("enable link registration: %v", appErr)
	}
	challenge, appErr := service.StartEmailRegistration(ctx, EmailRegistrationStartInput{Email: "student@link.edu"})
	if appErr != nil {
		t.Fatalf("start linkable student registration: %v", appErr)
	}
	const password = "LinkableStudent1!"
	user, session, appErr := service.ConfirmEmailRegistration(ctx, EmailRegistrationConfirmInput{Email: challenge.Email, Code: challenge.DevCode, Username: "linkable-student", Password: password})
	if appErr != nil {
		t.Fatalf("confirm linkable student registration: %v", appErr)
	}
	return user, session, password
}
