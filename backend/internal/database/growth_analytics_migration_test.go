package database

import (
	"strings"
	"testing"
)

func TestGrowthAnalyticsMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000073_growth_analytics.up.sql")
	for _, required := range []string{
		"ADD COLUMN analytics_user_id uuid NOT NULL DEFAULT gen_random_uuid()",
		"CREATE TABLE user_registration_attributions",
		"registration_method IN ('oauth_linux_do', 'email', 'unknown')",
		"source_type IN ('campaign', 'referral', 'direct', 'unknown')",
		"position('?' IN landing_path) = 0",
		"CREATE TABLE user_activity_daily",
		"PRIMARY KEY (user_id, activity_date)",
		"ADD COLUMN first_published_at timestamptz",
		"status IN ('active', 'paused')",
		"publication_status IN ('online', 'owner_paused')",
		"CREATE TRIGGER trg_carpool_first_published_at",
		"CREATE TRIGGER trg_api_service_first_published_at",
		"OLD.first_published_at IS NOT NULL",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("growth analytics migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000073_growth_analytics.down.sql")
	for _, required := range []string{
		"DROP TRIGGER IF EXISTS trg_api_service_first_published_at ON api_services",
		"DROP TRIGGER IF EXISTS trg_carpool_first_published_at ON carpool_listings",
		"DROP TABLE IF EXISTS user_activity_daily",
		"DROP TABLE IF EXISTS user_registration_attributions",
		"DROP COLUMN IF EXISTS analytics_user_id",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("growth analytics rollback missing %q", required)
		}
	}
}
