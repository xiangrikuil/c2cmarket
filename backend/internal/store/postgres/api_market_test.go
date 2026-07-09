package postgres

import (
	"c2c-market/backend/internal/module/apimarket"
	"testing"
	"time"
)

func TestStoreBuildPaymentOptionsSkipsDisabledEmptyInstructions(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	options := storeBuildPaymentOptions("00000000-0000-0000-0000-000000000001", nil, []apimarket.PaymentOptionInput{
		{
			PaymentMethod:       apimarket.PaymentMethodWechat,
			Enabled:             true,
			PaymentInstructions: "微信收款二维码请按商户站外确认展示。",
		},
		{
			PaymentMethod:       apimarket.PaymentMethodAlipay,
			Enabled:             false,
			PaymentInstructions: " ",
		},
	}, now)

	if len(options) != 1 {
		t.Fatalf("expected one persisted payment option, got %#v", options)
	}
	if options[0].PaymentMethod != apimarket.PaymentMethodWechat || !options[0].Enabled || options[0].PaymentInstructions == "" {
		t.Fatalf("unexpected persisted payment option: %#v", options[0])
	}
}
