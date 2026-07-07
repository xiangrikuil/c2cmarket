package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/health"
	app "c2c-market/backend/internal/module/core"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()

	NewRouter().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var body HealthResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("expected status ok, got %q", body.Status)
	}
	if body.Service != "c2c-market-backend" {
		t.Fatalf("expected backend service name, got %q", body.Service)
	}
}

func TestRouterReturnsMethodNotAllowed(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/health", nil)
	response := httptest.NewRecorder()

	NewRouter().ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
}

func TestReadinessWithoutDatabase(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	NewRouter().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected ready status %d, got %d body %s", http.StatusOK, response.Code, response.Body.String())
	}

	var body readinessResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness: %v", err)
	}
	if body.Status != "ok" || body.Database != "not_configured" {
		t.Fatalf("unexpected readiness body: %+v", body)
	}
}

func TestReadinessReportsDatabaseFailure(t *testing.T) {
	reason := "schema migration query failed"
	checker := fakeReadinessChecker{status: health.Status{
		Configured:     true,
		OK:             false,
		CheckedAt:      time.Date(2026, 6, 21, 7, 0, 0, 0, time.UTC),
		FailureSummary: reason,
	}}
	server := NewServer(app.NewService(), ServerOptions{
		EnableDevAuth:    true,
		ReadinessChecker: checker,
	})
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected service unavailable, got %d body %s", response.Code, response.Body.String())
	}
	var body readinessResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness: %v", err)
	}
	if body.Status != "degraded" || body.Database != "error" || body.Reason == nil || *body.Reason != reason {
		t.Fatalf("unexpected readiness body: %+v", body)
	}
}

func TestReadinessReportsExpectedMigrationVersion(t *testing.T) {
	version := int64(35)
	dirty := false
	reason := "schema migration version is behind expected version"
	checker := fakeReadinessChecker{status: health.Status{
		Configured:            true,
		OK:                    false,
		SchemaVersion:         &version,
		SchemaDirty:           &dirty,
		ExpectedSchemaVersion: 36,
		CheckedAt:             time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
		FailureSummary:        reason,
	}}
	server := NewServer(app.NewService(), ServerOptions{
		EnableDevAuth:    true,
		ReadinessChecker: checker,
	})
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected service unavailable, got %d body %s", response.Code, response.Body.String())
	}
	var body readinessResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode readiness: %v", err)
	}
	if body.ExpectedSchemaVersion != 36 || body.SchemaVersion == nil || *body.SchemaVersion != version || body.SchemaDirty == nil || *body.SchemaDirty {
		t.Fatalf("unexpected readiness schema fields: %+v", body)
	}
	if body.Reason == nil || *body.Reason != reason {
		t.Fatalf("unexpected readiness reason: %+v", body)
	}
}

func TestDevSessionCanBeDisabled(t *testing.T) {
	server := NewServer(app.NewService(), ServerOptions{EnableDevAuth: false})
	request := newJSONRequest(http.MethodPost, "/api/v1/auth/dev-session", `{"username":"buyer"}`)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected dev session disabled as not found, got %d body %s", response.Code, response.Body.String())
	}
}

func TestProductionRoutesMatchOpenAPI(t *testing.T) {
	server := &Server{
		app:           app.NewService(),
		mux:           chi.NewRouter(),
		enableDevAuth: false,
	}
	server.routes()
	runtimeRoutes := collectChiRoutes(t, server.mux)
	openAPIRoutes := collectOpenAPIRoutes(t, false)

	assertRouteSetsEqual(t, runtimeRoutes, openAPIRoutes)
	for route := range runtimeRoutes {
		if strings.Contains(route, "/dev/") || strings.Contains(route, "/auth/dev-session") {
			t.Fatalf("production route must not include dev endpoint: %s", route)
		}
	}
}

func TestDevRoutesMatchOpenAPI(t *testing.T) {
	server := &Server{
		app:           app.NewService(),
		mux:           chi.NewRouter(),
		enableDevAuth: true,
	}
	server.routes()
	assertRouteSetsEqual(t, collectChiRoutes(t, server.mux), collectOpenAPIRoutes(t, true))
}

func TestDevSessionAndCSRF(t *testing.T) {
	server := newTestServer(time.Now())
	session := createSession(t, server, "admin", true)

	request := newJSONRequest(http.MethodPost, "/api/v1/official-price-leads", `{}`)
	addCookie(request, session.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected missing csrf forbidden, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "CSRF_TOKEN_INVALID")
}

func TestEmailRegistrationDisabled(t *testing.T) {
	server := newTestServer(time.Now())

	start := newJSONRequest(http.MethodPost, "/api/v1/auth/email-registration/start", `{"email":"new.user@example.com"}`)
	startResponse := httptest.NewRecorder()
	server.ServeHTTP(startResponse, start)
	if startResponse.Code != http.StatusForbidden {
		t.Fatalf("expected email registration disabled, got %d body %s", startResponse.Code, startResponse.Body.String())
	}
	assertProblemCode(t, startResponse, "EMAIL_REGISTRATION_DISABLED")
	for _, cookie := range startResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			t.Fatalf("disabled email registration start must not set session cookie: %+v", cookie)
		}
	}

	confirm := newJSONRequest(http.MethodPost, "/api/v1/auth/email-registration/confirm", `{"email":"new.user@example.com","code":"123456"}`)
	confirmResponse := httptest.NewRecorder()
	server.ServeHTTP(confirmResponse, confirm)
	if confirmResponse.Code != http.StatusForbidden {
		t.Fatalf("expected email registration confirm disabled, got %d body %s", confirmResponse.Code, confirmResponse.Body.String())
	}
	assertProblemCode(t, confirmResponse, "EMAIL_REGISTRATION_DISABLED")
	for _, cookie := range confirmResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			t.Fatalf("disabled email registration must not set session cookie: %+v", cookie)
		}
	}
}

func TestPublicResourceIDsAreUUIDs(t *testing.T) {
	server := newTestServer(time.Now())
	buyerSession := createSession(t, server, "buyer", false)
	adminSession := createSession(t, server, "admin", true)

	assertUUID(t, buyerSession.userID, "user id")
	lead := submitLead(t, server, buyerSession, "uuid-lead")
	assertUUID(t, lead.ID, "lead id")

	approveBody := `{
		"reason":"来源可访问，价格字段完整。",
		"resolvedProductPlanId":"00000000-0000-0000-0000-000000000303",
		"validFrom":"2026-06-21T00:00:00Z",
		"fxSnapshot":{"rateToCny":"0.12210000","source":"admin_configured_snapshot","observedAt":"2026-06-21T06:00:00Z"}
	}`
	approved := approveLead(t, server, adminSession, lead.ID, approveBody, "uuid-approve")
	assertUUID(t, approved.Record.ID, "price record id")

	contact := createContactMethod(t, server, buyerSession, "telegram", "UUID TG", "@uuid")
	assertUUID(t, contact.ID, "contact method id")

	sellerSession := createSession(t, server, "seller", false)
	sellerContact := createContactMethod(t, server, sellerSession, "telegram", "Seller UUID TG", "@seller_uuid")
	request := newJSONRequest(http.MethodPost, "/api/v1/dev/contact-sessions", `{
		"sellerUsername":"seller",
		"buyerContactMethodId":"`+contact.ID+`",
		"sellerContactMethodId":"`+sellerContact.ID+`",
		"durationSeconds":60
	}`)
	addAuth(request, buyerSession, "uuid-contact-session")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create contact session status %d body %s", response.Code, response.Body.String())
	}
	var created contactSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode contact session: %v", err)
	}
	assertUUID(t, created.ID, "contact session id")
}

func TestSessionCookieRemainsOpaqueToken(t *testing.T) {
	server := newTestServer(time.Now())
	session := createSession(t, server, "buyer", false)

	if _, err := uuid.Parse(session.cookie); err == nil {
		t.Fatalf("expected session cookie to remain opaque and not be a public UUID")
	}
}

func TestOfficialPriceLeadSubmitRejectsAuthorityFields(t *testing.T) {
	server := newTestServer(time.Now())
	session := createSession(t, server, "buyer", false)

	request := newJSONRequest(http.MethodPost, "/api/v1/official-price-leads", `{
		"productText":"ChatGPT Pro",
		"regionCode":"ph",
		"channel":"web",
		"openingMethod":"official_web",
		"sourceUrl":"https://linux.do/t/example/123",
		"observedAt":"2026-06-21T06:30:00Z",
		"billingPeriod":"monthly",
		"currency":"PHP",
		"originalAmount":"799.00",
		"originalPriceText":"PHP 799 / month",
		"taxIncluded":true,
		"fxRate":"0.12"
	}`)
	addAuth(request, session, "lead-authority")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for unknown authority field, got %d body %s", response.Code, response.Body.String())
	}
}

func TestOfficialPriceLeadSubmitRejectsCredentialURL(t *testing.T) {
	server := newTestServer(time.Now())
	session := createSession(t, server, "buyer", false)

	request := newJSONRequest(http.MethodPost, "/api/v1/official-price-leads", leadPayload(`https://example.com/post?access_token=secret`))
	addAuth(request, session, "lead-url")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected url validation error, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "SECRET_CONTENT_DETECTED")
}

func TestOfficialPriceLeadApproveIsIdempotent(t *testing.T) {
	server := newTestServer(time.Now())
	buyerSession := createSession(t, server, "buyer", false)
	adminSession := createSession(t, server, "admin", true)

	lead := submitLead(t, server, buyerSession, "lead-create")
	approveBody := `{
		"reason":"来源可访问，价格字段完整。",
		"resolvedProductPlanId":"00000000-0000-0000-0000-000000000303",
		"validFrom":"2026-06-21T00:00:00Z",
		"fxSnapshot":{"rateToCny":"0.12210000","source":"admin_configured_snapshot","observedAt":"2026-06-21T06:00:00Z"}
	}`

	first := approveLead(t, server, adminSession, lead.ID, approveBody, "approve-key")
	second := approveLead(t, server, adminSession, lead.ID, approveBody, "approve-key")

	if first.Record.ID == "" || first.Record.ID != second.Record.ID {
		t.Fatalf("expected idempotent record replay, got %q and %q", first.Record.ID, second.Record.ID)
	}
	if first.Record.NormalizedMonthlyCNY != "97.56" {
		t.Fatalf("expected normalized price 97.56, got %q", first.Record.NormalizedMonthlyCNY)
	}
}

func TestOfficialPriceLeadApproveRequiresIfMatch(t *testing.T) {
	server := newTestServer(time.Now())
	buyerSession := createSession(t, server, "buyer", false)
	adminSession := createSession(t, server, "admin", true)
	lead := submitLead(t, server, buyerSession, "lead-if-match-create")

	body := `{
		"reason":"来源可访问，价格字段完整。",
		"resolvedProductPlanId":"00000000-0000-0000-0000-000000000303",
		"validFrom":"2026-06-21T00:00:00Z",
		"fxSnapshot":{"rateToCny":"0.12210000","source":"admin_configured_snapshot","observedAt":"2026-06-21T06:00:00Z"}
	}`
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/official-price-leads/"+lead.ID+"/approve", body)
	addAuth(request, adminSession, "approve-no-if-match")
	request.Header.Del("If-Match")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected precondition required, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "PRECONDITION_REQUIRED")
}

func TestPublicOfficialPricesExposeApprovedRecords(t *testing.T) {
	server := newTestServer(time.Now())
	buyerSession := createSession(t, server, "buyer", false)
	adminSession := createSession(t, server, "admin", true)

	lead := submitLead(t, server, buyerSession, "public-price-create")
	approveBody := `{
		"reason":"来源可访问，价格字段完整。",
		"resolvedProductPlanId":"00000000-0000-0000-0000-000000000303",
		"validFrom":"2026-06-21T00:00:00Z",
		"fxSnapshot":{"rateToCny":"0.12210000","source":"admin_configured_snapshot","observedAt":"2026-06-21T06:00:00Z"}
	}`
	approved := approveLead(t, server, adminSession, lead.ID, approveBody, "public-price-approve")

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/official-prices", nil)
	listResponseRecorder := httptest.NewRecorder()
	server.ServeHTTP(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("list public prices status %d body %s", listResponseRecorder.Code, listResponseRecorder.Body.String())
	}
	var list listResponse[priceRecordResponse]
	if err := json.NewDecoder(listResponseRecorder.Body).Decode(&list); err != nil {
		t.Fatalf("decode public price list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("expected one public price record, got %d", len(list.Items))
	}
	if list.Items[0].ID != approved.Record.ID || list.Items[0].NormalizedMonthlyCNY != "97.56" {
		t.Fatalf("unexpected public price item: %+v", list.Items[0])
	}
	if list.Items[0].FXObservedAt == "" || list.Items[0].SourceURL == "" {
		t.Fatalf("expected public price item to include source and fx observation metadata: %+v", list.Items[0])
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/official-prices/"+approved.Record.ID, nil)
	detailResponseRecorder := httptest.NewRecorder()
	server.ServeHTTP(detailResponseRecorder, detailRequest)
	if detailResponseRecorder.Code != http.StatusOK {
		t.Fatalf("get public price status %d body %s", detailResponseRecorder.Code, detailResponseRecorder.Body.String())
	}
	var detail priceRecordResponse
	if err := json.NewDecoder(detailResponseRecorder.Body).Decode(&detail); err != nil {
		t.Fatalf("decode public price detail: %v", err)
	}
	if detail.ID != approved.Record.ID || detail.OfferKey == "" {
		t.Fatalf("unexpected public price detail: %+v", detail)
	}
}

func TestPublicOfficialPricesSortByPriceAndMarkLowestReference(t *testing.T) {
	server := newTestServer(time.Now())
	buyerSession := createSession(t, server, "buyer", false)
	adminSession := createSession(t, server, "admin", true)

	expensive := createApprovedOfficialPriceRecord(t, server, buyerSession, adminSession, "public-price-expensive", "999.00", "0.12000000")
	cheap := createApprovedOfficialPriceRecord(t, server, buyerSession, adminSession, "public-price-cheap", "799.00", "0.12000000")

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/official-prices", nil)
	listResponseRecorder := httptest.NewRecorder()
	server.ServeHTTP(listResponseRecorder, listRequest)
	if listResponseRecorder.Code != http.StatusOK {
		t.Fatalf("list public prices status %d body %s", listResponseRecorder.Code, listResponseRecorder.Body.String())
	}
	var list listResponse[priceRecordResponse]
	if err := json.NewDecoder(listResponseRecorder.Body).Decode(&list); err != nil {
		t.Fatalf("decode public price list: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("expected two public price records, got %d", len(list.Items))
	}
	if list.Items[0].ID != cheap.Record.ID || list.Items[0].NormalizedMonthlyCNY != "95.88" {
		t.Fatalf("expected cheapest record first, got %+v", list.Items[0])
	}
	if !list.Items[0].IsLowestReference || !list.Items[1].IsLowestReference {
		t.Fatalf("expected active records in separate API-created groups to include lowest reference marker, got %+v", list.Items)
	}
	if list.Items[1].ID != expensive.Record.ID || list.Items[1].NormalizedMonthlyCNY != "119.88" {
		t.Fatalf("expected expensive record second, got %+v", list.Items[1])
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/official-prices/"+cheap.Record.ID, nil)
	detailResponseRecorder := httptest.NewRecorder()
	server.ServeHTTP(detailResponseRecorder, detailRequest)
	if detailResponseRecorder.Code != http.StatusOK {
		t.Fatalf("get public price status %d body %s", detailResponseRecorder.Code, detailResponseRecorder.Body.String())
	}
	var detail priceRecordResponse
	if err := json.NewDecoder(detailResponseRecorder.Body).Decode(&detail); err != nil {
		t.Fatalf("decode public price detail: %v", err)
	}
	if !detail.IsLowestReference {
		t.Fatalf("expected public price detail to include lowest reference marker: %+v", detail)
	}
}

func TestAdminProductPlanCRUD(t *testing.T) {
	server := newTestServer(time.Now())
	adminSession := createSession(t, server, "admin", true)

	createRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/product-plans", productPlanPayload("cursor", "cursor-ultra", "Cursor Ultra", "allowed", "normal", false))
	addAuth(createRequest, adminSession, "product-plan-create")
	createResponse := httptest.NewRecorder()
	server.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create product plan status %d body %s", createResponse.Code, createResponse.Body.String())
	}
	var created productPlanResponse
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode created product plan: %v", err)
	}
	if created.Slug != "cursor-ultra" || !created.Active || created.PolicyVersion != 1 {
		t.Fatalf("unexpected created product plan: %+v", created)
	}

	updateRequest := newJSONRequest(http.MethodPatch, "/api/v1/admin/product-plans/"+created.ID, productPlanPayload("cursor", "cursor-ultra", "Cursor Ultra", "info_only", "elevated", false))
	addAuth(updateRequest, adminSession, "product-plan-update")
	updateResponse := httptest.NewRecorder()
	server.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update product plan status %d body %s", updateResponse.Code, updateResponse.Body.String())
	}
	var updated productPlanResponse
	if err := json.NewDecoder(updateResponse.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated product plan: %v", err)
	}
	if updated.PublishPolicy != "info_only" || updated.PolicyVersion != 2 {
		t.Fatalf("expected policy update to increment version, got %+v", updated)
	}

	deactivateRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/product-plans/"+created.ID+"/deactivate", `{}`)
	addAuth(deactivateRequest, adminSession, "product-plan-deactivate")
	deactivateResponse := httptest.NewRecorder()
	server.ServeHTTP(deactivateResponse, deactivateRequest)
	if deactivateResponse.Code != http.StatusOK {
		t.Fatalf("deactivate product plan status %d body %s", deactivateResponse.Code, deactivateResponse.Body.String())
	}

	publicList := httptest.NewRequest(http.MethodGet, "/api/v1/product-plans?category=cursor", nil)
	publicListResponse := httptest.NewRecorder()
	server.ServeHTTP(publicListResponse, publicList)
	if publicListResponse.Code != http.StatusOK {
		t.Fatalf("public product plan list status %d body %s", publicListResponse.Code, publicListResponse.Body.String())
	}
	if strings.Contains(publicListResponse.Body.String(), `"slug":"cursor-ultra"`) {
		t.Fatalf("public product plan list must hide inactive rows, got %s", publicListResponse.Body.String())
	}

	adminList := httptest.NewRequest(http.MethodGet, "/api/v1/admin/product-plans?category=cursor", nil)
	addCookie(adminList, adminSession.cookie)
	adminListResponse := httptest.NewRecorder()
	server.ServeHTTP(adminListResponse, adminList)
	if adminListResponse.Code != http.StatusOK {
		t.Fatalf("admin product plan list status %d body %s", adminListResponse.Code, adminListResponse.Body.String())
	}
	if !strings.Contains(adminListResponse.Body.String(), `"slug":"cursor-ultra"`) || !strings.Contains(adminListResponse.Body.String(), `"active":false`) {
		t.Fatalf("admin product plan list must include inactive rows, got %s", adminListResponse.Body.String())
	}
}

func TestAdminAPIModelCRUDAndPublicVisibility(t *testing.T) {
	server := newTestServer(time.Now())
	adminSession := createSession(t, server, "admin", true)

	provider := createAdminAPIModelProvider(t, server, adminSession, "openai-local", "api-model-provider-create")

	createRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models", apiModelPayload(provider.ID, "GPT-Test-Model", "GPT Test Model", "0.150", "0.075", "0.600", "pricing-v1", true))
	addAuth(createRequest, adminSession, "api-model-create")
	createResponse := httptest.NewRecorder()
	server.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create API model status %d body %s", createResponse.Code, createResponse.Body.String())
	}
	var created apiModelResponse
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode created API model: %v", err)
	}
	if created.ModelKey != "GPT-Test-Model" || !created.Active || created.InputPricePerMillion != "0.150000" || created.CurrentPriceVersionID == "" {
		t.Fatalf("unexpected created API model: %+v", created)
	}
	if created.ProviderID != provider.ID || created.Provider != provider.DisplayName || created.ProviderCategory != provider.ProviderCategory {
		t.Fatalf("expected model to derive provider fields from provider catalog, got model=%+v provider=%+v", created, provider)
	}
	if got := strings.Join(created.Capabilities, ","); got != "text,chat,vision" {
		t.Fatalf("expected normalized capabilities, got %q", got)
	}

	publicList := httptest.NewRequest(http.MethodGet, "/api/v1/api-models", nil)
	publicListResponse := httptest.NewRecorder()
	server.ServeHTTP(publicListResponse, publicList)
	if publicListResponse.Code != http.StatusOK {
		t.Fatalf("public API model list status %d body %s", publicListResponse.Code, publicListResponse.Body.String())
	}
	if !strings.Contains(publicListResponse.Body.String(), `"modelKey":"GPT-Test-Model"`) {
		t.Fatalf("public API model list should include active model, got %s", publicListResponse.Body.String())
	}

	samePriceRequest := newJSONRequest(http.MethodPatch, "/api/v1/admin/api-models/"+created.ID, apiModelPayload(provider.ID, "GPT-Test-Model", "GPT Test Model", "0.1500", "0.07500", "0.6000", "pricing-v1", true))
	addAuth(samePriceRequest, adminSession, "api-model-same-price")
	samePriceResponse := httptest.NewRecorder()
	server.ServeHTTP(samePriceResponse, samePriceRequest)
	if samePriceResponse.Code != http.StatusOK {
		t.Fatalf("same price update status %d body %s", samePriceResponse.Code, samePriceResponse.Body.String())
	}
	var samePrice apiModelResponse
	if err := json.NewDecoder(samePriceResponse.Body).Decode(&samePrice); err != nil {
		t.Fatalf("decode same price API model: %v", err)
	}
	if samePrice.CurrentPriceVersionID != created.CurrentPriceVersionID {
		t.Fatalf("numeric-equivalent price update must not create new version: before=%s after=%s", created.CurrentPriceVersionID, samePrice.CurrentPriceVersionID)
	}

	updateRequest := newJSONRequest(http.MethodPatch, "/api/v1/admin/api-models/"+created.ID, apiModelPayload(provider.ID, "GPT-Test-Model", "GPT Test Model", "0.200", "0.100", "0.700", "pricing-v2", true))
	addAuth(updateRequest, adminSession, "api-model-update")
	updateResponse := httptest.NewRecorder()
	server.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update API model status %d body %s", updateResponse.Code, updateResponse.Body.String())
	}
	var updated apiModelResponse
	if err := json.NewDecoder(updateResponse.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated API model: %v", err)
	}
	if updated.InputPricePerMillion != "0.200000" || updated.CurrentPriceSourceVersion != "pricing-v2" || updated.CurrentPriceVersionID == created.CurrentPriceVersionID {
		t.Fatalf("expected price update to create new current version, got %+v", updated)
	}

	clearRequest := newJSONRequest(http.MethodPatch, "/api/v1/admin/api-models/"+created.ID, apiModelPayload(provider.ID, "GPT-Test-Model", "GPT Test Model", "", "", "", "", true))
	addAuth(clearRequest, adminSession, "api-model-clear-price")
	clearResponse := httptest.NewRecorder()
	server.ServeHTTP(clearResponse, clearRequest)
	if clearResponse.Code != http.StatusOK {
		t.Fatalf("clear price status %d body %s", clearResponse.Code, clearResponse.Body.String())
	}
	var cleared apiModelResponse
	if err := json.NewDecoder(clearResponse.Body).Decode(&cleared); err != nil {
		t.Fatalf("decode cleared API model: %v", err)
	}
	if cleared.InputPricePerMillion != "" || cleared.CachedInputPricePerMillion != "" || cleared.OutputPricePerMillion != "" || cleared.CurrentPriceVersionID == updated.CurrentPriceVersionID {
		t.Fatalf("expected clearing prices to create current null version, got %+v", cleared)
	}

	deactivateRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models/"+created.ID+"/deactivate", `{}`)
	addAuth(deactivateRequest, adminSession, "api-model-deactivate")
	deactivateResponse := httptest.NewRecorder()
	server.ServeHTTP(deactivateResponse, deactivateRequest)
	if deactivateResponse.Code != http.StatusOK {
		t.Fatalf("deactivate API model status %d body %s", deactivateResponse.Code, deactivateResponse.Body.String())
	}

	publicAfterDeactivate := httptest.NewRequest(http.MethodGet, "/api/v1/api-models", nil)
	publicAfterDeactivateResponse := httptest.NewRecorder()
	server.ServeHTTP(publicAfterDeactivateResponse, publicAfterDeactivate)
	if publicAfterDeactivateResponse.Code != http.StatusOK {
		t.Fatalf("public API model list after deactivate status %d body %s", publicAfterDeactivateResponse.Code, publicAfterDeactivateResponse.Body.String())
	}
	if strings.Contains(publicAfterDeactivateResponse.Body.String(), `"modelKey":"GPT-Test-Model"`) {
		t.Fatalf("public API model list must hide inactive rows, got %s", publicAfterDeactivateResponse.Body.String())
	}

	adminList := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-models", nil)
	addCookie(adminList, adminSession.cookie)
	adminListResponse := httptest.NewRecorder()
	server.ServeHTTP(adminListResponse, adminList)
	if adminListResponse.Code != http.StatusOK {
		t.Fatalf("admin API model list status %d body %s", adminListResponse.Code, adminListResponse.Body.String())
	}
	if !strings.Contains(adminListResponse.Body.String(), `"modelKey":"GPT-Test-Model"`) || !strings.Contains(adminListResponse.Body.String(), `"active":false`) {
		t.Fatalf("admin API model list must include inactive rows, got %s", adminListResponse.Body.String())
	}
}

func TestAdminAPIModelValidationAndAuth(t *testing.T) {
	server := newTestServer(time.Now())
	adminSession := createSession(t, server, "admin-api-model-validation", true)
	userSession := createSession(t, server, "user-api-model-validation", false)
	provider := createAdminAPIModelProvider(t, server, adminSession, "validation-openai", "api-model-validation-provider-create")

	nonAdminList := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-models", nil)
	addCookie(nonAdminList, userSession.cookie)
	nonAdminListResponse := httptest.NewRecorder()
	server.ServeHTTP(nonAdminListResponse, nonAdminList)
	if nonAdminListResponse.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin list forbidden, got %d body %s", nonAdminListResponse.Code, nonAdminListResponse.Body.String())
	}

	missingCSRF := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models", apiModelPayload(provider.ID, "csrf-model", "CSRF Model", "0.1", "", "0.2", "pricing", true))
	addCookie(missingCSRF, adminSession.cookie)
	missingCSRFResponse := httptest.NewRecorder()
	server.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("expected missing CSRF forbidden, got %d body %s", missingCSRFResponse.Code, missingCSRFResponse.Body.String())
	}

	created := createAdminAPIModel(t, server, adminSession, "Duplicate-Model", "duplicate-create")
	duplicate := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models", apiModelPayload(provider.ID, created.ModelKey, "Duplicate Model Two", "0.1", "", "0.2", "pricing", true))
	addAuth(duplicate, adminSession, "duplicate-create-two")
	duplicateResponse := httptest.NewRecorder()
	server.ServeHTTP(duplicateResponse, duplicate)
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("expected duplicate modelKey conflict, got %d body %s", duplicateResponse.Code, duplicateResponse.Body.String())
	}

	cases := []struct {
		name string
		body string
	}{
		{name: "invalid-provider", body: apiModelPayload("00000000-0000-0000-0000-00000000ffff", "bad-provider-model", "Bad Provider", "0.1", "", "0.2", "pricing", true)},
		{name: "empty-display-name", body: apiModelPayload(provider.ID, "empty-display-model", "", "0.1", "", "0.2", "pricing", true)},
		{name: "empty-capabilities", body: apiModelPayloadWithCapabilities(provider.ID, "empty-cap-model", "Empty Cap", []string{}, "0.1", "", "0.2", "pricing", true)},
		{name: "invalid-capabilities", body: apiModelPayloadWithCapabilities(provider.ID, "bad-cap-model", "Bad Cap", []string{"chat", "audio"}, "0.1", "", "0.2", "pricing", true)},
		{name: "negative-price", body: apiModelPayload(provider.ID, "negative-price-model", "Negative Price", "-0.1", "", "0.2", "pricing", true)},
		{name: "non-number-price", body: apiModelPayload(provider.ID, "non-number-model", "Non Number", "abc", "", "0.2", "pricing", true)},
		{name: "nan-price", body: apiModelPayload(provider.ID, "nan-model", "NaN Model", "NaN", "", "0.2", "pricing", true)},
		{name: "infinity-price", body: apiModelPayload(provider.ID, "infinity-model", "Infinity Model", "Infinity", "", "0.2", "pricing", true)},
	}
	for _, item := range cases {
		request := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models", item.body)
		addAuth(request, adminSession, "api-model-validation-"+item.name)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: expected validation failure, got %d body %s", item.name, response.Code, response.Body.String())
		}
	}

	deactivateProviderRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/api-model-providers/"+provider.ID+"/deactivate", `{}`)
	addAuth(deactivateProviderRequest, adminSession, "api-model-provider-deactivate-validation")
	deactivateProviderResponse := httptest.NewRecorder()
	server.ServeHTTP(deactivateProviderResponse, deactivateProviderRequest)
	if deactivateProviderResponse.Code != http.StatusOK {
		t.Fatalf("deactivate provider status %d body %s", deactivateProviderResponse.Code, deactivateProviderResponse.Body.String())
	}
	inactiveProviderModel := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models", apiModelPayload(provider.ID, "inactive-provider-model", "Inactive Provider Model", "0.1", "", "0.2", "pricing", true))
	addAuth(inactiveProviderModel, adminSession, "api-model-inactive-provider")
	inactiveProviderResponse := httptest.NewRecorder()
	server.ServeHTTP(inactiveProviderResponse, inactiveProviderModel)
	if inactiveProviderResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected inactive provider validation failure, got %d body %s", inactiveProviderResponse.Code, inactiveProviderResponse.Body.String())
	}
}

func TestInactiveAPIModelCannotBeUsedForAPIServiceCreateOrUpdate(t *testing.T) {
	server := newTestServer(time.Now())
	adminSession := createSession(t, server, "admin-api-model-inactive", true)
	ownerSession := createLinuxDoSession(t, server, "api-model-inactive-owner")
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Inactive API Owner TG", "@inactive_api_owner")
	model := createAdminAPIModel(t, server, adminSession, "inactive-service-model", "inactive-model-create")

	deactivateRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models/"+model.ID+"/deactivate", `{}`)
	addAuth(deactivateRequest, adminSession, "inactive-model-deactivate")
	deactivateResponse := httptest.NewRecorder()
	server.ServeHTTP(deactivateResponse, deactivateRequest)
	if deactivateResponse.Code != http.StatusOK {
		t.Fatalf("deactivate model status %d body %s", deactivateResponse.Code, deactivateResponse.Body.String())
	}

	createWithInactive := newJSONRequest(http.MethodPost, "/api/v1/owner/api-services", apiServicePayloadWithModel(ownerContact.ID, model.ID))
	addAuth(createWithInactive, ownerSession, "api-service-inactive-model-create")
	createWithInactiveResponse := httptest.NewRecorder()
	server.ServeHTTP(createWithInactiveResponse, createWithInactive)
	if createWithInactiveResponse.Code != http.StatusNotFound {
		t.Fatalf("inactive model create should fail, got %d body %s", createWithInactiveResponse.Code, createWithInactiveResponse.Body.String())
	}

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "api-service-active-model-create")
	updateWithInactive := newJSONRequest(http.MethodPatch, "/api/v1/owner/api-services/"+service.ID, apiServicePayloadWithModel(ownerContact.ID, model.ID))
	addAuth(updateWithInactive, ownerSession, "api-service-inactive-model-update")
	updateWithInactive.Header.Set("If-Match", `"`+strconv.FormatInt(service.Version, 10)+`"`)
	updateWithInactiveResponse := httptest.NewRecorder()
	server.ServeHTTP(updateWithInactiveResponse, updateWithInactive)
	if updateWithInactiveResponse.Code != http.StatusNotFound {
		t.Fatalf("inactive model update should fail, got %d body %s", updateWithInactiveResponse.Code, updateWithInactiveResponse.Body.String())
	}
}

func TestInactiveAPIModelProviderHidesPublicModels(t *testing.T) {
	server := newTestServer(time.Now())
	adminSession := createSession(t, server, "admin-api-model-provider-inactive", true)
	provider := createAdminAPIModelProvider(t, server, adminSession, "provider-hide-public", "provider-hide-public-create")

	createModelRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models", apiModelPayload(provider.ID, "provider-hidden-model", "Provider Hidden Model", "0.1", "", "0.2", "pricing", true))
	addAuth(createModelRequest, adminSession, "provider-hidden-model-create")
	createModelResponse := httptest.NewRecorder()
	server.ServeHTTP(createModelResponse, createModelRequest)
	if createModelResponse.Code != http.StatusCreated {
		t.Fatalf("create model under provider status %d body %s", createModelResponse.Code, createModelResponse.Body.String())
	}

	publicBeforeDeactivate := httptest.NewRequest(http.MethodGet, "/api/v1/api-models", nil)
	publicBeforeDeactivateResponse := httptest.NewRecorder()
	server.ServeHTTP(publicBeforeDeactivateResponse, publicBeforeDeactivate)
	if publicBeforeDeactivateResponse.Code != http.StatusOK {
		t.Fatalf("public list before provider deactivate status %d body %s", publicBeforeDeactivateResponse.Code, publicBeforeDeactivateResponse.Body.String())
	}
	if !strings.Contains(publicBeforeDeactivateResponse.Body.String(), `"modelKey":"provider-hidden-model"`) {
		t.Fatalf("public list should include model before provider deactivate, got %s", publicBeforeDeactivateResponse.Body.String())
	}

	deactivateProviderRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/api-model-providers/"+provider.ID+"/deactivate", `{}`)
	addAuth(deactivateProviderRequest, adminSession, "provider-hide-public-deactivate")
	deactivateProviderResponse := httptest.NewRecorder()
	server.ServeHTTP(deactivateProviderResponse, deactivateProviderRequest)
	if deactivateProviderResponse.Code != http.StatusOK {
		t.Fatalf("deactivate provider status %d body %s", deactivateProviderResponse.Code, deactivateProviderResponse.Body.String())
	}

	publicAfterDeactivate := httptest.NewRequest(http.MethodGet, "/api/v1/api-models", nil)
	publicAfterDeactivateResponse := httptest.NewRecorder()
	server.ServeHTTP(publicAfterDeactivateResponse, publicAfterDeactivate)
	if publicAfterDeactivateResponse.Code != http.StatusOK {
		t.Fatalf("public list after provider deactivate status %d body %s", publicAfterDeactivateResponse.Code, publicAfterDeactivateResponse.Body.String())
	}
	if strings.Contains(publicAfterDeactivateResponse.Body.String(), `"modelKey":"provider-hidden-model"`) {
		t.Fatalf("public list must hide models under inactive providers, got %s", publicAfterDeactivateResponse.Body.String())
	}

	adminList := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-models", nil)
	addCookie(adminList, adminSession.cookie)
	adminListResponse := httptest.NewRecorder()
	server.ServeHTTP(adminListResponse, adminList)
	if adminListResponse.Code != http.StatusOK {
		t.Fatalf("admin list status %d body %s", adminListResponse.Code, adminListResponse.Body.String())
	}
	if !strings.Contains(adminListResponse.Body.String(), `"modelKey":"provider-hidden-model"`) || !strings.Contains(adminListResponse.Body.String(), `"providerActive":false`) {
		t.Fatalf("admin list must retain models under inactive providers, got %s", adminListResponse.Body.String())
	}
}

func TestAdminProductCategoryCRUDAndPublicVisibility(t *testing.T) {
	server := newTestServer(time.Now())
	adminSession := createSession(t, server, "admin", true)

	createCategoryRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/product-categories", `{
		"code":"vpn",
		"displayName":"VPN",
		"sortOrder":60,
		"active":true
	}`)
	addAuth(createCategoryRequest, adminSession, "product-category-create")
	createCategoryResponse := httptest.NewRecorder()
	server.ServeHTTP(createCategoryResponse, createCategoryRequest)
	if createCategoryResponse.Code != http.StatusCreated {
		t.Fatalf("create product category status %d body %s", createCategoryResponse.Code, createCategoryResponse.Body.String())
	}
	var category productCategoryResponse
	if err := json.NewDecoder(createCategoryResponse.Body).Decode(&category); err != nil {
		t.Fatalf("decode created category: %v", err)
	}
	if category.Code != "vpn" || category.DisplayName != "VPN" || !category.Active {
		t.Fatalf("unexpected created category: %+v", category)
	}

	createPlanRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/product-plans", productPlanPayloadWithCategoryID(category.ID, category.Code, "vpn-basic", "VPN Basic", "allowed", "normal", false))
	addAuth(createPlanRequest, adminSession, "product-plan-create-vpn")
	createPlanResponse := httptest.NewRecorder()
	server.ServeHTTP(createPlanResponse, createPlanRequest)
	if createPlanResponse.Code != http.StatusCreated {
		t.Fatalf("create product plan under dynamic category status %d body %s", createPlanResponse.Code, createPlanResponse.Body.String())
	}
	var plan productPlanResponse
	if err := json.NewDecoder(createPlanResponse.Body).Decode(&plan); err != nil {
		t.Fatalf("decode created plan: %v", err)
	}
	if plan.CategoryID != category.ID || plan.CategoryCode != "vpn" {
		t.Fatalf("expected plan to use created category, got %+v", plan)
	}

	deactivateCategoryRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/product-categories/"+category.ID+"/deactivate", `{}`)
	addAuth(deactivateCategoryRequest, adminSession, "product-category-deactivate")
	deactivateCategoryResponse := httptest.NewRecorder()
	server.ServeHTTP(deactivateCategoryResponse, deactivateCategoryRequest)
	if deactivateCategoryResponse.Code != http.StatusOK {
		t.Fatalf("deactivate product category status %d body %s", deactivateCategoryResponse.Code, deactivateCategoryResponse.Body.String())
	}

	publicCategories := httptest.NewRequest(http.MethodGet, "/api/v1/product-categories", nil)
	publicCategoriesResponse := httptest.NewRecorder()
	server.ServeHTTP(publicCategoriesResponse, publicCategories)
	if publicCategoriesResponse.Code != http.StatusOK {
		t.Fatalf("public product categories status %d body %s", publicCategoriesResponse.Code, publicCategoriesResponse.Body.String())
	}
	if strings.Contains(publicCategoriesResponse.Body.String(), `"code":"vpn"`) {
		t.Fatalf("public categories must hide inactive category, got %s", publicCategoriesResponse.Body.String())
	}

	publicPlans := httptest.NewRequest(http.MethodGet, "/api/v1/product-plans?category=vpn", nil)
	publicPlansResponse := httptest.NewRecorder()
	server.ServeHTTP(publicPlansResponse, publicPlans)
	if publicPlansResponse.Code != http.StatusOK {
		t.Fatalf("public product plans status %d body %s", publicPlansResponse.Code, publicPlansResponse.Body.String())
	}
	if strings.Contains(publicPlansResponse.Body.String(), `"slug":"vpn-basic"`) {
		t.Fatalf("public product plans must hide plans under inactive category, got %s", publicPlansResponse.Body.String())
	}

	adminCategories := httptest.NewRequest(http.MethodGet, "/api/v1/admin/product-categories", nil)
	addCookie(adminCategories, adminSession.cookie)
	adminCategoriesResponse := httptest.NewRecorder()
	server.ServeHTTP(adminCategoriesResponse, adminCategories)
	if adminCategoriesResponse.Code != http.StatusOK {
		t.Fatalf("admin product categories status %d body %s", adminCategoriesResponse.Code, adminCategoriesResponse.Body.String())
	}
	if !strings.Contains(adminCategoriesResponse.Body.String(), `"code":"vpn"`) || !strings.Contains(adminCategoriesResponse.Body.String(), `"active":false`) {
		t.Fatalf("admin categories must include inactive category, got %s", adminCategoriesResponse.Body.String())
	}
}

func TestOfficialPriceLeadSubmitRejectsDeprecatedPublicPricingFields(t *testing.T) {
	server := newTestServer(time.Now())
	buyerSession := createSession(t, server, "buyer", false)

	request := newJSONRequest(http.MethodPost, "/api/v1/official-price-leads", `{
		"productText":"ChatGPT Pro",
		"regionCode":"ph",
		"channel":"web",
		"openingMethod":"official_web",
		"sourceUrl":"https://linux.do/t/example/seat-quantity",
		"sourceTitle":"用户低价线索帖",
		"evidenceSummary":"帖子中展示菲律宾区 Web 价格。",
		"observedAt":"2026-06-21T06:30:00Z",
		"billingPeriod":"monthly",
		"commitmentMonths":12,
		"priceUnit":"per_seat",
		"seatCount":6,
		"quantity":9,
		"currency":"PHP",
		"originalAmount":"799.00",
		"originalPriceText":"PHP 799 / month",
		"taxIncluded":true
	}`)
	addAuth(request, buyerSession, "single-account-contract")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected deprecated pricing fields to be rejected, got %d body %s", response.Code, response.Body.String())
	}
}

func TestApprovedLeadRejectsFurtherReviewActions(t *testing.T) {
	server := newTestServer(time.Now())
	buyerSession := createSession(t, server, "buyer", false)
	adminSession := createSession(t, server, "admin", true)

	lead := submitLead(t, server, buyerSession, "lead-state-create")
	approveBody := `{
		"reason":"来源可访问，价格字段完整。",
		"resolvedProductPlanId":"00000000-0000-0000-0000-000000000303",
		"validFrom":"2026-06-21T00:00:00Z",
		"fxSnapshot":{"rateToCny":"0.12210000","source":"admin_configured_snapshot","observedAt":"2026-06-21T06:00:00Z"}
	}`
	_ = approveLead(t, server, adminSession, lead.ID, approveBody, "state-approve")

	request := newJSONRequest(http.MethodPost, "/api/v1/admin/official-price-leads/"+lead.ID+"/request-changes", `{
		"reason":"重复点击复核动作。"
	}`)
	addAuth(request, adminSession, "state-changes")
	request.Header.Set("If-Match", `"2"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected invalid transition conflict, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "INVALID_STATE_TRANSITION")
}

func TestIdempotencyKeyConflict(t *testing.T) {
	server := newTestServer(time.Now())
	session := createSession(t, server, "buyer", false)

	_ = submitLead(t, server, session, "same-key")

	request := newJSONRequest(http.MethodPost, "/api/v1/official-price-leads", strings.Replace(leadPayload("https://linux.do/t/example/123"), "799.00", "899.00", 1))
	addAuth(request, session, "same-key")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected idempotency conflict, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "IDEMPOTENCY_KEY_REUSED")
}

func TestContactSessionReadAndExpiry(t *testing.T) {
	now := time.Date(2026, 6, 21, 6, 0, 0, 0, time.UTC)
	current := now
	service := app.NewServiceWithClock(func() time.Time { return current })
	server := NewServer(service)
	buyerSession := createSession(t, server, "buyer", false)
	sellerSession := createSession(t, server, "seller", false)

	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "Buyer TG", "@buyer")
	sellerContact := createContactMethod(t, server, sellerSession, "telegram", "Seller TG", "@seller")

	request := newJSONRequest(http.MethodPost, "/api/v1/dev/contact-sessions", `{
		"sellerUsername":"seller",
		"buyerContactMethodId":"`+buyerContact.ID+`",
		"sellerContactMethodId":"`+sellerContact.ID+`",
		"durationSeconds":60
	}`)
	addAuth(request, buyerSession, "contact-session")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create contact session status %d body %s", response.Code, response.Body.String())
	}
	var created contactSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode contact session: %v", err)
	}

	read := httptest.NewRequest(http.MethodGet, "/api/v1/contact-sessions/"+created.ID+"/contacts", nil)
	addCookie(read, buyerSession.cookie)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read contact status %d body %s", readResponse.Code, readResponse.Body.String())
	}
	if got := readResponse.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store, got %q", got)
	}
	if !strings.Contains(readResponse.Body.String(), "@seller") {
		t.Fatalf("expected seller contact in participant response")
	}

	current = now.Add(2 * time.Minute)
	expired := httptest.NewRequest(http.MethodGet, "/api/v1/contact-sessions/"+created.ID+"/contacts", nil)
	addCookie(expired, buyerSession.cookie)
	expiredResponse := httptest.NewRecorder()
	server.ServeHTTP(expiredResponse, expired)
	if expiredResponse.Code != http.StatusConflict {
		t.Fatalf("expected expired conflict, got %d body %s", expiredResponse.Code, expiredResponse.Body.String())
	}
	if strings.Contains(expiredResponse.Body.String(), "@seller") {
		t.Fatalf("expired response must not include contact value")
	}
}

func TestCarpoolCreateReviewApplyAndAcceptFlow(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "seller")
	buyerSession := createSession(t, server, "buyer", false)
	adminSession := createSession(t, server, "admin", true)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Owner Carpool TG", "@owner_carpool")
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "Buyer Carpool TG", "@buyer_carpool")

	withoutAck := newJSONRequest(http.MethodPost, "/api/v1/carpools", carpoolPayload(ownerContact.ID))
	addAuth(withoutAck, ownerSession, "carpool-no-risk")
	withoutAckResponse := httptest.NewRecorder()
	server.ServeHTTP(withoutAckResponse, withoutAck)
	if withoutAckResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected risk ack required, got %d body %s", withoutAckResponse.Code, withoutAckResponse.Body.String())
	}
	assertProblemCode(t, withoutAckResponse, "RISK_ACK_REQUIRED")

	listing := createCarpool(t, server, ownerSession, ownerContact.ID, "carpool-create")
	if listing.Status != app.CarpoolListingStatusDraft || listing.Version != 1 {
		t.Fatalf("unexpected created listing: %+v", listing)
	}
	if listing.ServiceMultiplier != "1.3500" || listing.MonthlyQuotaAmount != "200.00" || listing.QuotaLabel != "额度" || listing.QuotaUnit != "USD" || listing.QuotaPeriod != "monthly" {
		t.Fatalf("expected structured multiplier and quota fields, got %+v", listing)
	}
	if listing.CycleTerm == nil || listing.CycleTerm.BillingPeriod != "monthly" || listing.CycleTerm.ExitPolicy == "" || listing.CycleTerm.UsageRules == "" {
		t.Fatalf("expected listing cycle term in response, got %+v", listing.CycleTerm)
	}

	publicBefore := httptest.NewRequest(http.MethodGet, "/api/v1/carpools/"+listing.ID, nil)
	publicBeforeResponse := httptest.NewRecorder()
	server.ServeHTTP(publicBeforeResponse, publicBefore)
	if publicBeforeResponse.Code != http.StatusNotFound {
		t.Fatalf("draft listing must not be public, got %d body %s", publicBeforeResponse.Code, publicBeforeResponse.Body.String())
	}

	published := submitCarpoolReview(t, server, ownerSession, listing.ID, listing.Version, "carpool-submit-review")
	if published.Status != app.CarpoolListingStatusActive || published.Version != 2 {
		t.Fatalf("unexpected published listing: %+v", published)
	}
	assertPublicCarpoolVisible(t, server, published.ID, true)

	paused := reviewCarpool(t, server, adminSession, published.ID, "pause", published.Version, "carpool-pause")
	if paused.Status != app.CarpoolListingStatusPaused || paused.Version != 3 {
		t.Fatalf("unexpected paused listing: %+v", paused)
	}
	assertPublicCarpoolVisible(t, server, paused.ID, false)
	pausedApplicationRequest := newJSONRequest(http.MethodPost, "/api/v1/carpools/"+paused.ID+"/applications", carpoolApplicationPayload(buyerContact.ID))
	addAuth(pausedApplicationRequest, buyerSession, "carpool-apply-paused")
	pausedApplicationResponse := httptest.NewRecorder()
	server.ServeHTTP(pausedApplicationResponse, pausedApplicationRequest)
	if pausedApplicationResponse.Code != http.StatusNotFound {
		t.Fatalf("paused listing must not accept applications, got %d body %s", pausedApplicationResponse.Code, pausedApplicationResponse.Body.String())
	}

	restored := reviewCarpool(t, server, adminSession, paused.ID, "restore", paused.Version, "carpool-restore")
	if restored.Status != app.CarpoolListingStatusActive || restored.Version != 4 {
		t.Fatalf("unexpected restored listing: %+v", restored)
	}
	assertPublicCarpoolVisible(t, server, restored.ID, true)

	application := createCarpoolApplication(t, server, buyerSession, restored.ID, buyerContact.ID, "carpool-apply")
	if application.Status != app.CarpoolApplicationStatusPendingOwner || application.Version != 1 {
		t.Fatalf("unexpected application: %+v", application)
	}

	duplicate := newJSONRequest(http.MethodPost, "/api/v1/carpools/"+restored.ID+"/applications", carpoolApplicationPayload(buyerContact.ID))
	addAuth(duplicate, buyerSession, "carpool-apply-duplicate")
	duplicateResponse := httptest.NewRecorder()
	server.ServeHTTP(duplicateResponse, duplicate)
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("expected duplicate application conflict, got %d body %s", duplicateResponse.Code, duplicateResponse.Body.String())
	}
	assertProblemCode(t, duplicateResponse, "ACTIVE_APPLICATION_EXISTS")

	accepted := acceptCarpoolApplication(t, server, ownerSession, application.ID, application.Version, "carpool-accept")
	replayed := acceptCarpoolApplication(t, server, ownerSession, application.ID, application.Version, "carpool-accept")
	if accepted.Status != app.CarpoolApplicationStatusAcceptedReserved || accepted.ContactSessionID == "" || accepted.ReservationExpiresAt == nil {
		t.Fatalf("unexpected accepted application: %+v", accepted)
	}
	if accepted.ContactSessionID != replayed.ContactSessionID || accepted.Version != replayed.Version {
		t.Fatalf("expected idempotent accept replay, got %+v and %+v", accepted, replayed)
	}

	readContact := httptest.NewRequest(http.MethodGet, "/api/v1/contact-sessions/"+accepted.ContactSessionID+"/contacts", nil)
	addCookie(readContact, buyerSession.cookie)
	readContactResponse := httptest.NewRecorder()
	server.ServeHTTP(readContactResponse, readContact)
	if readContactResponse.Code != http.StatusOK {
		t.Fatalf("read accepted contact status %d body %s", readContactResponse.Code, readContactResponse.Body.String())
	}
	if got := readContactResponse.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store on accepted contact window, got %q", got)
	}
	if !strings.Contains(readContactResponse.Body.String(), "@owner_carpool") {
		t.Fatalf("expected owner contact in contact window response")
	}

	buyerConfirmed := confirmCarpoolJoin(t, server, buyerSession, "me", accepted.ID, accepted.Version, "carpool-buyer-confirm")
	if buyerConfirmed.Status != app.CarpoolApplicationStatusAcceptedReserved || buyerConfirmed.BuyerConfirmedAt == nil || buyerConfirmed.OwnerConfirmedAt != nil {
		t.Fatalf("unexpected buyer-confirmed application: %+v", buyerConfirmed)
	}
	ownerConfirmed := confirmCarpoolJoin(t, server, ownerSession, "owner", accepted.ID, buyerConfirmed.Version, "carpool-owner-confirm")
	if ownerConfirmed.Status != app.CarpoolApplicationStatusJoined || ownerConfirmed.JoinedAt == nil || ownerConfirmed.ReservationExpiresAt != nil {
		t.Fatalf("unexpected joined application: %+v", ownerConfirmed)
	}

	membership := firstCarpoolMembership(t, server, buyerSession, "me", ownerConfirmed.ID)
	if membership.Status != app.CarpoolMembershipStatusActive {
		t.Fatalf("unexpected active membership: %+v", membership)
	}
	buyerCompleted := confirmCarpoolMembershipComplete(t, server, buyerSession, "me", membership.ID, membership.Version, "carpool-buyer-complete")
	if buyerCompleted.Status != app.CarpoolMembershipStatusActive || buyerCompleted.BuyerCompletedAt == nil || buyerCompleted.OwnerCompletedAt != nil {
		t.Fatalf("unexpected buyer-completed membership: %+v", buyerCompleted)
	}
	ownerCompleted := confirmCarpoolMembershipComplete(t, server, ownerSession, "owner", membership.ID, buyerCompleted.Version, "carpool-owner-complete")
	if ownerCompleted.Status != app.CarpoolMembershipStatusCompleted || ownerCompleted.CompletedAt == nil || ownerCompleted.EndedAt == nil {
		t.Fatalf("unexpected completed membership: %+v", ownerCompleted)
	}
}

func TestCarpoolApplicationCancelAndWithdrawLifecycle(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "cancel-owner")
	pendingBuyer := createSession(t, server, "cancel-buyer-pending", false)
	reservedBuyer := createSession(t, server, "cancel-buyer-reserved", false)
	ownerWithdrawBuyer := createSession(t, server, "cancel-buyer-owner-withdraw", false)
	joinedBuyer := createSession(t, server, "cancel-buyer-joined", false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Cancel Owner TG", "@cancel_owner")
	pendingBuyerContact := createContactMethod(t, server, pendingBuyer, "telegram", "Cancel Buyer Pending TG", "@cancel_pending")
	reservedBuyerContact := createContactMethod(t, server, reservedBuyer, "telegram", "Cancel Buyer Reserved TG", "@cancel_reserved")
	ownerWithdrawBuyerContact := createContactMethod(t, server, ownerWithdrawBuyer, "telegram", "Cancel Buyer Owner Withdraw TG", "@cancel_owner_withdraw")
	joinedBuyerContact := createContactMethod(t, server, joinedBuyer, "telegram", "Cancel Buyer Joined TG", "@cancel_joined")

	listing := createCarpool(t, server, ownerSession, ownerContact.ID, "cancel-create")
	published := submitCarpoolReview(t, server, ownerSession, listing.ID, listing.Version, "cancel-submit")

	pendingApplication := createCarpoolApplication(t, server, pendingBuyer, published.ID, pendingBuyerContact.ID, "cancel-pending-apply")
	cancelledPending := cancelCarpoolApplication(t, server, pendingBuyer, pendingApplication.ID, pendingApplication.Version, "cancel-pending")
	if cancelledPending.Status != app.CarpoolApplicationStatusCancelledByBuyer || cancelledPending.ContactSessionID != "" {
		t.Fatalf("unexpected pending cancellation: %+v", cancelledPending)
	}

	reservedApplication := createCarpoolApplication(t, server, reservedBuyer, published.ID, reservedBuyerContact.ID, "cancel-reserved-apply")
	reservedAccepted := acceptCarpoolApplication(t, server, ownerSession, reservedApplication.ID, reservedApplication.Version, "cancel-reserved-accept")
	cancelledReserved := cancelCarpoolApplication(t, server, reservedBuyer, reservedAccepted.ID, reservedAccepted.Version, "cancel-reserved")
	if cancelledReserved.Status != app.CarpoolApplicationStatusCancelledByBuyer || cancelledReserved.ContactSessionID != reservedAccepted.ContactSessionID {
		t.Fatalf("unexpected reserved cancellation: %+v", cancelledReserved)
	}
	assertContactSessionConflict(t, server, reservedBuyer, reservedAccepted.ContactSessionID)

	ownerWithdrawApplication := createCarpoolApplication(t, server, ownerWithdrawBuyer, published.ID, ownerWithdrawBuyerContact.ID, "cancel-owner-withdraw-apply")
	ownerWithdrawAccepted := acceptCarpoolApplication(t, server, ownerSession, ownerWithdrawApplication.ID, ownerWithdrawApplication.Version, "cancel-owner-withdraw-accept")
	withdrawn := withdrawCarpoolAcceptance(t, server, ownerSession, ownerWithdrawAccepted.ID, ownerWithdrawAccepted.Version, "cancel-owner-withdraw")
	if withdrawn.Status != app.CarpoolApplicationStatusCancelledByOwner || withdrawn.ContactSessionID != ownerWithdrawAccepted.ContactSessionID {
		t.Fatalf("unexpected owner withdrawal: %+v", withdrawn)
	}
	assertContactSessionConflict(t, server, ownerWithdrawBuyer, ownerWithdrawAccepted.ContactSessionID)

	joinedApplication := createCarpoolApplication(t, server, joinedBuyer, published.ID, joinedBuyerContact.ID, "cancel-joined-apply")
	joinedAccepted := acceptCarpoolApplication(t, server, ownerSession, joinedApplication.ID, joinedApplication.Version, "cancel-joined-accept")
	buyerConfirmed := confirmCarpoolJoin(t, server, joinedBuyer, "me", joinedAccepted.ID, joinedAccepted.Version, "cancel-joined-buyer-confirm")
	joined := confirmCarpoolJoin(t, server, ownerSession, "owner", joinedAccepted.ID, buyerConfirmed.Version, "cancel-joined-owner-confirm")
	cancelJoined := newJSONRequest(http.MethodPost, "/api/v1/me/carpool-applications/"+joined.ID+"/cancel", `{"reason":"已加入后应使用退出拼车。"}`)
	addAuth(cancelJoined, joinedBuyer, "cancel-joined-conflict")
	cancelJoined.Header.Set("If-Match", `"`+strconv.FormatInt(joined.Version, 10)+`"`)
	cancelJoinedResponse := httptest.NewRecorder()
	server.ServeHTTP(cancelJoinedResponse, cancelJoined)
	if cancelJoinedResponse.Code != http.StatusConflict {
		t.Fatalf("expected joined cancel conflict, got %d body %s", cancelJoinedResponse.Code, cancelJoinedResponse.Body.String())
	}
	assertProblemCode(t, cancelJoinedResponse, "INVALID_STATE_TRANSITION")
}

func TestCarpoolDirectPublishCreatesActiveListing(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "direct-publish-owner")
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Direct Publish TG", "@direct_publish_owner")

	request := newJSONRequest(http.MethodPost, "/api/v1/carpools/publish", carpoolPayloadWithRiskAck(ownerContact.ID))
	addAuth(request, ownerSession, "carpool-direct-publish")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("direct publish status %d body %s", response.Code, response.Body.String())
	}
	var listing createdCarpool
	if err := json.NewDecoder(response.Body).Decode(&listing); err != nil {
		t.Fatalf("decode direct published carpool: %v", err)
	}
	if listing.Status != app.CarpoolListingStatusActive || listing.Version != 1 {
		t.Fatalf("expected direct publish active listing v1, got %+v", listing)
	}
	assertPublicCarpoolVisible(t, server, listing.ID, true)
}

func TestCarpoolDirectPublishRequiresLinuxDoBindingWithoutDraftResidue(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createSession(t, server, "direct-publish-unbound-owner", false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Unbound Direct Publish TG", "@direct_unbound_owner")

	request := newJSONRequest(http.MethodPost, "/api/v1/carpools/publish", carpoolPayloadWithRiskAck(ownerContact.ID))
	addAuth(request, ownerSession, "carpool-direct-publish-unbound")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unbound direct publish failure, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "VALIDATION_FAILED")

	mine := httptest.NewRequest(http.MethodGet, "/api/v1/me/carpools", nil)
	addCookie(mine, ownerSession.cookie)
	mineResponse := httptest.NewRecorder()
	server.ServeHTTP(mineResponse, mine)
	if mineResponse.Code != http.StatusOK {
		t.Fatalf("list my carpools status %d body %s", mineResponse.Code, mineResponse.Body.String())
	}
	if strings.Contains(mineResponse.Body.String(), "ChatGPT Pro 20x Web 费用分摊") {
		t.Fatalf("failed direct publish must not leave draft residue, got %s", mineResponse.Body.String())
	}
}

func TestCarpoolPublishRequiresLinuxDoBinding(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createSession(t, server, "carpool-unbound-owner", false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Unbound Owner TG", "@unbound_owner")
	listing := createCarpool(t, server, ownerSession, ownerContact.ID, "carpool-unbound-create")

	request := newJSONRequest(http.MethodPost, "/api/v1/carpools/"+listing.ID+"/submit-review", `{}`)
	addAuth(request, ownerSession, "carpool-unbound-publish")
	request.Header.Set("If-Match", `"`+strconv.FormatInt(listing.Version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unbound linux.do publish failure, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "VALIDATION_FAILED")
	assertPublicCarpoolVisible(t, server, listing.ID, false)
}

func TestCarpoolMembershipLeaveAndOwnerRemove(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "member-owner")
	buyerSession := createSession(t, server, "member-buyer", false)
	secondBuyerSession := createSession(t, server, "member-buyer-two", false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Member Owner TG", "@member_owner")
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "Member Buyer TG", "@member_buyer")
	secondBuyerContact := createContactMethod(t, server, secondBuyerSession, "telegram", "Member Buyer Two TG", "@member_buyer_two")

	listing := createCarpool(t, server, ownerSession, ownerContact.ID, "member-create")
	published := submitCarpoolReview(t, server, ownerSession, listing.ID, listing.Version, "member-submit")
	application := createCarpoolApplication(t, server, buyerSession, published.ID, buyerContact.ID, "member-apply")
	accepted := acceptCarpoolApplication(t, server, ownerSession, application.ID, application.Version, "member-accept")
	buyerConfirmed := confirmCarpoolJoin(t, server, buyerSession, "me", accepted.ID, accepted.Version, "member-buyer-confirm")
	joined := confirmCarpoolJoin(t, server, ownerSession, "owner", accepted.ID, buyerConfirmed.Version, "member-owner-confirm")
	membership := firstCarpoolMembership(t, server, buyerSession, "me", joined.ID)
	left := endCarpoolMembership(t, server, buyerSession, "me", "leave", membership.ID, membership.Version, "member-leave")
	if left.Status != app.CarpoolMembershipStatusLeft || left.EndedAt == nil || left.EndedReason == "" || left.EndedByUserID != buyerSession.userID {
		t.Fatalf("unexpected left membership: %+v", left)
	}
	assertContactSessionConflict(t, server, buyerSession, accepted.ContactSessionID)

	secondApplication := createCarpoolApplication(t, server, secondBuyerSession, published.ID, secondBuyerContact.ID, "member-apply-two")
	secondAccepted := acceptCarpoolApplication(t, server, ownerSession, secondApplication.ID, secondApplication.Version, "member-accept-two")
	secondBuyerConfirmed := confirmCarpoolJoin(t, server, secondBuyerSession, "me", secondAccepted.ID, secondAccepted.Version, "member-buyer-confirm-two")
	secondJoined := confirmCarpoolJoin(t, server, ownerSession, "owner", secondAccepted.ID, secondBuyerConfirmed.Version, "member-owner-confirm-two")
	secondMembership := firstCarpoolMembership(t, server, ownerSession, "owner", secondJoined.ID)
	removed := endCarpoolMembership(t, server, ownerSession, "owner", "remove", secondMembership.ID, secondMembership.Version, "member-remove")
	if removed.Status != app.CarpoolMembershipStatusRemoved || removed.EndedAt == nil || removed.EndedReason == "" || removed.EndedByUserID != ownerSession.userID {
		t.Fatalf("unexpected removed membership: %+v", removed)
	}
	assertContactSessionConflict(t, server, secondBuyerSession, secondAccepted.ContactSessionID)
}

func TestAPIServiceCreateReviewPublishFlow(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "api-owner")
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "API Owner TG", "@api_owner")

	bad := newJSONRequest(http.MethodPost, "/api/v1/owner/api-services", apiServicePayload(ownerContact.ID, "1.2000"))
	addAuth(bad, ownerSession, "api-service-bad-multiplier")
	badResponse := httptest.NewRecorder()
	server.ServeHTTP(badResponse, bad)
	if badResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected Sub2API multiplier validation failure, got %d body %s", badResponse.Code, badResponse.Body.String())
	}
	assertProblemCode(t, badResponse, "VALIDATION_FAILED")

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "api-service-create")
	if service.ReviewStatus != app.APIServiceReviewStatusDraft ||
		service.PublicationStatus != app.APIServicePublicationStatusOffline ||
		service.ModerationStatus != app.APIServiceModerationStatusClear ||
		service.Models[0].MerchantMultiplier != "1.0000" {
		t.Fatalf("unexpected created API service: %+v", service)
	}

	publicBefore := httptest.NewRequest(http.MethodGet, "/api/v1/api-services/"+service.ID, nil)
	publicBeforeResponse := httptest.NewRecorder()
	server.ServeHTTP(publicBeforeResponse, publicBefore)
	if publicBeforeResponse.Code != http.StatusNotFound {
		t.Fatalf("draft API service must not be public, got %d body %s", publicBeforeResponse.Code, publicBeforeResponse.Body.String())
	}

	unboundSession := createSession(t, server, "api-unbound-owner", false)
	unboundContact := createContactMethod(t, server, unboundSession, "telegram", "API Unbound Owner TG", "@api_unbound_owner")
	unboundService := createAPIService(t, server, unboundSession, unboundContact.ID, "api-service-unbound-create")
	unboundSubmit := newJSONRequest(http.MethodPost, "/api/v1/owner/api-services/"+unboundService.ID+"/submit-review", `{}`)
	addAuth(unboundSubmit, unboundSession, "api-service-unbound-submit")
	unboundSubmit.Header.Set("If-Match", `"`+strconv.FormatInt(unboundService.Version, 10)+`"`)
	unboundResponse := httptest.NewRecorder()
	server.ServeHTTP(unboundResponse, unboundSubmit)
	if unboundResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected unbound linux.do submit failure, got %d body %s", unboundResponse.Code, unboundResponse.Body.String())
	}
	assertProblemCode(t, unboundResponse, "VALIDATION_FAILED")
	unboundCurrent := getOwnerAPIService(t, server, unboundSession, unboundService.ID)
	if unboundCurrent.ReviewStatus != app.APIServiceReviewStatusDraft || unboundCurrent.Version != unboundService.Version {
		t.Fatalf("unbound submit must keep draft state, got %+v", unboundCurrent)
	}

	submitted := ownerAPIServiceAction(t, server, ownerSession, service.ID, "submit-review", service.Version, "api-service-submit")
	if submitted.ReviewStatus != app.APIServiceReviewStatusApproved ||
		submitted.PublicationStatus != app.APIServicePublicationStatusOffline ||
		submitted.Version != 2 {
		t.Fatalf("unexpected submitted API service: %+v", submitted)
	}
	publicSubmitted := httptest.NewRequest(http.MethodGet, "/api/v1/api-services/"+submitted.ID, nil)
	publicSubmittedResponse := httptest.NewRecorder()
	server.ServeHTTP(publicSubmittedResponse, publicSubmitted)
	if publicSubmittedResponse.Code != http.StatusNotFound {
		t.Fatalf("auto-approved offline API service must not be public, got %d body %s", publicSubmittedResponse.Code, publicSubmittedResponse.Body.String())
	}
	published := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "api-service-publish")
	if published.PublicationStatus != app.APIServicePublicationStatusOnline || published.Version != 3 {
		t.Fatalf("unexpected published API service: %+v", published)
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/api-services", nil)
	listResponse := httptest.NewRecorder()
	server.ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK || strings.Contains(listResponse.Body.String(), published.ID) {
		t.Fatalf("expected published service without order settings hidden from public list, status %d body %s", listResponse.Code, listResponse.Body.String())
	}
	assertPublicAPIServiceBody(t, listResponse.Body.String(), ownerContact.ID)
	detail := httptest.NewRequest(http.MethodGet, "/api/v1/api-services/"+published.ID, nil)
	detailResponse := httptest.NewRecorder()
	server.ServeHTTP(detailResponse, detail)
	if detailResponse.Code != http.StatusNotFound {
		t.Fatalf("expected published service without order settings hidden from public detail, got %d body %s", detailResponse.Code, detailResponse.Body.String())
	}
	createInvisibleFavorite := newJSONRequest(http.MethodPut, "/api/v1/me/favorites/api_service/"+published.ID, `{}`)
	addAuth(createInvisibleFavorite, ownerSession, "api-service-invisible-favorite")
	createInvisibleFavoriteResponse := httptest.NewRecorder()
	server.ServeHTTP(createInvisibleFavoriteResponse, createInvisibleFavorite)
	if createInvisibleFavoriteResponse.Code != http.StatusNotFound {
		t.Fatalf("expected non-orderable API service favorite rejected, got %d body %s", createInvisibleFavoriteResponse.Code, createInvisibleFavoriteResponse.Body.String())
	}
	searchHidden := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+url.QueryEscape("Sub2API 美元额度"), nil)
	searchHiddenResponse := httptest.NewRecorder()
	server.ServeHTTP(searchHiddenResponse, searchHidden)
	if searchHiddenResponse.Code != http.StatusOK || strings.Contains(searchHiddenResponse.Body.String(), published.ID) {
		t.Fatalf("expected non-orderable API service hidden from search, status %d body %s", searchHiddenResponse.Code, searchHiddenResponse.Body.String())
	}
	orderable := updateAPIServiceOrderSettings(t, server, ownerSession, published.ID, published.Version, true, "api-service-order-settings")
	if !orderable.AcceptingOrders || !orderable.IsOrderable || orderable.PaymentWindowMinutes != 10 {
		t.Fatalf("unexpected orderable settings response: %+v", orderable)
	}
	detailAfterSettings := httptest.NewRequest(http.MethodGet, "/api/v1/api-services/"+published.ID, nil)
	detailAfterSettingsResponse := httptest.NewRecorder()
	server.ServeHTTP(detailAfterSettingsResponse, detailAfterSettings)
	if detailAfterSettingsResponse.Code != http.StatusOK {
		t.Fatalf("expected orderable public detail, got %d body %s", detailAfterSettingsResponse.Code, detailAfterSettingsResponse.Body.String())
	}
	assertPublicAPIServiceBody(t, detailAfterSettingsResponse.Body.String(), ownerContact.ID)
	filtered := httptest.NewRequest(http.MethodGet, "/api/v1/api-services?paymentMethod=wechat", nil)
	filteredResponse := httptest.NewRecorder()
	server.ServeHTTP(filteredResponse, filtered)
	if filteredResponse.Code != http.StatusOK || !strings.Contains(filteredResponse.Body.String(), published.ID) {
		t.Fatalf("expected orderable service in payment filtered list, status %d body %s", filteredResponse.Code, filteredResponse.Body.String())
	}
	createFavorite := newJSONRequest(http.MethodPut, "/api/v1/me/favorites/api_service/"+published.ID, `{}`)
	addAuth(createFavorite, ownerSession, "api-service-orderable-favorite")
	createFavoriteResponse := httptest.NewRecorder()
	server.ServeHTTP(createFavoriteResponse, createFavorite)
	if createFavoriteResponse.Code != http.StatusOK || !strings.Contains(createFavoriteResponse.Body.String(), published.ID) {
		t.Fatalf("expected orderable API service favorite success, status %d body %s", createFavoriteResponse.Code, createFavoriteResponse.Body.String())
	}
	searchVisible := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+url.QueryEscape("Sub2API 美元额度"), nil)
	searchVisibleResponse := httptest.NewRecorder()
	server.ServeHTTP(searchVisibleResponse, searchVisible)
	if searchVisibleResponse.Code != http.StatusOK || !strings.Contains(searchVisibleResponse.Body.String(), published.ID) {
		t.Fatalf("expected orderable API service visible in search, status %d body %s", searchVisibleResponse.Code, searchVisibleResponse.Body.String())
	}
}

func TestAPIPurchaseIntentCreateContactAndLifecycleFlow(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "api-intent-owner")
	buyerSession := createSession(t, server, "api-intent-buyer", false)
	adminSession := createSession(t, server, "api-intent-admin", true)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "API Intent Owner TG", "@api_intent_owner")
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "API Intent Buyer TG", "@api_intent_buyer")

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "api-intent-service-create")
	submitted := ownerAPIServiceAction(t, server, ownerSession, service.ID, "submit-review", service.Version, "api-intent-service-submit")
	published := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "api-intent-service-publish")
	orderable := updateAPIServiceOrderSettings(t, server, ownerSession, published.ID, published.Version, true, "api-intent-service-settings")

	intent := createAPIPurchaseIntent(t, server, buyerSession, orderable.ID, buyerContact.ID, "api-intent-create")
	replayed := createAPIPurchaseIntent(t, server, buyerSession, orderable.ID, buyerContact.ID, "api-intent-create")
	if intent.ID == "" || intent.ID != replayed.ID {
		t.Fatalf("expected idempotent API purchase intent replay, got %+v and %+v", intent, replayed)
	}
	if intent.Status != app.APIPurchaseIntentStatusOpen ||
		intent.RequestedCNYAmount != "16.00" ||
		intent.RequestedUSDAllowance != "20.000000" ||
		intent.DeclaredCNYPerUSDAllowanceSnapshot != "0.8000" ||
		intent.DeclaredMaxUSDAllowancePerIntentSnapshot != "20.000000" ||
		intent.PricingSnapshot == "" ||
		intent.Version != 1 {
		t.Fatalf("unexpected API purchase intent: %+v", intent)
	}
	if intent.MerchantContact == nil || intent.MerchantContact.Value != "@api_intent_owner" || intent.MerchantContact.MaskedValue == "" {
		t.Fatalf("expected merchant contact in create response: %+v", intent.MerchantContact)
	}
	if intent.SelectedAccessMode != "buyer_dedicated_sub_key" || intent.OwnerUserID != "" || intent.OwnerContactMethodID != "" {
		t.Fatalf("create response leaked owner identity or missed selected access mode: %+v", intent)
	}
	if intent.BuyerContact != nil {
		t.Fatalf("create response must not include buyer contact: %+v", intent.BuyerContact)
	}

	myDetail := getAPIPurchaseIntent(t, server, buyerSession, "me", intent.ID)
	if myDetail.ID != intent.ID || myDetail.OwnerUserID != "" || myDetail.OwnerContactMethodID != "" || myDetail.MerchantContact == nil || myDetail.MerchantContact.Value != "@api_intent_owner" {
		t.Fatalf("unexpected buyer API intent detail: %+v", myDetail)
	}
	ownerDetail := getAPIPurchaseIntent(t, server, ownerSession, "owner", intent.ID)
	if ownerDetail.ID != intent.ID || ownerDetail.BuyerContactMethodID != buyerContact.ID || ownerDetail.OwnerContactMethodID != "" || ownerDetail.BuyerContact == nil || ownerDetail.BuyerContact.Value != "@api_intent_buyer" {
		t.Fatalf("unexpected owner API intent detail: %+v", ownerDetail)
	}
	adminDetail := getAPIPurchaseIntent(t, server, adminSession, "admin", intent.ID)
	if adminDetail.ID != intent.ID || adminDetail.MerchantContact != nil || adminDetail.BuyerContact != nil {
		t.Fatalf("unexpected admin API intent detail: %+v", adminDetail)
	}

	contacted := ownerAPIPurchaseIntentAction(t, server, ownerSession, intent.ID, "mark-contacted", intent.Version, "api-intent-contacted", `{}`)
	if contacted.Status != app.APIPurchaseIntentStatusContacted || contacted.ContactedAt == nil || contacted.Version != 2 {
		t.Fatalf("unexpected contacted API intent: %+v", contacted)
	}
	cancelled := cancelAPIPurchaseIntent(t, server, buyerSession, contacted.ID, contacted.Version, "api-intent-cancel")
	if cancelled.Status != app.APIPurchaseIntentStatusBuyerCancelled || cancelled.BuyerCancelledAt == nil || cancelled.BuyerCancelReason == "" || cancelled.Version != 3 {
		t.Fatalf("unexpected cancelled API intent: %+v", cancelled)
	}
}

func TestAPIServiceInstantOrderFlow(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "api-order-owner")
	buyerSession := createSession(t, server, "api-order-buyer", false)
	adminSession := createSession(t, server, "api-order-admin", true)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "API Order Owner TG", "@api_order_owner")
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "API Order Buyer TG", "@api_order_buyer")

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "api-order-service-create")
	submitted := ownerAPIServiceAction(t, server, ownerSession, service.ID, "submit-review", service.Version, "api-order-service-submit")
	published := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "api-order-service-publish")

	unconfiguredIntent := newJSONRequest(http.MethodPost, "/api/v1/api-services/"+published.ID+"/purchase-intents", apiPurchaseIntentPayload(buyerContact.ID))
	addAuth(unconfiguredIntent, buyerSession, "api-order-unconfigured-intent")
	unconfiguredResponse := httptest.NewRecorder()
	server.ServeHTTP(unconfiguredResponse, unconfiguredIntent)
	if unconfiguredResponse.Code != http.StatusNotFound {
		t.Fatalf("expected non-orderable service intent hidden, got %d body %s", unconfiguredResponse.Code, unconfiguredResponse.Body.String())
	}
	assertProblemCode(t, unconfiguredResponse, "OBJECT_NOT_FOUND")

	orderable := updateAPIServiceOrderSettings(t, server, ownerSession, published.ID, published.Version, true, "api-order-service-settings")
	if !orderable.IsOrderable || !orderable.AcceptingOrders {
		t.Fatalf("expected orderable service after settings update: %+v", orderable)
	}
	intent := createAPIPurchaseIntent(t, server, buyerSession, orderable.ID, buyerContact.ID, "api-order-intent")

	order := createAPIOrder(t, server, buyerSession, intent.ID, "wechat", "api-order-create")
	replayed := createAPIOrder(t, server, buyerSession, intent.ID, "wechat", "api-order-create")
	if order.ID == "" || order.ID != replayed.ID || order.Version != replayed.Version {
		t.Fatalf("expected idempotent order replay, got %+v and %+v", order, replayed)
	}
	if order.Status != "pending_payment" ||
		order.DisputeStatus != "none" ||
		order.APIPurchaseIntentID != intent.ID ||
		order.APIServiceID != orderable.ID ||
		order.SellerUserID != ownerSession.userID ||
		order.BuyerUserID != "" ||
		order.Amount != "16.00" ||
		order.Currency != "CNY" ||
		order.SelectedPaymentMethod != "wechat" ||
		order.PaymentWindowMinutesSnapshot != 10 ||
		order.PaymentSummary != "" ||
		order.DeliveryNote != "" ||
		order.Version != 1 {
		t.Fatalf("unexpected created API order: %+v", order)
	}

	cancelOrderedIntent := newJSONRequest(http.MethodPost, "/api/v1/me/api-purchase-intents/"+intent.ID+"/cancel", `{"reason":"已进入订单流程后不能再取消普通意向。"}`)
	addAuth(cancelOrderedIntent, buyerSession, "api-order-intent-cancel-after-order")
	cancelOrderedIntent.Header.Set("If-Match", `"`+strconv.FormatInt(intent.Version, 10)+`"`)
	cancelOrderedIntentResponse := httptest.NewRecorder()
	server.ServeHTTP(cancelOrderedIntentResponse, cancelOrderedIntent)
	if cancelOrderedIntentResponse.Code != http.StatusConflict {
		t.Fatalf("expected ordered intent cancel conflict, got %d body %s", cancelOrderedIntentResponse.Code, cancelOrderedIntentResponse.Body.String())
	}
	assertProblemCode(t, cancelOrderedIntentResponse, "API_PURCHASE_INTENT_HAS_ORDER")

	closeOrderedIntent := newJSONRequest(http.MethodPost, "/api/v1/owner/api-purchase-intents/"+intent.ID+"/close", `{"reason":"已进入订单流程后不能再关闭普通意向。"}`)
	addAuth(closeOrderedIntent, ownerSession, "api-order-intent-close-after-order")
	closeOrderedIntent.Header.Set("If-Match", `"`+strconv.FormatInt(intent.Version, 10)+`"`)
	closeOrderedIntentResponse := httptest.NewRecorder()
	server.ServeHTTP(closeOrderedIntentResponse, closeOrderedIntent)
	if closeOrderedIntentResponse.Code != http.StatusConflict {
		t.Fatalf("expected ordered intent close conflict, got %d body %s", closeOrderedIntentResponse.Code, closeOrderedIntentResponse.Body.String())
	}
	assertProblemCode(t, closeOrderedIntentResponse, "API_PURCHASE_INTENT_HAS_ORDER")

	instructions := readAPIOrderPaymentInstructions(t, server, buyerSession, order.ID)
	if instructions.OrderID != order.ID ||
		instructions.PaymentMethod != "wechat" ||
		instructions.PaymentInstructions == "" ||
		!strings.Contains(instructions.PaymentInstructions, "微信收款二维码") {
		t.Fatalf("unexpected payment instructions: %+v", instructions)
	}

	paid := apiOrderAction(t, server, buyerSession, "me", order.ID, "submit-payment", order.Version, "api-order-submit-payment", `{"paymentSummary":"已按站外确认金额完成微信付款，尾号 1234。"}`)
	if paid.Status != "payment_submitted" || paid.PaymentSummary == "" || paid.Version != 2 {
		t.Fatalf("unexpected payment-submitted order: %+v", paid)
	}

	disputeIntent := createAPIPurchaseIntent(t, server, buyerSession, orderable.ID, buyerContact.ID, "api-order-dispute-intent")
	disputeOrder := createAPIOrder(t, server, buyerSession, disputeIntent.ID, "wechat", "api-order-dispute-create")
	disputePaid := apiOrderAction(t, server, buyerSession, "me", disputeOrder.ID, "submit-payment", disputeOrder.Version, "api-order-dispute-submit-payment", `{"paymentSummary":"已付款但站外确认存在争议。"}`)
	disputed := apiOrderAction(t, server, buyerSession, "me", disputePaid.ID, "dispute", disputePaid.Version, "api-order-open-dispute", `{"reason":"付款后商户未按站外确认说明继续处理。"}`)
	if disputed.Status != "payment_submitted" || disputed.DisputeStatus != "open" || disputed.DisputeCaseID == "" || disputed.Version != 3 {
		t.Fatalf("unexpected disputed API order: %+v", disputed)
	}
	adminDisputes := listAdminDisputes(t, server, adminSession)
	foundDispute := false
	for _, item := range adminDisputes.Items {
		if item.ID != disputed.DisputeCaseID {
			continue
		}
		foundDispute = true
		if item.TargetType != "api_order" || item.TargetID != disputed.ID || item.Status != "open" {
			t.Fatalf("unexpected admin API order dispute: %+v", item)
		}
		if item.PublicSummary == "" || item.PublicResult == "" {
			t.Fatalf("expected public-safe dispute summary/result: %+v", item)
		}
	}
	if !foundDispute {
		t.Fatalf("expected admin disputes to include API order dispute %s: %+v", disputed.DisputeCaseID, adminDisputes.Items)
	}

	completeEarly := newJSONRequest(http.MethodPost, "/api/v1/me/api-orders/"+paid.ID+"/confirm-complete", `{}`)
	addAuth(completeEarly, buyerSession, "api-order-complete-early")
	completeEarly.Header.Set("If-Match", `"`+strconv.FormatInt(paid.Version, 10)+`"`)
	completeEarlyResponse := httptest.NewRecorder()
	server.ServeHTTP(completeEarlyResponse, completeEarly)
	if completeEarlyResponse.Code != http.StatusConflict {
		t.Fatalf("expected early complete conflict, got %d body %s", completeEarlyResponse.Code, completeEarlyResponse.Body.String())
	}
	assertProblemCode(t, completeEarlyResponse, "INVALID_STATE_TRANSITION")

	confirmed := apiOrderAction(t, server, ownerSession, "owner", paid.ID, "confirm-payment", paid.Version, "api-order-confirm-payment", `{}`)
	if confirmed.Status != "paid_confirmed" || confirmed.PaidConfirmedAt == nil || confirmed.BuyerUserID != buyerSession.userID || confirmed.Version != 3 {
		t.Fatalf("unexpected paid-confirmed order: %+v", confirmed)
	}

	rejectedDeliveryNotes := []string{
		"Authorization: Bearer sk-test",
		"X-API-Key: abcdef",
		"apiKey: sk-test",
		"OPENAI_API_KEY=sk-proj-test",
		"ANTHROPIC_API_KEY=sk-ant-test",
		"vless://example",
		"clash://install-config",
		"hysteria://example",
		"hy2://example",
		"tuic://example",
		"sub://example",
		"ssr://example",
		"socks5://user:pass@example.com:1080",
		"https://example.com/api/v1/client/subscribe?token=abc",
		"https://example.com/sub?target=clash&url=xxx",
		"https://example.com/sub?url=https%3A%2F%2Fvendor.example%2Fapi%2Fv1%2Fclient%2Fsubscribe%3Ftoken%3Dabc123",
		"[订阅链接](https://example.com/sub?target=clash&url=xxx)",
		`{"delivery":"https://example.com/api/v1/client/subscribe?token=abc"}`,
		"abc.def.ghi",
	}
	for index, note := range rejectedDeliveryNotes {
		secretDelivery := newJSONRequest(http.MethodPost, "/api/v1/owner/api-orders/"+confirmed.ID+"/submit-delivery", `{"deliveryNote":`+strconv.Quote(note)+`}`)
		addAuth(secretDelivery, ownerSession, "api-order-secret-delivery-"+strconv.Itoa(index))
		secretDelivery.Header.Set("If-Match", `"`+strconv.FormatInt(confirmed.Version, 10)+`"`)
		secretDeliveryResponse := httptest.NewRecorder()
		server.ServeHTTP(secretDeliveryResponse, secretDelivery)
		if secretDeliveryResponse.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected secret delivery validation failure for %q, got %d body %s", note, secretDeliveryResponse.Code, secretDeliveryResponse.Body.String())
		}
		assertProblemCode(t, secretDeliveryResponse, "SECRET_CONTENT_DETECTED")
	}

	delivered := apiOrderAction(t, server, ownerSession, "owner", confirmed.ID, "submit-delivery", confirmed.Version, "api-order-submit-delivery", `{"deliveryNote":"已站外确认接入安排，买家可按商户说明完成后续操作。"}`)
	if delivered.Status != "delivery_submitted" || delivered.DeliveryNote == "" || delivered.Version != 4 {
		t.Fatalf("unexpected delivered order: %+v", delivered)
	}
	completed := apiOrderAction(t, server, buyerSession, "me", delivered.ID, "confirm-complete", delivered.Version, "api-order-confirm-complete", `{}`)
	if completed.Status != "completed" || completed.CompletedAt == nil || completed.Version != 5 {
		t.Fatalf("unexpected completed order: %+v", completed)
	}

	duplicateAfterCompleted := newJSONRequest(http.MethodPost, "/api/v1/me/api-purchase-intents/"+intent.ID+"/orders", `{"paymentMethod":"wechat"}`)
	addAuth(duplicateAfterCompleted, buyerSession, "api-order-duplicate-after-completed")
	duplicateResponse := httptest.NewRecorder()
	server.ServeHTTP(duplicateResponse, duplicateAfterCompleted)
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("expected duplicate order conflict, got %d body %s", duplicateResponse.Code, duplicateResponse.Body.String())
	}
	assertProblemCode(t, duplicateResponse, "API_PURCHASE_INTENT_HAS_ORDER")
}

func TestConcurrentAPIOrderCreateForSameIntentReturnsStableConflict(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "api-order-race-owner")
	buyerSession := createSession(t, server, "api-order-race-buyer", false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "API Race Owner TG", "@api_order_race_owner")
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "API Race Buyer TG", "@api_order_race_buyer")

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "api-order-race-service-create")
	submitted := ownerAPIServiceAction(t, server, ownerSession, service.ID, "submit-review", service.Version, "api-order-race-service-submit")
	published := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "api-order-race-service-publish")
	orderable := updateAPIServiceOrderSettings(t, server, ownerSession, published.ID, published.Version, true, "api-order-race-service-settings")
	intent := createAPIPurchaseIntent(t, server, buyerSession, orderable.ID, buyerContact.ID, "api-order-race-intent")

	const attempts = 6
	var wg sync.WaitGroup
	responses := make([]*httptest.ResponseRecorder, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			request := newJSONRequest(http.MethodPost, "/api/v1/me/api-purchase-intents/"+intent.ID+"/orders", `{"paymentMethod":"wechat"}`)
			addAuth(request, buyerSession, "api-order-race-create-"+strconv.Itoa(index))
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			responses[index] = response
		}(i)
	}
	wg.Wait()

	successes := 0
	conflicts := 0
	for index, response := range responses {
		if response == nil {
			t.Fatalf("missing response for attempt %d", index)
		}
		switch response.Code {
		case http.StatusCreated:
			successes++
		case http.StatusConflict:
			conflicts++
			assertProblemCode(t, response, "API_PURCHASE_INTENT_HAS_ORDER")
		default:
			t.Fatalf("expected created or conflict for attempt %d, got %d body %s", index, response.Code, response.Body.String())
		}
	}
	if successes != 1 || conflicts != attempts-1 {
		t.Fatalf("expected one success and %d conflicts, got successes=%d conflicts=%d", attempts-1, successes, conflicts)
	}

	listOrders := httptest.NewRequest(http.MethodGet, "/api/v1/me/api-orders", nil)
	addCookie(listOrders, buyerSession.cookie)
	listOrdersResponse := httptest.NewRecorder()
	server.ServeHTTP(listOrdersResponse, listOrders)
	if listOrdersResponse.Code != http.StatusOK {
		t.Fatalf("list API orders status %d body %s", listOrdersResponse.Code, listOrdersResponse.Body.String())
	}
	var payload struct {
		Items []createdAPIOrder `json:"items"`
	}
	if err := json.NewDecoder(listOrdersResponse.Body).Decode(&payload); err != nil {
		t.Fatalf("decode API orders: %v", err)
	}
	intentOrders := 0
	for _, order := range payload.Items {
		if order.APIPurchaseIntentID == intent.ID {
			intentOrders++
		}
	}
	if intentOrders != 1 {
		t.Fatalf("expected exactly one order for intent %s, got %d in %+v", intent.ID, intentOrders, payload.Items)
	}
}

func TestCarpoolAcceptRejectsWhenNoSeatAvailable(t *testing.T) {
	server := newTestServer(time.Now())
	ownerSession := createLinuxDoSession(t, server, "seat-owner")
	firstBuyer := createSession(t, server, "seat-buyer-one", false)
	secondBuyer := createSession(t, server, "seat-buyer-two", false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Seat Owner TG", "@seat_owner")
	firstBuyerContact := createContactMethod(t, server, firstBuyer, "telegram", "Seat Buyer One TG", "@seat_buyer_one")
	secondBuyerContact := createContactMethod(t, server, secondBuyer, "telegram", "Seat Buyer Two TG", "@seat_buyer_two")

	listing := createCarpool(t, server, ownerSession, ownerContact.ID, "seat-carpool-create")
	published := submitCarpoolReview(t, server, ownerSession, listing.ID, listing.Version, "seat-carpool-submit-review")
	firstApplication := createCarpoolApplication(t, server, firstBuyer, published.ID, firstBuyerContact.ID, "seat-apply-one")
	secondApplication := createCarpoolApplication(t, server, secondBuyer, published.ID, secondBuyerContact.ID, "seat-apply-two")

	_ = acceptCarpoolApplication(t, server, ownerSession, firstApplication.ID, firstApplication.Version, "seat-accept-one")

	request := newJSONRequest(http.MethodPost, "/api/v1/owner/carpool-applications/"+secondApplication.ID+"/accept", `{}`)
	addAuth(request, ownerSession, "seat-accept-two")
	request.Header.Set("If-Match", `"`+strconv.FormatInt(secondApplication.Version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected no-seat accept conflict, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "SEAT_UNAVAILABLE")
}

func TestExpiredCarpoolReservationReleasesSeatAndBuyerApplicationSlot(t *testing.T) {
	current := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	server := NewServer(app.NewServiceWithClock(func() time.Time { return current }))
	ownerSession := createLinuxDoSession(t, server, "expiry-owner")
	firstBuyer := createSession(t, server, "expiry-buyer-one", false)
	secondBuyer := createSession(t, server, "expiry-buyer-two", false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "Expiry Owner TG", "@expiry_owner")
	firstBuyerContact := createContactMethod(t, server, firstBuyer, "telegram", "Expiry Buyer One TG", "@expiry_buyer_one")
	secondBuyerContact := createContactMethod(t, server, secondBuyer, "telegram", "Expiry Buyer Two TG", "@expiry_buyer_two")

	listing := createCarpool(t, server, ownerSession, ownerContact.ID, "expiry-carpool-create")
	published := submitCarpoolReview(t, server, ownerSession, listing.ID, listing.Version, "expiry-carpool-submit-review")
	firstApplication := createCarpoolApplication(t, server, firstBuyer, published.ID, firstBuyerContact.ID, "expiry-apply-one")
	_ = acceptCarpoolApplication(t, server, ownerSession, firstApplication.ID, firstApplication.Version, "expiry-accept-one")

	current = current.Add(31 * time.Minute)
	secondApplication := createCarpoolApplication(t, server, secondBuyer, published.ID, secondBuyerContact.ID, "expiry-apply-two")
	if secondApplication.Status != app.CarpoolApplicationStatusPendingOwner {
		t.Fatalf("expected second buyer to apply after reservation expiry, got %+v", secondApplication)
	}
	reapplied := createCarpoolApplication(t, server, firstBuyer, published.ID, firstBuyerContact.ID, "expiry-reapply-one")
	if reapplied.Status != app.CarpoolApplicationStatusPendingOwner {
		t.Fatalf("expected original buyer to reapply after reservation expiry, got %+v", reapplied)
	}
}

func TestFeedbackLoopUnreadReadSupplementAndIsolation(t *testing.T) {
	server := newTestServer(time.Now())
	submitter := createSession(t, server, "feedback-user", false)
	otherUser := createSession(t, server, "feedback-other", false)
	admin := createSession(t, server, "feedback-admin", true)

	create := newJSONRequest(http.MethodPost, "/api/v1/me/feedback-tickets", `{
		"type":"function_issue",
		"impact":"blocks_operation",
		"title":"发布表单无法继续",
		"description":"我在发布 API 服务时，选择接入方式后保存没有反应。",
		"contextPageLabel":"发布 API 服务",
		"contextTargetType":"api_service_publish",
		"contextTargetId":"",
		"contextTargetLabel":"新建服务表单",
		"contextRoleLabel":"商户"
	}`)
	addAuth(create, submitter, "feedback-create")
	createResponse := httptest.NewRecorder()
	server.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create feedback status %d body %s", createResponse.Code, createResponse.Body.String())
	}
	var created feedbackTicketResponse
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode feedback create: %v", err)
	}
	if created.Status != "submitted" || created.Unread {
		t.Fatalf("unexpected created feedback: %+v", created)
	}
	if created.AdminInternalNote != "" {
		t.Fatalf("submitter response leaked internal note: %+v", created)
	}

	adminRead := httptest.NewRequest(http.MethodGet, "/api/v1/admin/feedback-tickets/"+created.ID, nil)
	addCookie(adminRead, admin.cookie)
	adminReadResponse := httptest.NewRecorder()
	server.ServeHTTP(adminReadResponse, adminRead)
	if adminReadResponse.Code != http.StatusOK {
		t.Fatalf("admin read feedback status %d body %s", adminReadResponse.Code, adminReadResponse.Body.String())
	}

	handle := newJSONRequest(http.MethodPost, "/api/v1/admin/feedback-tickets/"+created.ID+"/handle", `{
		"status":"needs_user_info",
		"response":"已定位到发布流程，需要你补充当时选择的接入方式名称。",
		"internalNote":"疑似发布表单状态同步问题。"
	}`)
	addAuth(handle, admin, "feedback-handle")
	handle.Header.Set("If-Match", `"`+strconv.FormatInt(created.Version, 10)+`"`)
	handleResponse := httptest.NewRecorder()
	server.ServeHTTP(handleResponse, handle)
	if handleResponse.Code != http.StatusOK {
		t.Fatalf("handle feedback status %d body %s", handleResponse.Code, handleResponse.Body.String())
	}
	var handled feedbackTicketResponse
	if err := json.NewDecoder(handleResponse.Body).Decode(&handled); err != nil {
		t.Fatalf("decode handled feedback: %v", err)
	}
	if handled.Status != "needs_user_info" || !handled.Unread || handled.AdminInternalNote == "" {
		t.Fatalf("unexpected handled feedback: %+v", handled)
	}

	unread := httptest.NewRequest(http.MethodGet, "/api/v1/me/feedback-tickets/unread-count", nil)
	addCookie(unread, submitter.cookie)
	unreadResponse := httptest.NewRecorder()
	server.ServeHTTP(unreadResponse, unread)
	if unreadResponse.Code != http.StatusOK || !strings.Contains(unreadResponse.Body.String(), `"count":1`) {
		t.Fatalf("expected one unread feedback result, got %d body %s", unreadResponse.Code, unreadResponse.Body.String())
	}

	otherRead := httptest.NewRequest(http.MethodGet, "/api/v1/me/feedback-tickets/"+created.ID, nil)
	addCookie(otherRead, otherUser.cookie)
	otherReadResponse := httptest.NewRecorder()
	server.ServeHTTP(otherReadResponse, otherRead)
	if otherReadResponse.Code != http.StatusNotFound {
		t.Fatalf("expected feedback isolation not found, got %d body %s", otherReadResponse.Code, otherReadResponse.Body.String())
	}

	submitterRead := httptest.NewRequest(http.MethodGet, "/api/v1/me/feedback-tickets/"+created.ID, nil)
	addCookie(submitterRead, submitter.cookie)
	submitterReadResponse := httptest.NewRecorder()
	server.ServeHTTP(submitterReadResponse, submitterRead)
	if submitterReadResponse.Code != http.StatusOK {
		t.Fatalf("submitter read feedback status %d body %s", submitterReadResponse.Code, submitterReadResponse.Body.String())
	}
	var submitterDetail feedbackTicketResponse
	if err := json.NewDecoder(submitterReadResponse.Body).Decode(&submitterDetail); err != nil {
		t.Fatalf("decode submitter feedback: %v", err)
	}
	if submitterDetail.AdminInternalNote != "" {
		t.Fatalf("submitter detail leaked internal note: %+v", submitterDetail)
	}

	read := newJSONRequest(http.MethodPost, "/api/v1/me/feedback-tickets/"+created.ID+"/read", `{}`)
	addAuth(read, submitter, "feedback-read")
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("mark feedback read status %d body %s", readResponse.Code, readResponse.Body.String())
	}
	unreadAgain := httptest.NewRequest(http.MethodGet, "/api/v1/me/feedback-tickets/unread-count", nil)
	addCookie(unreadAgain, submitter.cookie)
	unreadAgainResponse := httptest.NewRecorder()
	server.ServeHTTP(unreadAgainResponse, unreadAgain)
	if unreadAgainResponse.Code != http.StatusOK || !strings.Contains(unreadAgainResponse.Body.String(), `"count":0`) {
		t.Fatalf("expected zero unread feedback results, got %d body %s", unreadAgainResponse.Code, unreadAgainResponse.Body.String())
	}

	supplement := newJSONRequest(http.MethodPost, "/api/v1/me/feedback-tickets/"+created.ID+"/supplements", `{
		"message":"当时选择的是 API 请求地址接入说明，浏览器没有错误提示。"
	}`)
	addAuth(supplement, submitter, "feedback-supplement")
	supplementResponse := httptest.NewRecorder()
	server.ServeHTTP(supplementResponse, supplement)
	if supplementResponse.Code != http.StatusOK {
		t.Fatalf("supplement feedback status %d body %s", supplementResponse.Code, supplementResponse.Body.String())
	}
	var supplemented feedbackTicketResponse
	if err := json.NewDecoder(supplementResponse.Body).Decode(&supplemented); err != nil {
		t.Fatalf("decode supplemented feedback: %v", err)
	}
	if supplemented.Status != "submitted" || len(supplemented.Events) == 0 {
		t.Fatalf("unexpected supplemented feedback: %+v", supplemented)
	}
}

func TestProfileContactAndMerchantProfileFlow(t *testing.T) {
	server := newTestServer(time.Now())
	session := createLinuxDoSession(t, server, "profile-owner")

	updateProfile := newJSONRequest(http.MethodPatch, "/api/v1/me/profile", `{
		"displayName":"Profile Owner",
		"username":"profile-owner",
		"bio":"只公开必要业务资料。",
		"regionCode":"cn",
		"timezone":"Asia/Shanghai",
		"avatarMode":"linuxdo",
		"privacy":{
			"showCreatedAt":true,
			"showLastActiveAt":false,
			"showCompletedCarpoolCount":true,
			"showCompletedApiIntentCount":true,
			"showResponseMedian":true,
			"showResolvedDisputeSummary":true,
			"allowPublicProfileReport":true
		}
	}`)
	addAuth(updateProfile, session, "")
	updateProfileResponse := httptest.NewRecorder()
	server.ServeHTTP(updateProfileResponse, updateProfile)
	if updateProfileResponse.Code != http.StatusOK {
		t.Fatalf("update profile status %d body %s", updateProfileResponse.Code, updateProfileResponse.Body.String())
	}
	if !strings.Contains(updateProfileResponse.Body.String(), `"displayName":"Profile Owner"`) {
		t.Fatalf("expected updated display name, got %s", updateProfileResponse.Body.String())
	}

	first := createContactMethod(t, server, session, "telegram", "Profile TG", "@profile_owner")
	second := createContactMethod(t, server, session, "email", "Profile Email", "profile@example.com")

	listContacts := httptest.NewRequest(http.MethodGet, "/api/v1/me/contact-methods", nil)
	addCookie(listContacts, session.cookie)
	listContactsResponse := httptest.NewRecorder()
	server.ServeHTTP(listContactsResponse, listContacts)
	if listContactsResponse.Code != http.StatusOK {
		t.Fatalf("list contacts status %d body %s", listContactsResponse.Code, listContactsResponse.Body.String())
	}
	if !strings.Contains(listContactsResponse.Body.String(), first.ID) || !strings.Contains(listContactsResponse.Body.String(), second.ID) {
		t.Fatalf("expected both contacts in list, got %s", listContactsResponse.Body.String())
	}

	updateContact := newJSONRequest(http.MethodPatch, "/api/v1/contact-methods/"+second.ID, `{
		"type":"email",
		"label":"Profile Email Updated",
		"displayValue":"updated-profile@example.com",
		"usageScopes":["api_merchant"],
		"isDefault":true,
		"enabled":true
	}`)
	addAuth(updateContact, session, "")
	updateContactResponse := httptest.NewRecorder()
	server.ServeHTTP(updateContactResponse, updateContact)
	if updateContactResponse.Code != http.StatusOK {
		t.Fatalf("update contact status %d body %s", updateContactResponse.Code, updateContactResponse.Body.String())
	}
	if !strings.Contains(updateContactResponse.Body.String(), `"isDefault":true`) {
		t.Fatalf("expected updated contact default, got %s", updateContactResponse.Body.String())
	}

	verifyContact := newJSONRequest(http.MethodPost, "/api/v1/contact-methods/"+second.ID+"/verify", `{}`)
	addAuth(verifyContact, session, "")
	verifyContactResponse := httptest.NewRecorder()
	server.ServeHTTP(verifyContactResponse, verifyContact)
	if verifyContactResponse.Code != http.StatusOK || !strings.Contains(verifyContactResponse.Body.String(), `"verified":true`) {
		t.Fatalf("verify contact status %d body %s", verifyContactResponse.Code, verifyContactResponse.Body.String())
	}

	deleteContact := newJSONRequest(http.MethodDelete, "/api/v1/contact-methods/"+first.ID, `{}`)
	addAuth(deleteContact, session, "")
	deleteContactResponse := httptest.NewRecorder()
	server.ServeHTTP(deleteContactResponse, deleteContact)
	if deleteContactResponse.Code != http.StatusOK || !strings.Contains(deleteContactResponse.Body.String(), `"enabled":false`) {
		t.Fatalf("delete contact status %d body %s", deleteContactResponse.Code, deleteContactResponse.Body.String())
	}

	upsertMerchant := newJSONRequest(http.MethodPost, "/api/v1/me/merchant-profile", `{
		"slug":"profile-store",
		"displayName":"Profile Store",
		"avatarUrl":""
	}`)
	addAuth(upsertMerchant, session, "")
	upsertMerchantResponse := httptest.NewRecorder()
	server.ServeHTTP(upsertMerchantResponse, upsertMerchant)
	if upsertMerchantResponse.Code != http.StatusOK {
		t.Fatalf("upsert merchant status %d body %s", upsertMerchantResponse.Code, upsertMerchantResponse.Body.String())
	}
	var merchant createdMerchantProfile
	if err := json.NewDecoder(upsertMerchantResponse.Body).Decode(&merchant); err != nil {
		t.Fatalf("decode merchant profile: %v", err)
	}
	if merchant.ID == "" || merchant.OwnerUserID != session.userID {
		t.Fatalf("expected self merchant profile with owner id, got %+v", merchant)
	}

	publicUser := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile-owner/public-profile", nil)
	publicUserResponse := httptest.NewRecorder()
	server.ServeHTTP(publicUserResponse, publicUser)
	if publicUserResponse.Code != http.StatusOK {
		t.Fatalf("public user status %d body %s", publicUserResponse.Code, publicUserResponse.Body.String())
	}
	publicUserBody := publicUserResponse.Body.String()
	if strings.Contains(publicUserBody, "updated-profile@example.com") || strings.Contains(publicUserBody, second.ID) {
		t.Fatalf("public user profile leaked contact data: %s", publicUserBody)
	}
	if !strings.Contains(publicUserBody, `"lastActiveAt":null`) {
		t.Fatalf("expected privacy setting to hide lastActiveAt, got %s", publicUserBody)
	}

	publicMerchant := httptest.NewRequest(http.MethodGet, "/api/v1/merchant-profiles/profile-store", nil)
	publicMerchantResponse := httptest.NewRecorder()
	server.ServeHTTP(publicMerchantResponse, publicMerchant)
	if publicMerchantResponse.Code != http.StatusOK {
		t.Fatalf("public merchant status %d body %s", publicMerchantResponse.Code, publicMerchantResponse.Body.String())
	}
	publicMerchantBody := publicMerchantResponse.Body.String()
	if strings.Contains(publicMerchantBody, session.userID) || strings.Contains(publicMerchantBody, "updated-profile@example.com") {
		t.Fatalf("public merchant profile leaked owner/contact data: %s", publicMerchantBody)
	}
	if !strings.Contains(publicMerchantBody, `"username":"profile-store"`) {
		t.Fatalf("expected public merchant slug as username field, got %s", publicMerchantBody)
	}

	apiService := createAPIServiceWithPayload(t, server, session, strings.Replace(apiServicePayload(second.ID, "1.0000"), `"merchantIdentityMode":"public_profile"`, `"merchantProfileId":"`+merchant.ID+`","merchantIdentityMode":"store_alias"`, 1), "profile-store-api-service")
	submitted := ownerAPIServiceAction(t, server, session, apiService.ID, "submit-review", apiService.Version, "profile-store-api-submit")
	online := ownerAPIServiceAction(t, server, session, apiService.ID, "publish", submitted.Version, "profile-store-api-publish")
	orderable := updateAPIServiceOrderSettings(t, server, session, online.ID, online.Version, true, "profile-store-api-settings")

	publicService := httptest.NewRequest(http.MethodGet, "/api/v1/api-services/"+orderable.ID, nil)
	publicServiceResponse := httptest.NewRecorder()
	server.ServeHTTP(publicServiceResponse, publicService)
	if publicServiceResponse.Code != http.StatusOK {
		t.Fatalf("public API service status %d body %s", publicServiceResponse.Code, publicServiceResponse.Body.String())
	}
	publicServiceBody := publicServiceResponse.Body.String()
	if !strings.Contains(publicServiceBody, `"merchantDisplayName":"Profile Store"`) || !strings.Contains(publicServiceBody, `"merchantProfileSlug":"profile-store"`) {
		t.Fatalf("expected public API service to expose store alias, got %s", publicServiceBody)
	}
	if strings.Contains(publicServiceBody, session.userID) || strings.Contains(publicServiceBody, second.ID) || strings.Contains(publicServiceBody, "updated-profile@example.com") {
		t.Fatalf("public API service leaked owner/contact data: %s", publicServiceBody)
	}
}

func TestAccountIdentityProfilePasswordEmailAndAvatarFlow(t *testing.T) {
	server := newTestServer(time.Now())
	session := createLinuxDoSession(t, server, "identity-owner")

	setPassword := newJSONRequest(http.MethodPost, "/api/v1/auth/password", `{"newPassword":"backup-password-1"}`)
	addAuth(setPassword, session, "")
	setPasswordResponse := httptest.NewRecorder()
	server.ServeHTTP(setPasswordResponse, setPassword)
	if setPasswordResponse.Code != http.StatusNoContent {
		t.Fatalf("set backup password status %d body %s", setPasswordResponse.Code, setPasswordResponse.Body.String())
	}

	missingCurrentPassword := newJSONRequest(http.MethodPost, "/api/v1/auth/password", `{"newPassword":"backup-password-2"}`)
	addAuth(missingCurrentPassword, session, "")
	missingCurrentPasswordResponse := httptest.NewRecorder()
	server.ServeHTTP(missingCurrentPasswordResponse, missingCurrentPassword)
	if missingCurrentPasswordResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected current password requirement, got %d body %s", missingCurrentPasswordResponse.Code, missingCurrentPasswordResponse.Body.String())
	}
	assertProblemCode(t, missingCurrentPasswordResponse, "VALIDATION_FAILED")

	oldAvatarMode := newJSONRequest(http.MethodPatch, "/api/v1/me/profile", `{
		"displayName":"Identity Owner",
		"username":"identity-owner",
		"bio":"账号资料完善测试。",
		"regionCode":"cn",
		"timezone":"Asia/Shanghai",
		"avatarMode":"custom",
		"avatarUrl":"https://example.com/avatar.png",
		"privacy":{
			"showCreatedAt":true,
			"showLastActiveAt":true,
			"showCompletedCarpoolCount":true,
			"showCompletedApiIntentCount":true,
			"showResponseMedian":true,
			"showResolvedDisputeSummary":true,
			"allowPublicProfileReport":true
		}
	}`)
	addAuth(oldAvatarMode, session, "")
	oldAvatarModeResponse := httptest.NewRecorder()
	server.ServeHTTP(oldAvatarModeResponse, oldAvatarMode)
	if oldAvatarModeResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected old custom avatar mode rejection, got %d body %s", oldAvatarModeResponse.Code, oldAvatarModeResponse.Body.String())
	}
	assertProblemCode(t, oldAvatarModeResponse, "VALIDATION_FAILED")

	updateAvatar := newJSONRequest(http.MethodPatch, "/api/v1/me/profile", `{
		"displayName":"Identity Owner",
		"username":"identity-owner",
		"bio":"账号资料完善测试。",
		"regionCode":"cn",
		"timezone":"Asia/Shanghai",
		"avatarMode":"custom_url",
		"avatarUrl":"https://cdn.example.com/avatar.webp",
		"privacy":{
			"showCreatedAt":true,
			"showLastActiveAt":true,
			"showCompletedCarpoolCount":true,
			"showCompletedApiIntentCount":true,
			"showResponseMedian":true,
			"showResolvedDisputeSummary":true,
			"allowPublicProfileReport":true
		}
	}`)
	addAuth(updateAvatar, session, "")
	updateAvatarResponse := httptest.NewRecorder()
	server.ServeHTTP(updateAvatarResponse, updateAvatar)
	if updateAvatarResponse.Code != http.StatusOK {
		t.Fatalf("custom avatar update status %d body %s", updateAvatarResponse.Code, updateAvatarResponse.Body.String())
	}
	if !strings.Contains(updateAvatarResponse.Body.String(), `"avatarMode":"custom_url"`) || !strings.Contains(updateAvatarResponse.Body.String(), `"customAvatarUrl":"https://cdn.example.com/avatar.webp"`) {
		t.Fatalf("expected custom avatar URL response, got %s", updateAvatarResponse.Body.String())
	}

	startEmail := newJSONRequest(http.MethodPost, "/api/v1/me/email-verification/start", `{"email":"identity@example.com"}`)
	addAuth(startEmail, session, "")
	startEmailResponse := httptest.NewRecorder()
	server.ServeHTTP(startEmailResponse, startEmail)
	if startEmailResponse.Code != http.StatusOK {
		t.Fatalf("start email verification status %d body %s", startEmailResponse.Code, startEmailResponse.Body.String())
	}
	var challenge startEmailVerificationResponse
	if err := json.NewDecoder(startEmailResponse.Body).Decode(&challenge); err != nil {
		t.Fatalf("decode email challenge: %v", err)
	}
	if challenge.Email != "identity@example.com" || challenge.DevCode == "" {
		t.Fatalf("expected development email challenge code, got %+v", challenge)
	}

	confirmEmail := newJSONRequest(http.MethodPost, "/api/v1/me/email-verification/confirm", `{"email":"identity@example.com","code":"`+challenge.DevCode+`"}`)
	addAuth(confirmEmail, session, "")
	confirmEmailResponse := httptest.NewRecorder()
	server.ServeHTTP(confirmEmailResponse, confirmEmail)
	if confirmEmailResponse.Code != http.StatusOK {
		t.Fatalf("confirm email verification status %d body %s", confirmEmailResponse.Code, confirmEmailResponse.Body.String())
	}
	body := confirmEmailResponse.Body.String()
	if !strings.Contains(body, `"email":"identity@example.com"`) || !strings.Contains(body, `"emailVerified":true`) {
		t.Fatalf("expected verified email profile, got %s", body)
	}

	getProfile := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	addCookie(getProfile, session.cookie)
	getProfileResponse := httptest.NewRecorder()
	server.ServeHTTP(getProfileResponse, getProfile)
	if getProfileResponse.Code != http.StatusOK {
		t.Fatalf("get profile status %d body %s", getProfileResponse.Code, getProfileResponse.Body.String())
	}
	if !strings.Contains(getProfileResponse.Body.String(), `"passwordConfigured":true`) {
		t.Fatalf("expected profile to report configured backup password, got %s", getProfileResponse.Body.String())
	}
}

type testSession struct {
	cookie string
	csrf   string
	userID string
}

type createdLead struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
}

type approveResponse struct {
	Lead struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	} `json:"lead"`
	Record struct {
		ID                   string `json:"id"`
		NormalizedMonthlyCNY string `json:"normalizedMonthlyCny"`
		IsLowestReference    bool   `json:"isLowestReference"`
	} `json:"record"`
}

type createdContact struct {
	ID string `json:"id"`
}

type createdMerchantProfile struct {
	ID          string `json:"id"`
	OwnerUserID string `json:"ownerUserId"`
	Slug        string `json:"slug"`
	Version     int64  `json:"version"`
}

type createdCarpool struct {
	ID                 string                    `json:"id"`
	Status             string                    `json:"status"`
	ServiceMultiplier  string                    `json:"serviceMultiplier"`
	MonthlyQuotaAmount string                    `json:"monthlyQuotaAmount"`
	QuotaLabel         string                    `json:"quotaLabel"`
	QuotaUnit          string                    `json:"quotaUnit"`
	QuotaPeriod        string                    `json:"quotaPeriod"`
	AvailableSeats     int                       `json:"availableSeats"`
	CycleTerm          *carpoolCycleTermResponse `json:"cycleTerm"`
	Version            int64                     `json:"version"`
}

type createdCarpoolApplication struct {
	ID                       string  `json:"id"`
	Status                   string  `json:"status"`
	ContactSessionID         string  `json:"contactSessionId"`
	ReservationExpiresAt     *string `json:"reservationExpiresAt"`
	JoinConfirmationDeadline *string `json:"joinConfirmationDeadline"`
	BuyerConfirmedAt         *string `json:"buyerConfirmedAt"`
	OwnerConfirmedAt         *string `json:"ownerConfirmedAt"`
	JoinedAt                 *string `json:"joinedAt"`
	Version                  int64   `json:"version"`
}

type createdCarpoolMembership struct {
	ID                   string  `json:"id"`
	CarpoolListingID     string  `json:"carpoolListingId"`
	CarpoolApplicationID string  `json:"carpoolApplicationId"`
	Status               string  `json:"status"`
	BuyerCompletedAt     *string `json:"buyerCompletedAt"`
	OwnerCompletedAt     *string `json:"ownerCompletedAt"`
	CompletedAt          *string `json:"completedAt"`
	EndedAt              *string `json:"endedAt"`
	EndedReason          string  `json:"endedReason"`
	EndedByUserID        string  `json:"endedByUserId"`
	Version              int64   `json:"version"`
}

type createdAPIService struct {
	ID                     string                    `json:"id"`
	ReviewStatus           string                    `json:"reviewStatus"`
	PublicationStatus      string                    `json:"publicationStatus"`
	ModerationStatus       string                    `json:"moderationStatus"`
	AcceptingOrders        bool                      `json:"acceptingOrders"`
	PaymentWindowMinutes   int                       `json:"paymentWindowMinutes"`
	AcceptedPaymentMethods []string                  `json:"acceptedPaymentMethods"`
	IsOrderable            bool                      `json:"isOrderable"`
	OrderableReasons       []string                  `json:"orderableReasons"`
	Models                 []apiServiceModelResponse `json:"models"`
	Version                int64                     `json:"version"`
}

type createdAPIPurchaseIntent struct {
	ID                                       string           `json:"id"`
	APIServiceID                             string           `json:"apiServiceId"`
	BuyerUserID                              string           `json:"buyerUserId"`
	OwnerUserID                              string           `json:"ownerUserId"`
	BuyerContactMethodID                     string           `json:"buyerContactMethodId"`
	OwnerContactMethodID                     string           `json:"ownerContactMethodId"`
	Status                                   string           `json:"status"`
	RequestedCNYAmount                       string           `json:"requestedCnyAmount"`
	RequestedUSDAllowance                    string           `json:"requestedUsdAllowance"`
	SelectedAccessMode                       string           `json:"selectedAccessMode"`
	ServiceVersionSnapshot                   int64            `json:"serviceVersionSnapshot"`
	ServiceTitleSnapshot                     string           `json:"serviceTitleSnapshot"`
	DistributionSystemSnapshot               string           `json:"distributionSystemSnapshot"`
	BillingModeSnapshot                      string           `json:"billingModeSnapshot"`
	DeclaredCNYPerUSDAllowanceSnapshot       string           `json:"declaredCnyPerUsdAllowanceSnapshot"`
	DeclaredMaxUSDAllowancePerIntentSnapshot string           `json:"declaredMaxUsdAllowancePerIntentSnapshot"`
	MinimumIntentCNYSnapshot                 string           `json:"minimumIntentCnySnapshot"`
	MaximumIntentCNYSnapshot                 string           `json:"maximumIntentCnySnapshot"`
	PricingSnapshot                          string           `json:"pricingSnapshot"`
	BuyerNote                                string           `json:"buyerNote"`
	ContactedAt                              *string          `json:"contactedAt"`
	BuyerCancelledAt                         *string          `json:"buyerCancelledAt"`
	BuyerCancelReason                        string           `json:"buyerCancelReason"`
	OwnerClosedAt                            *string          `json:"ownerClosedAt"`
	OwnerCloseReason                         string           `json:"ownerCloseReason"`
	Version                                  int64            `json:"version"`
	MerchantContact                          *testContactItem `json:"merchantContact"`
	BuyerContact                             *testContactItem `json:"buyerContact"`
}

type createdAPIOrder struct {
	ID                           string  `json:"id"`
	APIPurchaseIntentID          string  `json:"apiPurchaseIntentId"`
	APIServiceID                 string  `json:"apiServiceId"`
	BuyerUserID                  string  `json:"buyerUserId"`
	SellerUserID                 string  `json:"sellerUserId"`
	Status                       string  `json:"status"`
	DisputeStatus                string  `json:"disputeStatus"`
	DisputeCaseID                string  `json:"disputeCaseId"`
	ServiceTitleSnapshot         string  `json:"serviceTitleSnapshot"`
	Amount                       string  `json:"amount"`
	Currency                     string  `json:"currency"`
	SelectedPaymentMethod        string  `json:"selectedPaymentMethod"`
	PaymentWindowMinutesSnapshot int     `json:"paymentWindowMinutesSnapshot"`
	PaymentExpiresAt             string  `json:"paymentExpiresAt"`
	PaymentSummary               string  `json:"paymentSummary"`
	PaidConfirmedAt              *string `json:"paidConfirmedAt"`
	DeliveryNote                 string  `json:"deliveryNote"`
	CompletedAt                  *string `json:"completedAt"`
	CancelReason                 string  `json:"cancelReason"`
	Version                      int64   `json:"version"`
}

type createdDispute struct {
	ID                   string `json:"id"`
	ReportID             string `json:"reportId"`
	TargetType           string `json:"targetType"`
	TargetID             string `json:"targetId"`
	TargetLabel          string `json:"targetLabel"`
	PrimaryUserID        string `json:"primaryUserId"`
	PrimaryUsername      string `json:"primaryUsername"`
	PrimaryDisplayName   string `json:"primaryDisplayName"`
	CounterpartyUserID   string `json:"counterpartyUserId"`
	CounterpartyUsername string `json:"counterpartyUsername"`
	CounterpartyName     string `json:"counterpartyName"`
	Status               string `json:"status"`
	PublicSummary        string `json:"publicSummary"`
	PublicResult         string `json:"publicResult"`
	OpenedByAdminID      string `json:"openedByAdminId"`
	Version              int64  `json:"version"`
}

type apiOrderPaymentInstructions struct {
	OrderID             string `json:"orderId"`
	PaymentMethod       string `json:"paymentMethod"`
	PaymentInstructions string `json:"paymentInstructions"`
	PaymentExpiresAt    string `json:"paymentExpiresAt"`
}

type testContactItem struct {
	Side        string `json:"side"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	MaskedValue string `json:"maskedValue"`
}

func newTestServer(now time.Time) http.Handler {
	return NewServer(app.NewServiceWithClock(func() time.Time { return now }))
}

func createSession(t *testing.T, server http.Handler, username string, admin bool) testSession {
	t.Helper()
	body := `{"username":"` + username + `","admin":` + boolString(admin) + `}`
	request := newJSONRequest(http.MethodPost, "/api/v1/auth/dev-session", body)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("dev session status %d body %s", response.Code, response.Body.String())
	}
	var payload sessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	cookie := response.Result().Cookies()[0]
	return testSession{cookie: cookie.Value, csrf: payload.CSRFToken, userID: payload.User.ID}
}

func createLinuxDoSession(t *testing.T, server http.Handler, username string) testSession {
	t.Helper()
	state := "oauth-state-" + username
	callbackURL := "/api/v1/auth/oauth/callback?state=" + url.QueryEscape(state) + "&code=" + url.QueryEscape(username)
	callback := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	callback.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: state})
	callbackResponse := httptest.NewRecorder()
	server.ServeHTTP(callbackResponse, callback)
	if callbackResponse.Code != http.StatusFound {
		t.Fatalf("oauth callback status %d body %s", callbackResponse.Code, callbackResponse.Body.String())
	}
	var sessionCookie string
	for _, cookie := range callbackResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			sessionCookie = cookie.Value
			break
		}
	}
	if sessionCookie == "" {
		t.Fatalf("expected oauth callback to set session cookie")
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	addCookie(request, sessionCookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("oauth session status %d body %s", response.Code, response.Body.String())
	}
	var payload sessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode oauth session: %v", err)
	}
	if !payload.User.LinuxDo.Bound {
		t.Fatalf("expected oauth session to bind linux.do, got %+v", payload.User.LinuxDo)
	}
	return testSession{cookie: sessionCookie, csrf: payload.CSRFToken, userID: payload.User.ID}
}

func submitLead(t *testing.T, server http.Handler, session testSession, key string) createdLead {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/official-price-leads", leadPayload("https://linux.do/t/example/123"))
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("submit lead status %d body %s", response.Code, response.Body.String())
	}
	var lead createdLead
	if err := json.NewDecoder(response.Body).Decode(&lead); err != nil {
		t.Fatalf("decode lead: %v", err)
	}
	return lead
}

func createApprovedOfficialPriceRecord(t *testing.T, server http.Handler, buyerSession, adminSession testSession, slug, amount, rate string) approveResponse {
	t.Helper()
	body := strings.Replace(leadPayload("https://linux.do/t/example/"+slug), `"openingMethod":"official_web"`, `"openingMethod":"official_web_`+slug+`"`, 1)
	body = strings.Replace(body, "799.00", amount, 1)
	request := newJSONRequest(http.MethodPost, "/api/v1/official-price-leads", body)
	addAuth(request, buyerSession, "lead-"+slug)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("submit lead %s status %d body %s", slug, response.Code, response.Body.String())
	}
	var lead createdLead
	if err := json.NewDecoder(response.Body).Decode(&lead); err != nil {
		t.Fatalf("decode lead %s: %v", slug, err)
	}

	approveBody := `{
		"reason":"来源可访问，价格字段完整。",
		"resolvedProductPlanId":"00000000-0000-0000-0000-000000000303",
		"validFrom":"2026-06-21T00:00:00Z",
		"fxSnapshot":{"rateToCny":"` + rate + `","source":"admin_configured_snapshot","observedAt":"2026-06-21T06:00:00Z"}
	}`
	return approveLead(t, server, adminSession, lead.ID, approveBody, "approve-"+slug)
}

func approveLead(t *testing.T, server http.Handler, session testSession, leadID, body, key string) approveResponse {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/official-price-leads/"+leadID+"/approve", body)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"1"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("approve lead status %d body %s", response.Code, response.Body.String())
	}
	var payload approveResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode approve: %v", err)
	}
	return payload
}

func createContactMethod(t *testing.T, server http.Handler, session testSession, typ, label, value string) createdContact {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/contact-methods", `{"type":"`+typ+`","label":"`+label+`","value":"`+value+`"}`)
	addAuth(request, session, "contact-"+label)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create contact status %d body %s", response.Code, response.Body.String())
	}
	var payload createdContact
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode contact: %v", err)
	}
	return payload
}

func createCarpool(t *testing.T, server http.Handler, session testSession, ownerContactID, key string) createdCarpool {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/carpools", carpoolPayloadWithRiskAck(ownerContactID))
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create carpool status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpool
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode carpool: %v", err)
	}
	return payload
}

func submitCarpoolReview(t *testing.T, server http.Handler, session testSession, listingID string, version int64, key string) createdCarpool {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/carpools/"+listingID+"/submit-review", `{}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("submit carpool review status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpool
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode submitted carpool: %v", err)
	}
	return payload
}

func reviewCarpool(t *testing.T, server http.Handler, session testSession, listingID, action string, version int64, key string) createdCarpool {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/carpools/"+listingID+"/"+action, `{"reason":"车源说明完整，风险确认已记录。"}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("review carpool status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpool
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode reviewed carpool: %v", err)
	}
	return payload
}

func assertPublicCarpoolVisible(t *testing.T, server http.Handler, listingID string, visible bool) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/carpools/"+listingID, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if visible && response.Code != http.StatusOK {
		t.Fatalf("expected public carpool visible, got %d body %s", response.Code, response.Body.String())
	}
	if !visible && response.Code != http.StatusNotFound {
		t.Fatalf("expected public carpool hidden, got %d body %s", response.Code, response.Body.String())
	}
}

func createCarpoolApplication(t *testing.T, server http.Handler, session testSession, listingID, buyerContactID, key string) createdCarpoolApplication {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/carpools/"+listingID+"/applications", carpoolApplicationPayload(buyerContactID))
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create carpool application status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpoolApplication
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode carpool application: %v", err)
	}
	return payload
}

func acceptCarpoolApplication(t *testing.T, server http.Handler, session testSession, applicationID string, version int64, key string) createdCarpoolApplication {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/carpool-applications/"+applicationID+"/accept", `{}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("accept carpool application status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpoolApplication
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode accepted application: %v", err)
	}
	return payload
}

func cancelCarpoolApplication(t *testing.T, server http.Handler, session testSession, applicationID string, version int64, key string) createdCarpoolApplication {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/me/carpool-applications/"+applicationID+"/cancel", `{"reason":"买家取消本次拼车申请。"}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("cancel carpool application status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpoolApplication
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode cancelled application: %v", err)
	}
	return payload
}

func withdrawCarpoolAcceptance(t *testing.T, server http.Handler, session testSession, applicationID string, version int64, key string) createdCarpoolApplication {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/carpool-applications/"+applicationID+"/withdraw-acceptance", `{"reason":"车主撤回本次预留。"}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("withdraw carpool acceptance status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpoolApplication
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode withdrawn application: %v", err)
	}
	return payload
}

func assertContactSessionConflict(t *testing.T, server http.Handler, session testSession, contactSessionID string) {
	t.Helper()
	if strings.TrimSpace(contactSessionID) == "" {
		t.Fatalf("expected contact session id")
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/contact-sessions/"+contactSessionID+"/contacts", nil)
	addCookie(request, session.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected contact session conflict, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "CONTACT_WINDOW_EXPIRED")
}

func confirmCarpoolJoin(t *testing.T, server http.Handler, session testSession, perspective, applicationID string, version int64, key string) createdCarpoolApplication {
	t.Helper()
	path := "/api/v1/me/carpool-applications/" + applicationID + "/confirm-join"
	if perspective == "owner" {
		path = "/api/v1/owner/carpool-applications/" + applicationID + "/confirm-join"
	}
	request := newJSONRequest(http.MethodPost, path, `{}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("confirm carpool join status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpoolApplication
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode confirmed application: %v", err)
	}
	return payload
}

func firstCarpoolMembership(t *testing.T, server http.Handler, session testSession, perspective, applicationID string) createdCarpoolMembership {
	t.Helper()
	path := "/api/v1/me/carpool-memberships"
	if perspective == "owner" {
		path = "/api/v1/owner/carpool-memberships"
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(request, session.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list memberships status %d body %s", response.Code, response.Body.String())
	}
	var payload struct {
		Items []createdCarpoolMembership `json:"items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode memberships: %v", err)
	}
	for _, membership := range payload.Items {
		if membership.CarpoolApplicationID == applicationID {
			return membership
		}
	}
	t.Fatalf("expected membership for application %s, got %+v", applicationID, payload.Items)
	return createdCarpoolMembership{}
}

func confirmCarpoolMembershipComplete(t *testing.T, server http.Handler, session testSession, perspective, membershipID string, version int64, key string) createdCarpoolMembership {
	t.Helper()
	path := "/api/v1/me/carpool-memberships/" + membershipID + "/confirm-complete"
	if perspective == "owner" {
		path = "/api/v1/owner/carpool-memberships/" + membershipID + "/confirm-complete"
	}
	request := newJSONRequest(http.MethodPost, path, `{}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("confirm carpool membership complete status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpoolMembership
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode completed membership: %v", err)
	}
	return payload
}

func endCarpoolMembership(t *testing.T, server http.Handler, session testSession, perspective, action, membershipID string, version int64, key string) createdCarpoolMembership {
	t.Helper()
	path := "/api/v1/me/carpool-memberships/" + membershipID + "/" + action
	if perspective == "owner" {
		path = "/api/v1/owner/carpool-memberships/" + membershipID + "/" + action
	}
	request := newJSONRequest(http.MethodPost, path, `{"reason":"成员周期调整，双方站外自行处理后续事项。"}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("end carpool membership status %d body %s", response.Code, response.Body.String())
	}
	var payload createdCarpoolMembership
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode ended membership: %v", err)
	}
	return payload
}

func createAPIService(t *testing.T, server http.Handler, session testSession, ownerContactID, key string) createdAPIService {
	t.Helper()
	return createAPIServiceWithPayload(t, server, session, apiServicePayload(ownerContactID, "1.0000"), key)
}

func createAdminAPIModel(t *testing.T, server http.Handler, session testSession, modelKey, key string) apiModelResponse {
	t.Helper()
	provider := createAdminAPIModelProvider(t, server, session, "provider-"+strings.ToLower(strings.ReplaceAll(modelKey, "_", "-")), key+"-provider")
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/api-models", apiModelPayload(provider.ID, modelKey, "Admin Test Model", "0.1", "", "0.2", "pricing", true))
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create admin API model status %d body %s", response.Code, response.Body.String())
	}
	var payload apiModelResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode admin API model: %v", err)
	}
	return payload
}

func createAdminAPIModelProvider(t *testing.T, server http.Handler, session testSession, code, key string) apiModelProviderResponse {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/api-model-providers", apiModelProviderPayload("gpt", code, "OpenAI "+code, true))
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create admin API model provider status %d body %s", response.Code, response.Body.String())
	}
	var payload apiModelProviderResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode admin API model provider: %v", err)
	}
	return payload
}

func createAPIServiceWithPayload(t *testing.T, server http.Handler, session testSession, requestBody, key string) createdAPIService {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/api-services", requestBody)
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create API service status %d body %s", response.Code, response.Body.String())
	}
	var payload createdAPIService
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode API service: %v", err)
	}
	return payload
}

func updateAPIService(t *testing.T, server http.Handler, session testSession, serviceID string, version int64, body string, key string) createdAPIService {
	t.Helper()
	request := newJSONRequest(http.MethodPatch, "/api/v1/owner/api-services/"+serviceID, body)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("update API service status %d body %s", response.Code, response.Body.String())
	}
	var payload createdAPIService
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode updated API service: %v", err)
	}
	return payload
}

func getOwnerAPIService(t *testing.T, server http.Handler, session testSession, serviceID string) createdAPIService {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/owner/api-services/"+serviceID, nil)
	addCookie(request, session.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("get owner API service status %d body %s", response.Code, response.Body.String())
	}
	var payload createdAPIService
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode owner API service: %v", err)
	}
	return payload
}

func ownerAPIServiceAction(t *testing.T, server http.Handler, session testSession, serviceID, action string, version int64, key string) createdAPIService {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/api-services/"+serviceID+"/"+action, `{}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("owner API service action %s status %d body %s", action, response.Code, response.Body.String())
	}
	var payload createdAPIService
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode owner API service action: %v", err)
	}
	return payload
}

func updateAPIServiceOrderSettings(t *testing.T, server http.Handler, session testSession, serviceID string, version int64, accepting bool, key string) createdAPIService {
	t.Helper()
	body := `{
		"acceptingOrders":` + boolString(accepting) + `,
		"paymentWindowMinutes":10,
		"paymentOptions":[
			{"paymentMethod":"wechat","enabled":true,"paymentInstructions":"微信收款二维码请按商户站外确认展示，付款后填写付款摘要。"},
			{"paymentMethod":"alipay","enabled":false,"paymentInstructions":"支付宝收款说明暂不启用。"}
		]
	}`
	request := newJSONRequest(http.MethodPatch, "/api/v1/owner/api-services/"+serviceID+"/order-settings", body)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("update API service order settings status %d body %s", response.Code, response.Body.String())
	}
	var payload createdAPIService
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode API service order settings: %v", err)
	}
	return payload
}

func adminAPIServiceAction(t *testing.T, server http.Handler, session testSession, serviceID, action string, version int64, key string) createdAPIService {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/api-services/"+serviceID+"/"+action, `{"reason":"资料完整，接入说明符合平台边界。"}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin API service action %s status %d body %s", action, response.Code, response.Body.String())
	}
	var payload createdAPIService
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode admin API service action: %v", err)
	}
	return payload
}

func createAPIPurchaseIntent(t *testing.T, server http.Handler, session testSession, serviceID, buyerContactID, key string) createdAPIPurchaseIntent {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/api-services/"+serviceID+"/purchase-intents", apiPurchaseIntentPayload(buyerContactID))
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create API purchase intent status %d body %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected private no-store on API purchase intent create, got %q", got)
	}
	if got := response.Header().Get("Location"); got == "" {
		t.Fatalf("expected Location on API purchase intent create")
	}
	if got := response.Header().Get("ETag"); got == "" {
		t.Fatalf("expected ETag on API purchase intent create")
	}
	body := response.Body.String()
	if strings.Contains(body, "subjectId") || strings.Contains(body, "ownerUserId") || strings.Contains(body, "ownerContactMethodId") {
		t.Fatalf("API purchase intent create leaked owner identity fields: %s", body)
	}
	var payload createdAPIPurchaseIntent
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&payload); err != nil {
		t.Fatalf("decode API purchase intent: %v", err)
	}
	return payload
}

func createAPIOrder(t *testing.T, server http.Handler, session testSession, intentID, paymentMethod, key string) createdAPIOrder {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/me/api-purchase-intents/"+intentID+"/orders", `{"paymentMethod":"`+paymentMethod+`"}`)
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create API order status %d body %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected private no-store on API order create, got %q", got)
	}
	if got := response.Header().Get("Location"); got == "" {
		t.Fatalf("expected Location on API order create")
	}
	if got := response.Header().Get("ETag"); got == "" {
		t.Fatalf("expected ETag on API order create")
	}
	body := response.Body.String()
	if strings.Contains(body, "paymentInstructions") {
		t.Fatalf("API order create must not include payment instructions: %s", body)
	}
	var payload createdAPIOrder
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&payload); err != nil {
		t.Fatalf("decode API order: %v", err)
	}
	return payload
}

func readAPIOrderPaymentInstructions(t *testing.T, server http.Handler, session testSession, orderID string) apiOrderPaymentInstructions {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/me/api-orders/"+orderID+"/payment-instructions", `{}`)
	addAuth(request, session, "api-order-payment-instructions-"+orderID)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("read API order payment instructions status %d body %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected private no-store on API order payment instructions, got %q", got)
	}
	var payload apiOrderPaymentInstructions
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode API order payment instructions: %v", err)
	}
	return payload
}

func apiOrderAction(t *testing.T, server http.Handler, session testSession, perspective, orderID, action string, version int64, key, body string) createdAPIOrder {
	t.Helper()
	path := "/api/v1/me/api-orders/" + orderID + "/" + action
	if perspective == "owner" {
		path = "/api/v1/owner/api-orders/" + orderID + "/" + action
	}
	request := newJSONRequest(http.MethodPost, path, body)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("API order action %s/%s status %d body %s", perspective, action, response.Code, response.Body.String())
	}
	if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("expected private no-store on API order action %s, got %q", action, got)
	}
	if got := response.Header().Get("ETag"); got == "" {
		t.Fatalf("expected ETag on API order action %s", action)
	}
	var payload createdAPIOrder
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode API order action %s: %v", action, err)
	}
	return payload
}

func listAdminDisputes(t *testing.T, server http.Handler, session testSession) listResponse[createdDispute] {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/disputes", nil)
	addCookie(request, session.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin disputes status %d body %s", response.Code, response.Body.String())
	}
	var payload listResponse[createdDispute]
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode admin disputes: %v", err)
	}
	return payload
}

func getAPIPurchaseIntent(t *testing.T, server http.Handler, session testSession, perspective, intentID string) createdAPIPurchaseIntent {
	t.Helper()
	path := "/api/v1/me/api-purchase-intents/" + intentID
	switch perspective {
	case "owner":
		path = "/api/v1/owner/api-purchase-intents/" + intentID
	case "admin":
		path = "/api/v1/admin/api-purchase-intents/" + intentID
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(request, session.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("get API purchase intent %s status %d body %s", perspective, response.Code, response.Body.String())
	}
	if perspective == "me" || perspective == "owner" {
		if got := response.Header().Get("Cache-Control"); got != "private, no-store" {
			t.Fatalf("expected private no-store on %s API purchase intent detail, got %q", perspective, got)
		}
	}
	body := response.Body.String()
	if (perspective == "me" || perspective == "owner") && strings.Contains(body, "subjectId") {
		t.Fatalf("API purchase intent %s detail leaked subjectId: %s", perspective, body)
	}
	if perspective == "me" && (strings.Contains(body, "ownerUserId") || strings.Contains(body, "ownerContactMethodId")) {
		t.Fatalf("buyer API purchase intent detail leaked owner identity fields: %s", body)
	}
	var payload createdAPIPurchaseIntent
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&payload); err != nil {
		t.Fatalf("decode API purchase intent detail: %v", err)
	}
	return payload
}

func ownerAPIPurchaseIntentAction(t *testing.T, server http.Handler, session testSession, intentID, action string, version int64, key, body string) createdAPIPurchaseIntent {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/api-purchase-intents/"+intentID+"/"+action, body)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("owner API purchase intent action %s status %d body %s", action, response.Code, response.Body.String())
	}
	var payload createdAPIPurchaseIntent
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode owner API purchase intent action: %v", err)
	}
	return payload
}

func cancelAPIPurchaseIntent(t *testing.T, server http.Handler, session testSession, intentID string, version int64, key string) createdAPIPurchaseIntent {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/me/api-purchase-intents/"+intentID+"/cancel", `{"reason":"买家已站外改约其他安排。"}`)
	addAuth(request, session, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("cancel API purchase intent status %d body %s", response.Code, response.Body.String())
	}
	var payload createdAPIPurchaseIntent
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode cancelled API purchase intent: %v", err)
	}
	return payload
}

func assertPublicAPIServiceBody(t *testing.T, body, ownerContactID string) {
	t.Helper()
	for _, forbidden := range []string{
		`"ownerUserId"`,
		`"merchantProfileId"`,
		`"ownerContactMethodId"`,
		`"merchantNote"`,
		`"reviewStatus"`,
		`"publicationStatus"`,
		`"moderationStatus"`,
		`"approvedByAdminId"`,
		`"moderationReason"`,
		ownerContactID,
		"仅后台可见，不展示给公开访客。",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public API service response leaked %q in body %s", forbidden, body)
		}
	}
}

func leadPayload(sourceURL string) string {
	return `{
		"productText":"ChatGPT Pro",
		"regionCode":"ph",
		"channel":"web",
		"openingMethod":"official_web",
		"sourceUrl":"` + sourceURL + `",
		"sourceTitle":"用户低价线索帖",
		"evidenceSummary":"帖子中展示菲律宾区 Web 价格。",
		"observedAt":"2026-06-21T06:30:00Z",
		"billingPeriod":"monthly",
		"currency":"PHP",
		"originalAmount":"799.00",
		"originalPriceText":"PHP 799 / month",
		"taxIncluded":true
	}`
}

func carpoolPayload(ownerContactID string) string {
	return `{
		"productPlanId":"00000000-0000-0000-0000-000000000303",
		"ownerContactMethodId":"` + ownerContactID + `",
		"cycleTerm":{
			"billingPeriod":"monthly",
			"cycleStartDay":1,
			"noticeDays":3,
			"exitPolicy":"按月分摊，退出需提前 3 天站外告知车主，平台不处理付款或补偿。",
			"usageRules":"仅按车主说明使用席位，不在平台填写、粘贴或上传任何密码、API Key、Token、Cookie 或 Session。"
		},
		"title":"ChatGPT Pro 20x Web 费用分摊",
		"summary":"车主说明每月账期、名额和站外联系安排。",
		"accessArrangement":"费用分摊方案，不在平台填写或上传任何密码、API Key、Token、Cookie、Session 或面板主账号凭据。",
		"sourceUrl":"https://linux.do/t/carpool/123",
		"priceMonthlyCny":"68.00",
		"serviceMultiplier":"1.3500",
		"monthlyQuotaAmount":"200.00",
		"buyerSeatCapacity":1,
		"activeBuyerMembers":0
	}`
}

func carpoolPayloadWithRiskAck(ownerContactID string) string {
	return strings.Replace(carpoolPayload(ownerContactID), "\n\t}", `,
		"riskAcknowledgement":{"riskNoticeCode":"openai_subscription_carpool","policyVersion":1}
	}`, 1)
}

func carpoolApplicationPayload(contactID string) string {
	return `{
		"buyerContactMethodId":"` + contactID + `",
		"riskAcknowledgement":{"riskNoticeCode":"openai_subscription_carpool","policyVersion":1}
	}`
}

func apiServicePayload(ownerContactID, multiplier string) string {
	return apiServicePayloadWithModelAndMultiplier(ownerContactID, "00000000-0000-0000-0000-000000000a01", multiplier)
}

func apiServicePayloadWithModel(ownerContactID, modelCatalogID string) string {
	return apiServicePayloadWithModelAndMultiplier(ownerContactID, modelCatalogID, "1.0000")
}

func apiServicePayloadWithModelAndMultiplier(ownerContactID, modelCatalogID, multiplier string) string {
	return `{
		"merchantIdentityMode":"public_profile",
		"ownerContactMethodId":"` + ownerContactID + `",
		"title":"Sub2API 美元额度意向服务",
		"shortDescription":"商户声明美元额度售价，双方站外确认具体安排。",
		"distributionSystem":"sub2api",
		"billingMode":"metered_usd_quota",
		"declaredCnyPerUsdAllowance":"0.8000",
		"declaredMaxUsdAllowancePerIntent":"20.000000",
		"quotaExpiresAt":"` + time.Now().Add(30*24*time.Hour).UTC().Format(time.RFC3339) + `",
		"minimumIntentCny":"10.00",
		"maximumIntentCny":"200.00",
		"usageVisibility":"merchant_reported",
		"publicAccessNote":"提交购买意向后直接查看商户联系方式，平台不保存任何调用凭据。",
		"merchantNote":"仅后台可见，不展示给公开访客。",
		"merchantSupportNote":"仅支持买家专属、可撤销的子级访问安排。",
		"accessModes":[
			{"accessMode":"buyer_dedicated_sub_key","publicNote":"站外确认买家专属、可撤销的访问方式。"}
		],
		"models":[
			{"modelCatalogId":"` + modelCatalogID + `","merchantMultiplier":"` + multiplier + `"}
		],
		"packages":[]
	}`
}

func apiPurchaseIntentPayload(buyerContactID string) string {
	return `{
		"buyerContactMethodId":"` + buyerContactID + `",
		"selectedAccessMode":"buyer_dedicated_sub_key",
		"requestedCnyAmount":"16.00",
		"requestedUsdAllowance":"20.000000",
		"buyerNote":"希望站外确认 20 美元额度。"
	}`
}

func productPlanPayload(categoryCode, slug, displayName, publishPolicy, riskLevel string, riskAckRequired bool) string {
	categoryID := map[string]string{
		"gpt":        "00000000-0000-0000-0000-000000000101",
		"claude":     "00000000-0000-0000-0000-000000000102",
		"cursor":     "00000000-0000-0000-0000-000000000103",
		"gemini":     "00000000-0000-0000-0000-000000000104",
		"perplexity": "00000000-0000-0000-0000-000000000105",
		"other":      "00000000-0000-0000-0000-000000000199",
	}[categoryCode]
	return productPlanPayloadWithCategoryID(categoryID, categoryCode, slug, displayName, publishPolicy, riskLevel, riskAckRequired)
}

func apiModelPayload(providerID, modelKey, displayName, inputPrice, cachedInputPrice, outputPrice, sourceVersion string, active bool) string {
	return apiModelPayloadWithCapabilities(providerID, modelKey, displayName, []string{" vision ", "chat", "text", "chat"}, inputPrice, cachedInputPrice, outputPrice, sourceVersion, active)
}

func apiModelPayloadWithCapabilities(providerID, modelKey, displayName string, capabilities []string, inputPrice, cachedInputPrice, outputPrice, sourceVersion string, active bool) string {
	capabilityJSON := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		capabilityJSON = append(capabilityJSON, strconv.Quote(capability))
	}
	sourceURL := "https://example.com/api-pricing"
	if sourceVersion == "" {
		sourceURL = ""
	}
	return `{
		"providerId":"` + providerID + `",
		"modelKey":"` + modelKey + `",
		"displayName":"` + displayName + `",
		"capabilities":[` + strings.Join(capabilityJSON, ",") + `],
		"inputTokenPrice":"` + inputPrice + `",
		"cachedInputTokenPrice":"` + cachedInputPrice + `",
		"outputTokenPrice":"` + outputPrice + `",
		"sourceUrl":"` + sourceURL + `",
		"sourceVersion":"` + sourceVersion + `",
		"active":` + boolString(active) + `,
		"sortOrder":333
	}`
}

func apiModelProviderPayload(providerCategory, code, displayName string, active bool) string {
	return `{
		"providerCategory":"` + providerCategory + `",
		"code":"` + code + `",
		"displayName":"` + displayName + `",
		"active":` + boolString(active) + `,
		"sortOrder":123
	}`
}

func productPlanPayloadWithCategoryID(categoryID, categoryCode, slug, displayName, publishPolicy, riskLevel string, riskAckRequired bool) string {
	riskNoticeCode := ""
	if riskAckRequired {
		riskNoticeCode = "openai_subscription_carpool"
	}
	return `{
		"categoryId":"` + categoryID + `",
		"providerCode":"` + categoryCode + `",
		"slug":"` + slug + `",
		"displayName":"` + displayName + `",
		"description":"后台 CRUD 测试套餐。",
		"publishPolicy":"` + publishPolicy + `",
		"accessMode":"owner_managed_access",
		"providerPolicyStatus":"unknown",
		"riskLevel":"` + riskLevel + `",
		"riskAckRequired":` + boolString(riskAckRequired) + `,
		"riskNoticeCode":"` + riskNoticeCode + `",
		"policyNote":"后台测试更新。",
		"quotaLabel":"额度",
		"quotaUnit":"USD",
		"quotaPeriod":"monthly",
		"active":true,
		"allowCustomVariant":true,
		"sortOrder":120
	}`
}

func newJSONRequest(method, path, body string) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func addAuth(request *http.Request, session testSession, key string) {
	addCookie(request, session.cookie)
	request.Header.Set(csrfHeaderName, session.csrf)
	request.Header.Set("Idempotency-Key", key)
}

func addCookie(request *http.Request, value string) {
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: value})
}

func assertProblemCode(t *testing.T, response *httptest.ResponseRecorder, code string) {
	t.Helper()
	var problem problemDetails
	if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if problem.Code != code {
		t.Fatalf("expected problem code %s, got %s", code, problem.Code)
	}
	if problem.RequestID == "" {
		t.Fatalf("expected request id in problem")
	}
}

func collectChiRoutes(t *testing.T, router chi.Routes) map[string]struct{} {
	t.Helper()
	routes := map[string]struct{}{}
	if err := chi.Walk(router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		if method == "" || method == "*" {
			return nil
		}
		routes[method+" "+route] = struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("walk chi routes: %v", err)
	}
	return routes
}

func collectOpenAPIRoutes(t *testing.T, includeDev bool) map[string]struct{} {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "openapi", "c2c-market-api-v1.yaml"))
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	pathRE := regexp.MustCompile(`^  (/[^:]+):$`)
	methodRE := regexp.MustCompile(`^    (get|post|put|patch|delete):$`)
	routes := map[string]struct{}{}
	var currentPath, currentMethod string
	var methodLines []string
	flush := func() {
		if currentPath == "" || currentMethod == "" {
			return
		}
		isDev := false
		for _, line := range methodLines {
			if strings.Contains(line, "x-dev-only: true") {
				isDev = true
				break
			}
		}
		if includeDev || !isDev {
			routes[strings.ToUpper(currentMethod)+" "+currentPath] = struct{}{}
		}
	}
	for _, line := range strings.Split(string(contents), "\n") {
		if matches := pathRE.FindStringSubmatch(line); matches != nil {
			flush()
			currentPath = matches[1]
			currentMethod = ""
			methodLines = nil
			continue
		}
		if currentPath == "" {
			continue
		}
		if matches := methodRE.FindStringSubmatch(line); matches != nil {
			flush()
			currentMethod = matches[1]
			methodLines = []string{line}
			continue
		}
		if currentMethod != "" {
			methodLines = append(methodLines, line)
		}
	}
	flush()
	return routes
}

func assertRouteSetsEqual(t *testing.T, runtimeRoutes, openAPIRoutes map[string]struct{}) {
	t.Helper()
	for route := range runtimeRoutes {
		if _, ok := openAPIRoutes[route]; !ok {
			t.Fatalf("runtime route missing from OpenAPI: %s", route)
		}
	}
	for route := range openAPIRoutes {
		if _, ok := runtimeRoutes[route]; !ok {
			t.Fatalf("OpenAPI route missing from runtime: %s", route)
		}
	}
}

func assertUUID(t *testing.T, value string, label string) {
	t.Helper()
	if _, err := uuid.Parse(value); err != nil {
		t.Fatalf("expected %s to be UUID, got %q: %v", label, value, err)
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

type fakeReadinessChecker struct {
	status health.Status
}

func (f fakeReadinessChecker) Readiness(context.Context) health.Status {
	return f.status
}
