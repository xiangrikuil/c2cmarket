package database

import (
	"context"
	"errors"
	"time"

	"c2c-market/backend/internal/health"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ExpectedMigrationVersion int64 = 50

func OpenPostgres(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func PostgresReadiness(ctx context.Context, pool *pgxpool.Pool) health.Status {
	snapshot := health.Status{
		Configured:            pool != nil,
		ExpectedSchemaVersion: ExpectedMigrationVersion,
		CheckedAt:             time.Now().UTC(),
	}
	if !snapshot.Configured {
		snapshot.ExpectedSchemaVersion = 0
		snapshot.OK = true
		return snapshot
	}

	if err := pool.Ping(ctx); err != nil {
		snapshot.FailureSummary = "database ping failed"
		return snapshot
	}

	var version int64
	var dirty bool
	err := pool.QueryRow(ctx, "select version, dirty from schema_migrations").Scan(&version, &dirty)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			snapshot.FailureSummary = "schema migration version missing"
		} else {
			snapshot.FailureSummary = "schema migration query failed"
		}
		return snapshot
	}

	return migrationReadinessStatus(version, dirty, snapshot.CheckedAt)
}

func migrationReadinessStatus(version int64, dirty bool, checkedAt time.Time) health.Status {
	schemaVersion := version
	schemaDirty := dirty
	status := health.Status{
		Configured:            true,
		OK:                    true,
		SchemaVersion:         &schemaVersion,
		SchemaDirty:           &schemaDirty,
		ExpectedSchemaVersion: ExpectedMigrationVersion,
		CheckedAt:             checkedAt,
	}
	if dirty {
		status.OK = false
		status.FailureSummary = "schema migration is dirty"
		return status
	}
	if version < ExpectedMigrationVersion {
		status.OK = false
		status.FailureSummary = "schema migration version is behind expected version"
	}
	return status
}
