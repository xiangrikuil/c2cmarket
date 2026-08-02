package apipromotion

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
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

func StatusAt(item Promotion, now time.Time) string {
	if item.StoppedAt != nil {
		return StatusStopped
	}
	if now.Before(item.StartsAt) {
		return StatusScheduled
	}
	if !now.Before(item.EndsAt) {
		return StatusFinished
	}
	if !item.Eligibility.Displayable {
		return StatusSuppressed
	}
	return StatusServing
}

func (s *Service) Public(ctx context.Context, placement string) ([]Promotion, *domain.AppError) {
	placement = strings.TrimSpace(placement)
	if placement == "" {
		placement = PlacementAPIMarketTop
	}
	if placement != PlacementAPIMarketTop {
		return nil, validationError("placement", "INVALID_PLACEMENT", "暂不支持该推广位置。")
	}
	if s == nil || s.repo == nil {
		return []Promotion{}, nil
	}
	now := s.now().UTC()
	items, appErr := s.repo.ListPublicAPIPromotions(ctx, placement, now)
	if appErr != nil {
		return nil, appErr
	}
	for i := range items {
		items[i].Status = StatusAt(items[i], now)
	}
	return items, nil
}

func (s *Service) AdminList(ctx context.Context, user auth.User) ([]Promotion, *domain.AppError) {
	if !user.IsAdmin {
		return nil, permissionDenied()
	}
	if s == nil || s.repo == nil {
		return []Promotion{}, nil
	}
	now := s.now().UTC()
	items, appErr := s.repo.ListAdminAPIPromotions(ctx, now)
	if appErr != nil {
		return nil, appErr
	}
	for i := range items {
		items[i].Status = StatusAt(items[i], now)
	}
	return items, nil
}

func (s *Service) Availability(ctx context.Context, user auth.User, input AvailabilityInput) (Availability, *domain.AppError) {
	if !user.IsAdmin {
		return Availability{}, permissionDenied()
	}
	input.APIServiceID = strings.TrimSpace(input.APIServiceID)
	input.Placement = strings.TrimSpace(input.Placement)
	if input.Placement == "" {
		input.Placement = PlacementAPIMarketTop
	}
	if appErr := validateAvailabilityInput(input); appErr != nil {
		return Availability{}, appErr
	}
	if s == nil || s.repo == nil {
		return Availability{}, internalError()
	}
	return s.repo.GetAPIPromotionAvailability(ctx, input, s.now().UTC())
}

func (s *Service) Create(ctx context.Context, user auth.User, input CreateInput) (Promotion, *domain.AppError) {
	if !user.IsAdmin {
		return Promotion{}, permissionDenied()
	}
	input.AdminUserID = user.ID
	if appErr := validateCreateInput(&input); appErr != nil {
		return Promotion{}, appErr
	}
	if s == nil || s.repo == nil {
		return Promotion{}, internalError()
	}
	now := s.now().UTC()
	eligibility, appErr := s.repo.GetAPIPromotionEligibility(ctx, input.APIServiceID, now)
	if appErr != nil {
		return Promotion{}, appErr
	}
	if !eligibility.Configurable {
		detail := "当前 API 服务不符合推广配置条件。"
		if len(eligibility.HardBlockReasons) > 0 {
			detail = eligibility.HardBlockReasons[0]
		}
		return Promotion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Promotion unavailable", detail)
	}
	item, appErr := s.repo.CreateAPIPromotion(ctx, input, now)
	if appErr != nil {
		return Promotion{}, appErr
	}
	item.Eligibility = eligibility
	item.Status = StatusAt(item, now)
	return item, nil
}

func (s *Service) CreateWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if !user.IsAdmin {
		return idempotency.Completion{}, permissionDenied()
	}
	input.AdminUserID = user.ID
	if appErr := validateCreateInput(&input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if s == nil || s.repo == nil || s.idempotency == nil {
		return idempotency.Completion{}, internalError()
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, internalError()
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	_, completion, appErr := s.repo.CreateAPIPromotionWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) Stop(ctx context.Context, user auth.User, input StopInput) (Promotion, *domain.AppError) {
	if !user.IsAdmin {
		return Promotion{}, permissionDenied()
	}
	input.AdminUserID = user.ID
	if appErr := validateStopInput(&input); appErr != nil {
		return Promotion{}, appErr
	}
	if s == nil || s.repo == nil {
		return Promotion{}, internalError()
	}
	now := s.now().UTC()
	item, appErr := s.repo.StopAPIPromotion(ctx, input, now)
	if appErr != nil {
		return Promotion{}, appErr
	}
	item.Status = StatusAt(item, now)
	return item, nil
}

func (s *Service) StopWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input StopInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if !user.IsAdmin {
		return idempotency.Completion{}, permissionDenied()
	}
	input.AdminUserID = user.ID
	if appErr := validateStopInput(&input); appErr != nil {
		return idempotency.Completion{}, appErr
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
	_, completion, appErr := s.repo.StopAPIPromotionWithIdempotency(ctx, *entry, input, s.now().UTC(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func validateAvailabilityInput(input AvailabilityInput) *domain.AppError {
	if input.APIServiceID == "" {
		return validationError("apiServiceId", "REQUIRED", "请选择 API 服务。")
	}
	if _, err := uuid.Parse(input.APIServiceID); err != nil {
		return validationError("apiServiceId", "INVALID", "API 服务 ID 格式不正确。")
	}
	if input.Placement != PlacementAPIMarketTop {
		return validationError("placement", "INVALID_PLACEMENT", "暂不支持该推广位置。")
	}
	if input.StartsAt.IsZero() {
		return validationError("startsAt", "REQUIRED", "请选择开始时间。")
	}
	if input.EndsAt.IsZero() || !input.EndsAt.After(input.StartsAt) {
		return validationError("endsAt", "INVALID_PERIOD", "结束时间必须晚于开始时间。")
	}
	return nil
}

func validateCreateInput(input *CreateInput) *domain.AppError {
	input.APIServiceID = strings.TrimSpace(input.APIServiceID)
	input.Placement = strings.TrimSpace(input.Placement)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Placement == "" {
		input.Placement = PlacementAPIMarketTop
	}
	if appErr := validateAvailabilityInput(AvailabilityInput{
		APIServiceID: input.APIServiceID,
		Placement:    input.Placement,
		StartsAt:     input.StartsAt,
		EndsAt:       input.EndsAt,
	}); appErr != nil {
		return appErr
	}
	if input.Reason == "" {
		return validationError("reason", "REQUIRED", "请填写设置推广的原因。")
	}
	if utf8.RuneCountInString(input.Reason) > 500 {
		return validationError("reason", "TOO_LONG", "设置推广的原因不能超过 500 个字符。")
	}
	return nil
}

func validateStopInput(input *StopInput) *domain.AppError {
	input.PromotionID = strings.TrimSpace(input.PromotionID)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.PromotionID == "" {
		return validationError("id", "REQUIRED", "推广记录不存在。")
	}
	if _, err := uuid.Parse(input.PromotionID); err != nil {
		return validationError("id", "INVALID", "推广记录 ID 格式不正确。")
	}
	if input.ExpectedVersion <= 0 {
		return validationError("version", "INVALID", "资源版本无效。")
	}
	if input.Reason == "" {
		return validationError("reason", "REQUIRED", "请填写提前停止原因。")
	}
	if utf8.RuneCountInString(input.Reason) > 500 {
		return validationError("reason", "TOO_LONG", "提前停止原因不能超过 500 个字符。")
	}
	return nil
}

func validationError(field, code, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Validation failed", "请求参数不符合要求。", field, code, message)
}

func permissionDenied() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
}

func internalError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "推广服务暂时不可用。")
}
