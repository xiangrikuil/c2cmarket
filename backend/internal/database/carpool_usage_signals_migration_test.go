package database

import (
	"strings"
	"testing"
)

func TestCarpoolUsageSignalsMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000069_carpool_usage_signals.up.sql")
	for _, required := range []string{
		"ADD COLUMN weekly_quota_amount numeric(12,2)",
		"ADD COLUMN follows_official_quota_reset boolean",
		"ADD COLUMN vps_region text",
		"ADD COLUMN supports_mainland_china_direct_connection boolean",
		"ADD COLUMN opening_channel_code text",
		"ADD COLUMN custom_opening_channel text",
		"ADD COLUMN payment_method_code text",
		"ADD COLUMN custom_payment_method text",
		"weekly_quota_amount IS NULL OR weekly_quota_amount > 0",
		"'u_card'",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("carpool usage signal migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000069_carpool_usage_signals.down.sql")
	for _, required := range []string{
		"DROP COLUMN custom_payment_method",
		"DROP COLUMN payment_method_code",
		"DROP COLUMN opening_channel_code",
		"DROP COLUMN supports_mainland_china_direct_connection",
		"DROP COLUMN weekly_quota_amount",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("carpool usage signal rollback missing %q", required)
		}
	}
}
