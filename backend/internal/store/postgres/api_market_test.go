package postgres

import (
	"c2c-market/backend/internal/module/apimarket"
	"strings"
	"testing"
	"time"
)

func TestAPIServiceColumnsProjectMerchantIdentityAvatar(t *testing.T) {
	for _, fragment := range []string{
		"merchant_identity_mode = 'store_alias'",
		"SELECT mp.avatar_url FROM merchant_profiles",
		"WHEN u.avatar_mode = 'custom_url'",
		"LEFT JOIN linux_do_bindings l ON l.user_id = u.id",
	} {
		if !strings.Contains(apiServiceColumns, fragment) {
			t.Fatalf("expected API service projection to contain %q", fragment)
		}
	}
}

func TestPublicAPIServicePredicateExcludesActiveSellerRestrictions(t *testing.T) {
	predicate := publicAPIServiceOrderablePredicate("service")
	for _, fragment := range []string{
		"FROM user_restrictions restriction",
		"restriction.role_scope IN ('seller', 'all')",
		"restriction.action_code IN ('api_service_publish', 'all')",
		"restriction.revoked_at IS NULL",
		"restriction.starts_at <= now()",
		"restriction.ends_at IS NULL OR restriction.ends_at > now()",
	} {
		if !strings.Contains(predicate, fragment) {
			t.Fatalf("public API service predicate missing %q", fragment)
		}
	}
}

func TestOwnerAPISalesAggregationUsesOneAuthoritativeChannelProjection(t *testing.T) {
	query := ownerAPISalesAggregationSQL()
	for _, fragment := range []string{
		"WITH flexible_channel AS",
		"limited_candidates AS",
		"limited_channel AS",
		"availableUsdAllowance",
		"availableCopies",
		"nextStartsAt",
		"saleCutoffAt",
		"expiresAt",
		"WHEN 'selling' THEN 0",
		"WHEN 'upcoming' THEN 1",
		"WHEN 'paused' THEN 2",
		"WHEN 'sold_out' THEN 3",
		"WHEN 'expired' THEN 4",
		"jsonb_agg",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("expected owner API sales aggregation to contain %q", fragment)
		}
	}
}

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
