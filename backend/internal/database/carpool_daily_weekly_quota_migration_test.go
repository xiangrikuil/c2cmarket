package database

import (
	"strings"
	"testing"
)

func TestCarpoolDailyWeeklyQuotaMigrationPreservesPeriodMapping(t *testing.T) {
	upSQL := readMigrationForTest(t, "000083_carpool_daily_weekly_quota.up.sql")
	for _, required := range []string{
		"RENAME COLUMN weekly_quota_amount TO daily_quota_amount",
		"RENAME COLUMN monthly_quota_amount TO weekly_quota_amount",
		"RENAME CONSTRAINT ck_carpool_listings_weekly_quota_positive",
		"TO ck_carpool_listings_daily_quota_positive",
		"RENAME CONSTRAINT carpool_listings_monthly_quota_amount_check",
		"TO ck_carpool_listings_weekly_quota_nonnegative",
		"RENAME CONSTRAINT carpool_listings_monthly_quota_amount_not_null",
		"TO carpool_listings_weekly_quota_amount_not_null",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("carpool daily/weekly quota migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000083_carpool_daily_weekly_quota.down.sql")
	weeklyToMonthly := strings.Index(downSQL, "RENAME COLUMN weekly_quota_amount TO monthly_quota_amount")
	dailyToWeekly := strings.Index(downSQL, "RENAME COLUMN daily_quota_amount TO weekly_quota_amount")
	if weeklyToMonthly < 0 || dailyToWeekly < 0 || weeklyToMonthly > dailyToWeekly {
		t.Fatal("carpool quota rollback must free weekly_quota_amount before restoring the former weekly column")
	}
}
