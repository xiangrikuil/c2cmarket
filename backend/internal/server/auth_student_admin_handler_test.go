package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStudentRegistrationAdminRoutesAreAuditedContractBoundaries(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC))
	admin := createSession(t, server, "student-policy-admin", true)
	member := createSession(t, server, "student-policy-member", false)

	denied := httptest.NewRequest(http.MethodGet, "/api/v1/admin/student-registration", nil)
	addCookie(denied, member.cookie)
	deniedResponse := httptest.NewRecorder()
	server.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("non-admin registration read status=%d body=%s", deniedResponse.Code, deniedResponse.Body.String())
	}
	assertProblemCode(t, deniedResponse, "CAPABILITY_REQUIRED")

	read := httptest.NewRequest(http.MethodGet, "/api/v1/admin/student-registration", nil)
	addCookie(read, admin.cookie)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK || readResponse.Header().Get("ETag") != `"1"` {
		t.Fatalf("initial registration config status=%d etag=%q body=%s", readResponse.Code, readResponse.Header().Get("ETag"), readResponse.Body.String())
	}
	var initial adminStudentRegistrationResponse
	if err := json.NewDecoder(readResponse.Body).Decode(&initial); err != nil || initial.Enabled || initial.Version != 1 {
		t.Fatalf("unexpected initial registration config=%+v error=%v", initial, err)
	}

	createBody := `{"domain":"example.edu","institutionName":"Example University","enabled":true,"reason":"批准院校注册测试"}`
	create := newJSONRequest(http.MethodPost, "/api/v1/admin/student-institution-domains", createBody)
	addAuth(create, admin, "create-example-university")
	create.Header.Set("If-Match", `"0"`)
	createResponse := httptest.NewRecorder()
	server.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated || createResponse.Header().Get("ETag") != `"1"` {
		t.Fatalf("create domain status=%d etag=%q body=%s", createResponse.Code, createResponse.Header().Get("ETag"), createResponse.Body.String())
	}
	var created adminStudentInstitutionDomainResponse
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil || created.ID == "" || created.Domain != "example.edu" || !created.Enabled {
		t.Fatalf("unexpected created domain=%+v error=%v", created, err)
	}

	replay := newJSONRequest(http.MethodPost, "/api/v1/admin/student-institution-domains", createBody)
	addAuth(replay, admin, "create-example-university")
	replay.Header.Set("If-Match", `"0"`)
	replayResponse := httptest.NewRecorder()
	server.ServeHTTP(replayResponse, replay)
	var replayed adminStudentInstitutionDomainResponse
	if replayResponse.Code != http.StatusCreated {
		t.Fatalf("replay domain status=%d body=%s", replayResponse.Code, replayResponse.Body.String())
	}
	if err := json.NewDecoder(replayResponse.Body).Decode(&replayed); err != nil || replayed.ID != created.ID {
		t.Fatalf("domain create did not replay idempotently: first=%+v replay=%+v error=%v", created, replayed, err)
	}

	enable := newJSONRequest(http.MethodPatch, "/api/v1/admin/student-registration", `{"enabled":true,"expectedVersion":1,"reason":"完成受控验证后开启"}`)
	addAuth(enable, admin, "enable-student-registration")
	enable.Header.Set("If-Match", `"1"`)
	enableResponse := httptest.NewRecorder()
	server.ServeHTTP(enableResponse, enable)
	if enableResponse.Code != http.StatusOK || enableResponse.Header().Get("ETag") != `"2"` {
		t.Fatalf("enable registration status=%d etag=%q body=%s", enableResponse.Code, enableResponse.Header().Get("ETag"), enableResponse.Body.String())
	}

	stale := newJSONRequest(http.MethodPatch, "/api/v1/admin/student-registration", `{"enabled":false,"expectedVersion":1,"reason":"过期写入"}`)
	addAuth(stale, admin, "stale-student-registration")
	stale.Header.Set("If-Match", `"1"`)
	staleResponse := httptest.NewRecorder()
	server.ServeHTTP(staleResponse, stale)
	if staleResponse.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale registration update status=%d body=%s", staleResponse.Code, staleResponse.Body.String())
	}
	assertProblemCode(t, staleResponse, "VERSION_CONFLICT")

	publicConfig := httptest.NewRequest(http.MethodGet, "/api/v1/auth/email-registration/config", nil)
	publicResponse := httptest.NewRecorder()
	server.ServeHTTP(publicResponse, publicConfig)
	var public emailRegistrationConfigResponse
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("public registration config status=%d body=%s", publicResponse.Code, publicResponse.Body.String())
	}
	if err := json.NewDecoder(publicResponse.Body).Decode(&public); err != nil || !public.Enabled || len(public.Institutions) != 1 || public.Institutions[0].Domain != "example.edu" {
		t.Fatalf("unexpected public registration projection=%+v error=%v", public, err)
	}

	immutable := newJSONRequest(http.MethodPatch, "/api/v1/admin/student-institution-domains/"+created.ID, `{"domain":"other.edu","institutionName":"Other","enabled":true,"expectedVersion":1,"reason":"尝试变更域名"}`)
	addAuth(immutable, admin, "immutable-domain")
	immutable.Header.Set("If-Match", `"1"`)
	immutableResponse := httptest.NewRecorder()
	server.ServeHTTP(immutableResponse, immutable)
	if immutableResponse.Code != http.StatusBadRequest {
		t.Fatalf("domain identity mutation was accepted status=%d body=%s", immutableResponse.Code, immutableResponse.Body.String())
	}

	update := newJSONRequest(http.MethodPatch, "/api/v1/admin/student-institution-domains/"+created.ID, `{"institutionName":"Example University Renamed","enabled":false,"expectedVersion":1,"reason":"暂停院校入口"}`)
	addAuth(update, admin, "disable-example-university")
	update.Header.Set("If-Match", `"1"`)
	updateResponse := httptest.NewRecorder()
	server.ServeHTTP(updateResponse, update)
	if updateResponse.Code != http.StatusOK || updateResponse.Header().Get("ETag") != `"2"` {
		t.Fatalf("update domain status=%d etag=%q body=%s", updateResponse.Code, updateResponse.Header().Get("ETag"), updateResponse.Body.String())
	}
}

func TestStudentRecentPasswordLinuxDoLinkRotatesHTTPAuthSession(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 12, 14, 0, 0, 0, time.UTC))
	student := createStudentSession(t, server, "http-link-student")

	tooEarly := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/start?purpose=link_linuxdo", nil)
	addCookie(tooEarly, student.cookie)
	tooEarlyResponse := httptest.NewRecorder()
	server.ServeHTTP(tooEarlyResponse, tooEarly)
	if tooEarlyResponse.Code != http.StatusForbidden {
		t.Fatalf("link without recent password status=%d body=%s", tooEarlyResponse.Code, tooEarlyResponse.Body.String())
	}
	assertProblemCode(t, tooEarlyResponse, "RECENT_REAUTHENTICATION_REQUIRED")

	wrong := newJSONRequest(http.MethodPost, "/api/v1/auth/password/reauthenticate", `{"password":"WrongPassword1!"}`)
	addAuth(wrong, student, "")
	wrongResponse := httptest.NewRecorder()
	server.ServeHTTP(wrongResponse, wrong)
	if wrongResponse.Code != http.StatusUnauthorized {
		t.Fatalf("wrong reauthentication status=%d body=%s", wrongResponse.Code, wrongResponse.Body.String())
	}
	assertProblemCode(t, wrongResponse, "INVALID_CREDENTIALS")

	reauthenticate := newJSONRequest(http.MethodPost, "/api/v1/auth/password/reauthenticate", `{"password":"StudentPassword1!"}`)
	addAuth(reauthenticate, student, "")
	reauthenticateResponse := httptest.NewRecorder()
	server.ServeHTTP(reauthenticateResponse, reauthenticate)
	if reauthenticateResponse.Code != http.StatusNoContent {
		t.Fatalf("reauthenticate status=%d body=%s", reauthenticateResponse.Code, reauthenticateResponse.Body.String())
	}

	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/start?purpose=link_linuxdo", nil)
	addCookie(start, student.cookie)
	startResponse := httptest.NewRecorder()
	server.ServeHTTP(startResponse, start)
	if startResponse.Code != http.StatusOK {
		t.Fatalf("link start status=%d body=%s", startResponse.Code, startResponse.Body.String())
	}
	var startPayload oauthStartResponse
	if err := json.NewDecoder(startResponse.Body).Decode(&startPayload); err != nil || startPayload.AuthorizationURL == "" {
		t.Fatalf("decode link start payload=%+v error=%v", startPayload, err)
	}
	var stateCookie *http.Cookie
	for _, cookie := range startResponse.Result().Cookies() {
		if cookie.Name == oauthStateCookieName {
			stateCookie = cookie
			break
		}
	}
	if stateCookie == nil {
		t.Fatal("link start did not set OAuth state cookie")
	}

	callback := httptest.NewRequest(http.MethodGet, startPayload.AuthorizationURL, nil)
	addCookie(callback, student.cookie)
	callback.AddCookie(stateCookie)
	callbackResponse := httptest.NewRecorder()
	server.ServeHTTP(callbackResponse, callback)
	if callbackResponse.Code != http.StatusFound {
		t.Fatalf("link callback status=%d body=%s", callbackResponse.Code, callbackResponse.Body.String())
	}
	var replacementCookie string
	for _, cookie := range callbackResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName && cookie.Value != "" {
			replacementCookie = cookie.Value
		}
	}
	if replacementCookie == "" || replacementCookie == student.cookie {
		t.Fatalf("link callback did not rotate session cookie: old=%q new=%q", student.cookie, replacementCookie)
	}

	oldRead := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	addCookie(oldRead, student.cookie)
	oldReadResponse := httptest.NewRecorder()
	server.ServeHTTP(oldReadResponse, oldRead)
	if oldReadResponse.Code != http.StatusUnauthorized {
		t.Fatalf("old link session remained usable status=%d body=%s", oldReadResponse.Code, oldReadResponse.Body.String())
	}

	newRead := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	addCookie(newRead, replacementCookie)
	newReadResponse := httptest.NewRecorder()
	server.ServeHTTP(newReadResponse, newRead)
	if newReadResponse.Code != http.StatusOK {
		t.Fatalf("rotated session read status=%d body=%s", newReadResponse.Code, newReadResponse.Body.String())
	}
	var linked sessionResponse
	if err := json.NewDecoder(newReadResponse.Body).Decode(&linked); err != nil || !linked.User.LinuxDo.Bound || len(linked.User.Capabilities) != 6 {
		t.Fatalf("unexpected linked session=%+v error=%v", linked, err)
	}
}
