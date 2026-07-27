package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"

	"github.com/go-chi/chi/v5"
)

type reputationRulesResponse struct {
	Rules reputation.RuleSet `json:"rules"`
}

type reputationScopeResponse struct {
	UserID      string                          `json:"userId"`
	Scope       string                          `json:"scope"`
	Reputations []reputation.ReputationSnapshot `json:"reputations"`
}

type myReputationResponse struct {
	RuleVersion string                          `json:"ruleVersion"`
	Items       []reputation.ReputationSnapshot `json:"items"`
}

type adminReputationAuditResponse struct {
	UserID                    string                                     `json:"userId"`
	RuleVersion               string                                     `json:"ruleVersion"`
	Items                     []reputation.ReputationSnapshot            `json:"items"`
	History                   []reputation.ReputationHistory             `json:"history"`
	Restrictions              []userRestrictionResponse                  `json:"restrictions"`
	Outcomes                  []disputeOutcomeResponse                   `json:"outcomes"`
	Appeals                   []reputationAppealResponse                 `json:"appeals"`
	SourceAuthorVerifications []reputation.SourceAuthorVerificationAudit `json:"sourceAuthorVerifications"`
}

type reputationAppealResponse struct {
	ID               string  `json:"id"`
	AppellantUserID  string  `json:"appellantUserId"`
	ReportID         string  `json:"reportId,omitempty"`
	DisputeID        string  `json:"disputeId,omitempty"`
	TargetType       string  `json:"targetType"`
	TargetID         string  `json:"targetId"`
	Title            string  `json:"title"`
	Statement        string  `json:"statement"`
	Status           string  `json:"status"`
	AdminReason      string  `json:"adminReason"`
	HandledByAdminID string  `json:"handledByAdminId,omitempty"`
	HandledAt        *string `json:"handledAt"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
	Version          int64   `json:"version"`
}

type updateSourceAuthorVerificationRequest struct {
	Status               string  `json:"status"`
	ActualExternalUserID string  `json:"actualExternalUserId"`
	VerificationMethod   string  `json:"verificationMethod"`
	ExpiresAt            *string `json:"expiresAt"`
	FailureReason        string  `json:"failureReason"`
}

type reputationSummaryResponse struct {
	Role                     string                           `json:"role"`
	Scope                    string                           `json:"scope"`
	Tier                     string                           `json:"tier"`
	State                    string                           `json:"state"`
	Confidence               string                           `json:"confidence"`
	RuleVersion              string                           `json:"ruleVersion"`
	CompletedCount           int                              `json:"completedCount"`
	RoleFaultCancelRate      *float64                         `json:"roleFaultCancelRate"`
	HasUnknownCancellation   bool                             `json:"hasUnknownCancellation"`
	UnresolvedDisputes       int                              `json:"unresolvedDisputes"`
	ActiveRestrictions       int                              `json:"activeRestrictions"`
	VerifiedReviewCount      int                              `json:"verifiedReviewCount"`
	WeightedRating           *float64                         `json:"weightedRating"`
	SourceAuthorVerification reputation.SourceAuthorAggregate `json:"sourceAuthorVerification"`
	Warnings                 []string                         `json:"warnings"`
	Badges                   []string                         `json:"badges"`
	CalculatedAt             time.Time                        `json:"calculatedAt"`
}

func toReputationSummary(snapshot *reputation.ReputationSnapshot) *reputationSummaryResponse {
	if snapshot == nil {
		return nil
	}
	return &reputationSummaryResponse{
		Role:                     snapshot.Role,
		Scope:                    snapshot.Scope,
		Tier:                     snapshot.Tier,
		State:                    snapshot.State,
		Confidence:               snapshot.Confidence,
		RuleVersion:              snapshot.RuleVersion,
		CompletedCount:           snapshot.Metrics.CompletedCount,
		RoleFaultCancelRate:      snapshot.Metrics.RoleFaultCancelRate,
		HasUnknownCancellation:   snapshot.Metrics.UnknownResponsibilityCancellationCount > 0,
		UnresolvedDisputes:       snapshot.Metrics.UnresolvedDisputeCount,
		ActiveRestrictions:       snapshot.Metrics.ActiveRestrictionCount,
		VerifiedReviewCount:      snapshot.Metrics.VerifiedReviewCount,
		WeightedRating:           snapshot.Metrics.WeightedRating,
		SourceAuthorVerification: snapshot.Metrics.SourceAuthorVerification,
		Warnings:                 snapshot.Warnings,
		Badges:                   snapshot.Badges,
		CalculatedAt:             snapshot.CalculatedAt,
	}
}

type sourceAuthorResourceSummaryResponse struct {
	Status     string  `json:"status"`
	VerifiedAt *string `json:"verifiedAt,omitempty"`
	ExpiresAt  *string `json:"expiresAt,omitempty"`
}

func toSourceAuthorResourceSummaryResponse(
	summary reputation.SourceAuthorResourceSummary,
) sourceAuthorResourceSummaryResponse {
	summary = reputation.NormalizeSourceAuthorResourceSummary(summary)
	return sourceAuthorResourceSummaryResponse{
		Status:     summary.Status,
		VerifiedAt: formatOptionalTime(summary.VerifiedAt),
		ExpiresAt:  formatOptionalTime(summary.ExpiresAt),
	}
}

func (s *Server) handleReputationRules(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, reputationRulesResponse{Rules: s.reputation.ReputationRules()})
}

func (s *Server) handlePublicUserReputation(w http.ResponseWriter, r *http.Request) {
	scope := strings.TrimSpace(r.URL.Query().Get("scope"))
	if scope == "" {
		scope = reputation.ScopeOverall
	}
	items, appErr := s.reputation.PublicUserReputation(r.Context(), chi.URLParam(r, "username"), scope)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	userID := ""
	if len(items) > 0 {
		userID = items[0].UserID
	}
	writeJSON(w, http.StatusOK, reputationScopeResponse{
		UserID:      userID,
		Scope:       scope,
		Reputations: items,
	})
}

func (s *Server) handleMyReputation(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.reputation.MyReputation(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, myReputationResponse{
		RuleVersion: reputation.RuleVersion,
		Items:       items,
	})
}

func (s *Server) handleAdminUserReputation(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	page, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	audit, appErr := s.reputation.AdminUserReputation(
		r.Context(),
		user,
		chi.URLParam(r, "id"),
		page.Limit,
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAdminReputationAuditResponse(audit))
}

func (s *Server) handleAdminRecalculateUserReputation(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.reputation.AdminRecalculateUserReputation(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAdminRecalculateAllReputation(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.reputation.AdminRecalculateAllReputation(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAdminSourceAuthorVerification(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	audit, appErr := s.reputation.AdminSourceAuthorVerification(
		r.Context(),
		user,
		chi.URLParam(r, "resourceType"),
		chi.URLParam(r, "resourceId"),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setSourceAuthorETag(w, audit.Verification.Version)
	writeJSON(w, http.StatusOK, audit)
}

func (s *Server) handleAdminUpdateSourceAuthorVerification(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	_, req, appErr := decodeStrictJSON[updateSourceAuthorVerificationRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersionAllowZero(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	expiresAt, appErr := parseSourceAuthorExpiresAt(req.ExpiresAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	audit, appErr := s.reputation.AdminUpdateSourceAuthorVerification(
		r.Context(),
		user,
		reputation.UpdateSourceAuthorVerificationInput{
			ResourceType:         chi.URLParam(r, "resourceType"),
			ResourceID:           chi.URLParam(r, "resourceId"),
			Status:               req.Status,
			ActualExternalUserID: req.ActualExternalUserID,
			VerificationMethod:   req.VerificationMethod,
			ExpiresAt:            expiresAt,
			FailureReason:        req.FailureReason,
			ExpectedVersion:      version,
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setSourceAuthorETag(w, audit.Verification.Version)
	writeJSON(w, http.StatusOK, audit)
}

func setSourceAuthorETag(w http.ResponseWriter, version int64) {
	w.Header().Set("ETag", `"`+strconv.FormatInt(version, 10)+`"`)
}

func parseSourceAuthorExpiresAt(value *string) (*time.Time, *domain.AppError) {
	if value == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(*value)
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, domain.NewFieldError(
			http.StatusUnprocessableEntity,
			domain.CodeValidationFailed,
			"Invalid expiresAt",
			"原帖作者验证过期时间格式不正确。",
			"expiresAt",
			"invalid",
			"过期时间必须是 RFC3339 时间。",
		)
	}
	return &parsed, nil
}

type createDisputeOutcomeRequest struct {
	SubjectUserID  string `json:"subjectUserId"`
	Responsibility string `json:"responsibility"`
	Severity       string `json:"severity"`
	RoleScope      string `json:"roleScope"`
	ReasonCode     string `json:"reasonCode"`
	PublicReason   string `json:"publicReason"`
	InternalReason string `json:"internalReason"`
}

type createUserRestrictionRequest struct {
	RestrictionType        string  `json:"restrictionType"`
	RoleScope              string  `json:"roleScope"`
	ActionCode             string  `json:"actionCode"`
	ReasonCode             string  `json:"reasonCode"`
	PublicReason           string  `json:"publicReason"`
	InternalReason         string  `json:"internalReason"`
	StartsAt               string  `json:"startsAt"`
	EndsAt                 *string `json:"endsAt"`
	SourceDisputeOutcomeID string  `json:"sourceDisputeOutcomeId"`
}

type revokeUserRestrictionRequest struct {
	Reason string `json:"reason"`
}

type disputeOutcomeResponse struct {
	ID                string  `json:"id"`
	DisputeCaseID     string  `json:"disputeCaseId"`
	SubjectUserID     string  `json:"subjectUserId"`
	Responsibility    string  `json:"responsibility"`
	Severity          string  `json:"severity"`
	RoleScope         string  `json:"roleScope"`
	Status            string  `json:"status"`
	ReasonCode        string  `json:"reasonCode"`
	PublicReason      string  `json:"publicReason"`
	InternalReason    string  `json:"internalReason"`
	DecidedByAdminID  string  `json:"decidedByAdminId"`
	DecidedAt         string  `json:"decidedAt"`
	ReversedAt        *string `json:"reversedAt,omitempty"`
	ReversedByAdminID string  `json:"reversedByAdminId,omitempty"`
	ReversalAppealID  string  `json:"reversalAppealId,omitempty"`
	ReversalReason    string  `json:"reversalReason,omitempty"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	Version           int64   `json:"version"`
	DisputeVersion    int64   `json:"disputeVersion"`
}

type userRestrictionResponse struct {
	ID                     string  `json:"id"`
	UserID                 string  `json:"userId"`
	RestrictionType        string  `json:"restrictionType"`
	RoleScope              string  `json:"roleScope"`
	ActionCode             string  `json:"actionCode"`
	ReasonCode             string  `json:"reasonCode"`
	PublicReason           string  `json:"publicReason"`
	InternalReason         string  `json:"internalReason"`
	StartsAt               string  `json:"startsAt"`
	EndsAt                 *string `json:"endsAt,omitempty"`
	SourceDisputeOutcomeID string  `json:"sourceDisputeOutcomeId,omitempty"`
	CreatedByAdminID       string  `json:"createdByAdminId"`
	RevokedAt              *string `json:"revokedAt,omitempty"`
	RevokedByAdminID       string  `json:"revokedByAdminId,omitempty"`
	RevocationReason       string  `json:"revocationReason,omitempty"`
	CreatedAt              string  `json:"createdAt"`
	UpdatedAt              string  `json:"updatedAt"`
	Version                int64   `json:"version"`
	UserVersion            int64   `json:"userVersion,omitempty"`
}

type reputationGovernanceMutationResponse struct {
	Outcome     *disputeOutcomeResponse  `json:"outcome,omitempty"`
	Restriction *userRestrictionResponse `json:"restriction,omitempty"`
}

func (s *Server) handleCreateDisputeReputationOutcome(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createDisputeOutcomeRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	disputeID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/disputes/{id}/reputation-outcome:" + disputeID
	completion, appErr := s.reputation.AdminCreateDisputeOutcomeWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		reputation.CreateOutcomeInput{
			DisputeCaseID:   disputeID,
			SubjectUserID:   req.SubjectUserID,
			Responsibility:  req.Responsibility,
			Severity:        req.Severity,
			RoleScope:       req.RoleScope,
			ReasonCode:      req.ReasonCode,
			PublicReason:    req.PublicReason,
			InternalReason:  req.InternalReason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		reputationGovernanceCompletionBuilder(http.StatusCreated),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreReputationGovernanceETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleCreateUserReputationRestriction(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createUserRestrictionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	startsAt, endsAt, appErr := parseRestrictionPeriod(req.StartsAt, req.EndsAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	userID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/users/{id}/reputation-restrictions:" + userID
	completion, appErr := s.reputation.AdminCreateUserRestrictionWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		reputation.CreateRestrictionInput{
			UserID:                 userID,
			RestrictionType:        req.RestrictionType,
			RoleScope:              req.RoleScope,
			ActionCode:             req.ActionCode,
			ReasonCode:             req.ReasonCode,
			PublicReason:           req.PublicReason,
			InternalReason:         req.InternalReason,
			StartsAt:               startsAt,
			EndsAt:                 endsAt,
			SourceDisputeOutcomeID: req.SourceDisputeOutcomeID,
			ExpectedUserVersion:    version,
			RequestID:              requestIDFrom(r),
		},
		reputationGovernanceCompletionBuilder(http.StatusCreated),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreReputationGovernanceETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleRevokeUserReputationRestriction(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[revokeUserRestrictionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restrictionID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/reputation-restrictions/{id}/revoke:" + restrictionID
	completion, appErr := s.reputation.AdminRevokeUserRestrictionWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		reputation.RevokeRestrictionInput{
			RestrictionID:   restrictionID,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		reputationGovernanceCompletionBuilder(http.StatusOK),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreReputationGovernanceETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func restoreReputationGovernanceETag(completion *idempotency.Completion) {
	if completion == nil || len(completion.Body) == 0 {
		return
	}
	if completion.Headers != nil && strings.TrimSpace(completion.Headers["ETag"]) != "" {
		return
	}
	var payload reputationGovernanceMutationResponse
	if err := json.Unmarshal(completion.Body, &payload); err != nil {
		return
	}
	version := int64(0)
	if payload.Outcome != nil {
		version = payload.Outcome.DisputeVersion
	}
	if payload.Restriction != nil {
		version = payload.Restriction.Version
		if completion.Status == http.StatusCreated && payload.Restriction.UserVersion > 0 {
			version = payload.Restriction.UserVersion
		}
	}
	if version <= 0 {
		return
	}
	if completion.Headers == nil {
		completion.Headers = make(map[string]string)
	}
	completion.Headers["ETag"] = `"` + strconv.FormatInt(version, 10) + `"`
}

func parseRestrictionPeriod(startsAt string, endsAt *string) (time.Time, *time.Time, *domain.AppError) {
	var start time.Time
	var err error
	if strings.TrimSpace(startsAt) != "" {
		start, err = time.Parse(time.RFC3339, strings.TrimSpace(startsAt))
		if err != nil {
			return time.Time{}, nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Start time invalid", "开始时间格式不正确。", "startsAt", "invalid", "开始时间必须是 ISO 8601。")
		}
	}
	if endsAt == nil || strings.TrimSpace(*endsAt) == "" {
		return start, nil, nil
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(*endsAt))
	if err != nil {
		return time.Time{}, nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "End time invalid", "结束时间格式不正确。", "endsAt", "invalid", "结束时间必须是 ISO 8601。")
	}
	return start, &end, nil
}

func reputationGovernanceCompletionBuilder(status int) reputation.GovernanceCompletionBuilder {
	return func(result reputation.GovernanceMutationResult) (idempotency.Completion, *domain.AppError) {
		payload := reputationGovernanceMutationResponse{}
		resourceType := "reputation_governance"
		resourceID := ""
		headers := map[string]string{}
		if result.Outcome != nil {
			value := toDisputeOutcomeResponse(*result.Outcome)
			payload.Outcome = &value
			resourceType = "dispute_reputation_outcome"
			resourceID = result.Outcome.ID
			headers["ETag"] = `"` + strconv.FormatInt(result.Outcome.DisputeVersion, 10) + `"`
		}
		if result.Restriction != nil {
			value := toUserRestrictionResponse(*result.Restriction)
			payload.Restriction = &value
			resourceType = "user_restriction"
			resourceID = result.Restriction.ID
			version := result.Restriction.Version
			if status == http.StatusCreated && result.Restriction.UserVersion > 0 {
				version = result.Restriction.UserVersion
			}
			headers["ETag"] = `"` + strconv.FormatInt(version, 10) + `"`
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       status,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			Headers:      headers,
		}, nil
	}
}

func toDisputeOutcomeResponse(item reputation.DisputeOutcome) disputeOutcomeResponse {
	return disputeOutcomeResponse{
		ID:                item.ID,
		DisputeCaseID:     item.DisputeCaseID,
		SubjectUserID:     item.SubjectUserID,
		Responsibility:    item.Responsibility,
		Severity:          item.Severity,
		RoleScope:         item.RoleScope,
		Status:            item.Status,
		ReasonCode:        item.ReasonCode,
		PublicReason:      item.PublicReason,
		InternalReason:    item.InternalReason,
		DecidedByAdminID:  item.DecidedByAdminID,
		DecidedAt:         item.DecidedAt.UTC().Format(time.RFC3339),
		ReversedAt:        formatOptionalTime(item.ReversedAt),
		ReversedByAdminID: item.ReversedByAdminID,
		ReversalAppealID:  item.ReversalAppealID,
		ReversalReason:    item.ReversalReason,
		CreatedAt:         item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:           item.Version,
		DisputeVersion:    item.DisputeVersion,
	}
}

func toUserRestrictionResponse(item reputation.UserRestriction) userRestrictionResponse {
	return userRestrictionResponse{
		ID:                     item.ID,
		UserID:                 item.UserID,
		RestrictionType:        item.RestrictionType,
		RoleScope:              item.RoleScope,
		ActionCode:             item.ActionCode,
		ReasonCode:             item.ReasonCode,
		PublicReason:           item.PublicReason,
		InternalReason:         item.InternalReason,
		StartsAt:               item.StartsAt.UTC().Format(time.RFC3339),
		EndsAt:                 formatOptionalTime(item.EndsAt),
		SourceDisputeOutcomeID: item.SourceDisputeOutcomeID,
		CreatedByAdminID:       item.CreatedByAdminID,
		RevokedAt:              formatOptionalTime(item.RevokedAt),
		RevokedByAdminID:       item.RevokedByAdminID,
		RevocationReason:       item.RevocationReason,
		CreatedAt:              item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:              item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:                item.Version,
		UserVersion:            item.UserVersion,
	}
}

func toAdminReputationAuditResponse(audit reputation.AdminReputationAudit) adminReputationAuditResponse {
	restrictions := make([]userRestrictionResponse, 0, len(audit.Restrictions))
	for _, item := range audit.Restrictions {
		restrictions = append(restrictions, toUserRestrictionResponse(item))
	}
	outcomes := make([]disputeOutcomeResponse, 0, len(audit.Outcomes))
	for _, item := range audit.Outcomes {
		outcomes = append(outcomes, toDisputeOutcomeResponse(item))
	}
	appeals := make([]reputationAppealResponse, 0, len(audit.Appeals))
	for _, item := range audit.Appeals {
		appeals = append(appeals, reputationAppealResponse{
			ID:               item.ID,
			AppellantUserID:  item.AppellantUserID,
			ReportID:         item.ReportID,
			DisputeID:        item.DisputeID,
			TargetType:       item.TargetType,
			TargetID:         item.TargetID,
			Title:            item.Title,
			Statement:        item.Statement,
			Status:           item.Status,
			AdminReason:      item.AdminReason,
			HandledByAdminID: item.HandledByAdminID,
			HandledAt:        formatOptionalTime(item.HandledAt),
			CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:        item.UpdatedAt.UTC().Format(time.RFC3339),
			Version:          item.Version,
		})
	}
	sourceAudits := audit.SourceAuthorVerifications
	if sourceAudits == nil {
		sourceAudits = []reputation.SourceAuthorVerificationAudit{}
	}
	return adminReputationAuditResponse{
		UserID:                    audit.UserID,
		RuleVersion:               audit.RuleVersion,
		Items:                     audit.Items,
		History:                   audit.History,
		Restrictions:              restrictions,
		Outcomes:                  outcomes,
		Appeals:                   appeals,
		SourceAuthorVerifications: sourceAudits,
	}
}
