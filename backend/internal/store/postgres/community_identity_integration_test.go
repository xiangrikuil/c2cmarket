package postgres

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/communityidentity"

	"github.com/google/uuid"
)

func TestPostgresCommunityIdentityBackfillCreatesEventAndNotification(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(store.Close)
	requireCommunityIdentityTestDatabase(t, ctx, store)

	userID := uuid.NewString()
	username := "community-backfill-" + strings.ToLower(uuid.NewString()[:8])
	qualifiedAt := time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO users (
			id, username, display_name, account_status, email_verified_at, created_at, updated_at
		)
		VALUES ($1, $2, 'Community Backfill Test', 'active', $3, $3, $3)
	`, userID, username, qualifiedAt); err != nil {
		t.Fatalf("insert community identity user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `
			DELETE FROM notifications WHERE user_id = $1;
			DELETE FROM domain_events
			WHERE aggregate_id IN (SELECT id FROM user_community_identities WHERE user_id = $1);
			DELETE FROM user_community_identities WHERE user_id = $1;
			DELETE FROM users WHERE id = $1;
		`, userID)
	})

	cutoff := time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)
	grantedAt := time.Date(2026, 8, 18, 2, 30, 0, 0, time.UTC)
	created, appErr := store.BackfillFounding(ctx, cutoff, grantedAt)
	if appErr != nil || created < 1 {
		t.Fatalf("backfill community identities: created=%d error=%v", created, appErr)
	}

	var identityID, eventID, notificationEventID string
	var eventType, actorKind, requestID string
	var metadataJSON []byte
	if err := store.pool.QueryRow(ctx, `
		SELECT identity.id::text, event.id::text, notification.source_event_id::text,
		       event.event_type, event.actor_kind, event.request_id, event.metadata_json::text
		FROM user_community_identities identity
		JOIN domain_events event
		  ON event.aggregate_type = 'community_identity'
		 AND event.aggregate_id = identity.id
		JOIN notifications notification
		  ON notification.user_id = identity.user_id
		 AND notification.target_type = 'community_identity'
		 AND notification.target_id = identity.id
		WHERE identity.user_id = $1
	`, userID).Scan(&identityID, &eventID, &notificationEventID, &eventType, &actorKind, &requestID, &metadataJSON); err != nil {
		t.Fatalf("read community identity event and notification: %v", err)
	}
	if notificationEventID != eventID {
		t.Fatalf("notification source event = %s, want %s", notificationEventID, eventID)
	}
	if eventType != communityidentity.NotificationEventType || actorKind != "system" || requestID != "community-identity-backfill" {
		t.Fatalf("unexpected community identity event: type=%q actor=%q request=%q", eventType, actorKind, requestID)
	}
	var metadata map[string]string
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		t.Fatalf("decode community identity event metadata: %v", err)
	}
	if metadata["identityType"] != "FOUNDING_USER" || metadata["source"] != "BACKFILL" {
		t.Fatalf("unexpected community identity event metadata: %#v", metadata)
	}

	repeated, appErr := store.BackfillFounding(ctx, cutoff, grantedAt.Add(time.Minute))
	if appErr != nil || repeated != 0 {
		t.Fatalf("repeat community identity backfill: created=%d error=%v", repeated, appErr)
	}
	var identityCount, eventCount, notificationCount int
	if err := store.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*)::int FROM user_community_identities WHERE user_id = $1),
		  (SELECT count(*)::int FROM domain_events WHERE aggregate_type = 'community_identity' AND aggregate_id = $2),
		  (SELECT count(*)::int FROM notifications WHERE user_id = $1 AND target_type = 'community_identity' AND target_id = $2)
	`, userID, identityID).Scan(&identityCount, &eventCount, &notificationCount); err != nil {
		t.Fatalf("count repeated community identity side effects: %v", err)
	}
	if identityCount != 1 || eventCount != 1 || notificationCount != 1 {
		t.Fatalf("repeat backfill duplicated rows: identities=%d events=%d notifications=%d", identityCount, eventCount, notificationCount)
	}
}

func requireCommunityIdentityTestDatabase(t *testing.T, ctx context.Context, store *Store) {
	t.Helper()
	var databaseName string
	if err := store.pool.QueryRow(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read community identity test database name: %v", err)
	}
	if databaseName != "c2c_prelaunch_test" {
		t.Fatalf("refusing to run community identity integration test against non-dedicated database %q", databaseName)
	}
}
