package feedback

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/notification"

	"github.com/google/uuid"
)

type Service struct {
	mu            sync.Mutex
	now           func() time.Time
	repo          Repository
	notifications *notification.Service
	idempotency   *idempotency.Service
	tickets       map[string]Ticket
}

func NewService(repo Repository, notifications *notification.Service, idempotencyService *idempotency.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	return &Service{
		now:           now,
		repo:          repo,
		notifications: notifications,
		idempotency:   idempotencyService,
		tickets:       make(map[string]Ticket),
	}
}

func (s *Service) CreateWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.SubmitterUserID = user.ID
	input.SubmitterUsername = user.Username
	input.SubmitterName = displayName(user)
	input.Type = normalizeType(input.Type)
	input.Impact = normalizeImpact(input.Impact)
	if appErr := validateCreate(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.CreateFeedbackTicketWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	item := s.createMemory(input)
	completion, appErr := buildCompletion(item)
	return s.complete(ctx, entry, completion, appErr)
}

func (s *Service) createMemory(input CreateInput) Ticket {
	now := s.now()
	item := Ticket{
		ID:                 uuid.NewString(),
		SubmitterUserID:    input.SubmitterUserID,
		SubmitterUsername:  input.SubmitterUsername,
		SubmitterName:      input.SubmitterName,
		Type:               input.Type,
		Impact:             input.Impact,
		Status:             StatusSubmitted,
		Title:              normalizeTitle(input.Title, input.Description),
		Description:        strings.TrimSpace(input.Description),
		ContextPageLabel:   strings.TrimSpace(input.ContextPageLabel),
		ContextTargetType:  normalizeOptionalCode(input.ContextTargetType),
		ContextTargetID:    strings.TrimSpace(input.ContextTargetID),
		ContextTargetLabel: strings.TrimSpace(input.ContextTargetLabel),
		ContextRoleLabel:   strings.TrimSpace(input.ContextRoleLabel),
		CreatedAt:          now,
		UpdatedAt:          now,
		Version:            1,
	}
	item.Events = []Event{newEvent(item.ID, input.SubmitterUserID, input.SubmitterName, "user", EventSubmitted, "用户提交问题反馈", "", now)}
	s.mu.Lock()
	s.tickets[item.ID] = item
	s.mu.Unlock()
	return item
}

func (s *Service) MyTickets(ctx context.Context, user auth.User, page domain.PageRequest) (domain.Page[Ticket], *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return domain.Page[Ticket]{}, appErr
	}
	if s.repo != nil {
		return s.repo.ListFeedbackTicketsBySubmitter(ctx, user.ID, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Ticket, 0)
	for _, item := range s.tickets {
		if item.SubmitterUserID == user.ID {
			items = append(items, item)
		}
	}
	sortTickets(items)
	return domain.PageItems(items, page), nil
}

func (s *Service) MyTicket(ctx context.Context, user auth.User, id string) (Ticket, *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return Ticket{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetFeedbackTicketForSubmitter(ctx, user.ID, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tickets[id]
	if !ok || item.SubmitterUserID != user.ID {
		return Ticket{}, notFound()
	}
	return item, nil
}

func (s *Service) AddSupplementWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input SupplementInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.SubmitterUserID = user.ID
	if appErr := validateSupplement(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.AddFeedbackSupplementWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	item, appErr := s.addSupplementMemory(user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	return s.complete(ctx, entry, completion, appErr)
}

func (s *Service) addSupplementMemory(user auth.User, input SupplementInput) (Ticket, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tickets[input.ID]
	if !ok || item.SubmitterUserID != user.ID {
		return Ticket{}, notFound()
	}
	if item.Status == StatusClosed {
		return Ticket{}, invalidState("已关闭反馈不能继续补充。")
	}
	now := s.now()
	if item.Status == StatusNeedsUserInfo {
		item.Status = StatusSubmitted
	}
	item.Events = append(item.Events, newEvent(item.ID, user.ID, displayName(user), "user", EventUserSupplemented, strings.TrimSpace(input.Message), "", now))
	item.UpdatedAt = now
	item.Version++
	s.tickets[item.ID] = item
	return item, nil
}

func (s *Service) MarkRead(ctx context.Context, user auth.User, id string) (Ticket, *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return Ticket{}, appErr
	}
	if s.repo != nil {
		return s.repo.MarkFeedbackRead(ctx, user.ID, id, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tickets[id]
	if !ok || item.SubmitterUserID != user.ID {
		return Ticket{}, notFound()
	}
	if item.LatestAdminUpdateAt != nil {
		now := s.now()
		item.SubmitterReadAt = &now
		item.Events = append(item.Events, newEvent(item.ID, user.ID, displayName(user), "user", EventRead, "用户已查看处理结果", "", now))
		item.UpdatedAt = now
		item.Version++
		s.tickets[item.ID] = item
	}
	return item, nil
}

func (s *Service) UnreadCount(ctx context.Context, user auth.User) (int, *domain.AppError) {
	if appErr := requireSession(user); appErr != nil {
		return 0, appErr
	}
	if s.repo != nil {
		return s.repo.UnreadFeedbackCount(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, item := range s.tickets {
		if item.SubmitterUserID == user.ID && hasUnreadAdminUpdate(item) {
			count++
		}
	}
	return count, nil
}

func (s *Service) AdminTickets(ctx context.Context, user auth.User) ([]Ticket, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListAdminFeedbackTickets(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Ticket, 0, len(s.tickets))
	for _, item := range s.tickets {
		items = append(items, item)
	}
	sortTickets(items)
	return items, nil
}

func (s *Service) AdminTicket(ctx context.Context, user auth.User, id string) (Ticket, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Ticket{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetAdminFeedbackTicket(ctx, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tickets[id]
	if !ok {
		return Ticket{}, notFound()
	}
	return item, nil
}

func (s *Service) AdminHandleWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input AdminHandleInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = user.ID
	input.AdminName = displayName(user)
	input.Status = normalizeStatus(input.Status)
	if appErr := validateAdminHandle(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.HandleAdminFeedbackTicketWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	item, appErr := s.adminHandleMemory(user, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	return s.complete(ctx, entry, completion, appErr)
}

func (s *Service) adminHandleMemory(user auth.User, input AdminHandleInput) (Ticket, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.tickets[input.ID]
	if !ok {
		return Ticket{}, notFound()
	}
	if input.ExpectedVersion > 0 && item.Version != input.ExpectedVersion {
		return Ticket{}, versionConflict()
	}
	if item.Status == StatusClosed {
		return Ticket{}, invalidState("已关闭反馈不能继续处理。")
	}
	now := s.now()
	item.Status = input.Status
	item.AdminResponse = strings.TrimSpace(input.Response)
	item.AdminInternalNote = strings.TrimSpace(input.InternalNote)
	item.HandledByAdminID = user.ID
	item.HandledByAdminName = displayName(user)
	item.HandledAt = &now
	item.LatestAdminUpdateAt = &now
	item.SubmitterReadAt = nil
	item.UpdatedAt = now
	item.Version++
	item.Events = append(item.Events, newEvent(item.ID, user.ID, displayName(user), "admin", EventAdminHandled, item.AdminResponse, item.AdminInternalNote, now))
	s.tickets[item.ID] = item
	if s.notifications != nil {
		s.notifications.Add(notification.Notification{
			UserID:          item.SubmitterUserID,
			Type:            "问题反馈",
			Title:           "你的问题反馈已有处理结果",
			Body:            item.AdminResponse,
			TargetType:      "feedback_ticket",
			TargetID:        item.ID,
			TargetURL:       "/my/feedback/" + item.ID,
			SourceEventType: "feedback_ticket.admin_handled",
		})
	}
	return item, nil
}

func (s *Service) complete(ctx context.Context, entry *idempotency.Entry, completion idempotency.Completion, appErr *domain.AppError) (idempotency.Completion, *domain.AppError) {
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

func hasUnreadAdminUpdate(item Ticket) bool {
	if item.LatestAdminUpdateAt == nil {
		return false
	}
	return item.SubmitterReadAt == nil || item.SubmitterReadAt.Before(*item.LatestAdminUpdateAt)
}

func newEvent(ticketID, actorID, actorName, actorRole, action, publicMessage, internalNote string, now time.Time) Event {
	return Event{
		ID:            uuid.NewString(),
		TicketID:      ticketID,
		ActorUserID:   actorID,
		ActorName:     strings.TrimSpace(actorName),
		ActorRole:     actorRole,
		Action:        action,
		PublicMessage: strings.TrimSpace(publicMessage),
		InternalNote:  strings.TrimSpace(internalNote),
		CreatedAt:     now,
	}
}

func validateCreate(input CreateInput) *domain.AppError {
	if !validTypes[input.Type] {
		return fieldError("type", "反馈类型不支持。")
	}
	if !validImpacts[input.Impact] {
		return fieldError("impact", "影响程度不支持。")
	}
	if appErr := validateOptionalText("title", input.Title, 80); appErr != nil {
		return appErr
	}
	if appErr := validateText("description", input.Description, 4, 1600, "问题描述需为 4 至 1600 个字符。"); appErr != nil {
		return appErr
	}
	if appErr := validateText("contextPageLabel", input.ContextPageLabel, 2, 80, "当前页面需为 2 至 80 个字符。"); appErr != nil {
		return appErr
	}
	if appErr := validateOptionalText("contextTargetType", input.ContextTargetType, 64); appErr != nil {
		return appErr
	}
	if appErr := validateOptionalText("contextTargetId", input.ContextTargetID, 120); appErr != nil {
		return appErr
	}
	if appErr := validateOptionalText("contextTargetLabel", input.ContextTargetLabel, 120); appErr != nil {
		return appErr
	}
	if appErr := validateOptionalText("contextRoleLabel", input.ContextRoleLabel, 40); appErr != nil {
		return appErr
	}
	return nil
}

func validateSupplement(input SupplementInput) *domain.AppError {
	return validateText("message", input.Message, 2, 1200, "补充说明需为 2 至 1200 个字符。")
}

func validateAdminHandle(input AdminHandleInput) *domain.AppError {
	if !validStatuses[input.Status] || input.Status == StatusSubmitted {
		return fieldError("status", "处理状态不支持。")
	}
	if appErr := validateText("response", input.Response, 2, 1200, "处理说明需为 2 至 1200 个字符。"); appErr != nil {
		return appErr
	}
	if appErr := validateOptionalText("internalNote", input.InternalNote, 1200); appErr != nil {
		return appErr
	}
	return nil
}

func validateText(field, value string, min, max int, detail string) *domain.AppError {
	value = strings.TrimSpace(value)
	count := utf8.RuneCountInString(value)
	if count < min || count > max {
		return fieldError(field, detail)
	}
	if strings.ContainsAny(value, "\x00") {
		return fieldError(field, "文本内容包含非法字符。")
	}
	if looksLikeSecret(value) {
		return secretError(field)
	}
	return nil
}

func validateOptionalText(field, value string, max int) *domain.AppError {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	count := utf8.RuneCountInString(value)
	if count > max {
		return fieldError(field, "文本内容过长。")
	}
	if strings.ContainsAny(value, "\x00") {
		return fieldError(field, "文本内容包含非法字符。")
	}
	if looksLikeSecret(value) {
		return secretError(field)
	}
	return nil
}

func normalizeTitle(title, description string) string {
	title = strings.TrimSpace(title)
	if title != "" {
		return title
	}
	description = strings.TrimSpace(description)
	runes := []rune(description)
	if len(runes) > 32 {
		return string(runes[:32])
	}
	return description
}

func normalizeType(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "功能问题":
		return TypeFunctionIssue
	case "数据纠错":
		return TypeDataCorrection
	case "体验建议":
		return TypeExperienceSuggestion
	case "发布/联系受阻":
		return TypePublishContactBlock
	default:
		return value
	}
}

func normalizeImpact(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "一般":
		return ImpactGeneral
	case "影响操作":
		return ImpactBlocksOperation
	case "无法继续":
		return ImpactCannotContinue
	default:
		return value
	}
}

func normalizeStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "已记录":
		return StatusRecorded
	case "跟进中":
		return StatusFollowingUp
	case "已修复", "已调整", "已修复/已调整":
		return StatusResolved
	case "暂不处理":
		return StatusDeclined
	case "需要补充信息":
		return StatusNeedsUserInfo
	case "已关闭":
		return StatusClosed
	default:
		return value
	}
}

func normalizeOptionalCode(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func requireSession(user auth.User) *domain.AppError {
	if strings.TrimSpace(user.ID) == "" {
		return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	return nil
}

func requireAdmin(user auth.User) *domain.AppError {
	if !user.IsAdmin {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	return nil
}

func displayName(user auth.User) string {
	if strings.TrimSpace(user.DisplayName) != "" {
		return strings.TrimSpace(user.DisplayName)
	}
	return strings.TrimSpace(user.Username)
}

func looksLikeSecret(value string) bool {
	lower := strings.ToLower(value)
	needles := []string{
		"password", "密码", "api key", "api_key", "apikey", "sub2api key",
		"token", "session", "cookie", "secret", "bearer ", "access_token",
		"refresh_token", "恢复码", "mfa", "验证码",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func sortTickets(items []Ticket) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
}

func fieldError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Feedback validation failed", detail, field, "invalid", detail)
}

func secretError(field string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在问题反馈中填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含密码、API Key、Token、Session、Cookie 或恢复码。")
}

func notFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Feedback ticket not found", "问题反馈不存在。")
}

func invalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid feedback state", detail)
}

func versionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
}

var validTypes = map[string]bool{
	TypeFunctionIssue:        true,
	TypeDataCorrection:       true,
	TypeExperienceSuggestion: true,
	TypePublishContactBlock:  true,
}

var validImpacts = map[string]bool{
	ImpactGeneral:         true,
	ImpactBlocksOperation: true,
	ImpactCannotContinue:  true,
}

var validStatuses = map[string]bool{
	StatusSubmitted:     true,
	StatusRecorded:      true,
	StatusFollowingUp:   true,
	StatusResolved:      true,
	StatusDeclined:      true,
	StatusNeedsUserInfo: true,
	StatusClosed:        true,
}
