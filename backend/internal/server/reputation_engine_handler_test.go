package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	app "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"
)

type reputationRouteService struct {
	ApplicationService
	publicUsername  string
	publicScope     string
	myUserID        string
	adminAuditID    string
	adminAuditLimit int
	recalculatedID  string
	recalculateAll  bool
	sourceReadType  string
	sourceReadID    string
	sourceUpdate    reputation.UpdateSourceAuthorVerificationInput
	sanctionReadID  string
	sanctionApply   reputation.ApplyAPIOrderSanctionInput
}

func (s *reputationRouteService) ReputationRules() reputation.RuleSet {
	return reputation.CurrentRules()
}

func (s *reputationRouteService) PublicUserReputation(_ context.Context, username, scope string) ([]reputation.ReputationSnapshot, *domain.AppError) {
	s.publicUsername = username
	s.publicScope = scope
	return []reputation.ReputationSnapshot{
		testReputationSnapshot("11111111-1111-4111-8111-111111111111", reputation.RoleBuyer, scope),
		testReputationSnapshot("11111111-1111-4111-8111-111111111111", reputation.RoleSeller, scope),
	}, nil
}

func (s *reputationRouteService) MyReputation(_ context.Context, user auth.User) ([]reputation.ReputationSnapshot, *domain.AppError) {
	s.myUserID = user.ID
	return []reputation.ReputationSnapshot{
		testReputationSnapshot(user.ID, reputation.RoleBuyer, reputation.ScopeOverall),
	}, nil
}

func (s *reputationRouteService) MyActiveReputationRestrictions(_ context.Context, user auth.User) ([]reputation.UserRestriction, *domain.AppError) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	endsAt := now.Add(30 * 24 * time.Hour)
	return []reputation.UserRestriction{{
		ID:                     "77777777-7777-4777-8777-777777777777",
		UserID:                 user.ID,
		RestrictionType:        reputation.RestrictionTypeAPIOrderRemedyOverdue,
		RoleScope:              reputation.RoleSeller,
		ActionCode:             reputation.ActionAPIServicePublish,
		ReasonCode:             reputation.ReasonCodeAPIOrderRemedyOverdue,
		PublicReason:           "暂停新接单、发布和恢复；已成立订单仍可继续履约。",
		InternalReason:         "不得出现在自助响应中的内部说明",
		StartsAt:               now,
		EndsAt:                 &endsAt,
		SourceDisputeOutcomeID: "88888888-8888-4888-8888-888888888888",
		SourceDisputeRemedyID:  "99999999-9999-4999-8999-999999999999",
		CreatedByAdminID:       "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		CreatedAt:              now,
		UpdatedAt:              now,
		Version:                1,
	}}, nil
}

func (s *reputationRouteService) AdminAPIOrderSanctionRecommendation(_ context.Context, user auth.User, disputeCaseID string) (reputation.APIOrderSanctionRecommendation, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.APIOrderSanctionRecommendation{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	s.sanctionReadID = disputeCaseID
	return reputation.APIOrderSanctionRecommendation{
		Eligible:                 true,
		ReasonCode:               "eligible",
		DisputeCaseID:            disputeCaseID,
		RemedyID:                 "99999999-9999-4999-8999-999999999999",
		OutcomeID:                "88888888-8888-4888-8888-888888888888",
		SubjectUserID:            "77777777-7777-4777-8777-777777777777",
		ConfirmedBreaches180Days: 2,
		RecommendedDays:          30,
		SubjectUserVersion:       4,
	}, nil
}

func (s *reputationRouteService) AdminApplyAPIOrderSanctionWithIdempotency(
	_ context.Context,
	user auth.User,
	_, _, _ string,
	input reputation.ApplyAPIOrderSanctionInput,
	buildCompletion reputation.GovernanceCompletionBuilder,
) (idempotency.Completion, *domain.AppError) {
	if !user.IsAdmin {
		return idempotency.Completion{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	s.sanctionApply = input
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	endsAt := now.Add(30 * 24 * time.Hour)
	restriction := reputation.UserRestriction{
		ID:                     "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		UserID:                 "77777777-7777-4777-8777-777777777777",
		RestrictionType:        reputation.RestrictionTypeAPIOrderRemedyOverdue,
		RoleScope:              reputation.RoleSeller,
		ActionCode:             reputation.ActionAPIServicePublish,
		ReasonCode:             reputation.ReasonCodeAPIOrderRemedyOverdue,
		PublicReason:           "暂停新接单、发布和恢复；已成立订单仍可继续履约。",
		InternalReason:         input.InternalReason,
		StartsAt:               now,
		EndsAt:                 &endsAt,
		SourceDisputeOutcomeID: "88888888-8888-4888-8888-888888888888",
		SourceDisputeRemedyID:  "99999999-9999-4999-8999-999999999999",
		CreatedByAdminID:       user.ID,
		CreatedAt:              now,
		UpdatedAt:              now,
		Version:                1,
		UserVersion:            input.ExpectedUserVersion + 1,
	}
	return buildCompletion(reputation.GovernanceMutationResult{Restriction: &restriction})
}

func (s *reputationRouteService) AdminUserReputation(_ context.Context, user auth.User, userID string, historyLimit int) (reputation.AdminReputationAudit, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.AdminReputationAudit{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	s.adminAuditID = userID
	s.adminAuditLimit = historyLimit
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	return reputation.AdminReputationAudit{
		UserID:      userID,
		RuleVersion: reputation.RuleVersion,
		Items: []reputation.ReputationSnapshot{
			testReputationSnapshot(userID, reputation.RoleBuyer, reputation.ScopeOverall),
		},
		History: []reputation.ReputationHistory{},
		Restrictions: []reputation.UserRestriction{{
			ID:        "44444444-4444-4444-8444-444444444444",
			UserID:    userID,
			StartsAt:  now,
			CreatedAt: now,
			UpdatedAt: now,
		}},
		Outcomes: []reputation.DisputeOutcome{{
			ID:        "55555555-5555-4555-8555-555555555555",
			DecidedAt: now,
			CreatedAt: now,
			UpdatedAt: now,
		}},
		Appeals: []reputation.ReputationAppeal{{
			ID:              "66666666-6666-4666-8666-666666666666",
			AppellantUserID: userID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}},
		SourceAuthorVerifications: []reputation.SourceAuthorVerificationAudit{},
	}, nil
}

func (s *reputationRouteService) AdminRecalculateUserReputation(_ context.Context, user auth.User, userID string) (reputation.RecalculationResult, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.RecalculationResult{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	s.recalculatedID = userID
	return reputation.RecalculationResult{RequestedUsers: 1, RebuiltStates: 6, CompletedAt: time.Now().UTC()}, nil
}

func (s *reputationRouteService) AdminRecalculateAllReputation(_ context.Context, user auth.User) (reputation.RecalculationResult, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.RecalculationResult{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	s.recalculateAll = true
	return reputation.RecalculationResult{RequestedUsers: 2, RebuiltStates: 12, CompletedAt: time.Now().UTC()}, nil
}

func (s *reputationRouteService) AdminSourceAuthorVerification(
	_ context.Context,
	user auth.User,
	resourceType string,
	resourceID string,
) (reputation.SourceAuthorVerificationAudit, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.SourceAuthorVerificationAudit{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	s.sourceReadType = resourceType
	s.sourceReadID = resourceID
	return reputation.SourceAuthorVerificationAudit{
		Verification: reputation.SourceAuthorVerification{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Status:       reputation.SourceVerificationNotSubmitted,
			Version:      0,
		},
		Events: []reputation.SourceAuthorVerificationEvent{},
	}, nil
}

func (s *reputationRouteService) AdminUpdateSourceAuthorVerification(
	_ context.Context,
	user auth.User,
	input reputation.UpdateSourceAuthorVerificationInput,
) (reputation.SourceAuthorVerificationAudit, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.SourceAuthorVerificationAudit{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	s.sourceUpdate = input
	return reputation.SourceAuthorVerificationAudit{
		Verification: reputation.SourceAuthorVerification{
			ResourceType: input.ResourceType,
			ResourceID:   input.ResourceID,
			Status:       input.Status,
			Version:      input.ExpectedVersion + 1,
		},
		Events: []reputation.SourceAuthorVerificationEvent{},
	}, nil
}

func TestReputationRulesAndPublicScopeRoutes(t *testing.T) {
	t.Parallel()

	service := &reputationRouteService{ApplicationService: app.NewService()}
	server := NewServer(service)

	rulesRequest := httptest.NewRequest(http.MethodGet, "/api/v1/reputation/rules", nil)
	rulesResponse := httptest.NewRecorder()
	server.ServeHTTP(rulesResponse, rulesRequest)
	if rulesResponse.Code != http.StatusOK {
		t.Fatalf("rules status %d body %s", rulesResponse.Code, rulesResponse.Body.String())
	}
	var rulesBody reputationRulesResponse
	if err := json.NewDecoder(rulesResponse.Body).Decode(&rulesBody); err != nil {
		t.Fatalf("decode rules: %v", err)
	}
	if rulesBody.Rules.Version != reputation.RuleVersion || rulesBody.Rules.BayesianPriorWeight != 5 {
		t.Fatalf("unexpected rules: %#v", rulesBody.Rules)
	}

	publicRequest := httptest.NewRequest(http.MethodGet, "/api/v1/users/alice/reputation?scope=api", nil)
	publicResponse := httptest.NewRecorder()
	server.ServeHTTP(publicResponse, publicRequest)
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("public reputation status %d body %s", publicResponse.Code, publicResponse.Body.String())
	}
	if service.publicUsername != "alice" || service.publicScope != reputation.ScopeAPI {
		t.Fatalf("unexpected public inputs: username=%q scope=%q", service.publicUsername, service.publicScope)
	}
	var publicBody reputationScopeResponse
	if err := json.NewDecoder(publicResponse.Body).Decode(&publicBody); err != nil {
		t.Fatalf("decode public reputation: %v", err)
	}
	if publicBody.Scope != reputation.ScopeAPI || len(publicBody.Reputations) != 2 {
		t.Fatalf("unexpected public response: %#v", publicBody)
	}
}

func TestMyReputationRequiresSession(t *testing.T) {
	t.Parallel()

	service := &reputationRouteService{ApplicationService: app.NewService()}
	server := NewServer(service)
	unauthorized := httptest.NewRequest(http.MethodGet, "/api/v1/me/reputation", nil)
	unauthorizedResponse := httptest.NewRecorder()
	server.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body %s", unauthorizedResponse.Code, unauthorizedResponse.Body.String())
	}

	session := createSession(t, server, "reputation-user", false)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/reputation", nil)
	addCookie(request, session.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("my reputation status %d body %s", response.Code, response.Body.String())
	}
	if service.myUserID != session.userID {
		t.Fatalf("expected session user %q, got %q", session.userID, service.myUserID)
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode my reputation: %v", err)
	}
	var restrictions []map[string]json.RawMessage
	if err := json.Unmarshal(body["activeRestrictions"], &restrictions); err != nil {
		t.Fatalf("decode active restrictions: %v", err)
	}
	if len(restrictions) != 1 {
		t.Fatalf("expected one active restriction, got %#v", restrictions)
	}
	for _, forbidden := range []string{"id", "userId", "internalReason", "sourceDisputeOutcomeId", "sourceDisputeRemedyId", "createdByAdminId", "version"} {
		if _, exists := restrictions[0][forbidden]; exists {
			t.Fatalf("public active restriction leaked %q: %#v", forbidden, restrictions[0])
		}
	}
	for _, required := range []string{"restrictionType", "roleScope", "actionCode", "reasonCode", "publicReason", "startsAt", "endsAt"} {
		if _, exists := restrictions[0][required]; !exists {
			t.Fatalf("public active restriction missing %q: %#v", required, restrictions[0])
		}
	}
}

func TestAPIOrderSanctionRecommendationAndApplyRoutes(t *testing.T) {
	t.Parallel()

	service := &reputationRouteService{ApplicationService: app.NewService()}
	server := NewServer(service)
	nonAdmin := createSession(t, server, "sanction-member", false)
	admin := createSession(t, server, "sanction-admin", true)
	disputeID := "66666666-6666-4666-8666-666666666666"
	recommendationPath := "/api/v1/admin/disputes/" + disputeID + "/sanction-recommendation"

	forbidden := httptest.NewRequest(http.MethodGet, recommendationPath, nil)
	addCookie(forbidden, nonAdmin.cookie)
	forbiddenResponse := httptest.NewRecorder()
	server.ServeHTTP(forbiddenResponse, forbidden)
	if forbiddenResponse.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin recommendation rejection, got %d body %s", forbiddenResponse.Code, forbiddenResponse.Body.String())
	}

	read := httptest.NewRequest(http.MethodGet, recommendationPath, nil)
	addCookie(read, admin.cookie)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK || readResponse.Header().Get("ETag") != `"4"` || service.sanctionReadID != disputeID {
		t.Fatalf("unexpected recommendation response: status=%d etag=%q dispute=%q body=%s", readResponse.Code, readResponse.Header().Get("ETag"), service.sanctionReadID, readResponse.Body.String())
	}
	var recommendation apiOrderSanctionRecommendationResponse
	if err := json.NewDecoder(readResponse.Body).Decode(&recommendation); err != nil {
		t.Fatalf("decode recommendation: %v", err)
	}
	if !recommendation.Eligible || recommendation.ConfirmedBreaches180Days != 2 || recommendation.RecommendedDays != 30 || recommendation.SubjectUserVersion != 4 {
		t.Fatalf("unexpected recommendation: %#v", recommendation)
	}

	applyPath := "/api/v1/admin/disputes/" + disputeID + "/sanction"
	body := `{"internalReason":"管理员核对整改逾期事实后显式处罚。"}`
	missingVersion := httptest.NewRequest(http.MethodPost, applyPath, strings.NewReader(body))
	missingVersion.Header.Set("Content-Type", "application/json")
	addAuth(missingVersion, admin, "sanction-missing-version")
	missingVersionResponse := httptest.NewRecorder()
	server.ServeHTTP(missingVersionResponse, missingVersion)
	if missingVersionResponse.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected missing subject version rejection, got %d body %s", missingVersionResponse.Code, missingVersionResponse.Body.String())
	}

	apply := httptest.NewRequest(http.MethodPost, applyPath, strings.NewReader(body))
	apply.Header.Set("Content-Type", "application/json")
	apply.Header.Set("If-Match", `"4"`)
	addAuth(apply, admin, "sanction-apply")
	applyResponse := httptest.NewRecorder()
	server.ServeHTTP(applyResponse, apply)
	if applyResponse.Code != http.StatusCreated || applyResponse.Header().Get("ETag") != `"5"` {
		t.Fatalf("unexpected apply response: status=%d etag=%q body=%s", applyResponse.Code, applyResponse.Header().Get("ETag"), applyResponse.Body.String())
	}
	if service.sanctionApply.DisputeCaseID != disputeID || service.sanctionApply.ExpectedUserVersion != 4 || service.sanctionApply.InternalReason != "管理员核对整改逾期事实后显式处罚。" {
		t.Fatalf("unexpected sanction input: %#v", service.sanctionApply)
	}
	if !strings.Contains(applyResponse.Body.String(), `"sourceDisputeRemedyId":"99999999-9999-4999-8999-999999999999"`) {
		t.Fatalf("admin restriction response missing remedy source: %s", applyResponse.Body.String())
	}
}

func TestAdminReputationRecalculationRequiresCSRFAndAdmin(t *testing.T) {
	t.Parallel()

	service := &reputationRouteService{ApplicationService: app.NewService()}
	server := NewServer(service)
	nonAdmin := createSession(t, server, "reputation-member", false)
	admin := createSession(t, server, "reputation-admin", true)
	targetID := "22222222-2222-4222-8222-222222222222"

	missingCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+targetID+"/reputation/recalculate", nil)
	addCookie(missingCSRF, admin.cookie)
	missingCSRFResponse := httptest.NewRecorder()
	server.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("expected csrf rejection, got %d body %s", missingCSRFResponse.Code, missingCSRFResponse.Body.String())
	}

	forbidden := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+targetID+"/reputation/recalculate", nil)
	addAuth(forbidden, nonAdmin, "reputation-recalculate-member")
	forbiddenResponse := httptest.NewRecorder()
	server.ServeHTTP(forbiddenResponse, forbidden)
	if forbiddenResponse.Code != http.StatusForbidden {
		t.Fatalf("expected admin rejection, got %d body %s", forbiddenResponse.Code, forbiddenResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+targetID+"/reputation/recalculate", nil)
	addAuth(request, admin, "reputation-recalculate-user")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.recalculatedID != targetID {
		t.Fatalf("user recalculation failed: status=%d target=%q body=%s", response.Code, service.recalculatedID, response.Body.String())
	}

	allRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reputation/recalculate", nil)
	addAuth(allRequest, admin, "reputation-recalculate-all")
	allResponse := httptest.NewRecorder()
	server.ServeHTTP(allResponse, allRequest)
	if allResponse.Code != http.StatusOK || !service.recalculateAll {
		t.Fatalf("full recalculation failed: status=%d body=%s", allResponse.Code, allResponse.Body.String())
	}
}

func TestAdminReputationAuditRequiresAdminAndValidLimit(t *testing.T) {
	t.Parallel()

	service := &reputationRouteService{ApplicationService: app.NewService()}
	server := NewServer(service)
	nonAdmin := createSession(t, server, "reputation-audit-member", false)
	admin := createSession(t, server, "reputation-audit-admin", true)
	targetID := "33333333-3333-4333-8333-333333333333"
	path := "/api/v1/admin/users/" + targetID + "/reputation"

	unauthorized := httptest.NewRequest(http.MethodGet, path, nil)
	unauthorizedResponse := httptest.NewRecorder()
	server.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized audit, got %d body %s", unauthorizedResponse.Code, unauthorizedResponse.Body.String())
	}

	forbidden := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(forbidden, nonAdmin.cookie)
	forbiddenResponse := httptest.NewRecorder()
	server.ServeHTTP(forbiddenResponse, forbidden)
	if forbiddenResponse.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin audit rejection, got %d body %s", forbiddenResponse.Code, forbiddenResponse.Body.String())
	}

	invalidLimit := httptest.NewRequest(http.MethodGet, path+"?limit=101", nil)
	addCookie(invalidLimit, admin.cookie)
	invalidLimitResponse := httptest.NewRecorder()
	server.ServeHTTP(invalidLimitResponse, invalidLimit)
	if invalidLimitResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected invalid limit rejection, got %d body %s", invalidLimitResponse.Code, invalidLimitResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, path+"?limit=25", nil)
	addCookie(request, admin.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin reputation audit status %d body %s", response.Code, response.Body.String())
	}
	if service.adminAuditID != targetID || service.adminAuditLimit != 25 {
		t.Fatalf("unexpected audit inputs: id=%q limit=%d", service.adminAuditID, service.adminAuditLimit)
	}
	var audit adminReputationAuditResponse
	if err := json.NewDecoder(response.Body).Decode(&audit); err != nil {
		t.Fatalf("decode reputation audit: %v", err)
	}
	if audit.UserID != targetID || audit.RuleVersion != reputation.RuleVersion {
		t.Fatalf("unexpected reputation audit response: %#v", audit)
	}
	if len(audit.Restrictions) != 1 || len(audit.Outcomes) != 1 || len(audit.Appeals) != 1 || audit.SourceAuthorVerifications == nil {
		t.Fatalf("audit evidence missing from response: %#v", audit)
	}
}

func TestAdminSourceAuthorVerificationRequiresAdminAndSupportsInitialVersionZero(t *testing.T) {
	t.Parallel()

	service := &reputationRouteService{ApplicationService: app.NewService()}
	server := NewServer(service)
	nonAdmin := createSession(t, server, "source-author-member", false)
	admin := createSession(t, server, "source-author-admin", true)
	resourceID := "44444444-4444-4444-8444-444444444444"
	path := "/api/v1/admin/source-author-verifications/carpool/" + resourceID

	unauthorized := httptest.NewRequest(http.MethodGet, path, nil)
	unauthorizedResponse := httptest.NewRecorder()
	server.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized source read, got %d body %s", unauthorizedResponse.Code, unauthorizedResponse.Body.String())
	}

	forbidden := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(forbidden, nonAdmin.cookie)
	forbiddenResponse := httptest.NewRecorder()
	server.ServeHTTP(forbiddenResponse, forbidden)
	if forbiddenResponse.Code != http.StatusForbidden {
		t.Fatalf("expected non-admin source read rejection, got %d body %s", forbiddenResponse.Code, forbiddenResponse.Body.String())
	}

	read := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(read, admin.cookie)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK ||
		readResponse.Header().Get("ETag") != `"0"` ||
		service.sourceReadType != reputation.SourceResourceCarpool ||
		service.sourceReadID != resourceID {
		t.Fatalf("unexpected source read: status=%d etag=%q type=%q id=%q body=%s",
			readResponse.Code,
			readResponse.Header().Get("ETag"),
			service.sourceReadType,
			service.sourceReadID,
			readResponse.Body.String(),
		)
	}

	body := `{"status":"pending","actualExternalUserId":"","verificationMethod":"","failureReason":""}`
	missingVersion := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	missingVersion.Header.Set("Content-Type", "application/json")
	addAuth(missingVersion, admin, "source-author-missing-version")
	missingVersionResponse := httptest.NewRecorder()
	server.ServeHTTP(missingVersionResponse, missingVersion)
	if missingVersionResponse.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected missing If-Match rejection, got %d body %s", missingVersionResponse.Code, missingVersionResponse.Body.String())
	}

	create := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set("If-Match", `"0"`)
	addAuth(create, admin, "source-author-create")
	createResponse := httptest.NewRecorder()
	server.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusOK || createResponse.Header().Get("ETag") != `"1"` {
		t.Fatalf("expected initial version zero update, got %d etag=%q body %s", createResponse.Code, createResponse.Header().Get("ETag"), createResponse.Body.String())
	}
	if service.sourceUpdate.ExpectedVersion != 0 ||
		service.sourceUpdate.Status != reputation.SourceVerificationPending ||
		service.sourceUpdate.ResourceID != resourceID {
		t.Fatalf("unexpected source update input: %#v", service.sourceUpdate)
	}
}

func testReputationSnapshot(userID, role, scope string) reputation.ReputationSnapshot {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	return reputation.ReputationSnapshot{
		UserID:         userID,
		Role:           role,
		Scope:          scope,
		Tier:           reputation.TierInsufficient,
		State:          reputation.StateActive,
		Confidence:     reputation.ConfidenceLow,
		RuleVersion:    reputation.RuleVersion,
		Warnings:       []string{},
		Badges:         []string{},
		Progress:       []reputation.ReputationProgressItem{},
		TierEnteredAt:  now,
		StateEnteredAt: now,
		CalculatedAt:   now,
	}
}
