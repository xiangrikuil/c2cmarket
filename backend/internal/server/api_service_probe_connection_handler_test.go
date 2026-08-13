package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	app "c2c-market/backend/internal/module/core"
)

func TestOwnerAPIServiceProbeConnectionRouteSupportsRebindAndUnbind(t *testing.T) {
	now := time.Date(2026, 8, 8, 8, 0, 0, 0, time.UTC)
	handler := NewServer(app.NewServiceWithClock(func() time.Time { return now }), ServerOptions{EnableDevAuth: true})
	owner := createSession(t, handler, "probe-binding-owner", false)
	contact := createContactMethod(t, handler, owner, "telegram", "Owner", "@probe_binding_owner")
	created := createAPIService(t, handler, owner, contact.ID, "create-probe-binding-service")

	rebind := newJSONRequest(http.MethodPatch, "/api/v1/owner/api-services/"+created.ID+"/probe-connection", `{"probeConnectionId":"00000000-0000-0000-0000-000000000822"}`)
	addAuth(rebind, owner, "probe-rebind")
	rebind.Header.Set("If-Match", `"`+strconv.FormatInt(created.Version, 10)+`"`)
	rebindResponse := httptest.NewRecorder()
	handler.ServeHTTP(rebindResponse, rebind)
	if rebindResponse.Code != http.StatusOK {
		t.Fatalf("rebind status=%d body=%s", rebindResponse.Code, rebindResponse.Body.String())
	}
	var rebound apiServiceResponse
	if err := json.NewDecoder(rebindResponse.Body).Decode(&rebound); err != nil {
		t.Fatalf("decode rebound service: %v", err)
	}
	if rebound.ProbeConnectionID != "00000000-0000-0000-0000-000000000822" || !rebound.ProbeReady || rebound.Version != created.Version+1 {
		t.Fatalf("unexpected rebound service: %+v", rebound)
	}
	if rebindResponse.Header().Get("ETag") != `"`+strconv.FormatInt(rebound.Version, 10)+`"` {
		t.Fatalf("unexpected rebound ETag %q", rebindResponse.Header().Get("ETag"))
	}

	unbind := newJSONRequest(http.MethodPatch, "/api/v1/owner/api-services/"+created.ID+"/probe-connection", `{"probeConnectionId":""}`)
	addAuth(unbind, owner, "probe-unbind")
	unbind.Header.Set("If-Match", `"`+strconv.FormatInt(rebound.Version, 10)+`"`)
	unbindResponse := httptest.NewRecorder()
	handler.ServeHTTP(unbindResponse, unbind)
	if unbindResponse.Code != http.StatusOK {
		t.Fatalf("unbind status=%d body=%s", unbindResponse.Code, unbindResponse.Body.String())
	}
	var unbound apiServiceResponse
	if err := json.NewDecoder(unbindResponse.Body).Decode(&unbound); err != nil {
		t.Fatalf("decode unbound service: %v", err)
	}
	if unbound.ProbeConnectionID != "" || unbound.ProbeReady || unbound.IsOrderable {
		t.Fatalf("unexpected unbound service: %+v", unbound)
	}
}

func TestOwnerAPIServiceProbeConnectionRouteRequiresFieldAndIfMatch(t *testing.T) {
	handler := NewServer(app.NewServiceWithClock(time.Now), ServerOptions{EnableDevAuth: true})
	owner := createSession(t, handler, "probe-binding-validation-owner", false)

	missingField := newJSONRequest(http.MethodPatch, "/api/v1/owner/api-services/service-1/probe-connection", `{}`)
	addAuth(missingField, owner, "unused")
	missingField.Header.Set("If-Match", `"1"`)
	missingFieldResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingFieldResponse, missingField)
	if missingFieldResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing field status=%d body=%s", missingFieldResponse.Code, missingFieldResponse.Body.String())
	}

	missingVersion := newJSONRequest(http.MethodPatch, "/api/v1/owner/api-services/service-1/probe-connection", `{"probeConnectionId":""}`)
	addAuth(missingVersion, owner, "unused")
	missingVersionResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingVersionResponse, missingVersion)
	if missingVersionResponse.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing If-Match status=%d body=%s", missingVersionResponse.Code, missingVersionResponse.Body.String())
	}
}
