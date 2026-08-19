package core

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	contactmodule "c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/reputation"
)

type blockingReputationRepository struct {
	reputation.Repository
	calls                  []reputationActionCall
	restrictedContactUsers map[string]struct{}
}

type reputationActionCall struct {
	userID string
	role   string
	action string
}

func (r *blockingReputationRepository) FindActiveRestriction(_ context.Context, userID, role, action string, now time.Time) (*reputation.UserRestriction, *domain.AppError) {
	r.calls = append(r.calls, reputationActionCall{userID: userID, role: role, action: action})
	if action != reputation.ActionContactView {
		return nil, nil
	}
	if _, restricted := r.restrictedContactUsers[userID]; !restricted {
		return nil, nil
	}
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
	owner := createTestBoundUser(t, service, "seller-1")
	buyer := createTestBoundUser(t, service, "buyer-1")
	repo.restrictedContactUsers = map[string]struct{}{owner.ID: {}, buyer.ID: {}}
	ownerContact := createTestContactMethod(t, service, owner.ID, "wechat", "Owner TG", "@owner_restricted", contactmodule.AllUsageScopes())
	buyerContact := createTestContactMethod(t, service, buyer.ID, "wechat", "Buyer TG", "@buyer_restricted", contactmodule.DefaultUsageScopes())
	apiService := createOrderableAPIService(t, service, owner, ownerContact.ID)
	repo.calls = nil

	if _, appErr := service.CreateAPIPurchaseIntentWithIdempotency(
		ctx,
		buyer,
		"POST /api/v1/api-services/{id}/purchase-intents",
		"restricted-create",
		"hash-create",
		CreateAPIPurchaseIntentInput{
			APIServiceID:          apiService.ID,
			BuyerContactMethodID:  buyerContact.ID,
			RequestedCNYAmount:    "16.00",
			RequestedUSDAllowance: "20.000000",
			SelectedAccessMode:    "buyer_dedicated_sub_key",
			RequestID:             "request-create-restricted",
		},
		testAPIPurchaseIntentCompletion,
	); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected restricted API intent create, got %#v", appErr)
	}
	if _, appErr := service.MyAPIPurchaseIntent(ctx, User{ID: buyer.ID}, "intent-1", "request-buyer"); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected restricted buyer detail, got %#v", appErr)
	}
	if _, appErr := service.OwnerAPIPurchaseIntent(ctx, owner, "intent-1", "request-seller"); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected restricted seller detail, got %#v", appErr)
	}

	expected := []reputationActionCall{
		{userID: buyer.ID, role: reputation.RoleBuyer, action: reputation.ActionContactView},
		{userID: buyer.ID, role: reputation.RoleBuyer, action: reputation.ActionContactView},
		{userID: owner.ID, role: reputation.RoleSeller, action: reputation.ActionContactView},
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
