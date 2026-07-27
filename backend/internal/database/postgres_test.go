package database

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExpectedMigrationVersionMatchesLatestMigration(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	var latest int64
	for _, path := range paths {
		prefix, _, ok := strings.Cut(filepath.Base(path), "_")
		if !ok {
			continue
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err == nil && version > latest {
			latest = version
		}
	}
	if latest == 0 {
		t.Fatal("no numbered up migrations found")
	}
	if ExpectedMigrationVersion != latest {
		t.Fatalf("expected migration version %d, latest migration is %d", ExpectedMigrationVersion, latest)
	}
}

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

func TestOpenPostgresWithOptionsAppliesPoolAndSessionTimeouts(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}

	options := DefaultPostgresOptions()
	options.MaxConns = 7
	options.MinConns = 0
	options.StatementTimeout = 12 * time.Second
	options.LockTimeout = 4 * time.Second
	options.IdleInTransactionSessionTimeout = 45 * time.Second

	pool, err := OpenPostgresWithOptions(context.Background(), databaseURL, options)
	if err != nil {
		t.Fatalf("open PostgreSQL with options: %v", err)
	}
	defer pool.Close()

	stats := SnapshotPoolStats(pool)
	if stats.MaxConns != options.MaxConns {
		t.Fatalf("expected max pool size %d, got %d", options.MaxConns, stats.MaxConns)
	}

	var statementMatches, lockMatches, idleTransactionMatches bool
	err = pool.QueryRow(context.Background(), `
		SELECT
			current_setting('statement_timeout')::interval = $1::interval,
			current_setting('lock_timeout')::interval = $2::interval,
			current_setting('idle_in_transaction_session_timeout')::interval = $3::interval
	`, postgresDuration(options.StatementTimeout), postgresDuration(options.LockTimeout), postgresDuration(options.IdleInTransactionSessionTimeout)).
		Scan(&statementMatches, &lockMatches, &idleTransactionMatches)
	if err != nil {
		t.Fatalf("read PostgreSQL session timeouts: %v", err)
	}
	if !statementMatches || !lockMatches || !idleTransactionMatches {
		t.Fatalf(
			"session timeouts were not applied: statement=%t lock=%t idle_transaction=%t",
			statementMatches,
			lockMatches,
			idleTransactionMatches,
		)
	}
}
