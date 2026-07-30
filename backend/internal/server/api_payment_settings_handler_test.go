package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPIAccountPaymentSettingsRoutePersistsOneEnabledMethod(t *testing.T) {
	server := newTestServer(time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC))
	session := createSession(t, server, "payment-owner", false)

	initialRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me/api-payment-settings", nil)
	addCookie(initialRequest, session.cookie)
	initialResponse := httptest.NewRecorder()
	server.ServeHTTP(initialResponse, initialRequest)
	if initialResponse.Code != http.StatusOK {
		t.Fatalf("initial payment settings status %d body %s", initialResponse.Code, initialResponse.Body.String())
	}
	var initial apiAccountPaymentSettingsResponse
	if err := json.NewDecoder(initialResponse.Body).Decode(&initial); err != nil {
		t.Fatalf("decode initial payment settings: %v", err)
	}
	if len(initial.PaymentOptions) != 2 || initial.PaymentOptions[0].Enabled || initial.PaymentOptions[1].Enabled {
		t.Fatalf("unexpected initial payment settings: %+v", initial)
	}

	updateRequest := newJSONRequest(http.MethodPut, "/api/v1/me/api-payment-settings", `{
		"paymentWindowMinutes":10,
		"paymentOptions":[
			{"paymentMethod":"wechat","enabled":true,"paymentInstructions":"","paymentQrCodeDataUrl":"data:image/png;base64,d2VjaGF0"},
			{"paymentMethod":"alipay","enabled":false,"paymentInstructions":"备用支付宝","paymentQrCodeDataUrl":"data:image/png;base64,YWxpcGF5"}
		]
	}`)
	addAuth(updateRequest, session, "account-payment-update")
	updateResponse := httptest.NewRecorder()
	server.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update payment settings status %d body %s", updateResponse.Code, updateResponse.Body.String())
	}

	readRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me/api-payment-settings", nil)
	addCookie(readRequest, session.cookie)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, readRequest)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read payment settings status %d body %s", readResponse.Code, readResponse.Body.String())
	}
	var saved apiAccountPaymentSettingsResponse
	if err := json.NewDecoder(readResponse.Body).Decode(&saved); err != nil {
		t.Fatalf("decode saved payment settings: %v", err)
	}
	if !saved.PaymentOptions[0].Enabled || saved.PaymentOptions[1].Enabled || saved.PaymentOptions[1].PaymentQRCodeDataURL == "" {
		t.Fatalf("unexpected saved payment settings: %+v", saved)
	}
}

func TestAPIAccountPaymentSettingsRouteRejectsTwoEnabledMethods(t *testing.T) {
	server := newTestServer(time.Now())
	session := createSession(t, server, "payment-owner-conflict", false)
	request := newJSONRequest(http.MethodPut, "/api/v1/me/api-payment-settings", `{
		"paymentWindowMinutes":10,
		"paymentOptions":[
			{"paymentMethod":"wechat","enabled":true,"paymentInstructions":"","paymentQrCodeDataUrl":"data:image/png;base64,d2VjaGF0"},
			{"paymentMethod":"alipay","enabled":true,"paymentInstructions":"","paymentQrCodeDataUrl":"data:image/png;base64,YWxpcGF5"}
		]
	}`)
	addAuth(request, session, "account-payment-conflict")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected validation failure, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, "VALIDATION_FAILED")
}

func TestAPIAccountPaymentSettingsRouteRequiresSessionAndCSRF(t *testing.T) {
	server := newTestServer(time.Now())

	readRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me/api-payment-settings", nil)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, readRequest)
	if readResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated read to fail, got %d body %s", readResponse.Code, readResponse.Body.String())
	}

	session := createSession(t, server, "payment-owner-csrf", false)
	updateRequest := newJSONRequest(http.MethodPut, "/api/v1/me/api-payment-settings", `{
		"paymentWindowMinutes":10,
		"paymentOptions":[
			{"paymentMethod":"wechat","enabled":true,"paymentInstructions":"","paymentQrCodeDataUrl":"data:image/png;base64,d2VjaGF0"},
			{"paymentMethod":"alipay","enabled":false,"paymentInstructions":""}
		]
	}`)
	addCookie(updateRequest, session.cookie)
	updateResponse := httptest.NewRecorder()
	server.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusForbidden {
		t.Fatalf("expected missing CSRF to fail, got %d body %s", updateResponse.Code, updateResponse.Body.String())
	}
	assertProblemCode(t, updateResponse, "CSRF_TOKEN_INVALID")
}
