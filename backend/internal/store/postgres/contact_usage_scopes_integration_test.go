package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/contact"

	"github.com/google/uuid"
)

func TestPostgresOptionalTransactionContacts(t *testing.T) {
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
		t.Fatalf("inspect optional contact schema: %v", err)
	}
	if hasUsageScopes {
		t.Fatal("contact_methods.usage_scopes still exists; apply migration 117")
	}

	userID := uuid.NewString()
	username := "optional-contact-" + strings.ToLower(uuid.NewString()[:8])
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, 'Optional Contact Test', 'active', now(), now())
	`, userID, username); err != nil {
		t.Fatalf("insert optional contact user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `
			DELETE FROM domain_events WHERE actor_user_id = $1 OR aggregate_id IN (SELECT id FROM contact_methods WHERE user_id = $1);
			UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1;
			DELETE FROM contact_method_versions WHERE owner_user_id = $1;
			DELETE FROM contact_methods WHERE user_id = $1;
			DELETE FROM users WHERE id = $1;
		`, userID)
	})

	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	service := contact.NewService(store, func() time.Time { return now })
	wechat, appErr := service.CreateMethod(ctx, contact.ContactMethodInput{
		UserID: userID, Type: contact.MethodTypeWechat, Label: "微信", Value: "optional-wechat", Enabled: true, RequestID: "wechat-create",
	})
	if appErr != nil {
		t.Fatalf("create WeChat contact: %v", appErr)
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin WeChat lookup transaction: %v", err)
	}
	if _, _, appErr := lockTransactionContactVersionForOwner(ctx, tx, wechat.ID, userID, "ownerContactMethodId", "请选择有效的交易联系方式。"); appErr != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("enabled WeChat was not transaction eligible: %v", appErr)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback WeChat lookup transaction: %v", err)
	}

	if _, err := store.pool.Exec(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, is_default, enabled)
		VALUES ($1, $2, 'wechat', 'Duplicate WeChat', false, true)
	`, uuid.NewString(), userID); !isUniqueViolationOnConstraint(err, "ux_contact_methods_one_enabled_wechat") {
		t.Fatalf("duplicate enabled WeChat did not hit the expected unique index: %v", err)
	}

	unverified, appErr := service.CreateMethod(ctx, contact.ContactMethodInput{
		UserID: userID, Type: contact.MethodTypeEmail, Label: "未验证邮箱", Value: "unverified@example.com", Enabled: true, RequestID: "email-unverified-create",
	})
	if appErr != nil {
		t.Fatalf("create unverified email: %v", appErr)
	}
	tx, err = store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin email lookup transaction: %v", err)
	}
	if _, _, appErr := lockTransactionContactVersionForOwner(ctx, tx, unverified.ID, userID, "buyerContactMethodId", "请选择有效的交易联系方式。"); appErr == nil || appErr.Code != domain.CodeContactMethodRequired {
		_ = tx.Rollback(ctx)
		t.Fatalf("unverified email was transaction eligible: %#v", appErr)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback email lookup transaction: %v", err)
	}

	verifiedAt := now.Add(-time.Hour)
	verified, appErr := service.CreateMethod(ctx, contact.ContactMethodInput{
		UserID: userID, Type: contact.MethodTypeEmail, Label: "已验证邮箱", Value: "verified@example.com", Enabled: true, VerifiedAt: &verifiedAt, RequestID: "email-verified-create",
	})
	if appErr != nil {
		t.Fatalf("create verified email: %v", appErr)
	}
	tx, err = store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin verified email lookup transaction: %v", err)
	}
	if _, _, appErr := lockTransactionContactVersionForOwner(ctx, tx, verified.ID, userID, "buyerContactMethodId", "请选择有效的交易联系方式。"); appErr != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("verified email was not transaction eligible: %v", appErr)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback verified email lookup transaction: %v", err)
	}

	disabled, appErr := service.UpdateMethod(ctx, contact.UpdateContactMethodInput{
		UserID: userID, MethodID: wechat.ID, Type: contact.MethodTypeWechat, Label: "微信", Value: "optional-wechat", Enabled: false, RequestID: "wechat-disable",
	})
	if appErr != nil || disabled.Enabled {
		t.Fatalf("disable WeChat: method=%+v error=%v", disabled, appErr)
	}
	replacement, appErr := service.CreateMethod(ctx, contact.ContactMethodInput{
		UserID: userID, Type: contact.MethodTypeWechat, Label: "备用微信", Value: "replacement-wechat", Enabled: true, RequestID: "wechat-replacement-create",
	})
	if appErr != nil {
		t.Fatalf("create replacement WeChat: %v", appErr)
	}
	deleted, appErr := service.DeleteMethodWithRequestID(ctx, userID, replacement.ID, "wechat-delete")
	if appErr != nil || deleted.Enabled {
		t.Fatalf("delete optional WeChat: method=%+v error=%v", deleted, appErr)
	}
}
