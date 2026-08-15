package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	app "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/module/evidence"
	"c2c-market/backend/internal/module/idempotency"
)

type evidenceRouteRepository struct {
	asset           evidence.Asset
	participantIDs  map[string]bool
	created         []evidence.Asset
	quarantineCalls int
}

func (r *evidenceRouteRepository) CreateReadyAssets(_ context.Context, assets []evidence.Asset) *domain.AppError {
	r.created = append(r.created, assets...)
	return nil
}

func (r *evidenceRouteRepository) AuthorizedAsset(_ context.Context, assetID, viewerUserID string, admin bool) (evidence.Asset, *domain.AppError) {
	if assetID == r.asset.ID && r.asset.Status == "ready" && (admin || r.participantIDs[viewerUserID]) {
		return r.asset, nil
	}
	return evidence.Asset{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Evidence not found", "图片证据不存在或当前账号无权查看。")
}

func (r *evidenceRouteRepository) QuarantineAssetWithIdempotency(_ context.Context, _ idempotency.Entry, input evidence.AdminQuarantineInput, now time.Time, build evidence.AdminQuarantineCompletionBuilder) (evidence.AdminQuarantineResult, idempotency.Completion, *domain.AppError) {
	r.quarantineCalls++
	if input.AssetID != r.asset.ID {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Evidence not found", "图片证据不存在或当前账号无权查看。")
	}
	if input.ExpectedVersion != r.asset.Version {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "图片证据状态已变化，请刷新后重试。")
	}
	if r.asset.Status != "ready" {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Evidence cannot be quarantined", "图片证据已隔离或已进入销毁流程。")
	}
	r.asset.Status = "quarantined"
	r.asset.Version++
	result := evidence.AdminQuarantineResult{
		ID:                   r.asset.ID,
		Status:               r.asset.Status,
		QuarantinedExpiresAt: now.Add(evidence.QuarantineRetention),
		Version:              r.asset.Version,
	}
	completion, appErr := build(result)
	if appErr != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, appErr
	}
	return result, completion, nil
}

func (*evidenceRouteRepository) ClaimDestroyCandidates(context.Context, time.Time, int) ([]evidence.DestroyCandidate, *domain.AppError) {
	return nil, nil
}

func (*evidenceRouteRepository) MarkDestroyed(context.Context, string, time.Time) *domain.AppError {
	return nil
}

func TestEvidenceUploadDisabledAndMultipartValidation(t *testing.T) {
	disabled := NewServer(app.NewService())
	session := createSession(t, disabled, "evidence-disabled", false)
	request := newEvidenceUploadRequest(t, "/api/v1/me/api-orders/order-1/dispute-evidence", map[string][]string{
		"kind": {evidence.KindAPIError}, "redactionConfirmed": {"true"},
	}, map[string][][]byte{"files": {evidenceRoutePNG(t)}})
	addAuth(request, session, "evidence-disabled")
	response := httptest.NewRecorder()
	disabled.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled evidence status %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, domain.CodeCapabilityUnavailable)

	repo := &evidenceRouteRepository{}
	service := evidence.NewService(repo, evidence.NewMemoryObjectStore(), time.Now)
	enabled := NewServer(app.NewService(), ServerOptions{EnableDevAuth: true, Evidence: service})
	session = createSession(t, enabled, "evidence-multipart", false)

	tests := []struct {
		name   string
		values map[string][]string
		files  map[string][][]byte
	}{
		{name: "unknown text part", values: map[string][]string{"kind": {evidence.KindAPIError}, "redactionConfirmed": {"true"}, "note": {"not allowed"}}, files: map[string][][]byte{"files": {evidenceRoutePNG(t)}}},
		{name: "singular file alias", values: map[string][]string{"kind": {evidence.KindAPIError}, "redactionConfirmed": {"true"}}, files: map[string][][]byte{"file": {evidenceRoutePNG(t)}}},
		{name: "duplicate kind", values: map[string][]string{"kind": {evidence.KindAPIError, evidence.KindPaymentResult}, "redactionConfirmed": {"true"}}, files: map[string][][]byte{"files": {evidenceRoutePNG(t)}}},
		{name: "duplicate redaction confirmation", values: map[string][]string{"kind": {evidence.KindAPIError}, "redactionConfirmed": {"true", "true"}}, files: map[string][][]byte{"files": {evidenceRoutePNG(t)}}},
		{name: "missing redaction confirmation", values: map[string][]string{"kind": {evidence.KindAPIError}}, files: map[string][][]byte{"files": {evidenceRoutePNG(t)}}},
		{name: "false redaction confirmation", values: map[string][]string{"kind": {evidence.KindAPIError}, "redactionConfirmed": {"false"}}, files: map[string][][]byte{"files": {evidenceRoutePNG(t)}}},
		{name: "too many files", values: map[string][]string{"kind": {evidence.KindAPIError}, "redactionConfirmed": {"true"}}, files: map[string][][]byte{"files": {evidenceRoutePNG(t), evidenceRoutePNG(t), evidenceRoutePNG(t), evidenceRoutePNG(t)}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := newEvidenceUploadRequest(t, "/api/v1/me/api-orders/order-1/dispute-evidence", tc.values, tc.files)
			addAuth(request, session, "evidence-multipart-"+strings.ReplaceAll(tc.name, " ", "-"))
			response := httptest.NewRecorder()
			enabled.ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d body %s", response.Code, response.Body.String())
			}
			assertProblemCode(t, response, domain.CodeValidationFailed)
		})
	}

	malformed := httptest.NewRequest(http.MethodPost, "/api/v1/me/api-orders/order-1/dispute-evidence", strings.NewReader("not multipart"))
	malformed.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	addAuth(malformed, session, "evidence-malformed")
	malformedResponse := httptest.NewRecorder()
	enabled.ServeHTTP(malformedResponse, malformed)
	if malformedResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed multipart status %d body %s", malformedResponse.Code, malformedResponse.Body.String())
	}
	assertProblemCode(t, malformedResponse, domain.CodeValidationFailed)

	valid := newEvidenceUploadRequest(t, "/api/v1/me/api-orders/order-1/dispute-evidence", map[string][]string{
		"kind": {evidence.KindAPIError}, "redactionConfirmed": {"true"},
	}, map[string][][]byte{"files": {evidenceRoutePNG(t)}})
	addAuth(valid, session, "evidence-valid")
	validResponse := httptest.NewRecorder()
	enabled.ServeHTTP(validResponse, valid)
	if validResponse.Code != http.StatusCreated || len(repo.created) != 1 || repo.created[0].Version != 1 {
		t.Fatalf("valid upload status %d created=%+v body %s", validResponse.Code, repo.created, validResponse.Body.String())
	}
	if got := validResponse.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("valid upload cache-control %q", got)
	}
}

func TestAdminEvidenceQuarantineGuardsReplayAndImmediateUnreadability(t *testing.T) {
	now := time.Date(2026, 8, 13, 13, 0, 0, 0, time.UTC)
	asset := evidence.Asset{
		ID:         "11111111-1111-4111-8111-111111111111",
		ObjectKey:  "private/secret-object-key.png",
		OutputMIME: "image/png",
		Status:     "ready",
		Version:    1,
	}
	objects := evidence.NewMemoryObjectStore()
	if err := objects.Put(t.Context(), asset.ObjectKey, asset.OutputMIME, []byte("private-evidence")); err != nil {
		t.Fatal(err)
	}
	repo := &evidenceRouteRepository{asset: asset, participantIDs: make(map[string]bool)}
	service := evidence.NewService(repo, objects, func() time.Time { return now })
	server := NewServer(app.NewService(), ServerOptions{EnableDevAuth: true, Evidence: service})
	admin := createSession(t, server, "evidence-quarantine-admin", true)
	member := createSession(t, server, "evidence-quarantine-member", false)
	path := "/api/v1/admin/dispute-evidence/" + asset.ID + "/quarantine"
	body := `{"reason":"包含未遮挡的 API 凭据"}`

	unauthenticated := newJSONRequest(http.MethodPost, path, body)
	unauthenticatedResponse := httptest.NewRecorder()
	server.ServeHTTP(unauthenticatedResponse, unauthenticated)
	if unauthenticatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status %d body %s", unauthenticatedResponse.Code, unauthenticatedResponse.Body.String())
	}

	missingCSRF := newJSONRequest(http.MethodPost, path, body)
	addCookie(missingCSRF, admin.cookie)
	missingCSRFResponse := httptest.NewRecorder()
	server.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status %d body %s", missingCSRFResponse.Code, missingCSRFResponse.Body.String())
	}

	nonAdmin := newJSONRequest(http.MethodPost, path, body)
	addAuth(nonAdmin, member, "evidence-quarantine-non-admin")
	nonAdmin.Header.Set("If-Match", `"1"`)
	nonAdminResponse := httptest.NewRecorder()
	server.ServeHTTP(nonAdminResponse, nonAdmin)
	if nonAdminResponse.Code != http.StatusForbidden {
		t.Fatalf("non-admin status %d body %s", nonAdminResponse.Code, nonAdminResponse.Body.String())
	}
	assertProblemCode(t, nonAdminResponse, domain.CodePermissionDenied)

	missingVersion := newJSONRequest(http.MethodPost, path, body)
	addAuth(missingVersion, admin, "evidence-quarantine-missing-version")
	missingVersionResponse := httptest.NewRecorder()
	server.ServeHTTP(missingVersionResponse, missingVersion)
	if missingVersionResponse.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing version status %d body %s", missingVersionResponse.Code, missingVersionResponse.Body.String())
	}
	assertProblemCode(t, missingVersionResponse, domain.CodePreconditionRequired)

	missingKey := newJSONRequest(http.MethodPost, path, body)
	addCookie(missingKey, admin.cookie)
	missingKey.Header.Set(csrfHeaderName, admin.csrf)
	missingKey.Header.Set("If-Match", `"1"`)
	missingKeyResponse := httptest.NewRecorder()
	server.ServeHTTP(missingKeyResponse, missingKey)
	if missingKeyResponse.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency key status %d body %s", missingKeyResponse.Code, missingKeyResponse.Body.String())
	}

	for name, testCase := range map[string]struct {
		body   string
		status int
		code   string
	}{
		"missing reason": {body: `{"reason":""}`, status: http.StatusUnprocessableEntity, code: domain.CodeValidationFailed},
		"unknown field":  {body: `{"reason":"包含未遮挡的 API 凭据","assetKey":"must-not-be-accepted"}`, status: http.StatusBadRequest, code: domain.CodeValidationFailed},
		"secret reason":  {body: `{"reason":"发现 api_key=sk-do-not-store-in-audit"}`, status: http.StatusUnprocessableEntity, code: domain.CodeSecretContentDetected},
	} {
		t.Run(name, func(t *testing.T) {
			invalid := newJSONRequest(http.MethodPost, path, testCase.body)
			addAuth(invalid, admin, "evidence-quarantine-invalid-"+strings.ReplaceAll(name, " ", "-"))
			invalid.Header.Set("If-Match", `"1"`)
			invalidResponse := httptest.NewRecorder()
			server.ServeHTTP(invalidResponse, invalid)
			if invalidResponse.Code != testCase.status || invalidResponse.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatalf("invalid request status %d headers=%v body %s", invalidResponse.Code, invalidResponse.Header(), invalidResponse.Body.String())
			}
			assertProblemCode(t, invalidResponse, testCase.code)
		})
	}

	request := newJSONRequest(http.MethodPost, path, body)
	addAuth(request, admin, "evidence-quarantine-success")
	request.Header.Set("If-Match", `"1"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"2"` || response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("quarantine status %d headers=%v body %s", response.Code, response.Header(), response.Body.String())
	}
	responseBody := response.Body.String()
	var result evidence.AdminQuarantineResult
	if err := json.Unmarshal([]byte(responseBody), &result); err != nil {
		t.Fatalf("decode quarantine response: %v", err)
	}
	if result.ID != asset.ID || result.Status != "quarantined" || result.Version != 2 || !result.QuarantinedExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("unexpected quarantine result: %+v", result)
	}
	for _, forbidden := range []string{"objectKey", "secret-object-key", "private-evidence", "reason", "API 凭据"} {
		if strings.Contains(responseBody, forbidden) {
			t.Fatalf("quarantine response leaked %q: %s", forbidden, responseBody)
		}
	}

	replay := newJSONRequest(http.MethodPost, path, body)
	addAuth(replay, admin, "evidence-quarantine-success")
	replay.Header.Set("If-Match", `"1"`)
	replayResponse := httptest.NewRecorder()
	server.ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusOK || replayResponse.Header().Get("ETag") != `"2"` || replayResponse.Body.String() != responseBody || repo.quarantineCalls != 1 {
		t.Fatalf("unstable replay status=%d headers=%v calls=%d body=%s", replayResponse.Code, replayResponse.Header(), repo.quarantineCalls, replayResponse.Body.String())
	}

	reused := newJSONRequest(http.MethodPost, path, `{"reason":"另一项违规内容"}`)
	addAuth(reused, admin, "evidence-quarantine-success")
	reused.Header.Set("If-Match", `"1"`)
	reusedResponse := httptest.NewRecorder()
	server.ServeHTTP(reusedResponse, reused)
	if reusedResponse.Code != http.StatusConflict {
		t.Fatalf("reused key status %d body %s", reusedResponse.Code, reusedResponse.Body.String())
	}
	assertProblemCode(t, reusedResponse, domain.CodeIdempotencyKeyReused)

	stale := newJSONRequest(http.MethodPost, path, body)
	addAuth(stale, admin, "evidence-quarantine-stale")
	stale.Header.Set("If-Match", `"1"`)
	staleResponse := httptest.NewRecorder()
	server.ServeHTTP(staleResponse, stale)
	if staleResponse.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale version status %d body %s", staleResponse.Code, staleResponse.Body.String())
	}
	assertProblemCode(t, staleResponse, domain.CodeVersionConflict)

	content := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dispute-evidence/"+asset.ID+"/content", nil)
	addCookie(content, admin.cookie)
	contentResponse := httptest.NewRecorder()
	server.ServeHTTP(contentResponse, content)
	if contentResponse.Code != http.StatusNotFound || strings.Contains(contentResponse.Body.String(), "secret-object-key") {
		t.Fatalf("quarantined evidence remained readable: %d body %s", contentResponse.Code, contentResponse.Body.String())
	}
	assertProblemCode(t, contentResponse, domain.CodeObjectNotFound)
}

func TestEvidenceReferenceResponseIncludesVersion(t *testing.T) {
	items := toEvidenceReferenceResponses([]evidence.Reference{{
		PublicAsset: evidence.PublicAsset{
			ID: "asset-version", Kind: evidence.KindAPIError, MIME: "image/png",
			ByteSize: 32, Width: 4, Height: 4, CreatedAt: time.Date(2026, 8, 13, 14, 0, 0, 0, time.UTC), Version: 7,
		},
		Visibility: evidence.VisibilityParticipantsAdmin,
		Usage:      evidence.UsageMessage,
		SourceType: evidence.SourceDisputeMessage,
		SourceID:   "message-version",
	}}, true)
	if len(items) != 1 || items[0].Version != 7 || items[0].ContentPath != "/api/v1/admin/dispute-evidence/asset-version/content" {
		t.Fatalf("unexpected evidence response: %+v", items)
	}
}

func TestEvidenceReferenceFilteringPreservesSourceVisibility(t *testing.T) {
	participants := evidence.Reference{PublicAsset: evidence.PublicAsset{ID: "participants"}, UploaderUserID: "buyer", Visibility: evidence.VisibilityParticipantsAdmin}
	buyerSupplement := evidence.Reference{PublicAsset: evidence.PublicAsset{ID: "buyer-supplement"}, UploaderUserID: "buyer", Visibility: evidence.VisibilitySubmitterAdmin}
	buyerAppeal := evidence.Reference{PublicAsset: evidence.PublicAsset{ID: "buyer-appeal"}, UploaderUserID: "buyer", Visibility: evidence.VisibilityAppellantAdmin}
	sellerSupplement := evidence.Reference{PublicAsset: evidence.PublicAsset{ID: "seller-supplement"}, UploaderUserID: "seller", Visibility: evidence.VisibilitySubmitterAdmin}
	items := []evidence.Reference{participants, buyerSupplement, buyerAppeal, sellerSupplement}

	assertIDs := func(t *testing.T, got []evidence.Reference, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("unexpected evidence count got=%v want=%v", got, want)
		}
		for index := range want {
			if got[index].ID != want[index] {
				t.Fatalf("unexpected evidence at %d got=%s want=%s", index, got[index].ID, want[index])
			}
		}
	}
	assertIDs(t, filterEvidenceForUser(items, "buyer"), "participants", "buyer-supplement", "buyer-appeal")
	assertIDs(t, filterEvidenceForUser(items, "seller"), "participants", "seller-supplement")
}

func TestEvidenceContentReadAuthorizationMatrix(t *testing.T) {
	objects := evidence.NewMemoryObjectStore()
	asset := evidence.Asset{ID: "asset-route-1", ObjectKey: "private/asset-route-1.png", OutputMIME: "image/png", Status: "ready"}
	if err := objects.Put(t.Context(), asset.ObjectKey, asset.OutputMIME, []byte("private-evidence")); err != nil {
		t.Fatal(err)
	}
	repo := &evidenceRouteRepository{asset: asset, participantIDs: make(map[string]bool)}
	service := evidence.NewService(repo, objects, time.Now)
	server := NewServer(app.NewService(), ServerOptions{EnableDevAuth: true, Evidence: service})
	buyer := createSession(t, server, "evidence-buyer", false)
	seller := createSession(t, server, "evidence-seller", false)
	admin := createSession(t, server, "evidence-admin", true)
	outsider := createSession(t, server, "evidence-outsider", false)
	repo.participantIDs[buyer.userID] = true
	repo.participantIDs[seller.userID] = true

	for name, session := range map[string]testSession{"buyer": buyer, "seller": seller} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/me/dispute-evidence/"+asset.ID+"/content", nil)
			addCookie(request, session.cookie)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			assertEvidenceContentResponse(t, response)
		})
	}

	adminRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dispute-evidence/"+asset.ID+"/content", nil)
	addCookie(adminRequest, admin.cookie)
	adminResponse := httptest.NewRecorder()
	server.ServeHTTP(adminResponse, adminRequest)
	assertEvidenceContentResponse(t, adminResponse)

	outsiderRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me/dispute-evidence/"+asset.ID+"/content", nil)
	addCookie(outsiderRequest, outsider.cookie)
	outsiderResponse := httptest.NewRecorder()
	server.ServeHTTP(outsiderResponse, outsiderRequest)
	if outsiderResponse.Code != http.StatusNotFound {
		t.Fatalf("outsider status %d body %s", outsiderResponse.Code, outsiderResponse.Body.String())
	}
	assertProblemCode(t, outsiderResponse, domain.CodeObjectNotFound)

	nonAdminRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dispute-evidence/"+asset.ID+"/content", nil)
	addCookie(nonAdminRequest, buyer.cookie)
	nonAdminResponse := httptest.NewRecorder()
	server.ServeHTTP(nonAdminResponse, nonAdminRequest)
	if nonAdminResponse.Code != http.StatusForbidden {
		t.Fatalf("non-admin status %d body %s", nonAdminResponse.Code, nonAdminResponse.Body.String())
	}
	assertProblemCode(t, nonAdminResponse, domain.CodePermissionDenied)
}

func assertEvidenceContentResponse(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != http.StatusOK || response.Body.String() != "private-evidence" {
		t.Fatalf("content status %d body %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" ||
		response.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("unexpected evidence headers: %v", response.Header())
	}
}

func newEvidenceUploadRequest(t *testing.T, path string, values map[string][]string, files map[string][][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, items := range values {
		for _, value := range items {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	for name, items := range files {
		for index, value := range items {
			part, err := writer.CreateFormFile(name, "evidence-"+string(rune('a'+index))+".png")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write(value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func evidenceRoutePNG(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
