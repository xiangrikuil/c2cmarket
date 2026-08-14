package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestPasswordResetStartReturnsGenericAcceptedResponse(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC))

	request := newJSONRequest(http.MethodPost, "/api/v1/auth/password-reset/start", `{"email":"missing@example.edu","turnstileToken":"test-token"}`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("start status=%d body=%s", response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if len(payload) != 1 || payload["accepted"] != true {
		t.Fatalf("start response exposed unexpected data: %#v", payload)
	}
	if cookies := response.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("start created cookies: %+v", cookies)
	}
}

func TestPasswordResetConfirmUsesStableInvalidChallengeError(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 14, 12, 30, 0, 0, time.UTC))
	request := newJSONRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", `{"email":"missing@example.edu","code":"123456","newPassword":"New-password-2!"}`)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("confirm status=%d body=%s", response.Code, response.Body.String())
	}
	var problem problemDetails
	if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
		t.Fatalf("decode confirm problem: %v", err)
	}
	if problem.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("confirm code=%q body=%s", problem.Code, response.Body.String())
	}
	if cookies := response.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("confirm created cookies: %+v", cookies)
	}
}

func TestPasswordResetHandlersRejectUnknownJSONFields(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC))
	request := newJSONRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", `{"email":"missing@example.edu","code":"123456","newPassword":"New-password-2!","unexpected":true}`)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("confirm status=%d body=%s", response.Code, response.Body.String())
	}
}
