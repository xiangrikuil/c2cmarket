package database

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"c2c-market/backend/internal/health"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ExpectedMigrationVersion int64 = 80

const postgresReadinessTimeout = 3 * time.Second

type PostgresOptions struct {
	MaxConns                        int32
	MinConns                        int32
	MaxConnLifetime                 time.Duration
	MaxConnIdleTime                 time.Duration
	HealthCheckPeriod               time.Duration
	StatementTimeout                time.Duration
	LockTimeout                     time.Duration
	IdleInTransactionSessionTimeout time.Duration
}

type PoolStats struct {
	AcquireCount            int64
	AcquireDuration         time.Duration
	AcquiredConns           int32
	CanceledAcquireCount    int64
	ConstructingConns       int32
	EmptyAcquireCount       int64
	EmptyAcquireWaitTime    time.Duration
	IdleConns               int32
	MaxConns                int32
	MaxIdleDestroyCount     int64
	MaxLifetimeDestroyCount int64
	NewConnsCount           int64
	TotalConns              int32
}

func DefaultPostgresOptions() PostgresOptions {
	return PostgresOptions{
		MaxConns:                        20,
		MinConns:                        2,
		MaxConnLifetime:                 30 * time.Minute,
		MaxConnIdleTime:                 5 * time.Minute,
		HealthCheckPeriod:               time.Minute,
		StatementTimeout:                15 * time.Second,
		LockTimeout:                     5 * time.Second,
		IdleInTransactionSessionTimeout: 30 * time.Second,
	}
}

func (options PostgresOptions) Validate() error {
	if options.MaxConns < 1 || options.MaxConns > 100 {
		return fmt.Errorf("max connections must be between 1 and 100")
	}
	if options.MinConns < 0 || options.MinConns > options.MaxConns {
		return fmt.Errorf("min connections must be between 0 and max connections")
	}
	if options.MaxConnLifetime < time.Minute || options.MaxConnLifetime > 24*time.Hour {
		return fmt.Errorf("max connection lifetime must be between 1m and 24h")
	}
	if options.MaxConnIdleTime < 30*time.Second || options.MaxConnIdleTime > options.MaxConnLifetime {
		return fmt.Errorf("max connection idle time must be between 30s and max connection lifetime")
	}
	if options.HealthCheckPeriod < 10*time.Second || options.HealthCheckPeriod > 5*time.Minute {
		return fmt.Errorf("health check period must be between 10s and 5m")
	}
	if options.StatementTimeout < time.Second || options.StatementTimeout > 5*time.Minute {
		return fmt.Errorf("statement timeout must be between 1s and 5m")
	}
	if options.LockTimeout < 100*time.Millisecond || options.LockTimeout > options.StatementTimeout {
		return fmt.Errorf("lock timeout must be between 100ms and statement timeout")
	}
	if options.IdleInTransactionSessionTimeout < time.Second || options.IdleInTransactionSessionTimeout > 30*time.Minute {
		return fmt.Errorf("idle transaction timeout must be between 1s and 30m")
	}
	return nil
}

func OpenPostgres(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return OpenPostgresWithOptions(ctx, databaseURL, DefaultPostgresOptions())
}

func OpenPostgresWithOptions(ctx context.Context, databaseURL string, options PostgresOptions) (*pgxpool.Pool, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	poolConfig.MaxConns = options.MaxConns
	poolConfig.MinConns = options.MinConns
	poolConfig.MaxConnLifetime = options.MaxConnLifetime
	poolConfig.MaxConnIdleTime = options.MaxConnIdleTime
	poolConfig.HealthCheckPeriod = options.HealthCheckPeriod
	previousAfterConnect := poolConfig.AfterConnect
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		if previousAfterConnect != nil {
			if err := previousAfterConnect(ctx, conn); err != nil {
				return err
			}
		}
		_, err := conn.Exec(ctx, `
			SELECT
				set_config('statement_timeout', $1, false),
				set_config('lock_timeout', $2, false),
				set_config('idle_in_transaction_session_timeout', $3, false)
		`,
			postgresDuration(options.StatementTimeout),
			postgresDuration(options.LockTimeout),
			postgresDuration(options.IdleInTransactionSessionTimeout),
		)
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func postgresDuration(value time.Duration) string {
	return strconv.FormatInt(value.Milliseconds(), 10) + "ms"
}

func SnapshotPoolStats(pool *pgxpool.Pool) PoolStats {
	if pool == nil {
		return PoolStats{}
	}
	stats := pool.Stat()
	return PoolStats{
		AcquireCount:            stats.AcquireCount(),
		AcquireDuration:         stats.AcquireDuration(),
		AcquiredConns:           stats.AcquiredConns(),
		CanceledAcquireCount:    stats.CanceledAcquireCount(),
		ConstructingConns:       stats.ConstructingConns(),
		EmptyAcquireCount:       stats.EmptyAcquireCount(),
		EmptyAcquireWaitTime:    stats.EmptyAcquireWaitTime(),
		IdleConns:               stats.IdleConns(),
		MaxConns:                stats.MaxConns(),
		MaxIdleDestroyCount:     stats.MaxIdleDestroyCount(),
		MaxLifetimeDestroyCount: stats.MaxLifetimeDestroyCount(),
		NewConnsCount:           stats.NewConnsCount(),
		TotalConns:              stats.TotalConns(),
	}
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

	checkCtx, cancel := context.WithTimeout(ctx, postgresReadinessTimeout)
	defer cancel()
	if err := pool.Ping(checkCtx); err != nil {
		snapshot.FailureSummary = "database ping failed"
		return snapshot
	}

	var version int64
	var dirty bool
	err := pool.QueryRow(checkCtx, "select version, dirty from schema_migrations").Scan(&version, &dirty)
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
