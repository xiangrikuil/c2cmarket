package reputation

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

var reasonCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

type Service struct {
	mu           sync.Mutex
	repo         Repository
	engineRepo   EngineRepository
	sourceRepo   SourceAuthorRepository
	auditRepo    AuditRepository
	idempotency  *idempotency.Service
	now          func() time.Time
	outcomes     map[string]DisputeOutcome
	restrictions map[string]UserRestriction
}

func NewService(repo Repository, now func() time.Time, idempotencyServices ...*idempotency.Service) *Service {
	if now == nil {
		now = time.Now
	}
	var idempotencyService *idempotency.Service
	if len(idempotencyServices) > 0 {
		idempotencyService = idempotencyServices[0]
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	service := &Service{
		repo:         repo,
		idempotency:  idempotencyService,
		now:          now,
		outcomes:     make(map[string]DisputeOutcome),
		restrictions: make(map[string]UserRestriction),
	}
	if engineRepo, ok := repo.(EngineRepository); ok {
		service.engineRepo = engineRepo
	}
	if sourceRepo, ok := repo.(SourceAuthorRepository); ok {
		service.sourceRepo = sourceRepo
	}
	if auditRepo, ok := repo.(AuditRepository); ok {
		service.auditRepo = auditRepo
	}
	return service
}

func (s *Service) AggregateFacts(ctx context.Context, userIDs []string) (map[string]RawFacts, *domain.AppError) {
	normalized := normalizeUserIDs(userIDs)
	if len(normalized) == 0 {
		return map[string]RawFacts{}, nil
	}
	if s.repo == nil {
		return map[string]RawFacts{}, nil
	}
	facts, appErr := s.repo.AggregateFacts(ctx, normalized, s.now())
	if appErr != nil {
		return nil, appErr
	}
	if facts == nil {
		facts = make(map[string]RawFacts, len(normalized))
	}
	for _, userID := range normalized {
		value := facts[userID]
		value.UserID = userID
		if !value.Buyer.Overall.Aggregated {
			value.Buyer.Overall = mergeScopeFacts(value.Buyer.Carpool, value.Buyer.API)
		}
		if !value.Seller.Overall.Aggregated {
			value.Seller.Overall = mergeScopeFacts(value.Seller.Carpool, value.Seller.API)
		}
		facts[userID] = value
	}
	return facts, nil
}

func (s *Service) ExcludeTransaction(ctx context.Context, actor AdminActor, input ExcludeTransactionInput) (TransactionExclusion, *domain.AppError) {
	if appErr := validateAdminActor(actor); appErr != nil {
		return TransactionExclusion{}, appErr
	}
	mutation, appErr := validateExclusionMutation(actor.UserID, input.TransactionType, input.TransactionID, input.ReasonCode, input.Reason)
	if appErr != nil {
		return TransactionExclusion{}, appErr
	}
	if s.repo == nil {
		return TransactionExclusion{}, unavailableRepositoryError()
	}
	return s.repo.ExcludeTransaction(ctx, mutation, s.now())
}

func (s *Service) RestoreTransaction(ctx context.Context, actor AdminActor, input RestoreTransactionInput) (TransactionExclusion, *domain.AppError) {
	if appErr := validateAdminActor(actor); appErr != nil {
		return TransactionExclusion{}, appErr
	}
	mutation, appErr := validateExclusionMutation(actor.UserID, input.TransactionType, input.TransactionID, input.ReasonCode, input.Reason)
	if appErr != nil {
		return TransactionExclusion{}, appErr
	}
	if s.repo == nil {
		return TransactionExclusion{}, unavailableRepositoryError()
	}
	return s.repo.RestoreTransaction(ctx, mutation, s.now())
}

func (s *Service) CreateDisputeOutcomeWithIdempotency(ctx context.Context, actor AdminActor, routeKey, key, requestHash string, input CreateOutcomeInput, buildCompletion GovernanceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := validateAdminActor(actor); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = strings.TrimSpace(actor.UserID)
	if appErr := validateCreateOutcomeInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, input.AdminUserID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.CreateDisputeOutcomeWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	result, appErr := s.createDisputeOutcomeInMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.completeInMemoryMutation(ctx, entry, result, buildCompletion)
}

func (s *Service) CreateUserRestrictionWithIdempotency(ctx context.Context, actor AdminActor, routeKey, key, requestHash string, input CreateRestrictionInput, buildCompletion GovernanceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := validateAdminActor(actor); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = strings.TrimSpace(actor.UserID)
	if input.StartsAt.IsZero() {
		input.StartsAt = s.now()
	}
	if appErr := validateCreateRestrictionInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, input.AdminUserID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.CreateUserRestrictionWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	result, appErr := s.createUserRestrictionInMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.completeInMemoryMutation(ctx, entry, result, buildCompletion)
}

func (s *Service) RevokeUserRestrictionWithIdempotency(ctx context.Context, actor AdminActor, routeKey, key, requestHash string, input RevokeRestrictionInput, buildCompletion GovernanceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := validateAdminActor(actor); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = strings.TrimSpace(actor.UserID)
	if appErr := validateRevokeRestrictionInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, input.AdminUserID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.RevokeUserRestrictionWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	result, appErr := s.revokeUserRestrictionInMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.completeInMemoryMutation(ctx, entry, result, buildCompletion)
}

func (s *Service) CheckActionAllowed(ctx context.Context, userID, role, action string) *domain.AppError {
	userID = strings.TrimSpace(userID)
	role = strings.TrimSpace(role)
	action = strings.TrimSpace(action)
	if userID == "" || !validRole(role) || !validAction(action) || action == ActionAll {
		return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "信誉动作检查参数无效。")
	}
	var restriction *UserRestriction
	var appErr *domain.AppError
	if s.repo != nil {
		restriction, appErr = s.repo.FindActiveRestriction(ctx, userID, role, action, s.now())
		if appErr != nil {
			return appErr
		}
	} else {
		restriction = s.findActiveRestrictionInMemory(userID, role, action, s.now())
	}
	if restriction == nil {
		return nil
	}
	return domain.NewError(http.StatusForbidden, domain.CodeReputationActionRestricted, "Reputation action restricted", nonEmptyString(restriction.PublicReason, "当前信誉限制不允许执行该操作。"))
}

func (s *Service) completeInMemoryMutation(ctx context.Context, entry *idempotency.Entry, result GovernanceMutationResult, buildCompletion GovernanceCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) createDisputeOutcomeInMemory(input CreateOutcomeInput) (GovernanceMutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.outcomes {
		if existing.DisputeCaseID == input.DisputeCaseID {
			return GovernanceMutationResult{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Outcome exists", "该纠纷已有信誉裁定。")
		}
	}
	now := s.now()
	item := DisputeOutcome{
		ID:                uuid.NewString(),
		DisputeCaseID:     input.DisputeCaseID,
		SubjectUserID:     input.SubjectUserID,
		Responsibility:    input.Responsibility,
		Severity:          input.Severity,
		RoleScope:         input.RoleScope,
		Status:            OutcomeStatusActive,
		ReasonCode:        input.ReasonCode,
		PublicReason:      input.PublicReason,
		InternalReason:    input.InternalReason,
		DecidedByAdminID:  input.AdminUserID,
		DecidedAt:         now,
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           1,
		DisputeVersion:    input.ExpectedVersion + 1,
		apiOrderDispute:   input.APIOrderDispute,
		remedyOverdueFact: input.RemedyOverdueFact,
	}
	s.outcomes[item.ID] = item
	return GovernanceMutationResult{Outcome: &item}, nil
}

func (s *Service) createUserRestrictionInMemory(input CreateRestrictionInput) (GovernanceMutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.SourceDisputeOutcomeID != "" {
		outcome, ok := s.outcomes[input.SourceDisputeOutcomeID]
		if !ok {
			return GovernanceMutationResult{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Outcome not found", "关联信誉裁定不存在。")
		}
		if outcome.Status != OutcomeStatusActive || outcome.SubjectUserID != input.UserID {
			return GovernanceMutationResult{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Outcome unavailable", "关联信誉裁定已反转或主体不匹配。")
		}
		if outcome.RoleScope != RoleAll && input.RoleScope != outcome.RoleScope {
			return GovernanceMutationResult{}, validationField("roleScope", "限制角色不能超出关联裁定角色。")
		}
		if outcome.apiOrderDispute && !outcome.remedyOverdueFact {
			return GovernanceMutationResult{}, apiOrderRemedyOutcomeUnavailable()
		}
	}
	now := s.now()
	item := UserRestriction{
		ID:                     uuid.NewString(),
		UserID:                 input.UserID,
		RestrictionType:        input.RestrictionType,
		RoleScope:              input.RoleScope,
		ActionCode:             input.ActionCode,
		ReasonCode:             input.ReasonCode,
		PublicReason:           input.PublicReason,
		InternalReason:         input.InternalReason,
		StartsAt:               input.StartsAt,
		EndsAt:                 input.EndsAt,
		SourceDisputeOutcomeID: input.SourceDisputeOutcomeID,
		CreatedByAdminID:       input.AdminUserID,
		CreatedAt:              now,
		UpdatedAt:              now,
		Version:                1,
		UserVersion:            input.ExpectedUserVersion + 1,
	}
	s.restrictions[item.ID] = item
	return GovernanceMutationResult{Restriction: &item}, nil
}

func (s *Service) revokeUserRestrictionInMemory(input RevokeRestrictionInput) (GovernanceMutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.restrictions[input.RestrictionID]
	if !ok {
		return GovernanceMutationResult{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Restriction not found", "信誉限制不存在。")
	}
	if item.Version != input.ExpectedVersion {
		return GovernanceMutationResult{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if item.RevokedAt != nil {
		return GovernanceMutationResult{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Restriction revoked", "信誉限制已经撤销。")
	}
	now := s.now()
	item.RevokedAt = &now
	item.RevokedByAdminID = input.AdminUserID
	item.RevocationReason = input.Reason
	item.UpdatedAt = now
	item.Version++
	s.restrictions[item.ID] = item
	return GovernanceMutationResult{Restriction: &item}, nil
}

func (s *Service) findActiveRestrictionInMemory(userID, role, action string, now time.Time) *UserRestriction {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.restrictions {
		if item.UserID != userID || item.RevokedAt != nil || now.Before(item.StartsAt) {
			continue
		}
		if item.EndsAt != nil && !now.Before(*item.EndsAt) {
			continue
		}
		if item.RoleScope != RoleAll && item.RoleScope != role {
			continue
		}
		if item.ActionCode != ActionAll && item.ActionCode != action {
			continue
		}
		copy := item
		return &copy
	}
	return nil
}

func normalizeUserIDs(userIDs []string) []string {
	result := make([]string, 0, len(userIDs))
	seen := make(map[string]struct{}, len(userIDs))
	for _, raw := range userIDs {
		userID := strings.TrimSpace(raw)
		if userID == "" {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		result = append(result, userID)
	}
	return result
}

func mergeScopeFacts(left, right ScopeFacts) ScopeFacts {
	platformReviewCount := left.PlatformReviewCount + right.PlatformReviewCount
	platformAverageRating := 0.0
	if platformReviewCount > 0 {
		weightedPlatformRating := float64(left.PlatformReviewCount)*left.PlatformAverageRating +
			float64(right.PlatformReviewCount)*right.PlatformAverageRating
		platformAverageRating = weightedPlatformRating / float64(platformReviewCount)
	}
	return ScopeFacts{
		Aggregated:                             left.Aggregated && right.Aggregated,
		CompletedCount:                         left.CompletedCount + right.CompletedCount,
		CompletedCountLast90Days:               left.CompletedCountLast90Days + right.CompletedCountLast90Days,
		RoleResponsibilityCancellationCount:    left.RoleResponsibilityCancellationCount + right.RoleResponsibilityCancellationCount,
		RoleResponsibilityCancellationCount90d: left.RoleResponsibilityCancellationCount90d + right.RoleResponsibilityCancellationCount90d,
		UnknownResponsibilityCancellationCount: left.UnknownResponsibilityCancellationCount + right.UnknownResponsibilityCancellationCount,
		UnresolvedDisputeCount:                 left.UnresolvedDisputeCount + right.UnresolvedDisputeCount,
		ConfirmedFaultDisputeCount365d:         left.ConfirmedFaultDisputeCount365d + right.ConfirmedFaultDisputeCount365d,
		ConfirmedMajorFaultDisputeCount365d:    left.ConfirmedMajorFaultDisputeCount365d + right.ConfirmedMajorFaultDisputeCount365d,
		ActiveRestrictionCount:                 left.ActiveRestrictionCount + right.ActiveRestrictionCount,
		VerifiedReviewCount:                    left.VerifiedReviewCount + right.VerifiedReviewCount,
		RatingSum:                              left.RatingSum + right.RatingSum,
		RatingDistribution:                     mergeRatingDistribution(left.RatingDistribution, right.RatingDistribution),
		RecentReviewCount90d:                   left.RecentReviewCount90d + right.RecentReviewCount90d,
		PlatformReviewCount:                    platformReviewCount,
		PlatformAverageRating:                  platformAverageRating,
		SourceAuthorMismatch:                   left.SourceAuthorMismatch || right.SourceAuthorMismatch,
		SourceAuthorVerification:               mergeSourceAuthorAggregates(left.SourceAuthorVerification, right.SourceAuthorVerification),
		SourceDataUpdatedAt:                    latestTime(left.SourceDataUpdatedAt, right.SourceDataUpdatedAt),
		NextRecalculationAt:                    earliestTime(left.NextRecalculationAt, right.NextRecalculationAt),
	}
}

func mergeRatingDistribution(left, right RatingDistribution) RatingDistribution {
	return RatingDistribution{
		One:   left.One + right.One,
		Two:   left.Two + right.Two,
		Three: left.Three + right.Three,
		Four:  left.Four + right.Four,
		Five:  left.Five + right.Five,
	}
}

func latestTime(left, right *time.Time) *time.Time {
	if left == nil {
		return right
	}
	if right == nil || left.After(*right) {
		return left
	}
	return right
}

func earliestTime(left, right *time.Time) *time.Time {
	if left == nil {
		return right
	}
	if right == nil || left.Before(*right) {
		return left
	}
	return right
}

func validateAdminActor(actor AdminActor) *domain.AppError {
	if !actor.IsAdmin || strings.TrimSpace(actor.UserID) == "" {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "只有管理员可以排除或恢复信誉交易。")
	}
	return nil
}

func validateCreateOutcomeInput(input CreateOutcomeInput) *domain.AppError {
	if _, err := uuid.Parse(strings.TrimSpace(input.DisputeCaseID)); err != nil {
		return validationField("disputeCaseId", "纠纷 ID 必须是 UUID。")
	}
	if _, err := uuid.Parse(strings.TrimSpace(input.SubjectUserID)); err != nil {
		return validationField("subjectUserId", "主体用户 ID 必须是 UUID。")
	}
	if !validResponsibility(input.Responsibility) {
		return validationField("responsibility", "责任类型不支持。")
	}
	if !validSeverity(input.Severity) {
		return validationField("severity", "严重度不支持。")
	}
	if (input.Responsibility == ResponsibilityNotResponsible || input.Responsibility == ResponsibilityUndetermined) && input.Severity != SeverityNone {
		return validationField("severity", "未认定责任时严重度必须为 none。")
	}
	if !validRole(input.RoleScope) {
		return validationField("roleScope", "角色范围不支持。")
	}
	if !reasonCodePattern.MatchString(strings.TrimSpace(input.ReasonCode)) {
		return validationField("reasonCode", "原因代码格式不正确。")
	}
	if strings.TrimSpace(input.PublicReason) == "" {
		return validationField("publicReason", "必须填写公开原因。")
	}
	if strings.TrimSpace(input.InternalReason) == "" {
		return validationField("internalReason", "必须填写内部裁定原因。")
	}
	if input.ExpectedVersion <= 0 {
		return validationField("If-Match", "必须提供当前纠纷版本。")
	}
	if input.APIOrderDispute && faultResponsibility(input.Responsibility) && !input.RemedyOverdueFact {
		return apiOrderRemedyOutcomeUnavailable()
	}
	return nil
}

func faultResponsibility(value string) bool {
	return value == ResponsibilityResponsible || value == ResponsibilityShared
}

func apiOrderRemedyOutcomeUnavailable() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Overdue remedy required", "API 订单责任裁定只能基于管理员已确认的逾期未履行事实。")
}

func validateCreateRestrictionInput(input CreateRestrictionInput) *domain.AppError {
	if _, err := uuid.Parse(strings.TrimSpace(input.UserID)); err != nil {
		return validationField("userId", "用户 ID 必须是 UUID。")
	}
	if strings.TrimSpace(input.RestrictionType) == "" {
		return validationField("restrictionType", "必须填写限制类型。")
	}
	if !validRole(input.RoleScope) {
		return validationField("roleScope", "角色范围不支持。")
	}
	if !validAction(input.ActionCode) {
		return validationField("actionCode", "动作代码不支持。")
	}
	if !reasonCodePattern.MatchString(strings.TrimSpace(input.ReasonCode)) {
		return validationField("reasonCode", "原因代码格式不正确。")
	}
	if strings.TrimSpace(input.PublicReason) == "" {
		return validationField("publicReason", "必须填写公开原因。")
	}
	if strings.TrimSpace(input.InternalReason) == "" {
		return validationField("internalReason", "必须填写内部限制原因。")
	}
	if input.EndsAt != nil && !input.EndsAt.After(input.StartsAt) {
		return validationField("endsAt", "结束时间必须晚于开始时间。")
	}
	if input.SourceDisputeOutcomeID != "" {
		if _, err := uuid.Parse(strings.TrimSpace(input.SourceDisputeOutcomeID)); err != nil {
			return validationField("sourceDisputeOutcomeId", "关联裁定 ID 必须是 UUID。")
		}
	}
	if input.ExpectedUserVersion <= 0 {
		return validationField("If-Match", "必须提供当前用户版本。")
	}
	return nil
}

func validateRevokeRestrictionInput(input RevokeRestrictionInput) *domain.AppError {
	if _, err := uuid.Parse(strings.TrimSpace(input.RestrictionID)); err != nil {
		return validationField("restrictionId", "信誉限制 ID 必须是 UUID。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return validationField("reason", "必须填写撤销原因。")
	}
	if input.ExpectedVersion <= 0 {
		return validationField("If-Match", "必须提供当前限制版本。")
	}
	return nil
}

func validResponsibility(value string) bool {
	switch strings.TrimSpace(value) {
	case ResponsibilityResponsible, ResponsibilityShared, ResponsibilityNotResponsible, ResponsibilityUndetermined:
		return true
	default:
		return false
	}
}

func validSeverity(value string) bool {
	switch strings.TrimSpace(value) {
	case SeverityNone, SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	default:
		return false
	}
}

func validRole(value string) bool {
	switch strings.TrimSpace(value) {
	case RoleBuyer, RoleSeller, RoleAll:
		return true
	default:
		return false
	}
}

func validAction(value string) bool {
	switch strings.TrimSpace(value) {
	case ActionCarpoolPublish, ActionCarpoolApply, ActionCarpoolAccept, ActionAPIServicePublish, ActionAPIOrderCreate, ActionContactView, ActionReviewSubmit, ActionAll:
		return true
	default:
		return false
	}
}

func validationField(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reputation governance validation failed", detail, field, "invalid", detail)
}

func nonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func validateExclusionMutation(adminUserID, transactionType, transactionID, reasonCode, reason string) (ExclusionMutation, *domain.AppError) {
	transactionType = strings.TrimSpace(transactionType)
	if !validTransactionType(transactionType) {
		return ExclusionMutation{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Transaction type invalid", "交易类型不支持信誉排除。", "transactionType", "invalid", "交易类型不支持信誉排除。")
	}
	transactionID = strings.TrimSpace(transactionID)
	if _, err := uuid.Parse(transactionID); err != nil {
		return ExclusionMutation{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Transaction ID invalid", "交易 ID 必须是 UUID。", "transactionId", "invalid", "交易 ID 必须是 UUID。")
	}
	reasonCode = strings.TrimSpace(reasonCode)
	if !reasonCodePattern.MatchString(reasonCode) {
		return ExclusionMutation{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason code invalid", "原因代码格式不正确。", "reasonCode", "invalid", "原因代码必须使用小写字母、数字和下划线。")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ExclusionMutation{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "必须填写排除或恢复原因。", "reason", "required", "必须填写排除或恢复原因。")
	}
	return ExclusionMutation{
		TransactionType: transactionType,
		TransactionID:   transactionID,
		AdminUserID:     strings.TrimSpace(adminUserID),
		ReasonCode:      reasonCode,
		Reason:          reason,
	}, nil
}

func validTransactionType(value string) bool {
	switch value {
	case TransactionCarpoolApplication, TransactionCarpoolMembership, TransactionAPIPurchaseIntent, TransactionAPIOrder:
		return true
	default:
		return false
	}
}

func unavailableRepositoryError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "信誉仓储不可用。")
}
