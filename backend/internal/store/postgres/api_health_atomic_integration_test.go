package postgres

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

func TestPostgresAPIProbeMutationAuditAndIdempotencyCompletionAreAtomic(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	ownerUserID := uuid.NewString()
	username := "probe-atomic-" + strings.ToLower(uuid.NewString()[:8])
	requestPrefix := "probe-atomic-" + uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, 'Probe Atomic Test', 'active', $3, $3)
	`, ownerUserID, username, now); err != nil {
		t.Fatalf("insert probe owner: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connections WHERE owner_user_id = $1`, ownerUserID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM idempotency_keys WHERE user_id = $1`, ownerUserID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, ownerUserID)
	})

	newConnection := func(at time.Time) apihealth.Connection {
		verifiedAt := at
		connection := apihealth.Connection{
			OwnerUserID: ownerUserID, Name: "Atomic Probe", BaseURL: "https://atomic.example.com/v1",
			NormalizedBaseURL: "https://atomic.example.com/v1", Enabled: true,
			VerificationStatus: apihealth.VerificationVerified, VerifiedAt: &verifiedAt,
			ProbeModel: apihealth.DefaultGPTProbeModel, ProbeProtocol: apihealth.ProtocolResponsesV1,
			AvailableModels: []string{apihealth.DefaultGPTProbeModel}, ProbeEnvironment: apihealth.ProbeEnvironmentUSWestV1,
			MeasurementVersion: 1, Version: 1, CreatedAt: at, UpdatedAt: at,
		}
		connection.HealthSummary = apihealth.BuildSummary(&connection, nil, at)
		return connection
	}
	completionBuilder := func(connection apihealth.Connection) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{"id":"` + connection.ID + `"}`),
			ResourceType: "api_probe_connection", ResourceID: connection.ID,
		}, nil
	}

	failedEntry := beginProbeAtomicIdempotency(t, store, ownerUserID, "create-failure", "hash-create-failure", now)
	failedRequestID := requestPrefix + "-create-failure"
	_, _, appErr := store.CreateOwnerProbeConnectionWithIdempotency(
		ctx, *failedEntry, newConnection(now), "failure-secret",
		apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditCreated, RequestID: failedRequestID, OccurredAt: now},
		func(apihealth.Connection) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{
				Status: http.StatusCreated, ContentType: "application/json", Body: []byte(`{"ok":true}`),
				ResourceType: "api_probe_connection", ResourceID: "not-a-uuid",
			}, nil
		},
	)
	if appErr == nil {
		t.Fatal("completion failure unexpectedly committed")
	}
	assertProbeAtomicCounts(t, store, ownerUserID, failedRequestID, 0, 0)
	assertProbeIdempotencyState(t, store, failedEntry, "processing")
	if cancelErr := store.CancelIdempotency(ctx, failedEntry, now.Add(time.Second)); cancelErr != nil {
		t.Fatalf("cancel failed atomic request: %v", cancelErr)
	}

	createAt := now.Add(time.Minute)
	createEntry := beginProbeAtomicIdempotency(t, store, ownerUserID, "create", "hash-create", createAt)
	createRequestID := requestPrefix + "-create"
	connection, completion, appErr := store.CreateOwnerProbeConnectionWithIdempotency(
		ctx, *createEntry, newConnection(createAt), "probe-secret",
		apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditCreated, RequestID: createRequestID, OccurredAt: createAt},
		completionBuilder,
	)
	if appErr != nil || completion.Status != http.StatusOK || connection.ID == "" {
		t.Fatalf("atomic create connection=%+v completion=%+v error=%v", connection, completion, appErr)
	}
	assertProbeMutationCommitted(t, store, createEntry, createRequestID, apihealth.ProbeAuditCreated)

	replay, replayErr := store.BeginIdempotency(ctx, idempotency.Entry{
		UserID: ownerUserID, RouteKey: createEntry.RouteKey, Key: createEntry.Key, RequestHash: createEntry.RequestHash,
		CreatedAt: createAt.Add(time.Second), ExpiresAt: createAt.Add(idempotency.ProcessingLifetime),
	})
	if replayErr != nil || replay.State != "completed" || replay.ResourceID != connection.ID {
		t.Fatalf("completed replay=%+v error=%v", replay, replayErr)
	}
	assertProbeAtomicCounts(t, store, ownerUserID, createRequestID, 1, 1)

	runUpdate := func(name, expectedAction, requestedAction string, mutate func(*apihealth.Connection)) {
		t.Helper()
		at := connection.UpdatedAt.Add(time.Minute)
		updated := connection
		mutate(&updated)
		updated.Version = connection.Version + 1
		updated.UpdatedAt = at
		entry := beginProbeAtomicIdempotency(t, store, ownerUserID, name, "hash-"+name, at)
		requestID := requestPrefix + "-" + name
		stored, result, updateErr := store.UpdateOwnerProbeConnectionWithIdempotency(
			ctx, *entry, updated, nil, connection.Version,
			apihealth.ProbeAuditMutation{Action: requestedAction, RequestID: requestID, OccurredAt: at},
			completionBuilder,
		)
		if updateErr != nil || result.Status != http.StatusOK {
			t.Fatalf("atomic %s connection=%+v completion=%+v error=%v", name, stored, result, updateErr)
		}
		connection = stored
		assertProbeMutationCommitted(t, store, entry, requestID, expectedAction)
	}

	runUpdate("update", apihealth.ProbeAuditUpdated, apihealth.ProbeAuditUpdated, func(connection *apihealth.Connection) {
		connection.Name = "Atomic Probe Updated"
	})
	runUpdate("disable", apihealth.ProbeAuditDisabled, apihealth.ProbeAuditUpdated, func(connection *apihealth.Connection) {
		connection.Enabled = false
	})
	runUpdate("enable", apihealth.ProbeAuditEnabled, apihealth.ProbeAuditUpdated, func(connection *apihealth.Connection) {
		connection.Enabled = true
	})
	runUpdate("verify", apihealth.ProbeAuditVerifySucceeded, apihealth.ProbeAuditVerifySucceeded, func(connection *apihealth.Connection) {
		connection.VerificationStatus = apihealth.VerificationVerified
	})

	deleteAt := connection.UpdatedAt.Add(time.Minute)
	deleteEntry := beginProbeAtomicIdempotency(t, store, ownerUserID, "delete", "hash-delete", deleteAt)
	deleteRequestID := requestPrefix + "-delete"
	deleteCompletion, appErr := store.DeleteOwnerProbeConnectionWithIdempotency(
		ctx, *deleteEntry, ownerUserID, connection.ID, connection.Version,
		apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditDeleted, RequestID: deleteRequestID, OccurredAt: deleteAt},
		func(deleted apihealth.Connection) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{
				Status: http.StatusNoContent, ResourceType: "api_probe_connection", ResourceID: deleted.ID,
			}, nil
		},
	)
	if appErr != nil || deleteCompletion.Status != http.StatusNoContent {
		t.Fatalf("atomic delete completion=%+v error=%v", deleteCompletion, appErr)
	}
	assertProbeMutationCommitted(t, store, deleteEntry, deleteRequestID, apihealth.ProbeAuditDeleted)
	assertProbeAtomicCounts(t, store, ownerUserID, "", 0, 6)
}

func beginProbeAtomicIdempotency(t *testing.T, store *Store, userID, key, requestHash string, now time.Time) *idempotency.Entry {
	t.Helper()
	entry, appErr := store.BeginIdempotency(context.Background(), idempotency.Entry{
		UserID: userID, RouteKey: "TEST /api-probe-connections/" + key, Key: key, RequestHash: requestHash,
		State: "processing", CreatedAt: now, ExpiresAt: now.Add(idempotency.ProcessingLifetime),
	})
	if appErr != nil {
		t.Fatalf("begin probe idempotency %s: %v", key, appErr)
	}
	return entry
}

func assertProbeMutationCommitted(t *testing.T, store *Store, entry *idempotency.Entry, requestID, action string) {
	t.Helper()
	assertProbeIdempotencyState(t, store, entry, "completed")
	var storedAction string
	if err := store.pool.QueryRow(context.Background(), `
		SELECT action FROM api_probe_connection_events WHERE request_id = $1
	`, requestID).Scan(&storedAction); err != nil {
		t.Fatalf("read probe event %s: %v", requestID, err)
	}
	if storedAction != action {
		t.Fatalf("probe event %s action=%q want=%q", requestID, storedAction, action)
	}
}

func assertProbeIdempotencyState(t *testing.T, store *Store, entry *idempotency.Entry, expected string) {
	t.Helper()
	var state string
	if err := store.pool.QueryRow(context.Background(), `
		SELECT status FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, entry.UserID, entry.RouteKey, entry.Key).Scan(&state); err != nil {
		t.Fatalf("read idempotency state: %v", err)
	}
	if state != expected {
		t.Fatalf("idempotency state=%q want=%q", state, expected)
	}
}

func assertProbeAtomicCounts(t *testing.T, store *Store, ownerUserID, requestID string, wantConnections, wantEvents int) {
	t.Helper()
	var connectionCount, eventCount int
	if err := store.pool.QueryRow(context.Background(), `SELECT count(*) FROM api_probe_connections WHERE owner_user_id = $1`, ownerUserID).Scan(&connectionCount); err != nil {
		t.Fatalf("count probe connections: %v", err)
	}
	query := `SELECT count(*) FROM api_probe_connection_events WHERE owner_user_id = $1`
	arguments := []any{ownerUserID}
	if requestID != "" {
		query += ` AND request_id = $2`
		arguments = append(arguments, requestID)
	}
	if err := store.pool.QueryRow(context.Background(), query, arguments...).Scan(&eventCount); err != nil {
		t.Fatalf("count probe events: %v", err)
	}
	if connectionCount != wantConnections || eventCount != wantEvents {
		t.Fatalf("probe atomic counts connections=%d/%d events=%d/%d", connectionCount, wantConnections, eventCount, wantEvents)
	}
}
