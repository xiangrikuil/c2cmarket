package announcement

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	items       map[string]Announcement
	itemsBySlug map[string]string
	receipts    map[string]Receipt
	auditLogs   []AuditLog
	seeded      bool
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		now:         now,
		repo:        repo,
		items:       make(map[string]Announcement),
		itemsBySlug: make(map[string]string),
		receipts:    make(map[string]Receipt),
	}
}

func (s *Service) UserAnnouncements(ctx context.Context, user auth.User) ([]Announcement, *domain.AppError) {
	if s.repo != nil {
		return s.repo.UserAnnouncements(ctx, user.ID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	return s.userAnnouncementsLocked(user.ID, s.now(), ""), nil
}

func (s *Service) ActiveAnnouncements(ctx context.Context, user auth.User, channel string) ([]Announcement, *domain.AppError) {
	if appErr := validateChannelFilter(channel); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ActiveAnnouncements(ctx, user.ID, channel, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	now := s.now()
	items := s.userAnnouncementsLocked(user.ID, now, channel)
	active := make([]Announcement, 0, len(items))
	for _, item := range items {
		if displayStatus(item, now) == StatusPublished && (channel == "" || hasChannel(item, channel)) {
			active = append(active, item)
		}
	}
	sortByPublishDesc(active)
	return active, nil
}

func (s *Service) HomeAnnouncement(ctx context.Context, user auth.User) (*Announcement, *domain.AppError) {
	if s.repo != nil {
		return s.repo.HomeAnnouncement(ctx, user.ID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	now := s.now()
	candidates := make([]Announcement, 0)
	for _, item := range s.items {
		item = s.withReceiptLocked(user.ID, item)
		if displayStatus(item, now) == StatusPublished && hasChannel(item, ChannelHomeBanner) && !isDismissed(item) {
			candidates = append(candidates, item)
		}
	}
	sortForHome(candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	return &candidates[0], nil
}

func (s *Service) UserAnnouncementBySlug(ctx context.Context, user auth.User, slug string) (Announcement, *domain.AppError) {
	if s.repo != nil {
		return s.repo.UserAnnouncementBySlug(ctx, user.ID, slug, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	id := s.itemsBySlug[strings.TrimSpace(strings.ToLower(slug))]
	if id == "" {
		return Announcement{}, notFound()
	}
	item := s.withReceiptLocked(user.ID, s.items[id])
	if !isUserVisible(item, s.now()) {
		return Announcement{}, notFound()
	}
	return item, nil
}

func (s *Service) AnnouncementUnreadCount(ctx context.Context, user auth.User, importantOnly bool) (int, *domain.AppError) {
	if s.repo != nil {
		return s.repo.AnnouncementUnreadCount(ctx, user.ID, importantOnly, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	count := 0
	for _, item := range s.userAnnouncementsLocked(user.ID, s.now(), "") {
		if importantOnly && item.Level != LevelImportant {
			continue
		}
		if isUnread(item) {
			count++
		}
	}
	return count, nil
}

func (s *Service) MarkSeen(ctx context.Context, user auth.User, id string) (Receipt, *domain.AppError) {
	return s.upsertReceipt(ctx, user.ID, id, "seen")
}

func (s *Service) MarkRead(ctx context.Context, user auth.User, id string) (Receipt, *domain.AppError) {
	return s.upsertReceipt(ctx, user.ID, id, "read")
}

func (s *Service) Dismiss(ctx context.Context, user auth.User, id string) (Receipt, *domain.AppError) {
	return s.upsertReceipt(ctx, user.ID, id, "dismiss")
}

func (s *Service) AdminAnnouncements(ctx context.Context, user auth.User) ([]Announcement, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.AdminAnnouncements(ctx, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	items := make([]Announcement, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sortByPublishDesc(items)
	return items, nil
}

func (s *Service) AdminAnnouncement(ctx context.Context, user auth.User, id string) (Announcement, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Announcement{}, appErr
	}
	if s.repo != nil {
		return s.repo.AdminAnnouncementByID(ctx, id, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	item, ok := s.items[id]
	if !ok {
		return Announcement{}, notFound()
	}
	return item, nil
}

func (s *Service) CreateAnnouncement(ctx context.Context, user auth.User, form FormInput) (Announcement, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Announcement{}, appErr
	}
	input := CreateInput{OperatorID: user.ID, OperatorName: user.DisplayName, Form: form}
	if appErr := validateForm(input.Form); appErr != nil {
		return Announcement{}, appErr
	}
	if s.repo != nil {
		return s.repo.CreateAnnouncement(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	now := s.now()
	item := Announcement{
		ID:              uuid.NewString(),
		Slug:            s.uniqueSlugLocked(input.Form.Title),
		Title:           strings.TrimSpace(input.Form.Title),
		Summary:         strings.TrimSpace(input.Form.Summary),
		ContentMarkdown: strings.TrimSpace(input.Form.ContentMarkdown),
		Category:        input.Form.Category,
		Level:           input.Form.Level,
		Status:          StatusDraft,
		Channels:        normalizeChannels(input.Form.Channels),
		Audience:        Audience{Type: "all"},
		IsPinned:        input.Form.IsPinned,
		IsDismissible:   input.Form.IsDismissible,
		CTALabel:        strings.TrimSpace(input.Form.CTALabel),
		CTAURL:          strings.TrimSpace(input.Form.CTAURL),
		PublishAt:       input.Form.PublishAt,
		ExpireAt:        input.Form.ExpireAt,
		Version:         1,
		CreatedBy:       user.ID,
		UpdatedBy:       user.ID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.putLocked(item)
	s.appendAuditLocked(AuditCreated, item, user, "创建公告草稿", now)
	return item, nil
}

func (s *Service) UpdateAnnouncement(ctx context.Context, user auth.User, id string, form FormInput) (Announcement, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Announcement{}, appErr
	}
	input := UpdateInput{ID: id, OperatorID: user.ID, OperatorName: user.DisplayName, Form: form}
	if appErr := validateForm(input.Form); appErr != nil {
		return Announcement{}, appErr
	}
	if s.repo != nil {
		return s.repo.UpdateAnnouncement(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	item, ok := s.items[id]
	if !ok {
		return Announcement{}, notFound()
	}
	beforeStatus := displayStatus(item, s.now())
	item.Title = strings.TrimSpace(form.Title)
	item.Summary = strings.TrimSpace(form.Summary)
	item.ContentMarkdown = strings.TrimSpace(form.ContentMarkdown)
	item.Category = form.Category
	item.Level = form.Level
	item.Channels = normalizeChannels(form.Channels)
	item.IsPinned = form.IsPinned
	item.IsDismissible = form.IsDismissible
	item.CTALabel = strings.TrimSpace(form.CTALabel)
	item.CTAURL = strings.TrimSpace(form.CTAURL)
	item.PublishAt = form.PublishAt
	item.ExpireAt = form.ExpireAt
	item.UpdatedBy = user.ID
	item.UpdatedAt = s.now()
	item.Version++
	s.putLocked(item)
	if beforeStatus == StatusPublished {
		s.appendAuditLocked(AuditUpdated, item, user, "编辑已发布公告", s.now())
	}
	return item, nil
}

func (s *Service) PublishAnnouncement(ctx context.Context, user auth.User, id string) (Announcement, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Announcement{}, appErr
	}
	input := ActionInput{ID: id, OperatorID: user.ID, OperatorName: user.DisplayName}
	if s.repo != nil {
		return s.repo.PublishAnnouncement(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	item, ok := s.items[id]
	if !ok {
		return Announcement{}, notFound()
	}
	status := StatusPublished
	if item.PublishAt.After(s.now()) {
		status = StatusScheduled
	}
	item.Status = status
	item.UpdatedBy = user.ID
	item.UpdatedAt = s.now()
	item.Version++
	s.putLocked(item)
	reason := "立即发布公告"
	if status == StatusScheduled {
		reason = "设置未来发布时间"
	}
	s.appendAuditLocked(AuditPublished, item, user, reason, s.now())
	return item, nil
}

func (s *Service) OfflineAnnouncement(ctx context.Context, user auth.User, id, reason string) (Announcement, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Announcement{}, appErr
	}
	input := ActionInput{ID: id, OperatorID: user.ID, OperatorName: user.DisplayName, Reason: strings.TrimSpace(reason)}
	if strings.TrimSpace(input.Reason) == "" {
		return Announcement{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "下线公告必须填写原因。", "reason", "required", "下线公告必须填写原因。")
	}
	if s.repo != nil {
		return s.repo.OfflineAnnouncement(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	item, ok := s.items[id]
	if !ok {
		return Announcement{}, notFound()
	}
	status := displayStatus(item, s.now())
	if status != StatusPublished && status != StatusScheduled {
		return Announcement{}, invalidState("只有发布中或待发布公告可以下线。")
	}
	item.Status = StatusOffline
	item.UpdatedBy = user.ID
	item.UpdatedAt = s.now()
	item.Version++
	s.putLocked(item)
	s.appendAuditLocked(AuditOfflined, item, user, input.Reason, s.now())
	return item, nil
}

func (s *Service) DuplicateAnnouncement(ctx context.Context, user auth.User, id string) (Announcement, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Announcement{}, appErr
	}
	input := ActionInput{ID: id, OperatorID: user.ID, OperatorName: user.DisplayName}
	if s.repo != nil {
		return s.repo.DuplicateAnnouncement(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	source, ok := s.items[id]
	if !ok {
		return Announcement{}, notFound()
	}
	now := s.now()
	item := source
	item.ID = uuid.NewString()
	item.Slug = s.uniqueSlugLocked(source.Title + " 副本")
	item.Title = source.Title + " 副本"
	item.Status = StatusDraft
	item.Version = 1
	item.CreatedBy = user.ID
	item.UpdatedBy = user.ID
	item.CreatedAt = now
	item.UpdatedAt = now
	item.Receipt = nil
	s.putLocked(item)
	s.appendAuditLocked(AuditDuplicated, item, user, "复制自 "+source.Title, now)
	return item, nil
}

func (s *Service) AnnouncementAuditLogs(ctx context.Context, user auth.User) ([]AuditLog, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.AnnouncementAuditLogs(ctx, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	logs := append([]AuditLog(nil), s.auditLogs...)
	sort.SliceStable(logs, func(i, j int) bool { return logs[i].CreatedAt.After(logs[j].CreatedAt) })
	return logs, nil
}

func (s *Service) upsertReceipt(ctx context.Context, userID, id, action string) (Receipt, *domain.AppError) {
	input := ReceiptInput{AnnouncementID: id, UserID: userID, Action: action}
	if s.repo != nil {
		return s.repo.UpsertReceipt(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureSeedLocked()
	item, ok := s.items[id]
	if !ok || !isUserVisible(item, s.now()) {
		return Receipt{}, notFound()
	}
	key := receiptKey(userID, id)
	receipt := s.receipts[key]
	if receipt.AnnouncementVersion != item.Version {
		receipt = Receipt{AnnouncementID: id, AnnouncementVersion: item.Version}
	}
	now := s.now()
	if receipt.FirstSeenAt == nil {
		receipt.FirstSeenAt = &now
	}
	switch action {
	case "seen":
	case "read":
		receipt.ReadAt = &now
	case "dismiss":
		receipt.DismissedAt = &now
	default:
		return Receipt{}, invalidState("公告 receipt 动作不支持。")
	}
	s.receipts[key] = receipt
	return receipt, nil
}

func (s *Service) userAnnouncementsLocked(userID string, now time.Time, channel string) []Announcement {
	items := make([]Announcement, 0, len(s.items))
	for _, item := range s.items {
		item = s.withReceiptLocked(userID, item)
		if isUserVisible(item, now) && (channel == "" || hasChannel(item, channel)) {
			items = append(items, item)
		}
	}
	sortByPublishDesc(items)
	return items
}

func (s *Service) withReceiptLocked(userID string, item Announcement) Announcement {
	if receipt, ok := s.receipts[receiptKey(userID, item.ID)]; ok {
		copied := receipt
		item.Receipt = &copied
	}
	return item
}

func (s *Service) putLocked(item Announcement) {
	s.items[item.ID] = item
	s.itemsBySlug[item.Slug] = item.ID
}

func (s *Service) appendAuditLocked(action string, item Announcement, user auth.User, reason string, createdAt time.Time) {
	s.auditLogs = append([]AuditLog{{
		ID:                uuid.NewString(),
		Action:            action,
		AnnouncementID:    item.ID,
		AnnouncementTitle: item.Title,
		OperatorID:        user.ID,
		OperatorName:      user.DisplayName,
		Reason:            reason,
		CreatedAt:         createdAt,
	}}, s.auditLogs...)
}

func (s *Service) uniqueSlugLocked(title string) string {
	base := createSlug(title)
	slug := base
	for i := 2; s.itemsBySlug[slug] != ""; i++ {
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return slug
}

func (s *Service) ensureSeedLocked() {
	if s.seeded {
		return
	}
	now := s.now()
	publishAt := now.Add(-2 * time.Hour)
	item := Announcement{
		ID:              uuid.NewString(),
		Slug:            "platform-rules-api-service-publish-update",
		Title:           "API 服务发布规范已调整",
		Summary:         "平台已更新 API 服务发布规范，发布前请确认接入方式、意向金额和站外确认说明符合新要求。",
		ContentMarkdown: "## API 服务发布规范已调整\n\n- 不得在平台填写、粘贴或上传 API Key、账号密码、token 或 session。\n- 买家提交的是购买意向，不是平台内支付订单。",
		Category:        CategoryRules,
		Level:           LevelImportant,
		Status:          StatusPublished,
		Channels:        []string{ChannelMessageCenter, ChannelHomeBanner},
		Audience:        Audience{Type: "all"},
		IsPinned:        true,
		IsDismissible:   false,
		CTALabel:        "查看 API 集市",
		CTAURL:          "/api-market",
		PublishAt:       publishAt,
		Version:         1,
		CreatedBy:       "system",
		UpdatedBy:       "system",
		CreatedAt:       publishAt.Add(-20 * time.Minute),
		UpdatedAt:       publishAt,
	}
	s.putLocked(item)
	s.auditLogs = []AuditLog{{
		ID:                uuid.NewString(),
		Action:            AuditPublished,
		AnnouncementID:    item.ID,
		AnnouncementTitle: item.Title,
		OperatorID:        "system",
		OperatorName:      "系统",
		Reason:            "内存种子公告",
		CreatedAt:         publishAt,
	}}
	s.seeded = true
}

func validateForm(input FormInput) *domain.AppError {
	title := strings.TrimSpace(input.Title)
	summary := strings.TrimSpace(input.Summary)
	content := strings.TrimSpace(input.ContentMarkdown)
	if utf8.RuneCountInString(title) < 2 || utf8.RuneCountInString(title) > 80 {
		return fieldError("title", "标题需为 2 至 80 个字符。")
	}
	if utf8.RuneCountInString(summary) < 10 || utf8.RuneCountInString(summary) > 160 {
		return fieldError("summary", "摘要需为 10 至 160 个字符。")
	}
	if utf8.RuneCountInString(content) < 10 {
		return fieldError("contentMarkdown", "正文不少于 10 个字符。")
	}
	if !validCategories[input.Category] {
		return fieldError("category", "公告分类不支持。")
	}
	if !validLevels[input.Level] {
		return fieldError("level", "公告级别不支持。")
	}
	channels := normalizeChannels(input.Channels)
	if len(channels) == 0 || !contains(channels, ChannelMessageCenter) {
		return fieldError("channels", "展示渠道必须包含公告中心。")
	}
	if input.PublishAt.IsZero() {
		return fieldError("publishAt", "发布时间不能为空。")
	}
	if input.ExpireAt != nil && !input.ExpireAt.After(input.PublishAt) {
		return fieldError("expireAt", "结束时间必须晚于发布时间。")
	}
	if strings.TrimSpace(input.CTAURL) != "" && !validCTAURL(strings.TrimSpace(input.CTAURL)) {
		return fieldError("ctaUrl", "跳转地址只允许站内相对路径或白名单 HTTPS 地址。")
	}
	return nil
}

func requireAdmin(user auth.User) *domain.AppError {
	if !user.IsAdmin {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	return nil
}

func validateChannelFilter(channel string) *domain.AppError {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		return nil
	}
	if channel != ChannelMessageCenter && channel != ChannelHomeBanner {
		return fieldError("channel", "公告渠道不支持。")
	}
	return nil
}

func DisplayStatus(item Announcement, now time.Time) string {
	return displayStatus(item, now)
}

func IsUnread(item Announcement) bool {
	return isUnread(item)
}

func IsDismissed(item Announcement) bool {
	return isDismissed(item)
}

func SortForHome(items []Announcement) {
	sortForHome(items)
}

func HasChannel(item Announcement, channel string) bool {
	return hasChannel(item, channel)
}

func IsUserVisible(item Announcement, now time.Time) bool {
	return isUserVisible(item, now)
}

func displayStatus(item Announcement, now time.Time) string {
	if item.Status == StatusDraft || item.Status == StatusOffline || item.Status == StatusArchived {
		return item.Status
	}
	if item.ExpireAt != nil && !now.Before(*item.ExpireAt) {
		return StatusExpired
	}
	if item.PublishAt.After(now) {
		return StatusScheduled
	}
	return StatusPublished
}

func isUserVisible(item Announcement, now time.Time) bool {
	status := displayStatus(item, now)
	return hasChannel(item, ChannelMessageCenter) && (status == StatusPublished || status == StatusExpired)
}

func isUnread(item Announcement) bool {
	return item.Receipt == nil || item.Receipt.AnnouncementVersion != item.Version || item.Receipt.ReadAt == nil
}

func isDismissed(item Announcement) bool {
	return item.Receipt != nil && item.Receipt.AnnouncementVersion == item.Version && item.Receipt.DismissedAt != nil
}

func sortForHome(items []Announcement) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		aImportantUnread := a.Level == LevelImportant && isUnread(a)
		bImportantUnread := b.Level == LevelImportant && isUnread(b)
		if aImportantUnread != bImportantUnread {
			return aImportantUnread
		}
		aPinnedUnread := a.IsPinned && isUnread(a)
		bPinnedUnread := b.IsPinned && isUnread(b)
		if aPinnedUnread != bPinnedUnread {
			return aPinnedUnread
		}
		if a.IsPinned != b.IsPinned {
			return a.IsPinned
		}
		return a.PublishAt.After(b.PublishAt)
	})
}

func sortByPublishDesc(items []Announcement) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].PublishAt.After(items[j].PublishAt) })
}

func normalizeChannels(channels []string) []string {
	seen := map[string]bool{}
	result := []string{ChannelMessageCenter}
	seen[ChannelMessageCenter] = true
	for _, channel := range channels {
		channel = strings.TrimSpace(channel)
		if (channel == ChannelMessageCenter || channel == ChannelHomeBanner) && !seen[channel] {
			result = append(result, channel)
			seen[channel] = true
		}
	}
	return result
}

func hasChannel(item Announcement, channel string) bool {
	return contains(item.Channels, channel)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func createSlug(title string) string {
	value := strings.ToLower(strings.TrimSpace(title))
	value = slugPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "announcement"
	}
	return value
}

func validCTAURL(value string) bool {
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "https://linux.do/") ||
		strings.HasPrefix(lower, "https://www.linux.do/") ||
		strings.HasPrefix(lower, "https://openai.com/") ||
		strings.HasPrefix(lower, "https://help.openai.com/")
}

func receiptKey(userID, announcementID string) string {
	return userID + ":" + announcementID
}

func notFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Announcement not found", "公告不存在或当前不可见。")
}

func invalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid announcement state", detail)
}

func fieldError(field, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Announcement validation failed", "公告表单校验失败。", field, "invalid", message)
}

var slugPattern = regexp.MustCompile(`[^a-z0-9\p{Han}]+`)

var validCategories = map[string]bool{
	CategoryPlatform:    true,
	CategoryRules:       true,
	CategoryMaintenance: true,
	CategoryFeature:     true,
	CategoryRisk:        true,
	CategoryOperation:   true,
}

var validLevels = map[string]bool{
	LevelNormal:    true,
	LevelImportant: true,
}
