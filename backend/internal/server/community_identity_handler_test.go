package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAdminCommunityIdentityRoutesGrantListAndRevoke(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	adminSession := createSession(t, server, "community-identity-admin", true)
	memberSession := createSession(t, server, "community-identity-member", false)
	path := "/api/v1/admin/users/" + memberSession.userID + "/community-identities"

	grant := newJSONRequest(http.MethodPost, path, `{"identityType":"BETA_CONTRIBUTOR","reason":"参与内测并提交有效反馈"}`)
	addAuth(grant, adminSession, "community-identity-grant")
	grantResponse := httptest.NewRecorder()
	server.ServeHTTP(grantResponse, grant)
	if grantResponse.Code != http.StatusOK {
		t.Fatalf("grant status %d body %s", grantResponse.Code, grantResponse.Body.String())
	}
	grantBody := grantResponse.Body.String()
	var granted adminCommunityIdentityDTO
	if err := json.Unmarshal([]byte(grantBody), &granted); err != nil {
		t.Fatalf("decode grant response: %v", err)
	}
	if granted.Code != "BETA_CONTRIBUTOR" || granted.Source != "ADMIN" || granted.GrantReason == "" || granted.GrantedBy != adminSession.userID {
		t.Fatalf("unexpected granted identity: %+v", granted)
	}

	replay := newJSONRequest(http.MethodPost, path, `{"identityType":"BETA_CONTRIBUTOR","reason":"参与内测并提交有效反馈"}`)
	addAuth(replay, adminSession, "community-identity-grant")
	replayResponse := httptest.NewRecorder()
	server.ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusOK || replayResponse.Body.String() != grantBody {
		t.Fatalf("grant replay status %d body %s", replayResponse.Code, replayResponse.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(list, adminSession.cookie)
	listResponse := httptest.NewRecorder()
	server.ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status %d body %s", listResponse.Code, listResponse.Body.String())
	}
	var listed adminCommunityIdentityListResponse
	if err := json.NewDecoder(listResponse.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != granted.ID {
		t.Fatalf("unexpected identity list: %+v", listed)
	}

	revokePath := path + "/BETA_CONTRIBUTOR/revoke"
	revoke := newJSONRequest(http.MethodPost, revokePath, `{"reason":"内测阶段结束后撤销测试身份"}`)
	addAuth(revoke, adminSession, "community-identity-revoke")
	revokeResponse := httptest.NewRecorder()
	server.ServeHTTP(revokeResponse, revoke)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("revoke status %d body %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	var revoked adminCommunityIdentityDTO
	if err := json.NewDecoder(revokeResponse.Body).Decode(&revoked); err != nil {
		t.Fatalf("decode revoke response: %v", err)
	}
	if revoked.RevokedAt == nil || revoked.RevokedBy != adminSession.userID || revoked.RevokeReason == "" {
		t.Fatalf("unexpected revoked identity: %+v", revoked)
	}

	publicList := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(publicList, memberSession.cookie)
	publicListResponse := httptest.NewRecorder()
	server.ServeHTTP(publicListResponse, publicList)
	if publicListResponse.Code != http.StatusForbidden {
		t.Fatalf("member admin list status %d body %s", publicListResponse.Code, publicListResponse.Body.String())
	}
}
