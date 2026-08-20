package contact

import (
	"context"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type contactEmailTestSender struct {
	to   string
	code string
}

func (sender *contactEmailTestSender) SendVerificationCode(_ context.Context, email, code string, _ time.Time) *domain.AppError {
	sender.to = email
	sender.code = code
	return nil
}

func (*contactEmailTestSender) ExposeDevCode() bool { return true }

func TestContactEmailVerificationIsVersionBoundAndPreservesMetadataUpdates(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	sender := &contactEmailTestSender{}
	service := NewServiceWithOptions(nil, func() time.Time { return now }, ServiceOptions{
		EmailVerificationPepper: "contact-email-test-pepper-with-at-least-32-bytes",
		EmailSender:             sender,
	})
	method := createEmailMethodForTest(t, service, "owner", "trade@example.com")

	challenge, appErr := service.StartEmailVerification(ctx, "owner", method.ID)
	if appErr != nil || challenge.DevCode == "" || sender.to != "trade@example.com" || sender.code != challenge.DevCode {
		t.Fatalf("unexpected challenge=%+v sender=%+v error=%v", challenge, sender, appErr)
	}
	verified, _, changed, appErr := service.ConfirmEmailVerificationWithIdempotency(
		ctx, "owner", "contact-email-confirm", "confirm-key", "confirm-hash", method.ID, challenge.DevCode, "confirm-request", contactEmailTestCompletion,
	)
	if appErr != nil || !changed || verified.VerifiedAt == nil {
		t.Fatalf("verify contact email: method=%+v changed=%t error=%v", verified, changed, appErr)
	}
	verifiedVersionID := verified.CurrentVersionID

	now = now.Add(time.Minute)
	metadataUpdated, appErr := service.UpdateMethod(ctx, UpdateContactMethodInput{
		UserID: "owner", MethodID: method.ID, Type: "email", Label: "订单邮箱", Value: "trade@example.com",
		IsDefault: true, Enabled: true, RequestID: "metadata-update",
	})
	if appErr != nil {
		t.Fatalf("metadata update: %v", appErr)
	}
	if metadataUpdated.VerifiedAt == nil || metadataUpdated.CurrentVersionID != verifiedVersionID {
		t.Fatalf("metadata update changed verification/version: before=%+v after=%+v", verified, metadataUpdated)
	}

	staleChallenge, appErr := service.StartEmailVerification(ctx, "owner", method.ID)
	if appErr != nil {
		t.Fatalf("start stale challenge: %v", appErr)
	}
	now = now.Add(time.Minute)
	valueUpdated, appErr := service.UpdateMethod(ctx, UpdateContactMethodInput{
		UserID: "owner", MethodID: method.ID, Type: "email", Label: "订单邮箱", Value: "new-trade@example.com",
		IsDefault: true, Enabled: true, RequestID: "value-update",
	})
	if appErr != nil {
		t.Fatalf("value update: %v", appErr)
	}
	if valueUpdated.VerifiedAt != nil || valueUpdated.CurrentVersionID == verifiedVersionID {
		t.Fatalf("value update retained verification/version: before=%+v after=%+v", metadataUpdated, valueUpdated)
	}
	if _, _, _, appErr := service.ConfirmEmailVerificationWithIdempotency(
		ctx, "owner", "contact-email-confirm", "stale-key", "stale-hash", method.ID, staleChallenge.DevCode, "stale-request", contactEmailTestCompletion,
	); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("stale challenge result = %#v", appErr)
	}
}

func TestContactEmailVerificationRejectsInvalidTargetsAndBoundsAttempts(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 11, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	wechat, appErr := service.CreateMethod(ctx, ContactMethodInput{
		UserID: "owner", Type: "wechat", Label: "微信", Value: "wechat-id", Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create wechat: %v", appErr)
	}
	if _, appErr := service.StartEmailVerification(ctx, "owner", wechat.ID); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("non-email challenge error = %#v", appErr)
	}
	if _, appErr := service.StartEmailVerification(ctx, "owner", "missing"); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("missing challenge error = %#v", appErr)
	}
	if _, appErr := service.StartEmailVerification(ctx, "another-owner", wechat.ID); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("foreign challenge error = %#v", appErr)
	}

	disabled := createEmailMethodForTest(t, service, "owner", "disabled@example.com")
	if _, appErr := service.DeleteMethod(ctx, "owner", disabled.ID); appErr != nil {
		t.Fatalf("disable contact email: %v", appErr)
	}
	if _, appErr := service.StartEmailVerification(ctx, "owner", disabled.ID); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("disabled challenge error = %#v", appErr)
	}

	method := createEmailMethodForTest(t, service, "owner", "attempts@example.com")
	challenge, appErr := service.StartEmailVerification(ctx, "owner", method.ID)
	if appErr != nil {
		t.Fatalf("start challenge: %v", appErr)
	}
	wrongCode := "000000"
	if challenge.DevCode == wrongCode {
		wrongCode = "000001"
	}
	for attempt := 1; attempt <= ContactEmailVerificationMaxAttempts; attempt++ {
		if _, _, _, appErr := service.ConfirmEmailVerificationWithIdempotency(
			ctx, "owner", "contact-email-confirm", "wrong-key-"+time.Duration(attempt).String(), "wrong-hash", method.ID, wrongCode, "wrong-request", contactEmailTestCompletion,
		); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
			t.Fatalf("wrong attempt %d error = %#v", attempt, appErr)
		}
	}
	if _, _, _, appErr := service.ConfirmEmailVerificationWithIdempotency(
		ctx, "owner", "contact-email-confirm", "locked-key", "locked-hash", method.ID, challenge.DevCode, "locked-request", contactEmailTestCompletion,
	); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("locked challenge error = %#v", appErr)
	}
}

func TestContactEmailVerificationRejectsExpiredChallenge(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 11, 30, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	method := createEmailMethodForTest(t, service, "owner", "expired@example.com")
	challenge, appErr := service.StartEmailVerification(ctx, "owner", method.ID)
	if appErr != nil {
		t.Fatalf("start expiring challenge: %v", appErr)
	}
	now = now.Add(ContactEmailVerificationLifetime)
	if _, _, _, appErr := service.ConfirmEmailVerificationWithIdempotency(
		ctx, "owner", "contact-email-confirm", "expired-key", "expired-hash", method.ID, challenge.DevCode, "expired-request", contactEmailTestCompletion,
	); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("expired challenge error = %#v", appErr)
	}
}

func TestContactEmailVerificationConcurrentConfirmationChangesOnce(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	method := createEmailMethodForTest(t, service, "owner", "concurrent@example.com")
	challenge, appErr := service.StartEmailVerification(ctx, "owner", method.ID)
	if appErr != nil {
		t.Fatalf("start challenge: %v", appErr)
	}

	var wait sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for index := range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, _, changed, appErr := service.ConfirmEmailVerificationWithIdempotency(
				ctx, "owner", "contact-email-confirm", "concurrent-key-"+time.Duration(index).String(), "concurrent-hash", method.ID, challenge.DevCode, "concurrent-request", contactEmailTestCompletion,
			)
			if appErr == nil && changed {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	if successes != 1 {
		t.Fatalf("concurrent successes = %d", successes)
	}
	verifiedEvents := 0
	for _, event := range service.MethodAuditEvents() {
		if event.EventType == "contact_method.verified" {
			verifiedEvents++
		}
	}
	if verifiedEvents != 1 {
		t.Fatalf("verified audit events = %d", verifiedEvents)
	}
}

func createEmailMethodForTest(t *testing.T, service *Service, userID, email string) ContactMethod {
	t.Helper()
	method, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: userID, Type: "email", Label: "交易邮箱", Value: email, Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("create email method: %v", appErr)
	}
	return method
}

func contactEmailTestCompletion(method ContactMethod) (idempotency.Completion, *domain.AppError) {
	return idempotency.Completion{
		Status: 200, ContentType: "application/json", Body: []byte(`{}`), ResourceType: "contact_method", ResourceID: method.ID,
	}, nil
}
