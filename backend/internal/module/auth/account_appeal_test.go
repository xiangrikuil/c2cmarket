package auth

import (
	"context"
	"reflect"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestAccountAppealSessionUsesExistingIdentityAndFixedIsolatedLifetime(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	profile := OAuthProfile{
		Provider:         "linux_do",
		Subject:          "restricted-subject",
		Username:         "restricted-user",
		DisplayName:      "Restricted User",
		AvatarURL:        "https://example.com/original.png",
		LinuxDoUserID:    "restricted-subject",
		LinuxDoUsername:  "restricted-user",
		LinuxDoAvatarURL: "https://example.com/original.png",
	}
	user, ordinarySession, appErr := service.LoginWithOAuthProfile(context.Background(), profile)
	if appErr != nil {
		t.Fatalf("seed OAuth identity: %v", appErr)
	}
	service.mu.Lock()
	user.Status = AccountStatusSuspended
	service.users[user.ID] = user
	userBefore := service.users[user.ID]
	ordinaryBefore := service.sessions[ordinarySession.ID]
	normalSessionCount := len(service.sessions)
	identityCount := len(service.oauthUserIDs)
	userCount := len(service.users)
	service.mu.Unlock()

	changedProfile := profile
	changedProfile.Username = "renamed-provider-handle"
	changedProfile.DisplayName = "Must Not Sync"
	changedProfile.AvatarURL = "https://example.com/must-not-sync.png"
	verifiedUser, first, appErr := service.StartAccountAppealSession(context.Background(), changedProfile)
	if appErr != nil {
		t.Fatalf("start account appeal session: %v", appErr)
	}
	if verifiedUser.ID != user.ID || verifiedUser.Status != AccountStatusSuspended {
		t.Fatalf("unexpected verified account: %+v", verifiedUser)
	}
	if first.ID == "" || first.CSRFToken == "" || first.UserID != user.ID {
		t.Fatalf("missing opaque account appeal secrets: %+v", first)
	}
	if !first.CreatedAt.Equal(now) || !first.ExpiresAt.Equal(now.Add(AccountAppealSessionLifetime)) {
		t.Fatalf("account appeal expiry is not fixed at fifteen minutes: %+v", first)
	}

	service.mu.Lock()
	if !reflect.DeepEqual(service.users[user.ID], userBefore) {
		t.Fatalf("account appeal identity resolution mutated the user: before=%+v after=%+v", userBefore, service.users[user.ID])
	}
	if !reflect.DeepEqual(service.sessions[ordinarySession.ID], ordinaryBefore) || len(service.sessions) != normalSessionCount {
		t.Fatalf("account appeal flow mutated ordinary sessions: before=%+v after=%+v", ordinaryBefore, service.sessions[ordinarySession.ID])
	}
	if len(service.oauthUserIDs) != identityCount || len(service.users) != userCount {
		t.Fatalf("account appeal flow created auth state users=%d identities=%d", len(service.users), len(service.oauthUserIDs))
	}
	service.mu.Unlock()

	now = now.Add(5 * time.Minute)
	_, second, appErr := service.StartAccountAppealSession(context.Background(), profile)
	if appErr != nil {
		t.Fatalf("replace account appeal session: %v", appErr)
	}
	if _, _, appErr := service.GetAccountAppealSession(context.Background(), first.ID); appErr == nil || appErr.Code != domain.CodeSessionRevoked {
		t.Fatalf("replaced session remained usable: %v", appErr)
	}

	initialCSRF := second.CSRFToken
	fixedExpiry := second.ExpiresAt
	_, rotated, appErr := service.GetAccountAppealSession(context.Background(), second.ID)
	if appErr != nil {
		t.Fatalf("rotate account appeal CSRF: %v", appErr)
	}
	if rotated.CSRFToken == "" || rotated.CSRFToken == initialCSRF {
		t.Fatalf("account appeal CSRF did not rotate: before=%q after=%q", initialCSRF, rotated.CSRFToken)
	}
	if !rotated.ExpiresAt.Equal(fixedExpiry) {
		t.Fatalf("CSRF rotation renewed fixed expiry: before=%s after=%s", fixedExpiry, rotated.ExpiresAt)
	}
	if _, _, appErr := service.GetAccountAppealSessionWithCSRF(context.Background(), second.ID, initialCSRF); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("old account appeal CSRF remained usable: %v", appErr)
	}
	if _, validated, appErr := service.GetAccountAppealSessionWithCSRF(context.Background(), second.ID, rotated.CSRFToken); appErr != nil || !validated.ExpiresAt.Equal(fixedExpiry) {
		t.Fatalf("rotated account appeal CSRF did not validate fixed session: session=%+v err=%v", validated, appErr)
	}

	now = fixedExpiry
	if _, _, appErr := service.GetAccountAppealSession(context.Background(), second.ID); appErr == nil || appErr.Code != domain.CodeSessionExpired {
		t.Fatalf("expired account appeal session remained usable: %v", appErr)
	}
}

func TestAccountAppealStartUsesOneGenericIneligibleError(t *testing.T) {
	service := NewService(nil, time.Now)
	profile := OAuthProfile{Provider: "linux_do", Subject: "eligibility-subject", Username: "eligibility-user"}
	active, _, appErr := service.LoginWithOAuthProfile(context.Background(), profile)
	if appErr != nil {
		t.Fatalf("seed active OAuth identity: %v", appErr)
	}

	assertIneligible := func(label string, candidate OAuthProfile) {
		t.Helper()
		if _, _, appErr := service.StartAccountAppealSession(context.Background(), candidate); appErr == nil || appErr.Code != domain.CodeAccountAppealIneligible {
			t.Fatalf("%s returned a distinguishable result: %v", label, appErr)
		}
	}
	assertIneligible("unknown identity", OAuthProfile{Provider: "linux_do", Subject: "unknown-subject", Username: "unknown"})
	assertIneligible("active account", profile)

	service.mu.Lock()
	active.Status = AccountStatusArchived
	service.users[active.ID] = active
	service.mu.Unlock()
	assertIneligible("archived account", profile)
	assertIneligible("non-linux.do identity", OAuthProfile{Provider: "github", Subject: profile.Subject, Username: profile.Username})
}
