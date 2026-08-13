package operationaudit

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

const (
	defaultLimit       = 20
	maximumLimit       = 100
	defaultWindow      = 30 * 24 * time.Hour
	maximumWindow      = 90 * 24 * time.Hour
	maximumSearchBytes = 100
)

type LegacyReader interface {
	AdminAuditLogs(ctx context.Context, user auth.User, filter auth.AdminAuditLogFilter, page domain.PageRequest) (domain.Page[auth.AdminAuditLog], *domain.AppError)
}

type Service struct {
	repo   Repository
	legacy LegacyReader
	now    func() time.Time
}

func NewService(repo Repository, legacy LegacyReader, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, legacy: legacy, now: now}
}

func (s *Service) AdminOperationAuditLogs(ctx context.Context, user auth.User, filter Filter) (domain.Page[Entry], *domain.AppError) {
	if appErr := auth.RequireCapability(user, auth.CapabilityAdminAccess); appErr != nil {
		return domain.Page[Entry]{}, appErr
	}
	query, appErr := normalizeFilter(filter, s.now())
	if appErr != nil {
		return domain.Page[Entry]{}, appErr
	}
	var items []Entry
	if s.repo != nil {
		items, appErr = s.repo.ListOperationAudit(ctx, query)
	} else {
		items, appErr = s.listLegacy(ctx, user, query)
	}
	if appErr != nil {
		return domain.Page[Entry]{}, appErr
	}
	items = safeEntries(items)
	page := domain.Page[Entry]{Items: items}
	if len(items) <= query.Limit {
		return page, nil
	}
	page.Items = append([]Entry(nil), items[:query.Limit]...)
	last := page.Items[len(page.Items)-1]
	next := EncodeCursor(CursorPosition{OccurredAt: last.CreatedAt, SourceKind: last.SourceKind, EventID: last.SourceEventID})
	page.NextCursor = &next
	return page, nil
}

func normalizeFilter(filter Filter, now time.Time) (Query, *domain.AppError) {
	filter.SourceKind = strings.TrimSpace(filter.SourceKind)
	filter.Domain = strings.TrimSpace(filter.Domain)
	filter.Action = strings.TrimSpace(filter.Action)
	filter.ActorKind = strings.TrimSpace(filter.ActorKind)
	filter.ActorUserID = strings.TrimSpace(filter.ActorUserID)
	filter.TargetType = strings.TrimSpace(filter.TargetType)
	filter.TargetID = strings.TrimSpace(filter.TargetID)
	filter.Outcome = strings.TrimSpace(filter.Outcome)
	filter.Search = strings.ToLower(strings.TrimSpace(filter.Search))
	if filter.SourceKind != "" && !contains(SourceKinds, filter.SourceKind) {
		return Query{}, validationError("sourceKind", "来源类型无效。")
	}
	if filter.Domain != "" && !contains(Domains, filter.Domain) {
		return Query{}, validationError("domain", "业务领域无效。")
	}
	if filter.ActorKind != "" && !contains(ActorKinds, filter.ActorKind) {
		return Query{}, validationError("actorKind", "操作者类型无效。")
	}
	if filter.Outcome != "" && !contains(Outcomes, filter.Outcome) {
		return Query{}, validationError("outcome", "操作结果类型无效。")
	}
	if len(filter.Action) > 100 {
		return Query{}, validationError("action", "动作筛选最多 100 字。")
	}
	if len(filter.TargetType) > 100 {
		return Query{}, validationError("targetType", "目标类型筛选最多 100 字。")
	}
	if filter.Action != "" && !registryContainsFilter(filter.SourceKind, filter.Domain, filter.Action, filter.TargetType) {
		return Query{}, validationError("action", "动作筛选无效或与当前来源不匹配。")
	}
	if filter.TargetType != "" && !registryContainsFilter(filter.SourceKind, filter.Domain, filter.Action, filter.TargetType) {
		return Query{}, validationError("targetType", "目标类型筛选无效或与当前来源不匹配。")
	}
	if len(filter.Search) > maximumSearchBytes {
		return Query{}, validationError("search", "搜索内容最多 100 字。")
	}
	for field, value := range map[string]string{"actorUserId": filter.ActorUserID, "targetId": filter.TargetID} {
		if value != "" {
			if _, err := uuid.Parse(value); err != nil {
				return Query{}, validationError(field, field+" 格式不正确。")
			}
		}
	}

	now = now.UTC()
	to, appErr := parseTimeFilter("to", filter.To)
	if appErr != nil {
		return Query{}, appErr
	}
	from, appErr := parseTimeFilter("from", filter.From)
	if appErr != nil {
		return Query{}, appErr
	}
	if to.IsZero() {
		to = now
	}
	if from.IsZero() {
		from = to.Add(-defaultWindow)
	}
	if from.After(to) {
		return Query{}, validationError("from", "开始时间不能晚于结束时间。")
	}
	if to.Sub(from) > maximumWindow {
		return Query{}, validationError("from", "单次查询时间范围不能超过 90 天。")
	}

	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maximumLimit {
		return Query{}, validationError("limit", "limit 必须在 1 到 100 之间。")
	}
	cursor, appErr := DecodeCursor(filter.Cursor)
	if appErr != nil {
		return Query{}, appErr
	}
	if cursor != nil && (cursor.OccurredAt.Before(from) || cursor.OccurredAt.After(to)) {
		return Query{}, invalidCursorError()
	}
	return Query{
		SourceKind: filter.SourceKind, Domain: filter.Domain, Action: filter.Action,
		ActorKind: filter.ActorKind, ActorUserID: filter.ActorUserID,
		TargetType: filter.TargetType, TargetID: filter.TargetID, Outcome: filter.Outcome,
		From: from.UTC(), To: to.UTC(), Search: filter.Search, Limit: limit, Cursor: cursor,
	}, nil
}

func registryContainsFilter(sourceKind, domainName, action, targetType string) bool {
	for _, definition := range ActionRegistry() {
		if sourceKind != "" && definition.SourceKind != sourceKind ||
			domainName != "" && definition.Domain != domainName ||
			action != "" && definition.Action != action ||
			targetType != "" && definition.TargetType != targetType {
			continue
		}
		return true
	}
	return false
}

func parseTimeFilter(field, value string) (time.Time, *domain.AppError) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, validationError(field, field+" 必须是 RFC3339 时间。")
	}
	return parsed.UTC(), nil
}

func safeEntries(items []Entry) []Entry {
	safe := make([]Entry, 0, len(items))
	for _, item := range items {
		definition, ok := LookupAction(item.SourceKind, item.Action, item.TargetType)
		if !ok || !contains(AllowedActorKinds(definition), item.ActorKind) {
			continue
		}
		item.SourceEventID = strings.TrimSpace(item.SourceEventID)
		if _, err := uuid.Parse(item.SourceEventID); err != nil {
			continue
		}
		item.ActorUserID = strings.TrimSpace(item.ActorUserID)
		if item.ActorKind == ActorSystem {
			item.ActorUserID = ""
			item.ActorUsername = ""
		} else if _, err := uuid.Parse(item.ActorUserID); err != nil {
			continue
		}
		item.TargetID = strings.TrimSpace(item.TargetID)
		if item.TargetID != "" {
			if _, err := uuid.Parse(item.TargetID); err != nil {
				continue
			}
		}
		item.RequestID = safeRequestID(item.RequestID)
		item.ID = item.SourceKind + ":" + item.SourceEventID
		item.Domain = definition.Domain
		item.ActionLabel = definition.ActionLabel
		item.Outcome = definition.Outcome
		item.Summary = definition.Summary
		item.DetailPath = BuildDetailPath(definition, item.TargetID)
		item.TargetLabel = ""
		safe = append(safe, item)
	}
	return safe
}

func safeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 200 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return ""
	}
	return value
}

func (s *Service) listLegacy(ctx context.Context, user auth.User, query Query) ([]Entry, *domain.AppError) {
	if s.legacy == nil || (query.SourceKind != "" && query.SourceKind != SourceAdmin) || (query.ActorKind != "" && query.ActorKind != ActorAdmin) {
		return []Entry{}, nil
	}
	filter := auth.AdminAuditLogFilter{
		Action: query.Action, TargetType: query.TargetType,
		ActorUserID: query.ActorUserID, TargetID: query.TargetID,
	}
	legacyItems := make([]auth.AdminAuditLog, 0)
	pageRequest := domain.PageRequest{Limit: 100}
	for len(legacyItems) < 10000 {
		page, appErr := s.legacy.AdminAuditLogs(ctx, user, filter, pageRequest)
		if appErr != nil {
			return nil, appErr
		}
		legacyItems = append(legacyItems, page.Items...)
		if page.NextCursor == nil {
			break
		}
		pageRequest.Cursor = *page.NextCursor
	}
	items := make([]Entry, 0, len(legacyItems))
	for _, item := range legacyItems {
		definition, ok := LookupAction(SourceAdmin, item.Action, item.TargetType)
		if !ok || item.CreatedAt.Before(query.From) || item.CreatedAt.After(query.To) {
			continue
		}
		entry := Entry{
			SourceEventID: item.ID, SourceKind: SourceAdmin, Domain: definition.Domain,
			ActorKind: ActorAdmin, ActorUserID: item.ActorUserID, ActorUsername: item.ActorUsername,
			Action: item.Action, TargetType: item.TargetType, TargetID: item.TargetID,
			RequestID: item.RequestID, CreatedAt: item.CreatedAt.UTC(),
		}
		if !legacyEntryMatches(entry, definition, query) {
			continue
		}
		items = append(items, entry)
	}
	sort.Slice(items, func(i, j int) bool { return entryAfter(items[i], items[j]) })
	if len(items) > query.Limit+1 {
		items = items[:query.Limit+1]
	}
	return items, nil
}

func legacyEntryMatches(item Entry, definition ActionDefinition, query Query) bool {
	if query.Domain != "" && definition.Domain != query.Domain || query.Outcome != "" && definition.Outcome != query.Outcome {
		return false
	}
	if query.Cursor != nil && !tupleBefore(item.CreatedAt, item.SourceKind, item.SourceEventID, *query.Cursor) {
		return false
	}
	if query.Search == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{
		item.SourceEventID, item.SourceKind, definition.Domain, item.Action, definition.ActionLabel,
		item.ActorUsername, item.ActorUserID, item.TargetType, item.TargetID, item.RequestID, definition.Summary,
	}, " "))
	return strings.Contains(haystack, query.Search)
}

func tupleBefore(occurredAt time.Time, sourceKind, eventID string, cursor CursorPosition) bool {
	if !occurredAt.Equal(cursor.OccurredAt) {
		return occurredAt.Before(cursor.OccurredAt)
	}
	if sourceKind != cursor.SourceKind {
		return sourceKind < cursor.SourceKind
	}
	return eventID < cursor.EventID
}

func entryAfter(left, right Entry) bool {
	if !left.CreatedAt.Equal(right.CreatedAt) {
		return left.CreatedAt.After(right.CreatedAt)
	}
	if left.SourceKind != right.SourceKind {
		return left.SourceKind > right.SourceKind
	}
	return left.SourceEventID > right.SourceEventID
}

func contains(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

func validationError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid operation audit query", detail, field, "invalid", detail)
}
