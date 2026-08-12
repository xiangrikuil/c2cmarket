package server

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/module/operationaudit"
	"c2c-market/backend/internal/observability"
	"c2c-market/backend/internal/platform/turnstile"
)

type recordingTurnstileVerifier struct {
	inputs []turnstile.Verification
	err    error
}

func (verifier *recordingTurnstileVerifier) Verify(_ context.Context, input turnstile.Verification) error {
	verifier.inputs = append(verifier.inputs, input)
	return verifier.err
}

func TestProtectedAuthRoutesRejectFailedTurnstileBeforeExistingBehavior(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   string
		action string
	}{
		{name: "password login", path: "/api/v1/auth/password/login", body: `{"username":"missing","password":"password","turnstileToken":"bad-token"}`, action: turnstileActionPasswordLogin},
		{name: "student signup", path: "/api/v1/auth/email-registration/start", body: `{"email":"student@example.edu","turnstileToken":"bad-token"}`, action: turnstileActionStudentSignup},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifier := &recordingTurnstileVerifier{err: errors.New("provider detail must stay private")}
			handler := NewServer(core.NewService(), ServerOptions{TurnstileVerifier: verifier})
			request := newJSONRequest(http.MethodPost, test.path, test.body)
			request.RemoteAddr = "203.0.113.10:4321"
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d body %s", response.Code, response.Body.String())
			}
			body := response.Body.String()
			assertProblemCode(t, response, domain.CodeTurnstileVerificationFailed)
			if len(verifier.inputs) != 1 || verifier.inputs[0].Action != test.action || verifier.inputs[0].RemoteIP != "203.0.113.10" {
				t.Fatalf("unexpected verification input: %+v", verifier.inputs)
			}
			if body == "" || containsAny(body, "provider detail", "bad-token") {
				t.Fatalf("response disclosed verification detail: %s", body)
			}
		})
	}
}

func TestProtectedAuthRoutesContinueAfterValidTurnstile(t *testing.T) {
	verifier := &recordingTurnstileVerifier{}
	handler := NewServer(core.NewService(), ServerOptions{TurnstileVerifier: verifier})

	login := newJSONRequest(http.MethodPost, "/api/v1/auth/password/login", `{"username":"missing","password":"password","turnstileToken":"valid-token"}`)
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected existing invalid-credentials response, got %d body %s", loginResponse.Code, loginResponse.Body.String())
	}
	assertProblemCode(t, loginResponse, domain.CodeInvalidCredentials)

	registration := newJSONRequest(http.MethodPost, "/api/v1/auth/email-registration/start", `{"email":"student@example.edu","turnstileToken":"valid-token"}`)
	registrationResponse := httptest.NewRecorder()
	handler.ServeHTTP(registrationResponse, registration)
	if registrationResponse.Code != http.StatusForbidden {
		t.Fatalf("expected existing disabled-registration response, got %d body %s", registrationResponse.Code, registrationResponse.Body.String())
	}
	assertProblemCode(t, registrationResponse, domain.CodeEmailRegistrationDisabled)
}

func TestAuthenticationFailuresUseRedactedRuntimeTelemetryWithoutBusinessAuditRows(t *testing.T) {
	service := core.NewService()
	var failureLogs bytes.Buffer
	metrics := observability.New(observability.Sources{FailureLogger: log.New(&failureLogs, "", 0)})
	verifier := &recordingTurnstileVerifier{err: errors.New("private provider failure detail")}
	handler := NewServer(service, ServerOptions{TurnstileVerifier: verifier, Metrics: metrics})

	turnstileRequest := newJSONRequest(http.MethodPost, "/api/v1/auth/password/login", `{"username":"private-user@example.test","password":"private-password","turnstileToken":"private-turnstile-token"}`)
	turnstileRequest.Header.Set(requestIDHeader, "req-auth-turnstile")
	turnstileResponse := httptest.NewRecorder()
	handler.ServeHTTP(turnstileResponse, turnstileRequest)
	assertProblemCode(t, turnstileResponse, domain.CodeTurnstileVerificationFailed)

	verifier.err = nil
	credentialsRequest := newJSONRequest(http.MethodPost, "/api/v1/auth/password/login", `{"username":"private-user@example.test","password":"private-password","turnstileToken":"accepted-token"}`)
	credentialsRequest.Header.Set(requestIDHeader, "req-auth-credentials")
	credentialsResponse := httptest.NewRecorder()
	handler.ServeHTTP(credentialsResponse, credentialsRequest)
	assertProblemCode(t, credentialsResponse, domain.CodeInvalidCredentials)

	logOutput := failureLogs.String()
	for _, expected := range []string{
		`"request_id":"req-auth-turnstile"`, `"result_code":"TURNSTILE_VERIFICATION_FAILED"`,
		`"request_id":"req-auth-credentials"`, `"result_code":"INVALID_CREDENTIALS"`,
		`"route_key":"auth_password_login"`, `"actor_kind":"anonymous"`,
	} {
		if !strings.Contains(logOutput, expected) {
			t.Errorf("failure log is missing %q: %s", expected, logOutput)
		}
	}
	for _, forbidden := range []string{
		"private-user@example.test", "private-password", "private-turnstile-token", "accepted-token", "private provider failure detail",
	} {
		if strings.Contains(logOutput, forbidden) {
			t.Fatalf("failure log disclosed sensitive value %q: %s", forbidden, logOutput)
		}
	}

	metricsResponse := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(metricsResponse, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, expected := range []string{
		`c2c_market_operations_security_failures_total{category="human_verification",result="TURNSTILE_VERIFICATION_FAILED",route="auth_password_login"} 1`,
		`c2c_market_operations_security_failures_total{category="authentication",result="INVALID_CREDENTIALS",route="auth_password_login"} 1`,
	} {
		if !strings.Contains(metricsResponse.Body.String(), expected) {
			t.Errorf("metrics output is missing %q", expected)
		}
	}

	page, appErr := service.AdminOperationAuditLogs(context.Background(), auth.User{IsAdmin: true}, operationaudit.Filter{})
	if appErr != nil {
		t.Fatalf("read operation audit after failures: %v", appErr)
	}
	if len(page.Items) != 0 {
		t.Fatalf("authentication failures created persistent business audit entries: %+v", page.Items)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
