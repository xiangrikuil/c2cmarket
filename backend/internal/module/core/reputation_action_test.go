package core

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/reputation"
)

type blockingReputationRepository struct {
	reputation.Repository
	calls []reputationActionCall
}

type reputationActionCall struct {
	userID string
	role   string
	action string
}

func (r *blockingReputationRepository) FindActiveRestriction(_ context.Context, userID, role, action string, now time.Time) (*reputation.UserRestriction, *domain.AppError) {
	r.calls = append(r.calls, reputationActionCall{userID: userID, role: role, action: action})
	return &reputation.UserRestriction{
		ID:           "restriction-1",
		UserID:       userID,
		RoleScope:    role,
		ActionCode:   action,
		PublicReason: "当前不可查看交易联系方式。",
		StartsAt:     now.Add(-time.Minute),
	}, nil
}

func TestAPIPurchaseIntentContactDisclosureChecksReputationAction(t *testing.T) {
	t.Parallel()

	repo := &blockingReputationRepository{}
	service := newService(
		func() time.Time { return time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC) },
		Repositories{Reputation: repo},
	)
	ctx := context.Background()

	if _, appErr := service.CreateAPIPurchaseIntentWithIdempotency(
		ctx,
		"buyer-1",
		"POST /api/v1/api-services/{id}/purchase-intents",
		"restricted-create",
		"hash-create",
		CreateAPIPurchaseIntentInput{},
		nil,
	); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected restricted API intent create, got %#v", appErr)
	}
	if _, appErr := service.MyAPIPurchaseIntent(ctx, User{ID: "buyer-1"}, "intent-1", "request-buyer"); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected restricted buyer detail, got %#v", appErr)
	}
	if _, appErr := service.OwnerAPIPurchaseIntent(ctx, User{ID: "seller-1"}, "intent-1", "request-seller"); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected restricted seller detail, got %#v", appErr)
	}

	expected := []reputationActionCall{
		{userID: "buyer-1", role: reputation.RoleBuyer, action: reputation.ActionContactView},
		{userID: "buyer-1", role: reputation.RoleBuyer, action: reputation.ActionContactView},
		{userID: "seller-1", role: reputation.RoleSeller, action: reputation.ActionContactView},
	}
	if len(repo.calls) != len(expected) {
		t.Fatalf("expected %d reputation checks, got %#v", len(expected), repo.calls)
	}
	for index := range expected {
		if repo.calls[index] != expected[index] {
			t.Fatalf("unexpected reputation check %d: got %#v want %#v", index, repo.calls[index], expected[index])
		}
	}
}
