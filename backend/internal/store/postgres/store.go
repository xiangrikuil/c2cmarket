package postgres

import (
	"context"
	"time"

	"c2c-market/backend/internal/database"
	"c2c-market/backend/internal/health"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool         *pgxpool.Pool
	contactCodec *contactCodec
}

func Connect(ctx context.Context, databaseURL string) (*Store, error) {
	return ConnectWithContactCrypto(ctx, databaseURL, ContactCryptoConfig{
		EncryptionKey:         "c2cmarket-local-contact-encryption-key-v1",
		FingerprintKey:        "c2cmarket-local-contact-fingerprint-key-v1",
		EncryptionKeyVersion:  "local-dev-v1",
		FingerprintKeyVersion: "local-dev-v1",
		EncryptionKeys: map[string]string{
			"local-dev-v1": "c2cmarket-local-contact-encryption-key-v1",
		},
		FingerprintKeys: map[string]string{
			"local-dev-v1": "c2cmarket-local-contact-fingerprint-key-v1",
		},
	})
}

func ConnectWithContactCrypto(ctx context.Context, databaseURL string, contactCrypto ContactCryptoConfig) (*Store, error) {
	return ConnectWithContactCryptoAndOptions(
		ctx,
		databaseURL,
		contactCrypto,
		database.DefaultPostgresOptions(),
	)
}

func ConnectWithContactCryptoAndOptions(
	ctx context.Context,
	databaseURL string,
	contactCrypto ContactCryptoConfig,
	databaseOptions database.PostgresOptions,
) (*Store, error) {
	pool, err := database.OpenPostgresWithOptions(ctx, databaseURL, databaseOptions)
	if err != nil {
		return nil, err
	}
	codec, err := newContactCodec(contactCrypto)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, contactCodec: codec}, nil
}

func (s *Store) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

func (s *Store) Readiness(ctx context.Context) health.Status {
	if s == nil {
		return database.PostgresReadiness(ctx, nil)
	}
	return database.PostgresReadiness(ctx, s.pool)
}

func (s *Store) ContactCryptoStats() ContactCryptoStats {
	if s == nil || s.contactCodec == nil {
		return ContactCryptoStats{}
	}
	return s.contactCodec.stats()
}

func (s *Store) DatabasePoolStats() database.PoolStats {
	if s == nil {
		return database.PoolStats{}
	}
	return database.SnapshotPoolStats(s.pool)
}

func (s *Store) SlowActiveQueryCount(ctx context.Context, threshold time.Duration) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, nil
	}
	var count int64
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_stat_activity
		WHERE datname = current_database()
		  AND pid <> pg_backend_pid()
		  AND state = 'active'
		  AND query_start < clock_timestamp() - ($1 * interval '1 millisecond')
	`, threshold.Milliseconds()).Scan(&count)
	return count, err
}
