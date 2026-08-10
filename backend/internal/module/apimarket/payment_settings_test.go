package apimarket

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"
)

func TestAccountPaymentSettingsStartEmptyAndNormalized(t *testing.T) {
	manager := NewManager(nil, nil, nil, time.Now)
	settings, appErr := manager.GetAccountPaymentSettings(context.Background(), auth.User{ID: "user-1"})
	if appErr != nil {
		t.Fatalf("get account payment settings: %v", appErr)
	}
	if settings.PaymentWindowMinutes != DefaultPaymentWindowMinutes {
		t.Fatalf("expected fixed payment window, got %d", settings.PaymentWindowMinutes)
	}
	if len(settings.PaymentOptions) != 2 {
		t.Fatalf("expected normalized WeChat and Alipay options, got %#v", settings.PaymentOptions)
	}
	for _, option := range settings.PaymentOptions {
		if option.Enabled {
			t.Fatalf("expected initial options to be disabled, got %#v", option)
		}
	}
}

func TestAccountPaymentSettingsRequireExactlyOneEnabledMethod(t *testing.T) {
	manager := NewManager(nil, nil, nil, time.Now)
	user := auth.User{ID: "user-1"}
	input := UpdateAccountPaymentSettingsInput{
		PaymentWindowMinutes: DefaultPaymentWindowMinutes,
		PaymentOptions: []PaymentOptionInput{
			{PaymentMethod: PaymentMethodWechat, Enabled: true, PaymentQRCodeDataURL: "data:image/png;base64,d2VjaGF0"},
			{PaymentMethod: PaymentMethodAlipay, Enabled: true, PaymentQRCodeDataURL: "data:image/png;base64,YWxpcGF5"},
		},
	}
	if _, appErr := manager.UpdateAccountPaymentSettings(context.Background(), user, input); appErr == nil {
		t.Fatal("expected multiple enabled payment methods to be rejected")
	}

	input.PaymentOptions[1].Enabled = false
	settings, appErr := manager.UpdateAccountPaymentSettings(context.Background(), user, input)
	if appErr != nil {
		t.Fatalf("save one enabled payment method: %v", appErr)
	}
	if !settings.PaymentOptions[0].Enabled || settings.PaymentOptions[1].Enabled {
		t.Fatalf("expected only WeChat enabled, got %#v", settings.PaymentOptions)
	}

	reloaded, appErr := manager.GetAccountPaymentSettings(context.Background(), user)
	if appErr != nil {
		t.Fatalf("reload account payment settings: %v", appErr)
	}
	if !reloaded.PaymentOptions[0].Enabled || reloaded.PaymentOptions[1].PaymentQRCodeDataURL == "" {
		t.Fatalf("expected saved setting and inactive data to be retained, got %#v", reloaded.PaymentOptions)
	}
}

func TestHasSingleUsableAccountPaymentOptionUsesAccountValidation(t *testing.T) {
	valid := []PaymentOptionInput{
		{PaymentMethod: PaymentMethodWechat, Enabled: true, PaymentQRCodeDataURL: "data:image/png;base64,d2VjaGF0"},
		{PaymentMethod: PaymentMethodAlipay},
	}
	if !HasSingleUsableAccountPaymentOption(valid) {
		t.Fatal("expected validated account payment option to be usable")
	}
	valid[0].PaymentQRCodeDataURL = ""
	if HasSingleUsableAccountPaymentOption(valid) {
		t.Fatal("enabled QR payment without a QR code must be unusable")
	}
}

func TestAccountPaymentSettingsValidationBoundaries(t *testing.T) {
	validOptions := []PaymentOptionInput{
		{PaymentMethod: PaymentMethodWechat, Enabled: true, PaymentQRCodeDataURL: "data:image/png;base64,d2VjaGF0"},
		{PaymentMethod: PaymentMethodAlipay},
	}
	tests := []struct {
		name   string
		input  UpdateAccountPaymentSettingsInput
		field  string
		reason string
	}{
		{
			name: "fixed payment window",
			input: UpdateAccountPaymentSettingsInput{
				PaymentWindowMinutes: 9,
				PaymentOptions:       validOptions,
			},
			field:  "paymentWindowMinutes",
			reason: "fixed",
		},
		{
			name: "one enabled method required",
			input: UpdateAccountPaymentSettingsInput{
				PaymentWindowMinutes: DefaultPaymentWindowMinutes,
				PaymentOptions: []PaymentOptionInput{
					{PaymentMethod: PaymentMethodWechat},
					{PaymentMethod: PaymentMethodAlipay},
				},
			},
			field:  "paymentOptions",
			reason: "single_enabled",
		},
		{
			name: "duplicate methods rejected",
			input: UpdateAccountPaymentSettingsInput{
				PaymentWindowMinutes: DefaultPaymentWindowMinutes,
				PaymentOptions: []PaymentOptionInput{
					{PaymentMethod: PaymentMethodWechat, Enabled: true, PaymentQRCodeDataURL: "data:image/png;base64,d2VjaGF0"},
					{PaymentMethod: PaymentMethodWechat},
				},
			},
			field:  "paymentOptions.1.paymentMethod",
			reason: "duplicate",
		},
		{
			name: "enabled method requires QR code",
			input: UpdateAccountPaymentSettingsInput{
				PaymentWindowMinutes: DefaultPaymentWindowMinutes,
				PaymentOptions: []PaymentOptionInput{
					{PaymentMethod: PaymentMethodWechat, Enabled: true},
					{PaymentMethod: PaymentMethodAlipay},
				},
			},
			field:  "paymentOptions.0.paymentQrCodeDataUrl",
			reason: "required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appErr := validateAccountPaymentSettingsInput(test.input)
			if appErr == nil {
				t.Fatal("expected validation error")
			}
			if len(appErr.FieldErrors) != 1 {
				t.Fatalf("expected one field error, got %+v", appErr)
			}
			fieldErr := appErr.FieldErrors[0]
			if fieldErr.Field != test.field || fieldErr.Code != test.reason {
				t.Fatalf("expected %s/%s, got %+v", test.field, test.reason, fieldErr)
			}
		})
	}
}

func TestValidateOrderSettingsRejectsMultipleEnabledPaymentMethods(t *testing.T) {
	appErr := validateOrderSettingsInput(UpdateOrderSettingsInput{
		AcceptingOrders:      true,
		PaymentWindowMinutes: DefaultPaymentWindowMinutes,
		PaymentOptions: []PaymentOptionInput{
			{PaymentMethod: PaymentMethodWechat, Enabled: true, PaymentQRCodeDataURL: "data:image/png;base64,d2VjaGF0"},
			{PaymentMethod: PaymentMethodAlipay, Enabled: true, PaymentQRCodeDataURL: "data:image/png;base64,YWxpcGF5"},
		},
	})
	if appErr == nil {
		t.Fatal("expected service order settings with two enabled methods to be rejected")
	}
	if len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "paymentOptions" {
		t.Fatalf("expected paymentOptions field error, got %+v", appErr)
	}
}
