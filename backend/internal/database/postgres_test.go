package database

import (
	"testing"
	"time"
)

func TestMigrationReadinessFailsWhenVersionBehindExpected(t *testing.T) {
	checkedAt := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

	status := migrationReadinessStatus(ExpectedMigrationVersion-1, false, checkedAt)

	if status.OK {
		t.Fatalf("expected behind migration version to be degraded: %+v", status)
	}
	if status.ExpectedSchemaVersion != ExpectedMigrationVersion {
		t.Fatalf("expected schema target %d, got %d", ExpectedMigrationVersion, status.ExpectedSchemaVersion)
	}
	if status.SchemaVersion == nil || *status.SchemaVersion != ExpectedMigrationVersion-1 {
		t.Fatalf("unexpected schema version: %+v", status.SchemaVersion)
	}
	if status.SchemaDirty == nil || *status.SchemaDirty {
		t.Fatalf("expected clean dirty flag, got %+v", status.SchemaDirty)
	}
	if status.FailureSummary != "schema migration version is behind expected version" {
		t.Fatalf("unexpected failure summary: %q", status.FailureSummary)
	}
	if !status.CheckedAt.Equal(checkedAt) {
		t.Fatalf("unexpected checkedAt: %s", status.CheckedAt)
	}
}

func TestMigrationReadinessFailsWhenDirty(t *testing.T) {
	status := migrationReadinessStatus(ExpectedMigrationVersion, true, time.Now())

	if status.OK {
		t.Fatalf("expected dirty migration to be degraded")
	}
	if status.FailureSummary != "schema migration is dirty" {
		t.Fatalf("unexpected failure summary: %q", status.FailureSummary)
	}
}

func TestMigrationReadinessPassesAtExpectedVersion(t *testing.T) {
	status := migrationReadinessStatus(ExpectedMigrationVersion, false, time.Now())

	if !status.OK {
		t.Fatalf("expected current migration version to be ready: %+v", status)
	}
}
