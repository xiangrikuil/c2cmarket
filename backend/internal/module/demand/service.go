package demand

import (
	"context"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	idempotency *idempotency.Service
	items       map[string]Demand
}

func NewService(repo Repository, idempotencyService *idempotency.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	return &Service{
		now:         now,
		repo:        repo,
		idempotency: idempotencyService,
		items:       make(map[string]Demand),
	}
}

func (s *Service) Create(ctx context.Context, user auth.User, input CreateInput) (Demand, *domain.AppError) {
	input.PublisherUserID = user.ID
	if appErr := validateCreate(input); appErr != nil {
		return Demand{}, appErr
	}
	now := s.now()
	item := Demand{
		ID:                uuid.NewString(),
		PublisherUserID:   user.ID,
		PublisherUsername: user.Username,
		PublisherName:     displayName(user),
		Title:             strings.TrimSpace(input.Title),
		MaxPriceCNY:       normalizePrice(input.MaxPriceCNY),
		RegionCode:        normalizeCode(input.RegionCode, "any"),
		OwnerPreference:   normalizeOwnerPreference(input.OwnerPreference),
		SourceURL:         strings.TrimSpace(input.SourceURL),
		Note:              strings.TrimSpace(input.Note),
		Status:            StatusActive,
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           1,
	}
	if s.repo != nil {
		if appErr := s.repo.CreateDemand(ctx, item); appErr != nil {
			return Demand{}, appErr
		}
		return s.repo.GetDemandForPublisher(ctx, user.ID, item.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return item, nil
}

func (s *Service) PublicDemands(ctx context.Context) ([]Demand, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListPublicDemands(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Demand, 0, len(s.items))
	for _, item := range s.items {
		if item.Status == StatusActive {
			items = append(items, item)
		}
	}
	sortDemands(items)
	return items, nil
}

func (s *Service) PublicDemand(ctx context.Context, id string) (Demand, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetPublicDemand(ctx, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok || item.Status != StatusActive {
		return Demand{}, notFound()
	}
	return item, nil
}

func (s *Service) MyDemands(ctx context.Context, user auth.User) ([]Demand, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListDemandsByPublisher(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Demand, 0)
	for _, item := range s.items {
		if item.PublisherUserID == user.ID {
			items = append(items, item)
		}
	}
	sortDemands(items)
	return items, nil
}

func (s *Service) MyDemand(ctx context.Context, user auth.User, id string) (Demand, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetDemandForPublisher(ctx, user.ID, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok || item.PublisherUserID != user.ID {
		return Demand{}, notFound()
	}
	return item, nil
}

func (s *Service) AdminDemands(ctx context.Context, user auth.User) ([]Demand, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListAdminDemands(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Demand, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sortDemands(items)
	return items, nil
}

func (s *Service) AdminDemand(ctx context.Context, user auth.User, id string) (Demand, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Demand{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetAdminDemand(ctx, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[id]
	if !ok {
		return Demand{}, notFound()
	}
	return item, nil
}

func (s *Service) CloseWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input OwnerActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	return s.ownerActionWithIdempotency(ctx, userID, routeKey, key, requestHash, input, "close", buildCompletion)
}

func (s *Service) ReopenWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input OwnerActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	return s.ownerActionWithIdempotency(ctx, userID, routeKey, key, requestHash, input, "reopen", buildCompletion)
}

func (s *Service) AdminActionWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input AdminActionInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if input.AdminUserID == "" {
		input.AdminUserID = userID
	}
	if appErr := validateAdminAction(input.Action, input.Reason); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateDemandAdminStatusWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	item, appErr := s.updateAdminStatusMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) ownerActionWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input OwnerActionInput, action string, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.PublisherUserID = userID
	input.Action = action
	if appErr := validateOwnerAction(action); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateDemandOwnerStatusWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	item, appErr := s.updateOwnerStatusMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) updateOwnerStatusMemory(input OwnerActionInput) (Demand, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[input.ID]
	if !ok || item.PublisherUserID != input.PublisherUserID {
		return Demand{}, notFound()
	}
	if input.ExpectedVersion > 0 && item.Version != input.ExpectedVersion {
		return Demand{}, versionConflict()
	}
	now := s.now()
	switch input.Action {
	case "close":
		if item.Status == StatusClosed {
			return Demand{}, invalidState("需求已关闭。")
		}
		if item.Status != StatusActive && item.Status != StatusPendingReview && item.Status != StatusChangesRequested {
			return Demand{}, invalidState("当前需求状态不能关闭。")
		}
		item.Status = StatusClosed
		item.ClosedAt = &now
	case "reopen":
		if item.Status != StatusClosed {
			return Demand{}, invalidState("只有已关闭需求可以重新打开。")
		}
		item.Status = StatusActive
		item.ClosedAt = nil
	default:
		return Demand{}, invalidState("需求操作不支持。")
	}
	item.UpdatedAt = now
	item.Version++
	s.items[item.ID] = item
	return item, nil
}

func (s *Service) updateAdminStatusMemory(input AdminActionInput) (Demand, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[input.ID]
	if !ok {
		return Demand{}, notFound()
	}
	if input.ExpectedVersion > 0 && item.Version != input.ExpectedVersion {
		return Demand{}, versionConflict()
	}
	next, appErr := nextAdminStatus(item.Status, input.Action)
	if appErr != nil {
		return Demand{}, appErr
	}
	now := s.now()
	item.Status = next
	item.ReviewReason = strings.TrimSpace(input.Reason)
	item.ReviewedByAdminID = input.AdminUserID
	item.ReviewedAt = &now
	if next != StatusClosed {
		item.ClosedAt = nil
	}
	item.UpdatedAt = now
	item.Version++
	s.items[item.ID] = item
	return item, nil
}

func nextAdminStatus(current, action string) (string, *domain.AppError) {
	switch action {
	case "approve":
		if current != StatusPendingReview && current != StatusChangesRequested {
			return "", invalidState("只有待处理或需修改的需求可以标记公开。")
		}
		return StatusActive, nil
	case "request_changes":
		if current != StatusPendingReview && current != StatusActive {
			return "", invalidState("当前需求状态不能要求修改。")
		}
		return StatusChangesRequested, nil
	case "reject":
		if current != StatusPendingReview && current != StatusChangesRequested {
			return "", invalidState("当前需求状态不能拒绝。")
		}
		return StatusRejected, nil
	case "take_down":
		if current != StatusActive {
			return "", invalidState("只有匹配中的需求可以下架。")
		}
		return StatusTakenDown, nil
	case "restore":
		if current != StatusTakenDown {
			return "", invalidState("只有已下架需求可以恢复。")
		}
		return StatusActive, nil
	default:
		return "", invalidState("需求治理动作不支持。")
	}
}

func validateCreate(input CreateInput) *domain.AppError {
	title := strings.TrimSpace(input.Title)
	if utf8.RuneCountInString(title) < 2 || utf8.RuneCountInString(title) > 80 {
		return fieldError("title", "需求标题需为 2 至 80 个字符。")
	}
	price := normalizePrice(input.MaxPriceCNY)
	if price == "" {
		return fieldError("maxPriceCny", "预算必须大于 0。")
	}
	if !validOwnerPreferences[normalizeOwnerPreference(input.OwnerPreference)] {
		return fieldError("ownerPreference", "车主偏好不支持。")
	}
	if !validLinuxDoTopicURL(input.SourceURL) {
		return fieldError("sourceUrl", "必须填写 https://linux.do/t/* 求车原帖。")
	}
	if appErr := validateOptionalText("note", input.Note, 1000); appErr != nil {
		return appErr
	}
	if appErr := validateOptionalText("regionCode", input.RegionCode, 64); appErr != nil {
		return appErr
	}
	return nil
}

func validateOwnerAction(action string) *domain.AppError {
	if action != "close" && action != "reopen" {
		return invalidState("需求操作不支持。")
	}
	return nil
}

func validateAdminAction(action, reason string) *domain.AppError {
	switch action {
	case "approve", "restore":
	case "request_changes", "reject", "take_down":
		if strings.TrimSpace(reason) == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "该治理动作必须填写原因。", "reason", "required", "必须填写原因。")
		}
	default:
		return invalidState("需求治理动作不支持。")
	}
	return nil
}

func requireAdmin(user auth.User) *domain.AppError {
	if user.IsAdmin {
		return nil
	}
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
}

func fieldError(field, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Demand validation failed", message, field, "invalid", message)
}

func validateOptionalText(field, value string, maxRunes int) *domain.AppError {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Text too long", "文本内容过长。", field, "too_long", "文本内容过长。")
	}
	if strings.ContainsAny(value, "\x00") || looksLikeSecret(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在平台填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含 API Key、密码、Token、Session 或 Cookie。")
	}
	return nil
}

func validLinuxDoTopicURL(value string) bool {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "https://linux.do/t/") {
		return false
	}
	return !looksLikeSecret(value)
}

func normalizePrice(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	if value == "" {
		return ""
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok || rat.Sign() <= 0 {
		return ""
	}
	return rat.FloatString(2)
}

func normalizeCode(value, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return fallback
	}
	replacer := strings.NewReplacer(" ", "_", "区", "", "不限", "any")
	return replacer.Replace(value)
}

func normalizeOwnerPreference(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "only-personal", "only_personal":
		return "only_personal"
	case "personal":
		return "personal"
	default:
		return "any"
	}
}

func displayName(user auth.User) string {
	if strings.TrimSpace(user.DisplayName) != "" {
		return strings.TrimSpace(user.DisplayName)
	}
	return user.Username
}

func sortDemands(items []Demand) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
}

func notFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Demand not found", "求车需求不存在。")
}

func invalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid demand state", detail)
}

func versionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
}

func looksLikeSecret(value string) bool {
	lower := strings.ToLower(value)
	for _, needle := range []string{"bearer ", "api_key=", "apikey=", "access_token=", "refresh_token=", "password=", "secret=", "token="} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

var validOwnerPreferences = map[string]bool{
	"personal":      true,
	"only_personal": true,
	"any":           true,
}
