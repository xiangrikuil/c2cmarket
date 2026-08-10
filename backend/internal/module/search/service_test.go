package search

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/officialprice"
)

func TestSearchInMemoryTraversesEveryAPIServicePage(t *testing.T) {
	reader := &pagedPublicReader{}
	service := NewService(nil, reader)

	results, appErr := service.Search(context.Background(), "second page model")
	if appErr != nil {
		t.Fatalf("search API services: %v", appErr)
	}
	if len(results) != 1 || results[0].ID != "api-service-2" {
		t.Fatalf("expected second-page API service result, got %+v", results)
	}
	if len(reader.apiRequests) != 2 {
		t.Fatalf("expected two API service page requests, got %+v", reader.apiRequests)
	}
	if reader.apiRequests[0].Limit != 100 || reader.apiRequests[0].Cursor != "" || reader.apiRequests[1].Cursor != "api-page-2" {
		t.Fatalf("unexpected API service page requests: %+v", reader.apiRequests)
	}
}

type pagedPublicReader struct {
	apiRequests []domain.PageRequest
}

func (r *pagedPublicReader) PublicOfficialPriceRecords(context.Context) ([]officialprice.Record, *domain.AppError) {
	return []officialprice.Record{}, nil
}

func (r *pagedPublicReader) PublicCarpoolListings(context.Context, carpool.ListingFilter, domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError) {
	return domain.Page[carpool.Listing]{Items: []carpool.Listing{}}, nil
}

func (r *pagedPublicReader) PublicAPIServices(_ context.Context, _ apimarket.PublicServiceFilter, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError) {
	r.apiRequests = append(r.apiRequests, page)
	if page.Cursor == "" {
		next := "api-page-2"
		return domain.Page[apimarket.Service]{
			Items:      []apimarket.Service{{ID: "service-1", Title: "unrelated service"}},
			NextCursor: &next,
		}, nil
	}
	return domain.Page[apimarket.Service]{Items: []apimarket.Service{{
		ID:                  "service-2",
		Title:               "second page model",
		MerchantDisplayName: "seller",
		UpdatedAt:           time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC),
	}}}, nil
}
