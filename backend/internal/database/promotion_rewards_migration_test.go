package database

import (
	"strings"
	"testing"
)

func TestPromotionRewardsMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000073_promotion_rewards.up.sql")
	for _, required := range []string{
		"CREATE TABLE promotion_reward_campaigns",
		"'api_service_referral_v1'",
		"false,\n  false,\n  false",
		"CREATE TABLE referral_codes",
		"code ~ '^[23456789ABCDEFGHJKLMNPQRSTUVWXYZ]{8}$'",
		"CREATE TABLE referral_relations",
		"CHECK (inviter_user_id <> invitee_user_id)",
		"UNIQUE(invitee_user_id)",
		"CREATE TABLE promotion_coupons",
		"'welcome_first_api_service'",
		"'referral_inviter'",
		"'referral_invitee'",
		"'admin_grant'",
		"OR (status = 'revoked' AND promotion_ends_at = promotion_starts_at)",
		"ux_promotion_coupons_welcome_user",
		"ux_promotion_coupons_referral_source",
		"ix_promotion_coupons_reward_activation",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("promotion rewards migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000073_promotion_rewards.down.sql")
	for _, required := range []string{
		"DROP TABLE IF EXISTS promotion_coupons",
		"DROP TABLE IF EXISTS referral_relations",
		"DROP TABLE IF EXISTS referral_codes",
		"DROP TABLE IF EXISTS promotion_reward_campaigns",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("promotion rewards rollback missing %q", required)
		}
	}
}
