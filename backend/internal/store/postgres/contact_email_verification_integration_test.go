package postgres

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/profile"

	"github.com/google/uuid"
)

func TestPostgresContactEmailVerificationIsIndependentAndVersionBound(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	userID := uuid.NewString()
	username := "contact-email-" + strings.ToLower(uuid.NewString()[:8])
	accountEmail := "account-old-" + strings.ToLower(uuid.NewString()[:8]) + "@example.com"
	accountVerifiedAt := now.Add(-time.Hour)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO users (
			id, username, display_name, email, email_verified_at,
			account_status, created_at, updated_at
		)
		VALUES ($1, $2, 'Contact Email Test', $3, $4, 'active', $5, $5)
	`, userID, username, accountEmail, accountVerifiedAt, now); err != nil {
		t.Fatalf("insert contact email user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `
			DELETE FROM email_verification_codes WHERE user_id = $1;
			DELETE FROM domain_events WHERE actor_user_id = $1 OR aggregate_id IN (SELECT id FROM contact_methods WHERE user_id = $1);
			DELETE FROM idempotency_keys WHERE user_id = $1;
			UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1;
			DELETE FROM contact_method_versions WHERE owner_user_id = $1;
			DELETE FROM contact_methods WHERE user_id = $1;
			DELETE FROM users WHERE id = $1;
		`, userID)
	})

	service := contact.NewServiceWithOptions(store, func() time.Time { return now }, contact.ServiceOptions{
		EmailVerificationPepper: "postgres-contact-email-verification-pepper-value",
	})
	method, appErr := service.CreateMethod(ctx, contact.ContactMethodInput{
		UserID: userID, Type: "email", Label: "交易邮箱", Value: "trade@example.com",
		UsageScopes: contact.DefaultUsageScopes(), Enabled: true, RequestID: "contact-email-create",
	})
	if appErr != nil {
		t.Fatalf("create contact email: %v", appErr)
	}
	challenge, appErr := service.StartEmailVerification(ctx, userID, method.ID)
	if appErr != nil || challenge.DevCode == "" {
		t.Fatalf("start contact email verification: challenge=%+v error=%v", challenge, appErr)
	}
	verified, completion, changed, appErr := service.ConfirmEmailVerificationWithIdempotency(
		ctx, userID, "contact-email-confirm", "contact-email-confirm-key", "contact-email-confirm-hash",
		method.ID, challenge.DevCode, "contact-email-confirm", contactEmailIntegrationCompletion,
	)
	if appErr != nil || !changed || verified.VerifiedAt == nil || completion.ResourceID != method.ID {
		t.Fatalf("confirm contact email: method=%+v completion=%+v changed=%t error=%v", verified, completion, changed, appErr)
	}

	var storedAccountEmail string
	var storedAccountVerifiedAt time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT email, email_verified_at FROM users WHERE id = $1
	`, userID).Scan(&storedAccountEmail, &storedAccountVerifiedAt); err != nil {
		t.Fatalf("read account email after contact verification: %v", err)
	}
	if storedAccountEmail != accountEmail || !storedAccountVerifiedAt.Equal(accountVerifiedAt) {
		t.Fatalf("contact verification changed account email: email=%q verified_at=%s", storedAccountEmail, storedAccountVerifiedAt)
	}

	if _, replay, replayChanged, replayErr := service.ConfirmEmailVerificationWithIdempotency(
		ctx, userID, "contact-email-confirm", "contact-email-confirm-key", "contact-email-confirm-hash",
		method.ID, challenge.DevCode, "contact-email-confirm-replay", contactEmailIntegrationCompletion,
	); replayErr != nil || replayChanged || replay.ResourceID != method.ID {
		t.Fatalf("replay contact confirmation: completion=%+v changed=%t error=%v", replay, replayChanged, replayErr)
	}

	accountService := profile.NewServiceWithOptions(store, func() time.Time { return now }, profile.NewDevelopmentEmailSender(), profile.ServiceOptions{
		EmailVerificationPepper: "postgres-account-email-verification-pepper-value",
	})
	accountUser := auth.User{ID: userID, Username: username, DisplayName: "Contact Email Test", Status: auth.AccountStatusActive}
	accountChallenge, appErr := accountService.StartEmailVerification(ctx, accountUser, profile.EmailVerificationStartInput{Email: "account-new@example.com"})
	if appErr != nil || accountChallenge.DevCode == "" {
		t.Fatalf("start account email verification: challenge=%+v error=%v", accountChallenge, appErr)
	}
	now = now.Add(time.Minute)
	if _, appErr := accountService.ConfirmEmailVerification(ctx, accountUser, profile.EmailVerificationConfirmInput{
		Email: "account-new@example.com", Code: accountChallenge.DevCode,
	}); appErr != nil {
		t.Fatalf("confirm account email: %v", appErr)
	}
	methods, appErr := service.ListMethods(ctx, userID)
	if appErr != nil || len(methods) != 1 || methods[0].DisplayValue != "trade@example.com" || methods[0].VerifiedAt == nil {
		t.Fatalf("account email update changed contact email: methods=%+v error=%v", methods, appErr)
	}

	verifiedVersionID := methods[0].CurrentVersionID
	now = now.Add(time.Minute)
	metadataUpdated, appErr := service.UpdateMethod(ctx, contact.UpdateContactMethodInput{
		UserID: userID, MethodID: method.ID, Type: "email", Label: "订单邮箱", Value: "trade@example.com",
		UsageScopes: []string{contact.UsageScopeBuyer}, IsDefault: true, Enabled: true, RequestID: "contact-email-metadata-update",
	})
	if appErr != nil {
		t.Fatalf("update contact metadata: %v", appErr)
	}
	if metadataUpdated.CurrentVersionID != verifiedVersionID || metadataUpdated.VerifiedAt == nil {
		t.Fatalf("metadata update changed contact version or verification: %+v", metadataUpdated)
	}

	staleChallenge, appErr := service.StartEmailVerification(ctx, userID, method.ID)
	if appErr != nil {
		t.Fatalf("start stale contact challenge: %v", appErr)
	}
	now = now.Add(time.Minute)
	valueUpdated, appErr := service.UpdateMethod(ctx, contact.UpdateContactMethodInput{
		UserID: userID, MethodID: method.ID, Type: "email", Label: "订单邮箱", Value: "trade-new@example.com",
		UsageScopes: []string{contact.UsageScopeBuyer}, IsDefault: true, Enabled: true, RequestID: "contact-email-value-update",
	})
	if appErr != nil {
		t.Fatalf("update contact email value: %v", appErr)
	}
	if valueUpdated.CurrentVersionID == verifiedVersionID || valueUpdated.VerifiedAt != nil {
		t.Fatalf("value update retained contact version or verification: %+v", valueUpdated)
	}
	if _, _, _, appErr := service.ConfirmEmailVerificationWithIdempotency(
		ctx, userID, "contact-email-confirm", "contact-email-stale-key", "contact-email-stale-hash",
		method.ID, staleChallenge.DevCode, "contact-email-stale-confirm", contactEmailIntegrationCompletion,
	); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("stale contact challenge result = %#v", appErr)
	}

	now = now.Add(time.Minute)
	concurrentChallenge, appErr := service.StartEmailVerification(ctx, userID, method.ID)
	if appErr != nil {
		t.Fatalf("start concurrent contact challenge: %v", appErr)
	}
	var wait sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	successKey := ""
	for index := range 2 {
		key := "contact-email-concurrent-" + string(rune('a'+index))
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _, didChange, confirmErr := service.ConfirmEmailVerificationWithIdempotency(
				ctx, userID, "contact-email-confirm", key, "contact-email-concurrent-hash",
				method.ID, concurrentChallenge.DevCode, "contact-email-concurrent-confirm", contactEmailIntegrationCompletion,
			)
			if confirmErr == nil && didChange {
				mu.Lock()
				successes++
				successKey = key
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	if successes != 1 {
		t.Fatalf("concurrent contact confirmations succeeded %d times", successes)
	}
	if _, replay, replayChanged, replayErr := service.ConfirmEmailVerificationWithIdempotency(
		ctx, userID, "contact-email-confirm", successKey, "contact-email-concurrent-hash",
		method.ID, concurrentChallenge.DevCode, "contact-email-concurrent-replay", contactEmailIntegrationCompletion,
	); replayErr != nil || replayChanged || replay.ResourceID != method.ID {
		t.Fatalf("replay concurrent winner: completion=%+v changed=%t error=%v", replay, replayChanged, replayErr)
	}

	now = now.Add(time.Minute)
	typeUpdated, appErr := service.UpdateMethod(ctx, contact.UpdateContactMethodInput{
		UserID: userID, MethodID: method.ID, Type: "other", Label: "其他联系", Value: "support-handle",
		UsageScopes: []string{contact.UsageScopeBuyer}, IsDefault: true, Enabled: true, RequestID: "contact-email-type-update",
	})
	if appErr != nil {
		t.Fatalf("update verified email contact type: %v", appErr)
	}
	if typeUpdated.CurrentVersionID == valueUpdated.CurrentVersionID || typeUpdated.VerifiedAt != nil {
		t.Fatalf("type update retained contact version or verification: %+v", typeUpdated)
	}

	var verifiedEvents, completedConfirmations int
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM domain_events
		WHERE aggregate_type = 'contact_method'
		  AND aggregate_id = $1
		  AND event_type = 'contact_method.verified'
	`, method.ID).Scan(&verifiedEvents); err != nil {
		t.Fatalf("count contact verified events: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM idempotency_keys
		WHERE user_id = $1
		  AND route_key = 'contact-email-confirm'
		  AND status = 'completed'
	`, userID).Scan(&completedConfirmations); err != nil {
		t.Fatalf("count completed contact confirmations: %v", err)
	}
	if verifiedEvents != 2 || completedConfirmations != 2 {
		t.Fatalf("unexpected side-effect counts: verified_events=%d completed_confirmations=%d", verifiedEvents, completedConfirmations)
	}
}

func contactEmailIntegrationCompletion(method contact.ContactMethod) (idempotency.Completion, *domain.AppError) {
	return idempotency.Completion{
		Status: 200, ContentType: "application/json", Body: []byte(`{"verified":true}`),
		ResourceType: "contact_method", ResourceID: method.ID,
	}, nil
}
