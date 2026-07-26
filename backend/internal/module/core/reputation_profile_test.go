package core

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/reputation"
)

type profileReputationRepository struct {
	reputation.Repository
	facts map[string]reputation.RawFacts
}

func (r profileReputationRepository) AggregateFacts(_ context.Context, userIDs []string, _ time.Time) (map[string]reputation.RawFacts, *domain.AppError) {
	result := make(map[string]reputation.RawFacts, len(userIDs))
	for _, userID := range userIDs {
		value := r.facts[userID]
		value.UserID = userID
		result[userID] = value
	}
	return result, nil
}

func (profileReputationRepository) ExcludeTransaction(context.Context, reputation.ExclusionMutation, time.Time) (reputation.TransactionExclusion, *domain.AppError) {
	return reputation.TransactionExclusion{}, nil
}

func (profileReputationRepository) RestoreTransaction(context.Context, reputation.ExclusionMutation, time.Time) (reputation.TransactionExclusion, *domain.AppError) {
	return reputation.TransactionExclusion{}, nil
}

func TestPublicUserProfileUsesAggregatedReputationFacts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := profileReputationRepository{facts: make(map[string]reputation.RawFacts)}
	service := newService(func() time.Time {
		return time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	}, Repositories{Reputation: repo})
	user, _, appErr := service.CreateDevSession(ctx, "truthful-user", false)
	if appErr != nil {
		t.Fatalf("create dev session: %v", appErr)
	}
	if _, appErr := service.MyProfile(ctx, user); appErr != nil {
		t.Fatalf("ensure profile: %v", appErr)
	}
	repo.facts[user.ID] = reputation.RawFacts{
		UserID: user.ID,
		Buyer: reputation.RoleFacts{
			Carpool: reputation.ScopeFacts{CompletedCount: 2, CompletedCountLast90Days: 1, RoleResponsibilityCancellationCount: 1},
			API:     reputation.ScopeFacts{CompletedCount: 3, CompletedCountLast90Days: 2, UnknownResponsibilityCancellationCount: 1, UnresolvedDisputeCount: 1},
		},
		Seller: reputation.RoleFacts{
			API: reputation.ScopeFacts{CompletedCount: 4, CompletedCountLast90Days: 3, RoleResponsibilityCancellationCount: 2, UnresolvedDisputeCount: 2},
		},
	}

	publicProfile, appErr := service.PublicUserProfile(ctx, user.Username)
	if appErr != nil {
		t.Fatalf("public profile: %v", appErr)
	}
	assertCount := func(name string, value *int, expected int) {
		t.Helper()
		if value == nil || *value != expected {
			t.Fatalf("%s: expected %d, got %#v", name, expected, value)
		}
	}
	assertCount("completed carpools", publicProfile.Stats.CompletedCarpools, 2)
	assertCount("completed API orders", publicProfile.Stats.CompletedAPIOrders, 7)
	assertCount("completed API orders last 90 days", publicProfile.Stats.CompletedAPIOrdersLast90Days, 5)
	assertCount("buyer cancellations", publicProfile.Stats.BuyerResponsibilityCancellationCount, 1)
	assertCount("seller cancellations", publicProfile.Stats.SellerResponsibilityCancellationCount, 2)
	assertCount("unknown cancellations", publicProfile.Stats.UnknownResponsibilityCancellationCount, 1)
	assertCount("unresolved disputes", publicProfile.Stats.UnresolvedDisputeCount, 3)
}
