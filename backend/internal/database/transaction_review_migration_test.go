package database

import (
	"strings"
	"testing"
)

func TestTransactionReviewMigrationDefinesDualBlindAndAuditContracts(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000059_transaction_reviews.up.sql")
	for _, required := range []string{
		"CREATE TABLE transaction_reviews",
		"transaction_type text NOT NULL CHECK (transaction_type IN ('carpool_membership', 'api_order'))",
		"AND carpool_membership_id IS NOT NULL",
		"AND api_order_id IS NOT NULL",
		"CREATE UNIQUE INDEX ux_transaction_reviews_carpool_reviewer",
		"CREATE UNIQUE INDEX ux_transaction_reviews_api_order_reviewer",
		"status text NOT NULL DEFAULT 'sealed'",
		"review_deadline_at timestamptz NOT NULL",
		"CREATE TABLE transaction_review_revisions",
		"INSERT INTO transaction_reviews",
		"FROM carpool_reviews legacy",
		"CREATE TRIGGER trg_transaction_review_source",
		"CREATE TRIGGER trg_transaction_review_freeze",
		"CREATE TRIGGER trg_transaction_review_revisions_append_only",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("transaction review migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000059_transaction_reviews.down.sql")
	for _, required := range []string{
		"DROP TABLE IF EXISTS transaction_review_revisions",
		"DROP TABLE IF EXISTS transaction_reviews",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("transaction review rollback missing %q", required)
		}
	}
}
