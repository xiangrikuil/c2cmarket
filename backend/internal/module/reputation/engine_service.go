package reputation

import (
	"context"
	"net/http"
	"strings"

	"c2c-market/backend/internal/domain"
)

const recalculationBatchSize = 200

func (s *Service) Rules() RuleSet {
	return CurrentRules()
}

func (s *Service) EngineAvailable() bool {
	return s != nil && s.engineRepo != nil
}

func (s *Service) GetMany(ctx context.Context, userIDs []string, role, scope string) (map[string]ReputationSnapshot, *domain.AppError) {
	role = strings.TrimSpace(role)
	scope = strings.TrimSpace(scope)
	if !validEngineRole(role) {
		return nil, engineValidationError("role", "信誉角色必须是 buyer 或 seller。")
	}
	if !validScope(scope) {
		return nil, engineValidationError("scope", "信誉范围必须是 overall、carpool 或 api。")
	}
	normalized := normalizeUserIDs(userIDs)
	keys := make([]SnapshotKey, 0, len(normalized))
	for _, userID := range normalized {
		keys = append(keys, SnapshotKey{UserID: userID, Role: role, Scope: scope})
	}
	snapshots, appErr := s.getSnapshots(ctx, keys, false)
	if appErr != nil {
		return nil, appErr
	}
	result := make(map[string]ReputationSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		result[snapshot.UserID] = snapshot
	}
	return result, nil
}

func (s *Service) GetUserScope(ctx context.Context, userID, scope string) ([]ReputationSnapshot, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	scope = strings.TrimSpace(scope)
	if userID == "" {
		return nil, engineValidationError("userId", "用户 ID 不能为空。")
	}
	if !validScope(scope) {
		return nil, engineValidationError("scope", "信誉范围必须是 overall、carpool 或 api。")
	}
	return s.getSnapshots(ctx, []SnapshotKey{
		{UserID: userID, Role: RoleBuyer, Scope: scope},
		{UserID: userID, Role: RoleSeller, Scope: scope},
	}, false)
}

func (s *Service) GetUserReputation(ctx context.Context, userID string) ([]ReputationSnapshot, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, engineValidationError("userId", "用户 ID 不能为空。")
	}
	return s.getSnapshots(ctx, allSnapshotKeys([]string{userID}), false)
}

func (s *Service) RecalculateUser(ctx context.Context, userID string) (RecalculationResult, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return RecalculationResult{}, engineValidationError("userId", "用户 ID 不能为空。")
	}
	snapshots, appErr := s.getSnapshots(ctx, allSnapshotKeys([]string{userID}), true)
	if appErr != nil {
		return RecalculationResult{}, appErr
	}
	return RecalculationResult{
		RequestedUsers: 1,
		RebuiltStates:  len(snapshots),
		CompletedAt:    s.now(),
	}, nil
}

func (s *Service) RecalculateAll(ctx context.Context) (RecalculationResult, *domain.AppError) {
	if s.engineRepo == nil {
		return RecalculationResult{}, unavailableEngineRepositoryError()
	}
	userIDs, appErr := s.engineRepo.ListReputationUserIDs(ctx)
	if appErr != nil {
		return RecalculationResult{}, appErr
	}
	result := RecalculationResult{RequestedUsers: len(userIDs)}
	for start := 0; start < len(userIDs); start += recalculationBatchSize {
		end := start + recalculationBatchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		snapshots, appErr := s.getSnapshots(ctx, allSnapshotKeys(userIDs[start:end]), true)
		if appErr != nil {
			return RecalculationResult{}, appErr
		}
		result.RebuiltStates += len(snapshots)
	}
	result.CompletedAt = s.now()
	return result, nil
}

func (s *Service) History(ctx context.Context, userID string, limit int) ([]ReputationHistory, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, engineValidationError("userId", "用户 ID 不能为空。")
	}
	if limit <= 0 || limit > 100 {
		return nil, engineValidationError("limit", "历史记录条数必须在 1 到 100 之间。")
	}
	if s.engineRepo == nil {
		return nil, unavailableEngineRepositoryError()
	}
	return s.engineRepo.ListReputationHistory(ctx, userID, limit)
}

func (s *Service) AdminEvidence(ctx context.Context, userID string) (AdminReputationEvidence, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AdminReputationEvidence{}, engineValidationError("userId", "用户 ID 不能为空。")
	}
	if s.auditRepo == nil {
		return AdminReputationEvidence{
			Restrictions:              []UserRestriction{},
			Outcomes:                  []DisputeOutcome{},
			Appeals:                   []ReputationAppeal{},
			SourceAuthorVerifications: []SourceAuthorVerificationAudit{},
		}, nil
	}
	return s.auditRepo.LoadAdminReputationEvidence(ctx, userID, s.now())
}

func (s *Service) getSnapshots(ctx context.Context, keys []SnapshotKey, force bool) ([]ReputationSnapshot, *domain.AppError) {
	if len(keys) == 0 {
		return []ReputationSnapshot{}, nil
	}
	if s.engineRepo == nil {
		return nil, unavailableEngineRepositoryError()
	}

	userIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key.UserID) == "" || !validEngineRole(key.Role) || !validScope(key.Scope) {
			return nil, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "信誉快照键无效。")
		}
		userIDs = append(userIDs, key.UserID)
	}
	userIDs = normalizeUserIDs(userIDs)
	factsByUser, appErr := s.AggregateFacts(ctx, userIDs)
	if appErr != nil {
		return nil, appErr
	}
	cached, appErr := s.engineRepo.LoadReputationSnapshots(ctx, keys)
	if appErr != nil {
		return nil, appErr
	}

	now := s.now()
	rules := s.Rules()
	result := make([]ReputationSnapshot, 0, len(keys))
	changed := make([]ReputationSnapshot, 0, len(keys))
	for _, key := range keys {
		facts := scopeFactsValue(factsByUser[key.UserID], key.Role, key.Scope)
		previous, exists := cached[key]
		if !force && exists && SnapshotIsValid(previous, facts, now, rules.Version) {
			result = append(result, previous)
			continue
		}
		var previousPointer *ReputationSnapshot
		if exists {
			copy := previous
			previousPointer = &copy
		}
		snapshot := EvaluateSnapshot(key, facts, previousPointer, now, rules)
		result = append(result, snapshot)
		changed = append(changed, snapshot)
	}
	if appErr := s.engineRepo.SaveReputationSnapshots(ctx, changed); appErr != nil {
		return nil, appErr
	}
	return result, nil
}

func allSnapshotKeys(userIDs []string) []SnapshotKey {
	normalized := normalizeUserIDs(userIDs)
	keys := make([]SnapshotKey, 0, len(normalized)*6)
	for _, userID := range normalized {
		for _, role := range []string{RoleBuyer, RoleSeller} {
			for _, scope := range []string{ScopeOverall, ScopeCarpool, ScopeAPI} {
				keys = append(keys, SnapshotKey{UserID: userID, Role: role, Scope: scope})
			}
		}
	}
	return keys
}

func scopeFactsValue(value RawFacts, role, scope string) ScopeFacts {
	var roleFacts RoleFacts
	if role == RoleBuyer {
		roleFacts = value.Buyer
	} else {
		roleFacts = value.Seller
	}
	switch scope {
	case ScopeCarpool:
		return roleFacts.Carpool
	case ScopeAPI:
		return roleFacts.API
	default:
		return roleFacts.Overall
	}
}

func validEngineRole(value string) bool {
	return value == RoleBuyer || value == RoleSeller
}

func validScope(value string) bool {
	switch value {
	case ScopeOverall, ScopeCarpool, ScopeAPI:
		return true
	default:
		return false
	}
}

func engineValidationError(field, detail string) *domain.AppError {
	return domain.NewFieldError(
		http.StatusUnprocessableEntity,
		domain.CodeValidationFailed,
		"Reputation validation failed",
		detail,
		field,
		"invalid",
		detail,
	)
}

func unavailableEngineRepositoryError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "信誉引擎仓储不可用。")
}
