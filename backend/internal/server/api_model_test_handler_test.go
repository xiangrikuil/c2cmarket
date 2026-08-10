package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimodeltest"
	"c2c-market/backend/internal/module/auth"
	app "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/platform/openaiapi"
)

func TestAPIModelTesterRoutesRequireSessionAndCSRFAndNeverReturnKey(t *testing.T) {
	now := time.Date(2026, 8, 8, 8, 0, 0, 0, time.UTC)
	modelTester := &apiModelTesterRouteService{
		sources: []apimodeltest.OrderSource{{
			OrderID: "00000000-0000-0000-0000-000000000801", OrderNo: "API202608080001",
			ServiceTitle: "主 Sub2API", BaseURL: "https://api.example.com/v1", DeliveredAt: now,
		}},
		discovery: apimodeltest.Discovery{BaseURL: "https://api.example.com/v1", Models: []string{"gpt-4.1-mini"}, DiscoveredAt: now},
		test: apimodeltest.ModelTest{
			Model: "gpt-4.1-mini", TestedAt: now,
			Responses:       apimodeltest.ProtocolResult{Succeeded: true, HTTPStatusClass: 2, DurationMS: 11},
			ChatCompletions: apimodeltest.ProtocolResult{HTTPStatusClass: 4, DurationMS: 7, ErrorCode: openaiapi.ErrorProtocolUnsupported},
		},
	}
	handler := NewServer(app.NewServiceWithClock(func() time.Time { return now }), ServerOptions{EnableDevAuth: true, APIModelTester: modelTester})
	session := createSession(t, handler, "api-model-tester-user", false)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tools/api-model-tester/order-sources", nil)
	addCookie(listRequest, session.cookie)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), `"orderNo":"API202608080001"`) {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	assertPrivateNoStore(t, listResponse)

	body := `{"credentialSource":{"kind":"manual","baseUrl":"https://api.example.com/v1","apiKey":"sk-do-not-return","acknowledgeInsecureHttp":true}}`
	missingCSRF := newJSONRequest(http.MethodPost, "/api/v1/tools/api-model-tester/discover", body)
	addCookie(missingCSRF, session.cookie)
	missingCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden || modelTester.discoverCalls != 0 {
		t.Fatalf("missing csrf status=%d calls=%d body=%s", missingCSRFResponse.Code, modelTester.discoverCalls, missingCSRFResponse.Body.String())
	}

	discover := newJSONRequest(http.MethodPost, "/api/v1/tools/api-model-tester/discover", body)
	addAuth(discover, session, "unused")
	discoverResponse := httptest.NewRecorder()
	handler.ServeHTTP(discoverResponse, discover)
	if discoverResponse.Code != http.StatusOK || !strings.Contains(discoverResponse.Body.String(), `"gpt-4.1-mini"`) {
		t.Fatalf("discover status=%d body=%s", discoverResponse.Code, discoverResponse.Body.String())
	}
	assertPrivateNoStore(t, discoverResponse)
	if strings.Contains(discoverResponse.Body.String(), "sk-do-not-return") || modelTester.source.APIKey != "sk-do-not-return" || !modelTester.source.AcknowledgeInsecureHTTP {
		t.Fatalf("credential handling response=%s source=%+v", discoverResponse.Body.String(), modelTester.source)
	}

	testRequest := newJSONRequest(http.MethodPost, "/api/v1/tools/api-model-tester/test", `{"credentialSource":{"kind":"order","orderId":"00000000-0000-0000-0000-000000000801","acknowledgeInsecureHttp":true},"model":"gpt-4.1-mini"}`)
	addAuth(testRequest, session, "unused")
	testResponse := httptest.NewRecorder()
	handler.ServeHTTP(testResponse, testRequest)
	if testResponse.Code != http.StatusOK || !strings.Contains(testResponse.Body.String(), `"responsesResult":{"succeeded":true`) || !strings.Contains(testResponse.Body.String(), `"errorCode":"protocol_unsupported"`) {
		t.Fatalf("test status=%d body=%s", testResponse.Code, testResponse.Body.String())
	}
	assertPrivateNoStore(t, testResponse)
}

func assertPrivateNoStore(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("cache control=%q", response.Header().Get("Cache-Control"))
	}
}

type apiModelTesterRouteService struct {
	sources       []apimodeltest.OrderSource
	discovery     apimodeltest.Discovery
	test          apimodeltest.ModelTest
	source        apimodeltest.CredentialSource
	discoverCalls int
}

func (service *apiModelTesterRouteService) OrderSources(context.Context, auth.User) ([]apimodeltest.OrderSource, *domain.AppError) {
	return service.sources, nil
}

func (service *apiModelTesterRouteService) Discover(_ context.Context, _ auth.User, source apimodeltest.CredentialSource) (apimodeltest.Discovery, *domain.AppError) {
	service.source = source
	service.discoverCalls++
	return service.discovery, nil
}

func (service *apiModelTesterRouteService) Test(_ context.Context, _ auth.User, source apimodeltest.CredentialSource, model string) (apimodeltest.ModelTest, *domain.AppError) {
	service.source = source
	service.test.Model = model
	return service.test, nil
}
