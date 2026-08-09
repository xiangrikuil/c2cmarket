package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/v1/api-services?limit=1&paymentMethod=wechat&billingMode=fixed_package&packageModelCatalogId=model-1&packageDurationDays=7", nil)
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
		service.requests[0].Filter.PackageModelCatalogID != "model-1" || service.requests[0].Filter.PackageDurationDays != 7 ||
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

type apiMarketPaginationRequest struct {
	Filter apimarket.PublicServiceFilter
	Page   domain.PageRequest
}

type apiMarketPaginationRouteService struct {
	ApplicationService
	pages    map[string]domain.Page[apimarket.Service]
	requests []apiMarketPaginationRequest
}

func (s *apiMarketPaginationRouteService) PublicAPIServices(_ context.Context, filter apimarket.PublicServiceFilter, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError) {
	s.requests = append(s.requests, apiMarketPaginationRequest{Filter: filter, Page: page})
	return s.pages[page.Cursor], nil
}
