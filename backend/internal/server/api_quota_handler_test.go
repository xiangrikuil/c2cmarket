package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/apiquota"
	app "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/module/idempotency"
)

func TestAPIQuotaSystemSaleSlotsResponseAndSlotFilter(t *testing.T) {
	now := time.Date(2026, 7, 23, 23, 0, 0, 0, time.UTC)
	server := newTestServer(now)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-quota-sale-slots", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("slot list status %d body %s", response.Code, response.Body.String())
	}
	var payload apiQuotaSystemSaleSlotListResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode slot list: %v", err)
	}
	if payload.ServerNow != now.Format(time.RFC3339Nano) || len(payload.Items) != 21 {
		t.Fatalf("unexpected slot list: %+v", payload)
	}
	if first := payload.Items[0]; first.Key != "2026-07-24@09:00" || first.State != apiquota.SystemSlotStateRegistrationOpen {
		t.Fatalf("unexpected first slot: %+v", first)
	}

	invalid := httptest.NewRequest(http.MethodGet, "/api/v1/api-quota-offers?slotKey=2026-07-24%4010%3A15", nil)
	invalidResponse := httptest.NewRecorder()
	server.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid slot filter status %d body %s", invalidResponse.Code, invalidResponse.Body.String())
	}
	assertProblemCode(t, invalidResponse, domain.CodeValidationFailed)
}

func TestPublicAPIQuotaOfferListPassesMarketFiltersThrough(t *testing.T) {
	service := &quotaFilterRouteService{ApplicationService: app.NewServiceWithClock(time.Now)}
	server := NewServer(service)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-quota-offers?limit=7&cursor=next&distributionSystem=sub2api&modelCatalogId=model-1&maxMultiplier=1.2&onlyOrderable=true&saleMode=scheduled&search=gpt&excludeSystemSlots=true&sort=unit_price_asc", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("quota filter status %d body %s", response.Code, response.Body.String())
	}
	if service.page.Limit != 7 || service.page.Cursor != "next" {
		t.Fatalf("unexpected page request: %+v", service.page)
	}
	if service.filter.DistributionSystem != apiquota.DistributionSub2API || service.filter.ModelCatalogID != "model-1" ||
		service.filter.MaxMultiplier != "1.2" || !service.filter.OnlyOrderable || service.filter.SaleMode != apiquota.SaleModeScheduled ||
		service.filter.Search != "gpt" || !service.filter.ExcludeSystemSlots || service.filter.Sort != apiquota.PublicOfferSortUnitPriceAsc {
		t.Fatalf("unexpected quota filters: %+v", service.filter)
	}
}

type quotaFilterRouteService struct {
	ApplicationService
	filter apiquota.PublicOfferFilter
	page   domain.PageRequest
}

func (s *quotaFilterRouteService) PublicAPIQuotaOffers(_ context.Context, filter apiquota.PublicOfferFilter, page domain.PageRequest) (domain.Page[apiquota.OfferCard], *domain.AppError) {
	s.filter = filter
	s.page = page
	return domain.Page[apiquota.OfferCard]{Items: []apiquota.OfferCard{}}, nil
}

func TestAPIQuotaRushOfferManualPublication(t *testing.T) {
	now := time.Date(2026, 7, 23, 23, 0, 0, 0, time.UTC)
	base := app.NewServiceWithClock(func() time.Time { return now })
	service := &quotaRushHTTPTestService{
		ApplicationService: base,
		publication: apiquota.RushOfferPublication{
			Batch: apiquota.Batch{
				ID:                        "20000000-0000-0000-0000-000000000001",
				APIServiceID:              "10000000-0000-0000-0000-000000000001",
				SourceType:                apiquota.SourceTypeSub2API,
				Status:                    apiquota.BatchStatusPublished,
				DeclaredTotalUSDAllowance: "50.000000",
				UnallocatedUSDAllowance:   "0.000000",
				SaleCutoffAt:              now.Add(150 * time.Minute),
				ExpiresAt:                 now.Add(4 * time.Hour),
				SourceConfirmedAt:         now,
				Version:                   2,
			},
			Offer: apiquota.Offer{
				ID:                 "30000000-0000-0000-0000-000000000001",
				BatchID:            "20000000-0000-0000-0000-000000000001",
				APIServiceID:       "10000000-0000-0000-0000-000000000001",
				DistributionSystem: apiquota.DistributionSub2API,
				Name:               "$50 限时额度包",
				USDAllowance:       "50.000000",
				PriceCNY:           "5.00",
				CNYPerUSD:          "0.100000",
				ModelMultiplier:    "1.0000",
				DeliveryMode:       apiquota.DeliveryModeManual,
				DeliveryETAMinutes: 10,
				SaleMode:           apiquota.SaleModeScheduled,
				Status:             apiquota.OfferStatusPublished,
				Version:            2,
			},
			Round: apiquota.SaleRound{
				ID:            "40000000-0000-0000-0000-000000000001",
				BatchID:       "20000000-0000-0000-0000-000000000001",
				SystemSlotKey: "2026-07-24@09:00",
				Name:          "2026-07-24@09:00",
				StartsAt:      now.Add(2 * time.Hour),
				EndsAt:        now.Add(150 * time.Minute),
				Status:        apiquota.RoundStatusScheduled,
				Allocations: []apiquota.Allocation{{
					ID: "50000000-0000-0000-0000-000000000001", OfferID: "30000000-0000-0000-0000-000000000001",
					SaleRoundID: "40000000-0000-0000-0000-000000000001", SaleMode: apiquota.SaleModeScheduled,
					CopyLimit: 1, AvailableCopies: 1, AllocatedUSDAllowance: "50.000000",
					ReturnedUSDAllowance: "0.000000", Status: "active",
				}},
				Version: 1,
			},
			CredentialImported: 0,
			CredentialSummary: apiquota.CredentialSummary{
				OfferID: "30000000-0000-0000-0000-000000000001",
			},
		},
	}
	server := NewServer(service)
	session := createSession(t, server, "quota-rush-owner", false)
	payload := `{
		"sourceType":"sub2api",
		"sourceLabel":"",
		"name":"$50 限时额度包",
		"usdAllowance":"50",
		"priceCny":"5",
		"modelMultiplier":"1",
		"copies":1,
		"deliveryMode":"manual",
		"deliveryEtaMinutes":10,
		"slotKey":"2026-07-24@09:00",
		"expiresAt":"2026-07-24T11:00:00+08:00",
		"sourceConfirmedAt":"2026-07-24T07:00:00+08:00"
	}`
	request := newQuotaRushMultipartRequest(t, payload, "", "")
	addAuth(request, session, "quota-rush-http-create")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("rush publication status %d body %s", response.Code, response.Body.String())
	}
	if service.input.APIServiceID != "10000000-0000-0000-0000-000000000001" || service.input.DeliveryMode != apiquota.DeliveryModeManual || service.input.DeliveryKind != "" || len(service.input.CredentialRows) != 0 {
		t.Fatalf("handler did not pass manual publication input: %+v", service.input)
	}
	var result apiQuotaRushOfferResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode rush publication: %v", err)
	}
	if result.CredentialImported != 0 || result.Round.SystemSlotKey != "2026-07-24@09:00" {
		t.Fatalf("unexpected rush publication response: %+v", result)
	}
}

func TestAPIQuotaRushOfferMultipartRejectsMalformedAndMismatchedFiles(t *testing.T) {
	now := time.Date(2026, 7, 23, 23, 0, 0, 0, time.UTC)
	server := newTestServer(now)
	session := createSession(t, server, "quota-rush-boundary-owner", false)

	tests := []struct {
		name    string
		payload string
		file    string
	}{
		{name: "malformed payload", payload: `{"sourceType":`, file: ""},
		{name: "preimported without CSV", payload: `{"deliveryMode":"preimported","expiresAt":"2026-07-24T11:00:00+08:00","sourceConfirmedAt":"2026-07-24T07:00:00+08:00"}`, file: ""},
		{name: "manual with CSV", payload: `{"deliveryMode":"manual","expiresAt":"2026-07-24T11:00:00+08:00","sourceConfirmedAt":"2026-07-24T07:00:00+08:00"}`, file: "api_base_url,api_key,instructions\nhttps://api.example.test,sk-should-not-leak,buyer only\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fileName := ""
			if test.file != "" {
				fileName = "credentials.csv"
			}
			request := newQuotaRushMultipartRequest(t, test.payload, fileName, test.file)
			addAuth(request, session, "quota-rush-boundary-"+strings.ReplaceAll(test.name, " ", "-"))
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("boundary status %d body %s", response.Code, response.Body.String())
			}
			responseBody := response.Body.Bytes()
			var problem problemDetails
			if err := json.Unmarshal(responseBody, &problem); err != nil {
				t.Fatalf("decode boundary problem: %v", err)
			}
			if problem.Code != domain.CodeValidationFailed || problem.RequestID == "" {
				t.Fatalf("unexpected boundary problem: %+v", problem)
			}
			if test.name == "preimported without CSV" {
				if len(problem.Errors) != 1 || problem.Errors[0].Field != "deliveryMode" || problem.Errors[0].Code != "new_preimported_not_allowed" {
					t.Fatalf("unexpected preimported rejection: %+v", problem.Errors)
				}
			}
			if strings.Contains(string(responseBody), "sk-should-not-leak") {
				t.Fatalf("boundary response leaked uploaded credential: %s", responseBody)
			}
		})
	}
}

func TestAPIQuotaRoutesRejectInvalidBoundaryInput(t *testing.T) {
	now := time.Now().UTC()
	server := newTestServer(now)
	session := createSession(t, server, "quota-boundary-buyer", false)

	invalidFilter := httptest.NewRequest(http.MethodGet, "/api/v1/api-quota-offers?oneMultiplier=maybe", nil)
	invalidFilterResponse := httptest.NewRecorder()
	server.ServeHTTP(invalidFilterResponse, invalidFilter)
	if invalidFilterResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid filter status %d body %s", invalidFilterResponse.Code, invalidFilterResponse.Body.String())
	}
	assertProblemCode(t, invalidFilterResponse, domain.CodeValidationFailed)

	invalidDetail := httptest.NewRequest(http.MethodGet, "/api/v1/api-quota-offers/not-a-uuid", nil)
	invalidDetailResponse := httptest.NewRecorder()
	server.ServeHTTP(invalidDetailResponse, invalidDetail)
	if invalidDetailResponse.Code != http.StatusNotFound {
		t.Fatalf("invalid offer detail status %d body %s", invalidDetailResponse.Code, invalidDetailResponse.Body.String())
	}
	assertProblemCode(t, invalidDetailResponse, domain.CodeObjectNotFound)

	invalidOrder := newJSONRequest(http.MethodPost, "/api/v1/api-quota-offers/not-a-uuid/orders", `{
		"buyerContactMethodId":"10000000-0000-0000-0000-000000000001",
		"selectedAccessMode":"buyer_dedicated_sub_key",
		"paymentMethod":"wechat"
	}`)
	addAuth(invalidOrder, session, "quota-boundary-order")
	invalidOrderResponse := httptest.NewRecorder()
	server.ServeHTTP(invalidOrderResponse, invalidOrder)
	if invalidOrderResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid order status %d body %s", invalidOrderResponse.Code, invalidOrderResponse.Body.String())
	}
	assertProblemCode(t, invalidOrderResponse, domain.CodeValidationFailed)

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	if err := writer.WriteField("deliveryKind", "api_key_endpoint"); err != nil {
		t.Fatalf("write multipart field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	invalidCSV := httptest.NewRequest(http.MethodPost, "/api/v1/owner/api-quota-offers/not-a-uuid/credentials/import", &multipartBody)
	invalidCSV.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(invalidCSV, session, "quota-boundary-csv")
	invalidCSVResponse := httptest.NewRecorder()
	server.ServeHTTP(invalidCSVResponse, invalidCSV)
	if invalidCSVResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("incomplete CSV upload status %d body %s", invalidCSVResponse.Code, invalidCSVResponse.Body.String())
	}
	assertProblemCode(t, invalidCSVResponse, domain.CodeValidationFailed)
	if strings.Contains(invalidCSVResponse.Body.String(), "api-key-secret") {
		t.Fatalf("CSV error response must not contain credential material")
	}
}

type quotaRushHTTPTestService struct {
	ApplicationService
	publication apiquota.RushOfferPublication
	input       apiquota.CreateRushOfferInput
}

func (s *quotaRushHTTPTestService) CreateAPIQuotaRushOfferWithIdempotency(_ context.Context, _ app.User, _, _, _ string, input apiquota.CreateRushOfferInput, buildCompletion apiquota.RushOfferCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	s.input = input
	return buildCompletion(s.publication)
}

func newQuotaRushMultipartRequest(t *testing.T, payload, fileName, fileContents string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("payload", payload); err != nil {
		t.Fatalf("write rush payload: %v", err)
	}
	if fileName != "" {
		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			t.Fatalf("create rush file: %v", err)
		}
		if _, err := part.Write([]byte(fileContents)); err != nil {
			t.Fatalf("write rush file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close rush multipart: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/owner/api-services/10000000-0000-0000-0000-000000000001/quota-rush-offers", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestAPIServicePromptAuditDeclarationIsPublicWithoutRetiredPerformanceFields(t *testing.T) {
	now := time.Now().UTC()
	server := newTestServer(now)
	owner := createLinuxDoSession(t, server, "quota-performance-owner")
	ownerContact := createContactMethod(t, server, owner, "telegram", "额度包卖家", "@quota_performance_owner")
	service := createAPIServiceWithPayload(t, server, owner, apiServicePayload(ownerContact.ID, "1.0000"), "quota-performance-create")
	if service.DeclaredTTFTBand != "" || service.DeclaredMaxConcurrency != 8 || service.PerformanceConfirmedAt != nil || service.PromptAuditEnabled == nil || *service.PromptAuditEnabled {
		t.Fatalf("unexpected owner publish declarations: %+v", service)
	}

	submitted := ownerAPIServiceAction(t, server, owner, service.ID, "submit-review", service.Version, "quota-performance-submit")
	published := ownerAPIServiceAction(t, server, owner, submitted.ID, "publish", submitted.Version, "quota-performance-publish")
	orderable := updateAPIServiceOrderSettings(t, server, owner, published.ID, published.Version, true, "quota-performance-settings")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-services/"+orderable.ID, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("public API service status %d body %s", response.Code, response.Body.String())
	}
	var public struct {
		DeclaredMaxConcurrency int                             `json:"declaredMaxConcurrency"`
		PromptAuditEnabled     *bool                           `json:"promptAuditEnabled"`
		HealthSummary          apiServiceHealthSummaryResponse `json:"healthSummary"`
	}
	bodyBytes := response.Body.Bytes()
	if err := json.Unmarshal(bodyBytes, &public); err != nil {
		t.Fatalf("decode public API service: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		t.Fatalf("decode public API service fields: %v", err)
	}
	if _, exists := raw["declaredTtftBand"]; exists {
		t.Fatalf("public API service leaked seller-declared TTFT")
	}
	if _, exists := raw["performanceConfirmedAt"]; exists {
		t.Fatalf("public API service leaked seller performance confirmation time")
	}
	if public.DeclaredMaxConcurrency != 8 || public.PromptAuditEnabled == nil || *public.PromptAuditEnabled || public.HealthSummary.State != apihealth.HealthStateNoSample ||
		public.HealthSummary.AvailabilityReason == nil || *public.HealthSummary.AvailabilityReason != apihealth.AvailabilityUnconfigured ||
		len(public.HealthSummary.Samples) != apihealth.SummarySlotCount {
		t.Fatalf("unexpected public platform health projection: %+v", public)
	}
}
