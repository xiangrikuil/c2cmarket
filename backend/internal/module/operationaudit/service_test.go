package operationaudit

import (
	"context"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

type fakeRepository struct {
	query Query
	items []Entry
}

func (f *fakeRepository) ListOperationAudit(_ context.Context, query Query) ([]Entry, *domain.AppError) {
	f.query = query
	return append([]Entry(nil), f.items...), nil
}

func TestAdminOperationAuditLogsRequiresCapabilityAndBuildsStableCursor(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{items: []Entry{
		operationAuditTestEntry(SourceProbe, "created", "api_probe_connection", "30000000-0000-4000-8000-000000000003", ActorUser, now),
		operationAuditTestEntry(SourceDomain, "user.student_identity_assigned", "user", "30000000-0000-4000-8000-000000000002", ActorUser, now),
		operationAuditTestEntry(SourceAdmin, "user.account_status_changed", "user", "30000000-0000-4000-8000-000000000001", ActorAdmin, now),
	}}
	service := NewService(repo, nil, func() time.Time { return now })
	if _, appErr := service.AdminOperationAuditLogs(context.Background(), auth.User{StudentClaim: &auth.StudentEmailClaim{}}, Filter{}); appErr == nil || appErr.Code != domain.CodeCapabilityRequired {
		t.Fatalf("student buyer must not read operation audit: %+v", appErr)
	}
	page, appErr := service.AdminOperationAuditLogs(context.Background(), auth.User{IsAdmin: true}, Filter{Limit: 2})
	if appErr != nil {
		t.Fatalf("list operation audit: %v", appErr)
	}
	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("unexpected page: %+v", page)
	}
	position, appErr := DecodeCursor(*page.NextCursor)
	if appErr != nil || position.SourceKind != SourceDomain || position.EventID != "30000000-0000-4000-8000-000000000002" {
		t.Fatalf("unexpected composite cursor: %+v err=%v", position, appErr)
	}
	if page.Items[0].ID != SourceProbe+":"+page.Items[0].SourceEventID || page.Items[0].DetailPath != "" {
		t.Fatalf("unexpected safe projection: %+v", page.Items[0])
	}
	if !repo.query.From.Equal(now.Add(-defaultWindow)) || !repo.query.To.Equal(now) {
		t.Fatalf("unexpected bounded default window: %+v", repo.query)
	}
}

func TestAdminOperationAuditLogsRejectsInvalidFiltersAndUnsafeRows(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{items: []Entry{
		operationAuditTestEntry(SourceDomain, "unknown.event", "user", "30000000-0000-4000-8000-000000000001", ActorUser, now),
		operationAuditTestEntry(SourceAPIOrder, "api_order.auto_completed", "api_order", "30000000-0000-4000-8000-000000000002", ActorUser, now),
		func() Entry {
			item := operationAuditTestEntry(SourceAdmin, "user.account_status_changed", "user", "30000000-0000-4000-8000-000000000003", ActorAdmin, now)
			item.RequestID = "secret\nheader"
			return item
		}(),
	}}
	service := NewService(repo, nil, func() time.Time { return now })
	page, appErr := service.AdminOperationAuditLogs(context.Background(), auth.User{IsAdmin: true}, Filter{})
	if appErr != nil {
		t.Fatalf("list safe entries: %v", appErr)
	}
	if len(page.Items) != 1 || page.Items[0].RequestID != "" {
		t.Fatalf("unknown/wrong-actor rows must be omitted and unsafe request id redacted: %+v", page.Items)
	}

	invalidFilters := []Filter{
		{SourceKind: "request_log"},
		{Action: "private.secret"},
		{SourceKind: SourceProbe, Action: "api_order.created"},
		{TargetType: "request_log"},
		{ActorKind: "root"},
		{ActorUserID: "not-a-uuid"},
		{From: now.Add(-91 * 24 * time.Hour).Format(time.RFC3339), To: now.Format(time.RFC3339)},
		{From: "not-time"},
		{Limit: 101},
		{Cursor: "not-base64"},
	}
	for _, filter := range invalidFilters {
		if _, appErr := service.AdminOperationAuditLogs(context.Background(), auth.User{IsAdmin: true}, filter); appErr == nil || appErr.Code != domain.CodeValidationFailed {
			t.Fatalf("filter %+v must fail validation: %+v", filter, appErr)
		}
	}
}

func TestActionRegistryIsUniqueAndDetailPathsAreServerOwned(t *testing.T) {
	seen := map[string]struct{}{}
	allowedPatterns := map[string]bool{
		"": true, "/admin/student-registration": true, "/admin/api-orders/{id}": true,
	}
	for _, definition := range ActionRegistry() {
		key := strings.Join([]string{definition.SourceKind, definition.Action, definition.TargetType}, "|")
		if _, exists := seen[key]; exists {
			t.Fatalf("duplicate action registry key %s", key)
		}
		seen[key] = struct{}{}
		if strings.Contains(definition.Summary, "{") || strings.Contains(definition.DetailPattern, "http") {
			t.Fatalf("registry must use static summaries and local route templates: %+v", definition)
		}
		if !allowedPatterns[definition.DetailPattern] {
			t.Fatalf("registry exposes a route that is not implemented: %+v", definition)
		}
	}
	definition, ok := LookupAction(SourceAPIOrder, "api_order.created", "api_order")
	if !ok || BuildDetailPath(definition, "30000000-0000-4000-8000-000000000001") != "/admin/api-orders/30000000-0000-4000-8000-000000000001" {
		t.Fatal("expected implemented API order detail path")
	}
	if BuildDetailPath(definition, "../../logout") != "" {
		t.Fatal("untrusted target ids must never enter detail paths")
	}
	studentDefinition, ok := LookupAction(SourceAdmin, "student_registration.updated", "student_registration_setting")
	if !ok || BuildDetailPath(studentDefinition, "global") != "/admin/student-registration" {
		t.Fatal("expected implemented static student registration route")
	}
}

func operationAuditTestEntry(source, action, targetType, eventID, actorKind string, occurredAt time.Time) Entry {
	actorID := "10000000-0000-4000-8000-000000000001"
	if actorKind == ActorSystem {
		actorID = ""
	}
	return Entry{
		SourceEventID: eventID,
		SourceKind:    source,
		ActorKind:     actorKind,
		ActorUserID:   actorID,
		ActorUsername: "safe-user",
		Action:        action,
		TargetType:    targetType,
		TargetID:      "20000000-0000-4000-8000-000000000001",
		RequestID:     "req-safe",
		CreatedAt:     occurredAt,
	}
}
