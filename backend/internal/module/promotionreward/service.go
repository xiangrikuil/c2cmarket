package promotionreward

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

type Service struct {
	repo        Repository
	idempotency *idempotency.Service
	now         func() time.Time
}

func NewService(repo Repository, idempotencyService *idempotency.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, idempotency: idempotencyService, now: now}
}

func (s *Service) PublicConfig(ctx context.Context) (PublicConfig, *domain.AppError) {
	if s == nil || s.repo == nil {
		return PublicConfig{}, nil
	}
	return s.repo.GetPromotionRewardPublicConfig(ctx, s.now().UTC())
}

func (s *Service) MyReferral(ctx context.Context, user auth.User) (ReferralSummary, *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return ReferralSummary{}, appErr
	}
	if s == nil || s.repo == nil {
		return ReferralSummary{Records: []ReferralRecord{}}, featureDisabled()
	}
	summary, appErr := s.repo.GetReferralSummary(ctx, user.ID, s.now().UTC())
	if appErr != nil {
		return ReferralSummary{}, appErr
	}
	if !summary.Campaign.ProgramEnabled || !summary.Campaign.ReferralEnabled {
		return ReferralSummary{}, featureDisabled()
	}
	for i := range summary.Records {
		summary.Records[i].RiskFlags = nil
		summary.Records[i].InviterUserID = ""
		summary.Records[i].InviteeUserID = ""
	}
	return summary, nil
}

func (s *Service) MyCoupons(ctx context.Context, user auth.User, query CouponQuery) (CouponPage, *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return CouponPage{}, appErr
	}
	query = normalizeCouponQuery(query)
	if appErr := validateCouponQuery(query, false); appErr != nil {
		return CouponPage{}, appErr
	}
	if s == nil || s.repo == nil {
		return CouponPage{Items: []Coupon{}, Pagination: emptyPagination(query.Page, query.Limit)}, featureDisabled()
	}
	return s.repo.ListUserPromotionCoupons(ctx, user.ID, query, s.now().UTC())
}

func (s *Service) ApplyCouponWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input ApplyCouponInput, buildCompletion CouponCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.UserID = user.ID
	input.CouponID = strings.TrimSpace(input.CouponID)
	input.APIServiceID = strings.TrimSpace(input.APIServiceID)
	if input.CouponID == "" {
		return idempotency.Completion{}, validationError("couponId", "required", "必须提供推广券。")
	}
	if input.APIServiceID == "" {
		return idempotency.Completion{}, validationError("apiServiceId", "required", "必须选择 API 服务。")
	}
	if s == nil || s.repo == nil || s.idempotency == nil || buildCompletion == nil {
		return idempotency.Completion{}, internalError()
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	_, completion, appErr := s.repo.ApplyPromotionCouponWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) AdminCampaign(ctx context.Context, user auth.User) (Campaign, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Campaign{}, appErr
	}
	if s == nil || s.repo == nil {
		return Campaign{}, internalError()
	}
	return s.repo.GetAdminPromotionRewardCampaign(ctx)
}

func (s *Service) UpdateAdminCampaign(ctx context.Context, user auth.User, input UpdateCampaignInput) (Campaign, *domain.AppError) {
	input, appErr := prepareUpdateCampaignInput(user, input)
	if appErr != nil {
		return Campaign{}, appErr
	}
	if s == nil || s.repo == nil {
		return Campaign{}, internalError()
	}
	return s.repo.UpdateAdminPromotionRewardCampaign(ctx, input, s.now().UTC())
}

func (s *Service) UpdateAdminCampaignWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input UpdateCampaignInput, buildCompletion CampaignCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input, appErr := prepareUpdateCampaignInput(user, input)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, replay, appErr := s.beginMutationWithIdempotency(ctx, user.ID, routeKey, key, requestHash, buildCompletion != nil)
	if appErr != nil || replay != nil {
		return completionOrZero(replay), appErr
	}
	_, completion, appErr := s.repo.UpdateAdminPromotionRewardCampaignWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) AdminReferrals(ctx context.Context, user auth.User, query ReferralQuery) (ReferralPage, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return ReferralPage{}, appErr
	}
	query = normalizeReferralQuery(query)
	if appErr := validateReferralQuery(query); appErr != nil {
		return ReferralPage{}, appErr
	}
	if s == nil || s.repo == nil {
		return ReferralPage{}, internalError()
	}
	return s.repo.ListAdminReferrals(ctx, query)
}

func (s *Service) RevokeAdminReferral(ctx context.Context, user auth.User, input RevokeReferralInput) (ReferralRecord, *domain.AppError) {
	input, appErr := prepareRevokeReferralInput(user, input)
	if appErr != nil {
		return ReferralRecord{}, appErr
	}
	if s == nil || s.repo == nil {
		return ReferralRecord{}, internalError()
	}
	return s.repo.RevokeAdminReferral(ctx, input, s.now().UTC())
}

func (s *Service) RevokeAdminReferralWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input RevokeReferralInput, buildCompletion ReferralCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input, appErr := prepareRevokeReferralInput(user, input)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, replay, appErr := s.beginMutationWithIdempotency(ctx, user.ID, routeKey, key, requestHash, buildCompletion != nil)
	if appErr != nil || replay != nil {
		return completionOrZero(replay), appErr
	}
	_, completion, appErr := s.repo.RevokeAdminReferralWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) AdminCoupons(ctx context.Context, user auth.User, query CouponQuery) (CouponPage, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return CouponPage{}, appErr
	}
	query = normalizeCouponQuery(query)
	if appErr := validateCouponQuery(query, true); appErr != nil {
		return CouponPage{}, appErr
	}
	if s == nil || s.repo == nil {
		return CouponPage{}, internalError()
	}
	return s.repo.ListAdminPromotionCoupons(ctx, query, s.now().UTC())
}

func (s *Service) GrantAdminCoupon(ctx context.Context, user auth.User, input GrantCouponInput) (Coupon, *domain.AppError) {
	input, appErr := prepareGrantCouponInput(user, input)
	if appErr != nil {
		return Coupon{}, appErr
	}
	if s == nil || s.repo == nil {
		return Coupon{}, internalError()
	}
	return s.repo.GrantAdminPromotionCoupon(ctx, input, s.now().UTC())
}

func (s *Service) GrantAdminCouponWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input GrantCouponInput, buildCompletion CouponCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input, appErr := prepareGrantCouponInput(user, input)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, replay, appErr := s.beginMutationWithIdempotency(ctx, user.ID, routeKey, key, requestHash, buildCompletion != nil)
	if appErr != nil || replay != nil {
		return completionOrZero(replay), appErr
	}
	_, completion, appErr := s.repo.GrantAdminPromotionCouponWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) RevokeAdminCoupon(ctx context.Context, user auth.User, input RevokeCouponInput) (Coupon, *domain.AppError) {
	input, appErr := prepareRevokeCouponInput(user, input)
	if appErr != nil {
		return Coupon{}, appErr
	}
	if s == nil || s.repo == nil {
		return Coupon{}, internalError()
	}
	return s.repo.RevokeAdminPromotionCoupon(ctx, input, s.now().UTC())
}

func (s *Service) RevokeAdminCouponWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input RevokeCouponInput, buildCompletion CouponCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input, appErr := prepareRevokeCouponInput(user, input)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, replay, appErr := s.beginMutationWithIdempotency(ctx, user.ID, routeKey, key, requestHash, buildCompletion != nil)
	if appErr != nil || replay != nil {
		return completionOrZero(replay), appErr
	}
	_, completion, appErr := s.repo.RevokeAdminPromotionCouponWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) beginMutationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, completionBuilderPresent bool) (*idempotency.Entry, *idempotency.Completion, *domain.AppError) {
	if s == nil || s.repo == nil || s.idempotency == nil || !completionBuilderPresent {
		return nil, nil, internalError()
	}
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return nil, nil, appErr
	}
	if entry.State == "completed" {
		completion := idempotency.CompletionFromEntry(entry)
		return nil, &completion, nil
	}
	return entry, nil, nil
}

func completionOrZero(completion *idempotency.Completion) idempotency.Completion {
	if completion == nil {
		return idempotency.Completion{}
	}
	return *completion
}

func prepareUpdateCampaignInput(user auth.User, input UpdateCampaignInput) (UpdateCampaignInput, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return UpdateCampaignInput{}, appErr
	}
	input.AdminUserID = user.ID
	input.RulesText = strings.TrimSpace(input.RulesText)
	input.Reason = strings.TrimSpace(input.Reason)
	return input, validateCampaignInput(input)
}

func prepareRevokeReferralInput(user auth.User, input RevokeReferralInput) (RevokeReferralInput, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return RevokeReferralInput{}, appErr
	}
	input.AdminUserID = user.ID
	input.ReferralID = strings.TrimSpace(input.ReferralID)
	input.Reason = strings.TrimSpace(input.Reason)
	return input, validateRevocation(input.ReferralID, input.Reason, input.ExpectedVersion, "referralId")
}

func prepareGrantCouponInput(user auth.User, input GrantCouponInput) (GrantCouponInput, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return GrantCouponInput{}, appErr
	}
	input.AdminUserID = user.ID
	input.UserID = strings.TrimSpace(input.UserID)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.UserID == "" {
		return GrantCouponInput{}, validationError("userId", "required", "必须选择用户。")
	}
	if input.DurationHours < 1 || input.DurationHours > 168 {
		return GrantCouponInput{}, validationError("durationHours", "invalid", "推广时长必须在 1 到 168 小时之间。")
	}
	if input.ValidDays < 1 || input.ValidDays > 365 {
		return GrantCouponInput{}, validationError("validDays", "invalid", "有效期必须在 1 到 365 天之间。")
	}
	return input, validateReason(input.Reason)
}

func prepareRevokeCouponInput(user auth.User, input RevokeCouponInput) (RevokeCouponInput, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return RevokeCouponInput{}, appErr
	}
	input.AdminUserID = user.ID
	input.CouponID = strings.TrimSpace(input.CouponID)
	input.Reason = strings.TrimSpace(input.Reason)
	return input, validateRevocation(input.CouponID, input.Reason, input.ExpectedVersion, "couponId")
}

func normalizeCouponQuery(query CouponQuery) CouponQuery {
	if query.Page < 1 {
		query.Page = DefaultPage
	}
	if query.Limit < 1 {
		query.Limit = DefaultPageLimit
	}
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	if query.Status == "" {
		query.Status = CouponStatusAll
	}
	query.SourceType = strings.ToLower(strings.TrimSpace(query.SourceType))
	query.Search = strings.TrimSpace(query.Search)
	return query
}

func normalizeReferralQuery(query ReferralQuery) ReferralQuery {
	if query.Page < 1 {
		query.Page = DefaultPage
	}
	if query.Limit < 1 {
		query.Limit = DefaultPageLimit
	}
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	if query.Status == "" {
		query.Status = CouponStatusAll
	}
	query.Search = strings.TrimSpace(query.Search)
	return query
}

func validateCouponQuery(query CouponQuery, allowSearch bool) *domain.AppError {
	if query.Limit > MaximumPageLimit {
		return validationError("limit", "invalid", "每页数量不能超过 100。")
	}
	if !oneOf(query.Status, CouponStatusAll, CouponStatusPending, CouponStatusAvailable, CouponStatusUsed, CouponStatusExpired, CouponStatusRevoked) {
		return validationError("status", "invalid", "推广券状态筛选值无效。")
	}
	if query.SourceType != "" && !oneOf(query.SourceType, CouponSourceWelcome, CouponSourceReferralInviter, CouponSourceReferralInvitee, CouponSourceAdminGrant) {
		return validationError("source", "invalid", "推广券来源筛选值无效。")
	}
	if !allowSearch && query.Search != "" {
		return validationError("search", "not_allowed", "用户推广券列表不支持搜索。")
	}
	if utf8.RuneCountInString(query.Search) > 100 {
		return validationError("search", "too_long", "搜索内容不能超过 100 个字符。")
	}
	return nil
}

func validateReferralQuery(query ReferralQuery) *domain.AppError {
	if query.Limit > MaximumPageLimit {
		return validationError("limit", "invalid", "每页数量不能超过 100。")
	}
	if !oneOf(query.Status, CouponStatusAll, ReferralStatusBound, ReferralStatusQualified, ReferralStatusRewarded, ReferralStatusRejected, ReferralStatusRevoked) {
		return validationError("status", "invalid", "邀请状态筛选值无效。")
	}
	if utf8.RuneCountInString(query.Search) > 100 {
		return validationError("search", "too_long", "搜索内容不能超过 100 个字符。")
	}
	return nil
}

func validateCampaignInput(input UpdateCampaignInput) *domain.AppError {
	if input.ExpectedVersion < 1 {
		return validationError("version", "required", "必须提供当前活动版本。")
	}
	if input.StartsAt.IsZero() {
		return validationError("startsAt", "required", "必须提供活动开始时间。")
	}
	if input.EndsAt != nil && !input.EndsAt.After(input.StartsAt) {
		return validationError("endsAt", "invalid", "活动结束时间必须晚于开始时间。")
	}
	if input.PromotionDurationHours < 1 || input.PromotionDurationHours > 168 {
		return validationError("promotionDurationHours", "invalid", "推广时长必须在 1 到 168 小时之间。")
	}
	if input.CouponValidDays < 1 || input.CouponValidDays > 365 {
		return validationError("couponValidDays", "invalid", "推广券有效期必须在 1 到 365 天之间。")
	}
	if input.RewardDelayHours < 0 || input.RewardDelayHours > 720 {
		return validationError("rewardDelayHours", "invalid", "奖励延迟必须在 0 到 720 小时之间。")
	}
	if input.InviterMonthlyLimit < 0 || input.InviterMonthlyLimit > 1000 {
		return validationError("inviterMonthlyLimit", "invalid", "邀请人月度奖励上限必须在 0 到 1000 之间。")
	}
	if input.RulesText == "" || utf8.RuneCountInString(input.RulesText) > 2000 {
		return validationError("rulesText", "invalid", "活动规则不能为空且不能超过 2000 个字符。")
	}
	return validateReason(input.Reason)
}

func validateRevocation(id, reason string, expectedVersion int64, field string) *domain.AppError {
	if id == "" {
		return validationError(field, "required", "必须提供要撤销的记录。")
	}
	if expectedVersion < 1 {
		return validationError("version", "required", "必须提供当前记录版本。")
	}
	return validateReason(reason)
}

func validateReason(reason string) *domain.AppError {
	if utf8.RuneCountInString(reason) < 2 || utf8.RuneCountInString(reason) > 500 {
		return validationError("reason", "invalid", "操作原因必须为 2 到 500 个字符。")
	}
	return nil
}

func requireSession(user auth.User) *domain.AppError {
	if strings.TrimSpace(user.ID) == "" {
		return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	return nil
}

func requireAdmin(user auth.User) *domain.AppError {
	if appErr := requireSession(user); appErr != nil {
		return appErr
	}
	if !user.IsAdmin {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	return nil
}

func featureDisabled() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeFeatureDisabled, "Feature disabled", "推广权益活动当前未开启。")
}

func validationError(field, code, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Promotion reward validation failed", detail, field, code, detail)
}

func internalError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "推广权益操作失败。")
}

func emptyPagination(page, limit int) Pagination {
	return Pagination{Page: page, Limit: limit}
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
