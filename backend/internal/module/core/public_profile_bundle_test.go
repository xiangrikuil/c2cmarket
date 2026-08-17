package core

import (
	"fmt"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/review"
)

func TestPublicProfileMarketItemsFilterSortAndLimit(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	listings := []carpool.Listing{{ID: "hidden-draft", Status: carpool.ListingStatusDraft, UpdatedAt: base.Add(time.Hour)}}
	services := []apimarket.Service{{
		ID: "hidden-store", MerchantIdentityMode: "store_alias", ReviewStatus: apimarket.ServiceReviewStatusApproved,
		PublicationStatus: apimarket.ServicePublicationStatusOnline, ModerationStatus: apimarket.ServiceModerationStatusClear,
		UpdatedAt: base.Add(time.Hour),
	}}
	for index := 0; index < 8; index++ {
		updatedAt := base.Add(time.Duration(index/2) * time.Minute)
		id := fmt.Sprintf("item-%02d", index)
		listings = append(listings, carpool.Listing{ID: id, Title: id, Status: carpool.ListingStatusActive, UpdatedAt: updatedAt})
		services = append(services, apimarket.Service{
			ID: id, Title: id, MerchantIdentityMode: "public_profile",
			ReviewStatus:      apimarket.ServiceReviewStatusApproved,
			PublicationStatus: apimarket.ServicePublicationStatusOnline,
			ModerationStatus:  apimarket.ServiceModerationStatusClear,
			UpdatedAt:         updatedAt,
		})
	}

	carpools := publicProfileCarpools(listings)
	if len(carpools) != 6 {
		t.Fatalf("expected 6 public carpools, got %d", len(carpools))
	}
	apiServices := publicProfileAPIServices(services)
	if len(apiServices) != 6 {
		t.Fatalf("expected 6 public API services, got %d", len(apiServices))
	}
	wantIDs := []string{"item-07", "item-06", "item-05", "item-04", "item-03", "item-02"}
	for index, wantID := range wantIDs {
		if carpools[index].ID != wantID {
			t.Fatalf("carpool %d: expected %s, got %s", index, wantID, carpools[index].ID)
		}
		if apiServices[index].ID != wantID {
			t.Fatalf("API service %d: expected %s, got %s", index, wantID, apiServices[index].ID)
		}
	}
}

func TestPublicProfileCompletionsRespectPrivacyDeduplicateAndLimit(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	publicProfile := profile.PublicUserProfile{
		ID: "profile-user",
		Privacy: profile.PrivacySettings{
			ShowCompletedCarpoolCount:   true,
			ShowCompletedAPIIntentCount: true,
		},
	}
	buyerOrders := make([]apiorder.Order, 0, 8)
	sellerOrders := make([]apiorder.Order, 0, 8)
	for index := 0; index < 8; index++ {
		completedAt := base.Add(time.Duration(index+6) * time.Minute)
		item := apiorder.Order{
			ID: fmt.Sprintf("order-%02d", index), BuyerUserID: publicProfile.ID,
			Status: apiorder.StatusCompleted, CompletedAt: &completedAt,
		}
		buyerOrders = append(buyerOrders, item)
		sellerOrders = append(sellerOrders, item)
	}

	items := publicProfileCompletions(publicProfile, buyerOrders, sellerOrders)
	if len(items) != 8 {
		t.Fatalf("expected 8 unique API completions, got %d", len(items))
	}
	seen := make(map[string]struct{}, len(items))
	for index, item := range items {
		key := item.Kind + ":" + item.ID
		if _, exists := seen[key]; exists {
			t.Fatalf("completion %s was returned more than once", key)
		}
		seen[key] = struct{}{}
		if index > 0 && items[index-1].CompletedAt.Before(item.CompletedAt) {
			t.Fatalf("completions are not sorted newest first: %#v", items)
		}
	}

	publicProfile.Privacy.ShowCompletedCarpoolCount = false
	apiOnly := publicProfileCompletions(publicProfile, buyerOrders, sellerOrders)
	for _, item := range apiOnly {
		if item.Kind != "api_order" {
			t.Fatalf("carpool completion leaked with carpool privacy disabled: %#v", item)
		}
	}
	publicProfile.Privacy.ShowCompletedAPIIntentCount = false
	if hidden := publicProfileCompletions(publicProfile, buyerOrders, sellerOrders); len(hidden) != 0 {
		t.Fatalf("API completions leaked with API privacy disabled: %#v", hidden)
	}
}

func TestPublicProfileReviewsAndDisputesSortAndLimit(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	reviews := make([]review.PublicReview, 0, 12)
	disputes := make([]report.PublicDispute, 0, 12)
	for index := 0; index < 12; index++ {
		at := base.Add(time.Duration(index/2) * time.Minute)
		id := fmt.Sprintf("item-%02d", index)
		reviews = append(reviews, review.PublicReview{ID: id, Date: at})
		disputes = append(disputes, report.PublicDispute{ID: id, HandledAt: at})
	}

	limitedReviews := publicProfileReviews(reviews)
	limitedDisputes := publicProfileDisputes(disputes)
	if len(limitedReviews) != 10 || len(limitedDisputes) != 10 {
		t.Fatalf("expected review/dispute limits of 10, got %d/%d", len(limitedReviews), len(limitedDisputes))
	}
	if limitedReviews[0].ID != "item-11" || limitedDisputes[0].ID != "item-11" {
		t.Fatalf("expected stable newest-first ordering, got reviews=%s disputes=%s", limitedReviews[0].ID, limitedDisputes[0].ID)
	}
}
