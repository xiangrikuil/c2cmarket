package auth

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestRestrictedBusinessOAuthUsesExistingIdentityAndOneTimeState(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	profile := OAuthProfile{Provider: "linux_do", Subject: "restricted-oauth-subject", Username: "restricted-oauth-user"}
	user, normalSession, appErr := service.LoginWithOAuthProfile(context.Background(), profile)
	if appErr != nil {
		t.Fatalf("seed OAuth identity: %v", appErr)
	}
	service.mu.Lock()
	user.Status = AccountStatusSuspended
	user.GovernanceVersion = 2
	user.CurrentGovernanceActionID = "governance-action-2"
	service.users[user.ID] = user
	userCount := len(service.users)
	identityCount := len(service.oauthUserIDs)
	normalSessionCount := len(service.sessions)
	service.mu.Unlock()

	state, appErr := service.StartRestrictedBusinessOAuth(context.Background())
	if appErr != nil {
		t.Fatalf("start restricted OAuth: %v", appErr)
	}
	result, appErr := service.CompleteRestrictedBusinessOAuth(context.Background(), state, profile)
	if appErr != nil {
		t.Fatalf("complete restricted OAuth: %v", appErr)
	}
	if result.Audience != SessionAudienceRestrictedBusiness || result.User.ID != user.ID || result.RestrictedSession.ID == "" {
		t.Fatalf("unexpected restricted result: %+v", result)
	}
	if _, appErr := service.CompleteRestrictedBusinessOAuth(context.Background(), state, profile); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("restricted state replay remained usable: %v", appErr)
	}

	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.users) != userCount || len(service.oauthUserIDs) != identityCount || len(service.sessions) != normalSessionCount {
		t.Fatalf("restricted OAuth mutated registration/session state users=%d identities=%d sessions=%d", len(service.users), len(service.oauthUserIDs), len(service.sessions))
	}
	if _, ok := service.sessions[normalSession.ID]; !ok {
		t.Fatal("restricted OAuth removed the pre-governance normal session record")
	}
}

func TestRestrictedBusinessOAuthUnknownIdentityDoesNotRegister(t *testing.T) {
	service := NewService(nil, time.Now)
	state, appErr := service.StartRestrictedBusinessOAuth(context.Background())
	if appErr != nil {
		t.Fatalf("start restricted OAuth: %v", appErr)
	}
	profile := OAuthProfile{Provider: "linux_do", Subject: "unknown-restricted-subject", Username: "unknown-restricted-user"}
	if _, appErr := service.CompleteRestrictedBusinessOAuth(context.Background(), state, profile); appErr == nil || appErr.Code != domain.CodeAccountRestricted {
		t.Fatalf("unknown restricted identity result: %v", appErr)
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.users) != 0 || len(service.oauthUserIDs) != 0 || len(service.sessions) != 0 || len(service.restrictedBusinessSessions) != 0 {
		t.Fatalf("unknown restricted identity created state users=%d identities=%d sessions=%d restricted=%d", len(service.users), len(service.oauthUserIDs), len(service.sessions), len(service.restrictedBusinessSessions))
	}
}

func TestGovernanceOAuthPurposesCannotConsumeEachOthersState(t *testing.T) {
	service := NewService(nil, time.Now)
	profile := OAuthProfile{Provider: "linux_do", Subject: "purpose-subject", Username: "purpose-user"}
	restrictedState, appErr := service.StartRestrictedBusinessOAuth(context.Background())
	if appErr != nil {
		t.Fatalf("start restricted OAuth: %v", appErr)
	}
	if _, _, appErr := service.CompleteAccountAppealOAuth(context.Background(), restrictedState, profile); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("account appeal consumed restricted state: %v", appErr)
	}
	appealState, appErr := service.StartAccountAppealOAuth(context.Background())
	if appErr != nil {
		t.Fatalf("start account appeal OAuth: %v", appErr)
	}
	if _, appErr := service.CompleteRestrictedBusinessOAuth(context.Background(), appealState, profile); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("restricted OAuth consumed account appeal state: %v", appErr)
	}
}
