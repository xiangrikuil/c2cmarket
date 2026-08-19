package communityidentity

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"
)

func TestGrantFoundingUsesInclusiveCutoffAndIsIdempotent(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	cutoff := now.Add(24 * time.Hour)
	service := NewService(nil, nil, func() time.Time { return now }, cutoff)
	user := auth.User{ID: "user-1", Status: auth.AccountStatusActive}

	item, created, appErr := service.GrantFoundingForUser(context.Background(), user, cutoff)
	if appErr != nil || !created {
		t.Fatalf("expected founding identity at the cutoff, created=%t error=%v", created, appErr)
	}
	if item.Type != IdentityTypeFoundingUser || item.Source != SourceAuto {
		t.Fatalf("unexpected founding identity: %+v", item)
	}

	replayed, created, appErr := service.GrantFoundingForUser(context.Background(), user, cutoff)
	if appErr != nil || created || replayed.ID != item.ID {
		t.Fatalf("expected idempotent founding grant, replay=%+v created=%t error=%v", replayed, created, appErr)
	}

	tooLate, created, appErr := service.GrantFoundingForUser(context.Background(), auth.User{ID: "user-2", Status: auth.AccountStatusActive}, cutoff.Add(time.Nanosecond))
	if appErr != nil || created || tooLate.ID != "" {
		t.Fatalf("expected late verification to be ineligible, item=%+v created=%t error=%v", tooLate, created, appErr)
	}
}

func TestGrantFoundingRejectsInactiveAndAdminUsers(t *testing.T) {
	cutoff := time.Date(2026, 9, 30, 23, 59, 59, 0, time.FixedZone("CST", 8*60*60))
	service := NewService(nil, nil, time.Now, cutoff)
	qualifiedAt := cutoff.Add(-time.Hour)

	for _, user := range []auth.User{
		{ID: "suspended", Status: auth.AccountStatusSuspended},
		{ID: "admin", Status: auth.AccountStatusActive, IsAdmin: true},
	} {
		item, created, appErr := service.GrantFoundingForUser(context.Background(), user, qualifiedAt)
		if appErr != nil || created || item.ID != "" {
			t.Fatalf("expected user %q to be ineligible, item=%+v created=%t error=%v", user.ID, item, created, appErr)
		}
	}
}

func TestAdminGrantAndRevokeAreAdminOnlyAndRetainHistory(t *testing.T) {
	service := NewService(nil, nil, time.Now, time.Now().Add(time.Hour))
	target := "target-user"
	nonAdmin := auth.User{ID: "member", Status: auth.AccountStatusActive}
	if _, appErr := service.GrantAdmin(context.Background(), nonAdmin, GrantAdminInput{TargetUserID: target, Type: IdentityTypeBetaContributor, Reason: "有效内测反馈"}); appErr == nil {
		t.Fatal("expected non-admin grant to be rejected")
	}

	admin := auth.User{ID: "admin", Status: auth.AccountStatusActive, IsAdmin: true}
	item, appErr := service.GrantAdmin(context.Background(), admin, GrantAdminInput{TargetUserID: target, Type: IdentityTypeBetaContributor, Reason: "有效内测反馈"})
	if appErr != nil {
		t.Fatalf("grant beta contributor: %v", appErr)
	}
	if _, appErr := service.GrantAdmin(context.Background(), admin, GrantAdminInput{TargetUserID: target, Type: IdentityTypeBetaContributor, Reason: "重复发放"}); appErr == nil {
		t.Fatal("expected duplicate grant to be rejected")
	}

	if _, appErr := service.Revoke(context.Background(), nonAdmin, RevokeInput{TargetUserID: target, Type: IdentityTypeBetaContributor, Reason: "撤销"}); appErr == nil {
		t.Fatal("expected non-admin revoke to be rejected")
	}
	revoked, appErr := service.Revoke(context.Background(), admin, RevokeInput{TargetUserID: target, Type: IdentityTypeBetaContributor, Reason: "测试身份误发"})
	if appErr != nil || revoked.RevokedAt == nil || revoked.RevokeReason != "测试身份误发" {
		t.Fatalf("unexpected revoke result: %+v error=%v", revoked, appErr)
	}

	public, appErr := service.PublicForUser(context.Background(), target)
	if appErr != nil || len(public) != 0 {
		t.Fatalf("revoked identity must not be public, public=%+v error=%v", public, appErr)
	}
	history, appErr := service.ListForUser(context.Background(), target, true)
	if appErr != nil || len(history) != 1 || history[0].ID != item.ID {
		t.Fatalf("expected revoked history to remain available, history=%+v error=%v", history, appErr)
	}
}

func TestCompactPrefersBetaContributor(t *testing.T) {
	founding := ToPublic(Identity{Type: IdentityTypeFoundingUser, GrantedAt: time.Now()})
	contributor := ToPublic(Identity{Type: IdentityTypeBetaContributor, GrantedAt: time.Now()})
	selected := Compact([]PublicIdentity{founding, contributor})
	if selected == nil || selected.Code != IdentityTypeBetaContributor {
		t.Fatalf("expected beta contributor to win compact projection, got %+v", selected)
	}
}
