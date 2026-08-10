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
