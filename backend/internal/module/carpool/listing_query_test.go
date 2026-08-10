package carpool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

func TestAdminListingsFiltersBeforePagination(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, nil, nil, nil, func() time.Time { return now })
	for index := 0; index < 25; index++ {
		id := fmt.Sprintf("active-%02d", index)
		service.listingOrder = append(service.listingOrder, id)
		service.listings[id] = Listing{ID: id, Status: ListingStatusActive, UpdatedAt: now.Add(-time.Duration(index) * time.Minute)}
	}
	service.listingOrder = append(service.listingOrder, "older-exception")
	service.listings["older-exception"] = Listing{ID: "older-exception", Status: ListingStatusPaused, UpdatedAt: now.Add(-time.Hour)}

	page, appErr := service.AdminListings(context.Background(), auth.User{IsAdmin: true}, ListingFilter{View: ListingViewExceptions}, domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list admin exceptions: %v", appErr)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "older-exception" || page.NextCursor != nil {
		t.Fatalf("expected older exception on first filtered page, got %+v", page)
	}
}

func TestMyListingsViewsFilterBeforePagination(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, nil, nil, nil, func() time.Time { return now })
	owner := auth.User{ID: "owner-user"}
	fixtures := []Listing{
		{ID: "recruiting-b", OwnerUserID: owner.ID, Status: ListingStatusActive, ActiveBuyerMembers: 0, UpdatedAt: now},
		{ID: "recruiting-a", OwnerUserID: owner.ID, Status: ListingStatusActive, ActiveBuyerMembers: 0, UpdatedAt: now},
		{ID: "serving", OwnerUserID: owner.ID, Status: ListingStatusActive, ActiveBuyerMembers: 1, UpdatedAt: now.Add(-time.Minute)},
		{ID: "rejected", OwnerUserID: owner.ID, Status: ListingStatusRejected, UpdatedAt: now.Add(-2 * time.Minute)},
		{ID: "removed", OwnerUserID: owner.ID, Status: ListingStatusRemoved, UpdatedAt: now.Add(-3 * time.Minute)},
		{ID: "draft", OwnerUserID: owner.ID, Status: ListingStatusDraft, UpdatedAt: now.Add(-4 * time.Minute)},
		{ID: "changes", OwnerUserID: owner.ID, Status: ListingStatusChangesRequested, UpdatedAt: now.Add(-5 * time.Minute)},
		{ID: "pending", OwnerUserID: owner.ID, Status: ListingStatusPendingReview, UpdatedAt: now.Add(-6 * time.Minute)},
		{ID: "paused", OwnerUserID: owner.ID, Status: ListingStatusPaused, UpdatedAt: now.Add(-7 * time.Minute)},
		{ID: "other-owner", OwnerUserID: "other-user", Status: ListingStatusDraft, UpdatedAt: now.Add(time.Hour)},
	}
	for _, fixture := range fixtures {
		service.listingOrder = append(service.listingOrder, fixture.ID)
		service.listings[fixture.ID] = fixture
	}

	tests := []struct {
		view string
		want []string
	}{
		{view: OwnerListingViewRecruiting, want: []string{"recruiting-b", "recruiting-a"}},
		{view: OwnerListingViewServing, want: []string{"serving"}},
		{view: OwnerListingViewHistory, want: []string{"rejected", "removed"}},
		{view: OwnerListingViewNeedsEdit, want: []string{"draft", "changes", "pending", "paused"}},
	}
	for _, test := range tests {
		t.Run(test.view, func(t *testing.T) {
			page, appErr := service.MyListings(context.Background(), owner, test.view, domain.PageRequest{Limit: 1})
			if appErr != nil {
				t.Fatalf("list first page: %v", appErr)
			}
			got := make([]string, 0, len(test.want))
			got = append(got, page.Items[0].ID)
			for page.NextCursor != nil {
				page, appErr = service.MyListings(context.Background(), owner, test.view, domain.PageRequest{Limit: 1, Cursor: *page.NextCursor})
				if appErr != nil {
					t.Fatalf("list next page: %v", appErr)
				}
				for _, item := range page.Items {
					got = append(got, item.ID)
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(test.want) {
				t.Fatalf("view %s: expected %v, got %v", test.view, test.want, got)
			}
		})
	}

	if _, appErr := service.MyListings(context.Background(), owner, "unknown", domain.PageRequest{Limit: 20}); appErr == nil || appErr.Status != 422 {
		t.Fatalf("expected invalid owner view to return 422, got %v", appErr)
	}
	if listing, appErr := service.MyListing(context.Background(), owner, "draft"); appErr != nil || listing.ID != "draft" {
		t.Fatalf("expected owner detail, got listing=%+v error=%v", listing, appErr)
	}
	if _, appErr := service.MyListing(context.Background(), auth.User{ID: "other-user"}, "draft"); appErr == nil || appErr.Status != 404 {
		t.Fatalf("expected non-owner detail to return 404, got %v", appErr)
	}
}

func TestListingSortUsesStableFieldAndIDOrder(t *testing.T) {
	items := []Listing{
		{ID: "b", PriceMonthlyCNY: "20.00", AvailableSeats: 2},
		{ID: "a", PriceMonthlyCNY: "10.00", AvailableSeats: 2},
		{ID: "c", PriceMonthlyCNY: "10.00", AvailableSeats: 3},
	}

	byPrice := filterListings(items, ListingFilter{Sort: ListingSortPriceAsc})
	if byPrice[0].ID != "a" || byPrice[1].ID != "c" || byPrice[2].ID != "b" {
		t.Fatalf("unexpected price order: %+v", byPrice)
	}
	bySeats := filterListings(items, ListingFilter{Sort: ListingSortSeatsDesc})
	if bySeats[0].ID != "c" || bySeats[1].ID != "b" || bySeats[2].ID != "a" {
		t.Fatalf("unexpected seats order: %+v", bySeats)
	}
}
