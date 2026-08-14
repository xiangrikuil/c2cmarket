package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/officialprice"
)

func TestPublicCarpoolListPassesFiltersAndCursorThrough(t *testing.T) {
	next := "next-carpool-page"
	service := &carpoolPaginationRouteService{result: domain.Page[carpool.Listing]{
		Items:      []carpool.Listing{{ID: "listing-1", Title: "first"}},
		NextCursor: &next,
	}}
	handler := NewServer(service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/carpools?limit=7&cursor=current&q=needle&productPlanIds=plan-1,plan-2&region=CN&sort=price_asc", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d body %s", response.Code, response.Body.String())
	}
	var body listResponse[carpoolListingResponse]
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Items) != 1 || body.NextCursor == nil || *body.NextCursor != next {
		t.Fatalf("unexpected response: %+v", body)
	}
	if service.page.Limit != 7 || service.page.Cursor != "current" {
		t.Fatalf("unexpected page request: %+v", service.page)
	}
	if service.filter.Query != "needle" || service.filter.Region != "CN" || service.filter.Sort != carpool.ListingSortPriceAsc || len(service.filter.ProductPlanIDs) != 2 {
		t.Fatalf("unexpected filter: %+v", service.filter)
	}
}

type carpoolPaginationRouteService struct {
	ApplicationService
	filter carpool.ListingFilter
	page   domain.PageRequest
	result domain.Page[carpool.Listing]
}

func (s *carpoolPaginationRouteService) PublicCarpoolListings(_ context.Context, filter carpool.ListingFilter, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError) {
	s.filter = filter
	s.page = page
	return s.result, nil
}

func TestOfficialPriceStatusFilterRunsBeforePagination(t *testing.T) {
	items := make([]officialprice.Record, 0, 26)
	for index := 0; index < 25; index++ {
		items = append(items, officialprice.Record{ID: "active", Status: officialprice.RecordStatusActive})
	}
	items = append(items, officialprice.Record{ID: "older-taken-down", Status: officialprice.RecordStatusTakenDown})

	request := httptest.NewRequest("GET", "/api/v1/admin/official-price-records?status=taken_down&limit=20", nil)
	filtered := filterOfficialPriceRecords(request, items)
	page, appErr := paginateSlice(request, filtered)
	if appErr != nil {
		t.Fatalf("paginate filtered official prices: %v", appErr)
	}
	if len(page.Items) != 1 || page.Items[0].ID != "older-taken-down" {
		t.Fatalf("expected older taken-down record on first filtered page, got %+v", page.Items)
	}
}
