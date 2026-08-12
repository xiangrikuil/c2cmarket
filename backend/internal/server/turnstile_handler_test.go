package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/core"
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

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
