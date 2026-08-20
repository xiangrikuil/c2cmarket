package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
)

func TestPublicAPIServiceListPassesCursorAndFilterThrough(t *testing.T) {
	next := "next-api-page"
	service := &apiMarketPaginationRouteService{pages: map[string]domain.Page[apimarket.Service]{
		"": {
			Items:      []apimarket.Service{{ID: "service-1", Title: "first"}},
			NextCursor: &next,
		},
		"next-api-page": {
			Items: []apimarket.Service{{ID: "service-2", Title: "second"}},
		},
	}}
	handler := NewServer(service)

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/v1/api-services?limit=1&paymentMethod=wechat&billingMode=fixed_package&search=gpt&modelCatalogId=model-common&distributionSystem=sub2api&packageModelCatalogId=model-1&packageDurationDays=7&packagePriceCnyMax=20&packageMultiplierMax=1.2&maxCnyPerUsd=0.9&minimumIntentCnyMax=50&sort=package_price_asc", nil)
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRequest)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("first page status = %d body %s", firstResponse.Code, firstResponse.Body.String())
	}
	var first listResponse[publicAPIServiceResponse]
	if err := json.NewDecoder(firstResponse.Body).Decode(&first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != "service-1" || first.NextCursor == nil || *first.NextCursor != next {
		t.Fatalf("unexpected first page response: %+v", first)
	}

	secondRequest := httptest.NewRequest(http.MethodGet, "/api/v1/api-services?limit=1&paymentMethod=wechat&billingMode=fixed_package&packageModelCatalogId=model-1&packageDurationDays=7&cursor="+next, nil)
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("second page status = %d body %s", secondResponse.Code, secondResponse.Body.String())
	}
	var second listResponse[publicAPIServiceResponse]
	if err := json.NewDecoder(secondResponse.Body).Decode(&second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "service-2" || second.NextCursor != nil {
		t.Fatalf("unexpected second page response: %+v", second)
	}
	if len(service.requests) != 2 || service.requests[0].Filter.PaymentMethod != apimarket.PaymentMethodWechat ||
		service.requests[0].Filter.BillingMode != apimarket.ServiceBillingModeFixedPackage ||
		service.requests[0].Filter.Search != "gpt" || service.requests[0].Filter.ModelCatalogID != "model-common" ||
		service.requests[0].Filter.DistributionSystem != apimarket.ServiceDistributionSub2API ||
		service.requests[0].Filter.MaxCNYPerUSD != "0.9" || service.requests[0].Filter.MinimumIntentCNYMax != "50" ||
		service.requests[0].Filter.PackageModelCatalogID != "model-1" || service.requests[0].Filter.PackageDurationDays != 7 ||
		service.requests[0].Filter.PackagePriceCNYMax != "20" || service.requests[0].Filter.PackageMultiplierMax != "1.2" ||
		service.requests[0].Filter.Sort != apimarket.PublicServiceSortPackagePriceAsc ||
		service.requests[0].Page.Limit != 1 || service.requests[1].Page.Cursor != next {
		t.Fatalf("unexpected application requests: %+v", service.requests)
	}
}

func TestPublicAPIServiceListRejectsInvalidPagination(t *testing.T) {
	service := &apiMarketPaginationRouteService{}
	handler := NewServer(service)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-services?limit=0", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid pagination status = %d body %s", response.Code, response.Body.String())
	}
	if len(service.requests) != 0 {
		t.Fatalf("application service must not be called for invalid pagination: %+v", service.requests)
	}
}

func TestPublicAPIServiceListRejectsInvalidPackageDuration(t *testing.T) {
	service := &apiMarketPaginationRouteService{}
	handler := NewServer(service)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-services?packageDurationDays=weekly", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid package duration status = %d body %s", response.Code, response.Body.String())
	}
	if len(service.requests) != 0 {
		t.Fatalf("application service must not be called for invalid package duration: %+v", service.requests)
	}
}

func TestPublicAPIMarketAvailabilityReturnsAllSellableUnits(t *testing.T) {
	generatedAt := time.Date(2026, 8, 20, 8, 30, 0, 0, time.UTC)
	service := &apiMarketAvailabilityRouteService{availability: apimarket.PublicMarketAvailability{
		GeneratedAt: generatedAt, LimitedOffers: 4, FixedPackages: 6, MeteredServices: 2,
	}}
	handler := NewServer(service)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-market/availability", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("availability status = %d body %s", response.Code, response.Body.String())
	}
	var body publicAPIMarketAvailabilityResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode availability: %v", err)
	}
	if body.GeneratedAt != generatedAt.Format(time.RFC3339Nano) || body.LimitedOffers != 4 || body.FixedPackages != 6 || body.MeteredServices != 2 {
		t.Fatalf("unexpected availability response: %+v", body)
	}
}

type apiMarketPaginationRequest struct {
	Filter apimarket.PublicServiceFilter
	Page   domain.PageRequest
}

type apiMarketPaginationRouteService struct {
	ApplicationService
	pages    map[string]domain.Page[apimarket.Service]
	requests []apiMarketPaginationRequest
}

type apiMarketAvailabilityRouteService struct {
	ApplicationService
	availability apimarket.PublicMarketAvailability
}

func (s *apiMarketAvailabilityRouteService) PublicAPIMarketAvailability(context.Context) (apimarket.PublicMarketAvailability, *domain.AppError) {
	return s.availability, nil
}

func (s *apiMarketPaginationRouteService) PublicAPIServices(_ context.Context, filter apimarket.PublicServiceFilter, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError) {
	s.requests = append(s.requests, apiMarketPaginationRequest{Filter: filter, Page: page})
	return s.pages[page.Cursor], nil
}
