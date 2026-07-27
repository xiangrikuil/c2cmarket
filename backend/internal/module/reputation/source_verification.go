package reputation

import (
	"context"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
)

func (s *Service) GetSourceAuthorVerificationAudit(
	ctx context.Context,
	actor AdminActor,
	resourceType string,
	resourceID string,
) (SourceAuthorVerificationAudit, *domain.AppError) {
	if !actor.IsAdmin {
		return SourceAuthorVerificationAudit{}, sourceAuthorPermissionError()
	}
	resourceType, resourceID, appErr := validateSourceAuthorResource(resourceType, resourceID)
	if appErr != nil {
		return SourceAuthorVerificationAudit{}, appErr
	}
	if s == nil || s.sourceRepo == nil {
		return SourceAuthorVerificationAudit{}, unavailableSourceAuthorRepositoryError()
	}
	return s.sourceRepo.GetSourceAuthorVerificationAudit(ctx, resourceType, resourceID, s.now())
}

func (s *Service) UpdateSourceAuthorVerification(
	ctx context.Context,
	actor AdminActor,
	input UpdateSourceAuthorVerificationInput,
) (SourceAuthorVerificationAudit, *domain.AppError) {
	if !actor.IsAdmin {
		return SourceAuthorVerificationAudit{}, sourceAuthorPermissionError()
	}
	resourceType, resourceID, appErr := validateSourceAuthorResource(input.ResourceType, input.ResourceID)
	if appErr != nil {
		return SourceAuthorVerificationAudit{}, appErr
	}
	input.ResourceType = resourceType
	input.ResourceID = resourceID
	input.AdminUserID = strings.TrimSpace(actor.UserID)
	input.Status = strings.TrimSpace(input.Status)
	input.ActualExternalUserID = strings.TrimSpace(input.ActualExternalUserID)
	input.VerificationMethod = strings.TrimSpace(input.VerificationMethod)
	input.FailureReason = strings.TrimSpace(input.FailureReason)
	now := s.now()
	if appErr := validateSourceAuthorUpdate(input, now); appErr != nil {
		return SourceAuthorVerificationAudit{}, appErr
	}
	if s == nil || s.sourceRepo == nil {
		return SourceAuthorVerificationAudit{}, unavailableSourceAuthorRepositoryError()
	}
	return s.sourceRepo.UpdateSourceAuthorVerification(ctx, input, now)
}

func NormalizeSourceAuthorResourceSummary(value SourceAuthorResourceSummary) SourceAuthorResourceSummary {
	if !validSourceVerificationStatus(value.Status) {
		value.Status = SourceVerificationNotSubmitted
		value.VerifiedAt = nil
		value.ExpiresAt = nil
	}
	return value
}

func SourceAuthorAggregateForCounts(role string, counts SourceAuthorStatusCounts) SourceAuthorAggregate {
	if role == RoleBuyer {
		return SourceAuthorAggregate{State: SourceAggregateNotApplicable, Counts: counts}
	}
	state := SourceAggregatePending
	switch {
	case counts.Total == 0:
		state = SourceAggregateNoSources
	case counts.Mismatch > 0:
		state = SourceAggregateMismatch
	case counts.Verified == counts.Total:
		state = SourceAggregateVerified
	case counts.Verified > 0:
		state = SourceAggregatePartial
	}
	return SourceAuthorAggregate{State: state, Counts: counts}
}

func mergeSourceAuthorAggregates(left, right SourceAuthorAggregate) SourceAuthorAggregate {
	counts := SourceAuthorStatusCounts{
		Total:        left.Counts.Total + right.Counts.Total,
		NotSubmitted: left.Counts.NotSubmitted + right.Counts.NotSubmitted,
		Pending:      left.Counts.Pending + right.Counts.Pending,
		Verified:     left.Counts.Verified + right.Counts.Verified,
		Mismatch:     left.Counts.Mismatch + right.Counts.Mismatch,
		Expired:      left.Counts.Expired + right.Counts.Expired,
	}
	if left.State == SourceAggregateNotApplicable && right.State == SourceAggregateNotApplicable {
		return SourceAuthorAggregateForCounts(RoleBuyer, counts)
	}
	return SourceAuthorAggregateForCounts(RoleSeller, counts)
}

func validateSourceAuthorResource(resourceType, resourceID string) (string, string, *domain.AppError) {
	resourceType = strings.TrimSpace(resourceType)
	resourceID = strings.TrimSpace(resourceID)
	if resourceType != SourceResourceCarpool && resourceType != SourceResourceAPIService {
		return "", "", sourceAuthorFieldError("resourceType", "资源类型必须是 carpool 或 api_service。")
	}
	if uuid.Validate(resourceID) != nil {
		return "", "", sourceAuthorFieldError("resourceId", "资源 ID 必须是 UUID。")
	}
	return resourceType, resourceID, nil
}

func validateSourceAuthorUpdate(input UpdateSourceAuthorVerificationInput, now time.Time) *domain.AppError {
	if input.AdminUserID == "" {
		return sourceAuthorPermissionError()
	}
	if !validSourceVerificationStatus(input.Status) {
		return sourceAuthorFieldError("status", "验证状态无效。")
	}
	if input.ExpectedVersion < 0 {
		return sourceAuthorFieldError("If-Match", "验证版本不能小于 0。")
	}
	if len([]rune(input.ActualExternalUserID)) > 128 {
		return sourceAuthorFieldError("actualExternalUserId", "实际 linux.do 用户 ID 过长。")
	}
	if len([]rune(input.VerificationMethod)) > 64 {
		return sourceAuthorFieldError("verificationMethod", "验证方法过长。")
	}
	if len([]rune(input.FailureReason)) > 1000 {
		return sourceAuthorFieldError("failureReason", "失败原因过长。")
	}
	if input.Status == SourceVerificationVerified || input.Status == SourceVerificationMismatch {
		if input.ActualExternalUserID == "" {
			return sourceAuthorFieldError("actualExternalUserId", "已验证或不匹配状态必须填写实际 linux.do 用户 ID。")
		}
		if input.VerificationMethod == "" {
			return sourceAuthorFieldError("verificationMethod", "已验证或不匹配状态必须填写验证方法。")
		}
	}
	if input.Status == SourceVerificationMismatch && input.FailureReason == "" {
		return sourceAuthorFieldError("failureReason", "不匹配状态必须填写原因。")
	}
	if input.Status == SourceVerificationVerified && input.ExpiresAt != nil && !input.ExpiresAt.After(now) {
		return sourceAuthorFieldError("expiresAt", "已验证状态的过期时间必须晚于当前时间。")
	}
	return nil
}

func validSourceVerificationStatus(value string) bool {
	switch value {
	case SourceVerificationNotSubmitted,
		SourceVerificationPending,
		SourceVerificationVerified,
		SourceVerificationMismatch,
		SourceVerificationExpired:
		return true
	default:
		return false
	}
}

func sourceAuthorFieldError(field, detail string) *domain.AppError {
	return domain.NewFieldError(
		http.StatusUnprocessableEntity,
		domain.CodeValidationFailed,
		"Source author verification validation failed",
		detail,
		field,
		"invalid",
		detail,
	)
}

func sourceAuthorPermissionError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
}

func unavailableSourceAuthorRepositoryError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "原帖作者验证仓储不可用。")
}
