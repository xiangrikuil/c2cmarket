package postgres

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

func TestPostgresContactUsageScopesRoundTrip(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(store.Close)

	var hasUsageScopes bool
	if err := store.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'contact_methods'
			  AND column_name = 'usage_scopes'
		)
	`).Scan(&hasUsageScopes); err != nil {
		t.Fatalf("inspect contact usage scope schema: %v", err)
	}
	if !hasUsageScopes {
		t.Fatal("contact_methods.usage_scopes is missing; apply migration 93 to C2C_TEST_DATABASE_URL")
	}

	userID := uuid.NewString()
	username := "contact-scopes-" + strings.ToLower(uuid.NewString()[:8])
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, 'Contact Scope Test', 'active', now(), now())
	`, userID, username); err != nil {
		t.Fatalf("insert contact scope user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `
			DELETE FROM domain_events WHERE actor_user_id = $1 OR aggregate_id IN (SELECT id FROM contact_methods WHERE user_id = $1);
			DELETE FROM idempotency_keys WHERE user_id = $1;
			UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1;
			DELETE FROM contact_method_versions WHERE owner_user_id = $1;
			DELETE FROM contact_methods WHERE user_id = $1;
			DELETE FROM users WHERE id = $1;
		`, userID)
	})

	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	service := contact.NewService(store, func() time.Time { return now })
	createInput := contact.ContactMethodInput{
		UserID: userID, Type: "telegram", Label: "Telegram", Value: "postgres-scopes", Enabled: true,
		UsageScopes: []string{contact.UsageScopeDispute, contact.UsageScopeAPIMerchant, contact.UsageScopeBuyer},
		RequestID:   "contact-create",
	}
	buildCompletion := func(method contact.ContactMethod) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: 201, ContentType: "application/json", Body: []byte(`{"created":true}`),
			ResourceType: "contact_method", ResourceID: method.ID,
		}, nil
	}
	created, _, didCreate, appErr := service.CreateMethodWithIdempotency(ctx, userID, "contact-create", "contact-create-key", "contact-create-hash", createInput, buildCompletion)
	if appErr != nil {
		t.Fatalf("create contact method: %v", appErr)
	}
	if !didCreate {
		t.Fatal("first contact create was reported as replay")
	}
	if _, _, didCreate, appErr = service.CreateMethodWithIdempotency(ctx, userID, "contact-create", "contact-create-key", "contact-create-hash", createInput, buildCompletion); appErr != nil || didCreate {
		t.Fatalf("contact create replay: created=%t error=%v", didCreate, appErr)
	}
	rollbackInput := createInput
	rollbackInput.Label = "响应失败回滚"
	rollbackInput.Value = "must-not-persist"
	rollbackInput.RequestID = "contact-builder-failure"
	if _, _, _, appErr = service.CreateMethodWithIdempotency(
		ctx, userID, "contact-create-builder-failure", "contact-builder-failure-key", "contact-builder-failure-hash", rollbackInput,
		func(contact.ContactMethod) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{}, domain.NewError(500, domain.CodeInternalError, "Encoding failed", "测试响应编码失败。")
		},
	); appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected contact completion builder failure, got %#v", appErr)
	}
	var rolledBackMethods, rolledBackEvents int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM contact_methods WHERE user_id = $1 AND label = '响应失败回滚'`, userID).Scan(&rolledBackMethods); err != nil {
		t.Fatalf("count rolled-back contact methods: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM domain_events WHERE aggregate_type = 'contact_method' AND actor_user_id = $1 AND request_id = 'contact-builder-failure'`, userID).Scan(&rolledBackEvents); err != nil {
		t.Fatalf("count rolled-back contact events: %v", err)
	}
	if rolledBackMethods != 0 || rolledBackEvents != 0 {
		t.Fatalf("contact completion failure leaked rows: methods=%d events=%d", rolledBackMethods, rolledBackEvents)
	}
	wantCreated := []string{contact.UsageScopeAPIMerchant, contact.UsageScopeBuyer, contact.UsageScopeDispute}
	if !slices.Equal(created.UsageScopes, wantCreated) {
		t.Fatalf("created scopes = %v, want %v", created.UsageScopes, wantCreated)
	}

	methods, appErr := service.ListMethods(ctx, userID)
	if appErr != nil || len(methods) != 1 || !slices.Equal(methods[0].UsageScopes, wantCreated) {
		t.Fatalf("listed methods did not round-trip scopes: methods=%+v error=%v", methods, appErr)
	}

	updated, _, changed, appErr := service.UpdateMethodWithIdempotency(ctx, userID, "contact-update", "contact-update-key", "contact-update-hash", contact.UpdateContactMethodInput{
		UserID: userID, MethodID: created.ID, Type: "telegram", Label: "工作 Telegram", Value: "postgres-scopes-2", Enabled: true,
		UsageScopes: []string{contact.UsageScopeBuyer},
		RequestID:   "contact-update-scopes",
	}, buildCompletion)
	if appErr != nil || !changed {
		t.Fatalf("update contact method: changed=%t error=%v", changed, appErr)
	}
	if !slices.Equal(updated.UsageScopes, []string{contact.UsageScopeBuyer}) {
		t.Fatalf("updated scopes = %v", updated.UsageScopes)
	}
	if _, _, changed, appErr = service.UpdateMethodWithIdempotency(ctx, userID, "contact-update", "contact-update-key", "contact-update-hash", contact.UpdateContactMethodInput{
		UserID: userID, MethodID: created.ID, Type: "telegram", Label: "工作 Telegram", Value: "postgres-scopes-2", Enabled: true,
		UsageScopes: []string{contact.UsageScopeBuyer}, RequestID: "contact-update-scopes",
	}, buildCompletion); appErr != nil || changed {
		t.Fatalf("update contact replay: changed=%t error=%v", changed, appErr)
	}
	preserved, _, changed, appErr := service.UpdateMethodWithIdempotency(ctx, userID, "contact-update-preserved", "contact-update-preserved-key", "contact-update-preserved-hash", contact.UpdateContactMethodInput{
		UserID: userID, MethodID: created.ID, Type: "telegram", Label: "工作 Telegram", Value: "postgres-scopes-3", Enabled: true,
		RequestID: "contact-update-preserved",
	}, buildCompletion)
	if appErr != nil || !changed || !slices.Equal(preserved.UsageScopes, []string{contact.UsageScopeBuyer}) {
		t.Fatalf("omitted update scopes were not preserved: method=%+v changed=%t error=%v", preserved, changed, appErr)
	}
	failedUpdate := contact.UpdateContactMethodInput{
		UserID: userID, MethodID: created.ID, Type: "telegram", Label: "不应持久化", Value: "must-rollback-update", Enabled: true,
		UsageScopes: []string{contact.UsageScopeBuyer}, RequestID: "contact-update-builder-failure",
	}
	if _, _, _, appErr = service.UpdateMethodWithIdempotency(
		ctx, userID, "contact-update-builder-failure", "contact-update-builder-failure-key", "contact-update-builder-failure-hash", failedUpdate,
		func(contact.ContactMethod) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{}, domain.NewError(500, domain.CodeInternalError, "Encoding failed", "测试响应编码失败。")
		},
	); appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected contact update completion builder failure, got %#v", appErr)
	}
	methods, appErr = service.ListMethods(ctx, userID)
	if appErr != nil || len(methods) != 1 || methods[0].Version != preserved.Version || methods[0].Label != preserved.Label || methods[0].DisplayValue != preserved.DisplayValue {
		t.Fatalf("failed contact update was not rolled back: methods=%+v error=%v", methods, appErr)
	}

	setDefault, _, changed, appErr := service.SetDefaultMethodWithIdempotency(ctx, userID, "contact-default", "contact-default-key", "contact-default-hash", created.ID, "contact-default", buildCompletion)
	if appErr != nil || !changed || !setDefault.IsDefault {
		t.Fatalf("set default failed: method=%+v changed=%t error=%v", setDefault, changed, appErr)
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin scope lookup transaction: %v", err)
	}
	if _, _, appErr := lockContactVersionForOwnerAndScope(ctx, tx, created.ID, userID, contact.UsageScopeCarpoolOwner, "scope mismatch"); appErr == nil || appErr.Code != "CONTACT_METHOD_NOT_OWNED" {
		_ = tx.Rollback(ctx)
		t.Fatalf("buyer-only method was accepted for carpool owner scope: %#v", appErr)
	}
	if _, _, appErr := lockContactVersionForOwnerAndScope(ctx, tx, created.ID, userID, contact.UsageScopeBuyer, "scope mismatch"); appErr != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("buyer scope lookup failed: %v", appErr)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback scope lookup transaction: %v", err)
	}

	deleted, _, changed, appErr := service.DeleteMethodWithIdempotency(ctx, userID, "contact-disable", "contact-disable-key", "contact-disable-hash", created.ID, "contact-disable", buildCompletion)
	if appErr != nil || !changed || !slices.Equal(deleted.UsageScopes, []string{contact.UsageScopeBuyer}) {
		t.Fatalf("deleted method lost scopes: method=%+v changed=%t error=%v", deleted, changed, appErr)
	}

	requiredWechat, _, changed, appErr := service.CreateMethodWithIdempotency(ctx, userID, "required-wechat-create", "required-wechat-create-key", "required-wechat-create-hash", contact.ContactMethodInput{
		UserID: userID, Type: "wechat", Label: "微信", Value: "postgres-required-wechat", Enabled: true,
		UsageScopes: []string{contact.UsageScopeBuyer}, RequestID: "required-wechat-create",
	}, buildCompletion)
	if appErr != nil || !changed || !slices.Equal(requiredWechat.UsageScopes, contact.AllUsageScopes()) {
		t.Fatalf("required wechat create: method=%+v changed=%t error=%v", requiredWechat, changed, appErr)
	}
	if _, _, changed, appErr = service.UpdateMethodWithIdempotency(ctx, userID, "required-wechat-disable", "required-wechat-disable-key", "required-wechat-disable-hash", contact.UpdateContactMethodInput{
		UserID: userID, MethodID: requiredWechat.ID, Type: "wechat", Label: "微信", Value: "postgres-required-wechat", Enabled: false,
	}, buildCompletion); appErr == nil || changed || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("required wechat disable: changed=%t error=%#v", changed, appErr)
	}
	if _, _, changed, appErr = service.DeleteMethodWithIdempotency(ctx, userID, "required-wechat-delete", "required-wechat-delete-key", "required-wechat-delete-hash", requiredWechat.ID, "required-wechat-delete", buildCompletion); appErr == nil || changed || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("required wechat delete: changed=%t error=%#v", changed, appErr)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE contact_methods SET usage_scopes = ARRAY['buyer']::text[] WHERE id = $1`, requiredWechat.ID); err == nil {
		t.Fatal("database allowed required wechat scopes to be narrowed")
	}

	rows, err := store.pool.Query(ctx, `
		SELECT event_type, request_id, metadata_json::text
		FROM domain_events
		WHERE aggregate_type = 'contact_method' AND aggregate_id = $1
		ORDER BY aggregate_version
	`, created.ID)
	if err != nil {
		t.Fatalf("query contact audit events: %v", err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var action, requestID, metadata string
		if err := rows.Scan(&action, &requestID, &metadata); err != nil {
			t.Fatalf("scan contact audit event: %v", err)
		}
		if strings.Contains(metadata, "postgres-scopes") {
			t.Fatalf("contact audit metadata leaked contact value: %s", metadata)
		}
		if requestID == "" || requestID == "unknown" {
			t.Fatalf("contact audit event lost request id: action=%s request=%q", action, requestID)
		}
		actions = append(actions, action)
	}
	wantActions := []string{"contact_method.created", "contact_method.updated", "contact_method.updated", "contact_method.default_changed", "contact_method.disabled"}
	if !slices.Equal(actions, wantActions) {
		t.Fatalf("contact audit actions = %v, want %v", actions, wantActions)
	}

	directDefaultID := uuid.NewString()
	var directDefaultScopes []string
	if err := store.pool.QueryRow(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, is_default, enabled)
		VALUES ($1, $2, 'other', 'Direct default', false, false)
		RETURNING usage_scopes
	`, directDefaultID, userID).Scan(&directDefaultScopes); err != nil {
		t.Fatalf("insert direct default contact: %v", err)
	}
	if !slices.Equal(directDefaultScopes, contact.DefaultUsageScopes()) {
		t.Fatalf("database default scopes = %v, want %v", directDefaultScopes, contact.DefaultUsageScopes())
	}

	invalidScopeSets := [][]string{
		{},
		{contact.UsageScopeBuyer, contact.UsageScopeBuyer},
		{contact.UsageScopeDispute, contact.UsageScopeBuyer},
		{"unknown"},
	}
	for _, usageScopes := range invalidScopeSets {
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO contact_methods (id, user_id, type, label, usage_scopes, is_default, enabled)
			VALUES ($1, $2, 'other', 'Invalid scopes', $3, false, false)
		`, uuid.NewString(), userID, usageScopes); err == nil {
			t.Fatalf("database accepted invalid usage scopes %v", usageScopes)
		}
	}
}
