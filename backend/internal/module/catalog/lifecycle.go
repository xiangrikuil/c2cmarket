package catalog

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

func (s *Service) ApplyCatalogLifecycleWithIdempotency(
	ctx context.Context,
	user auth.User,
	routeKey, key, requestHash string,
	input LifecycleActionInput,
	buildCompletion LifecycleCompletionBuilder,
) (idempotency.Completion, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.OperatorID = user.ID
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.ResourceID = strings.TrimSpace(input.ResourceID)
	input.Action = strings.TrimSpace(input.Action)
	input.Reason = strings.TrimSpace(input.Reason)
	input.TargetStatus = strings.TrimSpace(input.TargetStatus)
	if appErr := validateLifecycleAction(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if s.idempotency == nil || buildCompletion == nil {
		return idempotency.Completion{}, internalCatalogError()
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.AdminApplyLifecycleWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	result, appErr := s.applyLifecycleInMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
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

func validateLifecycleAction(input LifecycleActionInput) *domain.AppError {
	if input.ResourceID == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Catalog resource required", "目录资源不能为空。", "id", "required", "目录资源不能为空。")
	}
	switch input.ResourceType {
	case ResourceProductCategory, ResourceProductPlan, ResourceAPIProvider, ResourceAPIModel:
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Catalog resource invalid", "目录资源类型无效。", "resourceType", "invalid", "目录资源类型无效。")
	}
	if utf8.RuneCountInString(input.Reason) < 2 || utf8.RuneCountInString(input.Reason) > 500 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Lifecycle reason invalid", "状态变更原因需为 2 到 500 个字符。", "reason", "invalid_length", "状态变更原因需为 2 到 500 个字符。")
	}
	switch input.Action {
	case LifecycleActionDeprecate, LifecycleActionBlock, LifecycleActionReactivate:
		if input.TargetStatus != "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Target status not allowed", "仅解除阻断动作可以指定目标状态。", "targetStatus", "not_allowed", "仅解除阻断动作可以指定目标状态。")
		}
	case LifecycleActionUnblock:
		if input.TargetStatus != StatusActive && input.TargetStatus != StatusDeprecated {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Target status invalid", "解除阻断后必须选择 active 或 deprecated。", "targetStatus", "invalid", "解除阻断后必须选择 active 或 deprecated。")
		}
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Lifecycle action invalid", "目录状态动作无效。", "action", "invalid", "目录状态动作无效。")
	}
	return nil
}

func lifecycleTargetStatus(action, unblockTarget string) string {
	switch action {
	case LifecycleActionDeprecate:
		return StatusDeprecated
	case LifecycleActionBlock:
		return StatusBlocked
	case LifecycleActionReactivate:
		return StatusActive
	case LifecycleActionUnblock:
		return unblockTarget
	default:
		return ""
	}
}

func validateLifecycleTransition(currentStatus string, input LifecycleActionInput) *domain.AppError {
	allowed := false
	switch input.Action {
	case LifecycleActionDeprecate:
		allowed = currentStatus == StatusActive
	case LifecycleActionBlock:
		allowed = currentStatus == StatusActive || currentStatus == StatusDeprecated
	case LifecycleActionReactivate:
		allowed = currentStatus == StatusDeprecated
	case LifecycleActionUnblock:
		allowed = currentStatus == StatusBlocked
	}
	if !allowed {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Catalog lifecycle transition invalid", "当前目录状态不允许执行该动作。")
	}
	return nil
}

func catalogVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "目录版本已变化，请刷新后重试。")
}

func (s *Service) applyLifecycleInMemory(input LifecycleActionInput) (LifecycleMutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	targetStatus := lifecycleTargetStatus(input.Action, input.TargetStatus)

	switch input.ResourceType {
	case ResourceProductCategory:
		item, ok := s.categories[input.ResourceID]
		if !ok {
			return LifecycleMutationResult{}, productCategoryNotFound()
		}
		if item.Version != input.ExpectedVersion {
			return LifecycleMutationResult{}, catalogVersionConflict()
		}
		if appErr := validateLifecycleTransition(item.Status, input); appErr != nil {
			return LifecycleMutationResult{}, appErr
		}
		item.Lifecycle = changedLifecycle(item.Lifecycle, targetStatus, input, now)
		item.Active = item.IsEffectiveActive()
		s.categories[item.ID] = item
		for id, plan := range s.productPlans {
			if plan.CategoryID == item.ID {
				plan = withProductPlanCategory(plan, item)
				s.productPlans[id] = plan
			}
		}
		return LifecycleMutationResult{ResourceType: input.ResourceType, Category: &item}, nil
	case ResourceProductPlan:
		item, ok := s.productPlans[input.ResourceID]
		if !ok {
			return LifecycleMutationResult{}, productPlanNotFound()
		}
		if item.Version != input.ExpectedVersion {
			return LifecycleMutationResult{}, catalogVersionConflict()
		}
		if appErr := validateLifecycleTransition(item.Status, input); appErr != nil {
			return LifecycleMutationResult{}, appErr
		}
		parentStatus := categoryStatusForPlanLocked(s.categories, item.CategoryID)
		if targetStatus == StatusActive && parentStatus != StatusActive {
			return LifecycleMutationResult{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Catalog parent unavailable", "父级分类当前不可用，不能恢复该套餐。")
		}
		item.Lifecycle = changedLifecycle(item.Lifecycle, targetStatus, input, now)
		item.EffectiveStatus = effectiveStatus(item.Status, parentStatus)
		item.EffectiveStatusSource = effectiveStatusSource(item.Status, parentStatus)
		item.Active = item.IsEffectiveActive()
		item.UpdatedAt = now
		s.productPlans[item.ID] = item
		return LifecycleMutationResult{ResourceType: input.ResourceType, Plan: &item}, nil
	case ResourceAPIProvider:
		item, ok := s.apiProviders[input.ResourceID]
		if !ok {
			return LifecycleMutationResult{}, apiModelProviderNotFound()
		}
		if item.Version != input.ExpectedVersion {
			return LifecycleMutationResult{}, catalogVersionConflict()
		}
		if appErr := validateLifecycleTransition(item.Status, input); appErr != nil {
			return LifecycleMutationResult{}, appErr
		}
		item.Lifecycle = changedLifecycle(item.Lifecycle, targetStatus, input, now)
		item.Active = item.IsEffectiveActive()
		item.UpdatedAt = now
		s.apiProviders[item.ID] = item
		for id, model := range s.apiModels {
			if model.ProviderID == item.ID {
				s.apiModels[id] = withAPIModelProvider(model, item)
			}
		}
		return LifecycleMutationResult{ResourceType: input.ResourceType, Provider: &item}, nil
	case ResourceAPIModel:
		item, ok := s.apiModels[input.ResourceID]
		if !ok {
			return LifecycleMutationResult{}, apiModelNotFound()
		}
		if item.Version != input.ExpectedVersion {
			return LifecycleMutationResult{}, catalogVersionConflict()
		}
		if appErr := validateLifecycleTransition(item.Status, input); appErr != nil {
			return LifecycleMutationResult{}, appErr
		}
		parentStatus := providerStatusForModelLocked(s.apiProviders, item.ProviderID)
		if targetStatus == StatusActive && parentStatus != StatusActive {
			return LifecycleMutationResult{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Catalog parent unavailable", "父级提供商当前不可用，不能恢复该模型。")
		}
		item.Lifecycle = changedLifecycle(item.Lifecycle, targetStatus, input, now)
		item.EffectiveStatus = effectiveStatus(item.Status, parentStatus)
		item.EffectiveStatusSource = effectiveStatusSource(item.Status, parentStatus)
		item.Active = item.IsEffectiveActive()
		item.UpdatedAt = now
		s.apiModels[item.ID] = item
		return LifecycleMutationResult{ResourceType: input.ResourceType, Model: &item}, nil
	default:
		return LifecycleMutationResult{}, internalCatalogError()
	}
}

func changedLifecycle(current Lifecycle, targetStatus string, input LifecycleActionInput, now time.Time) Lifecycle {
	current.Status = targetStatus
	current.EffectiveStatus = targetStatus
	current.EffectiveStatusSource = EffectiveStatusSourceSelf
	current.StatusChangedAt = now
	current.StatusReason = input.Reason
	current.StatusChangedBy = input.OperatorID
	current.Version++
	return current
}
