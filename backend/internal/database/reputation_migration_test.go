package database

import (
	"strings"
	"testing"
)

func TestReputationTransactionExclusionMigrationIsAuditableAndReversible(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000057_reputation_transaction_exclusions.up.sql")
	for _, required := range []string{
		"CREATE TABLE reputation_transaction_exclusions",
		"UNIQUE (transaction_type, transaction_id)",
		"excluded_by_admin_id uuid NOT NULL REFERENCES users(id)",
		"restored_by_admin_id uuid REFERENCES users(id)",
		"CREATE TABLE reputation_transaction_exclusion_events",
		"action text NOT NULL CHECK (action IN ('excluded', 'restored'))",
		"FOREIGN KEY (exclusion_id, transaction_type, transaction_id)",
		"CREATE TRIGGER trg_reputation_exclusion_events_append_only",
		"BEFORE UPDATE OR DELETE ON reputation_transaction_exclusion_events",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("reputation exclusion migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000057_reputation_transaction_exclusions.down.sql")
	for _, required := range []string{
		"DROP TRIGGER IF EXISTS trg_reputation_exclusion_events_append_only",
		"DROP FUNCTION IF EXISTS reject_reputation_exclusion_event_mutation()",
		"DROP TABLE IF EXISTS reputation_transaction_exclusion_events",
		"DROP TABLE IF EXISTS reputation_transaction_exclusions",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("reputation exclusion rollback missing %q", required)
		}
	}
}

func TestReputationGovernanceMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000058_reputation_governance.up.sql")
	for _, required := range []string{
		"subject_user_id",
		"CREATE TABLE dispute_reputation_outcomes",
		"role_scope",
		"action_code",
		"source_dispute_outcome_id",
		"CREATE TABLE reputation_governance_events",
		"CREATE TRIGGER trg_reputation_governance_events_append_only",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("reputation governance migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000058_reputation_governance.down.sql")
	if !strings.Contains(downSQL, "DROP TABLE IF EXISTS dispute_reputation_outcomes") {
		t.Fatal("reputation governance rollback must remove outcome table")
	}
}

func TestReputationEngineMigrationUsesOperationSafeDirtyTriggers(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000060_reputation_engine.up.sql")
	for _, required := range []string{
		"CREATE TABLE user_reputation_states",
		"CREATE TABLE user_reputation_history",
		"CREATE TRIGGER trg_user_reputation_history_append_only",
		"CREATE TRIGGER trg_carpool_memberships_reputation_dirty",
		"CREATE TRIGGER trg_api_orders_reputation_dirty",
		"CREATE TRIGGER trg_transaction_reviews_reputation_dirty",
		"CREATE TRIGGER trg_dispute_cases_reputation_dirty",
		"CREATE TRIGGER trg_dispute_outcomes_reputation_dirty",
		"CREATE TRIGGER trg_user_restrictions_reputation_dirty",
		"CREATE TRIGGER trg_linux_do_bindings_reputation_dirty",
		"IF TG_OP = 'INSERT' THEN",
		"ELSIF TG_OP = 'DELETE' THEN",
		"RETURN NULL;",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("reputation engine migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"COALESCE(NEW, OLD)",
		"COALESCE(NEW.reviewee_user_id, OLD.reviewee_user_id)",
		"COALESCE(NEW.api_order_id, OLD.api_order_id)",
		"COALESCE(NEW.transaction_type, OLD.transaction_type)",
		"WHERE role = review_role",
	} {
		if strings.Contains(upSQL, forbidden) {
			t.Fatalf("reputation engine migration contains operation-unsafe trigger expression %q", forbidden)
		}
	}

	downSQL := readMigrationForTest(t, "000060_reputation_engine.down.sql")
	for _, required := range []string{
		"DROP TRIGGER IF EXISTS trg_linux_do_bindings_reputation_dirty",
		"DROP TRIGGER IF EXISTS trg_user_reputation_history_append_only",
		"DROP TABLE IF EXISTS user_reputation_history",
		"DROP TABLE IF EXISTS user_reputation_states",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("reputation engine rollback missing %q", required)
		}
	}
}

func TestSourceAuthorVerificationMigrationIsVersionedAuditableAndDirty(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000061_source_author_verification.up.sql")
	for _, required := range []string{
		"CREATE TABLE source_author_verifications",
		"UNIQUE (resource_type, resource_id)",
		"status text NOT NULL CHECK (status IN ('not_submitted', 'pending', 'verified', 'mismatch', 'expired'))",
		"version bigint NOT NULL DEFAULT 1 CHECK (version > 0)",
		"CREATE TABLE source_author_verification_events",
		"CREATE TRIGGER trg_source_author_verification_events_append_only",
		"BEFORE UPDATE OR DELETE ON source_author_verification_events",
		"CREATE TRIGGER trg_source_author_verifications_reputation_dirty",
		"CREATE TRIGGER trg_carpool_listings_source_reputation_dirty",
		"CREATE TRIGGER trg_api_services_source_reputation_dirty",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("source-author migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000061_source_author_verification.down.sql")
	for _, required := range []string{
		"DROP TRIGGER IF EXISTS trg_source_author_verification_events_append_only",
		"DROP FUNCTION IF EXISTS reject_source_author_verification_event_mutation()",
		"DROP TABLE IF EXISTS source_author_verification_events",
		"DROP TABLE IF EXISTS source_author_verifications",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("source-author rollback missing %q", required)
		}
	}
}
