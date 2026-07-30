package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apimarket"

	"github.com/google/uuid"
)

func TestAPIAccountPaymentSettingsPersistOneEnabledMethod(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC)
	username := "payment-" + strings.ToLower(uuid.NewString()[:20])
	user, appErr := store.EnsureUser(ctx, username, false, now)
	if appErr != nil {
		t.Fatalf("ensure payment settings test user: %v", appErr)
	}
	defer func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_payment_account_options WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	}()

	wechatQR := "data:image/png;base64,d2VjaGF0"
	alipayQR := "data:image/png;base64,YWxpcGF5"
	initial, appErr := store.UpdateAPIAccountPaymentSettings(ctx, apimarket.UpdateAccountPaymentSettingsInput{
		UserID:               user.ID,
		PaymentWindowMinutes: apimarket.DefaultPaymentWindowMinutes,
		PaymentOptions: []apimarket.PaymentOptionInput{
			{PaymentMethod: apimarket.PaymentMethodWechat, Enabled: true, PaymentQRCodeDataURL: wechatQR},
			{PaymentMethod: apimarket.PaymentMethodAlipay, Enabled: false, PaymentInstructions: "备用支付宝", PaymentQRCodeDataURL: alipayQR},
		},
	}, now)
	if appErr != nil {
		t.Fatalf("save initial payment settings: %v", appErr)
	}
	if !initial.PaymentOptions[0].Enabled || initial.PaymentOptions[1].Enabled {
		t.Fatalf("unexpected initial payment settings: %#v", initial.PaymentOptions)
	}

	switchedAt := now.Add(time.Minute)
	_, appErr = store.UpdateAPIAccountPaymentSettings(ctx, apimarket.UpdateAccountPaymentSettingsInput{
		UserID:               user.ID,
		PaymentWindowMinutes: apimarket.DefaultPaymentWindowMinutes,
		PaymentOptions: []apimarket.PaymentOptionInput{
			{PaymentMethod: apimarket.PaymentMethodWechat, Enabled: false, PaymentQRCodeDataURL: wechatQR},
			{PaymentMethod: apimarket.PaymentMethodAlipay, Enabled: true, PaymentInstructions: "支付宝收款", PaymentQRCodeDataURL: alipayQR},
		},
	}, switchedAt)
	if appErr != nil {
		t.Fatalf("switch payment method: %v", appErr)
	}

	saved, appErr := store.GetAPIAccountPaymentSettings(ctx, user.ID)
	if appErr != nil {
		t.Fatalf("read switched payment settings: %v", appErr)
	}
	if len(saved.PaymentOptions) != 2 {
		t.Fatalf("expected both saved method records, got %#v", saved.PaymentOptions)
	}
	var enabledCount int
	for _, option := range saved.PaymentOptions {
		if option.Enabled {
			enabledCount++
			if option.PaymentMethod != apimarket.PaymentMethodAlipay {
				t.Fatalf("unexpected enabled method: %#v", option)
			}
		}
		if option.PaymentMethod == apimarket.PaymentMethodWechat && option.PaymentQRCodeDataURL != wechatQR {
			t.Fatalf("inactive WeChat data was not retained: %#v", option)
		}
	}
	if enabledCount != 1 {
		t.Fatalf("expected exactly one enabled method, got %d", enabledCount)
	}

	_, appErr = store.UpdateAPIAccountPaymentSettings(ctx, apimarket.UpdateAccountPaymentSettingsInput{
		UserID:               user.ID,
		PaymentWindowMinutes: apimarket.DefaultPaymentWindowMinutes,
		PaymentOptions: []apimarket.PaymentOptionInput{
			{PaymentMethod: apimarket.PaymentMethodWechat, Enabled: true, PaymentQRCodeDataURL: wechatQR},
			{PaymentMethod: apimarket.PaymentMethodAlipay, Enabled: true, PaymentQRCodeDataURL: alipayQR},
		},
	}, switchedAt.Add(time.Minute))
	if appErr == nil {
		t.Fatal("expected database constraint to reject two enabled methods")
	}

	afterFailure, readErr := store.GetAPIAccountPaymentSettings(ctx, user.ID)
	if readErr != nil {
		t.Fatalf("read settings after failed switch: %v", readErr)
	}
	var enabledAfterFailure string
	for _, option := range afterFailure.PaymentOptions {
		if option.Enabled {
			enabledAfterFailure = option.PaymentMethod
		}
	}
	if len(afterFailure.PaymentOptions) != 2 || enabledAfterFailure != apimarket.PaymentMethodAlipay {
		t.Fatalf("failed update did not roll back atomically: %#v", afterFailure.PaymentOptions)
	}
}
