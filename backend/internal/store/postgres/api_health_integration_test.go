package postgres

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/module/apihealth"

	"github.com/google/uuid"
)

func TestAPIHealthPostgresVersionConflictUsesPreconditionFailed(t *testing.T) {
	t.Parallel()
	appErr := apiHealthVersionConflict()
	if appErr.Status != http.StatusPreconditionFailed || appErr.Code != domain.CodeVersionConflict {
		t.Fatalf("unexpected version conflict: %+v", appErr)
	}
}

func TestPostgresAPIProbeConnectionLifecycle(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	connectionID := ""
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `UPDATE api_services SET probe_connection_id = NULL WHERE owner_user_id = $1`, sellerID)
		if connectionID != "" {
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connections WHERE id = $1`, connectionID)
		}
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connection_model_changes WHERE changed_by_user_id = $1`, sellerID)
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, now)

	verifiedAt := now
	connection, appErr := store.CreateOwnerProbeConnection(ctx, apihealth.Connection{
		OwnerUserID:        sellerID,
		Name:               "主 Sub2API",
		BaseURL:            "https://api.example.com/v1",
		NormalizedBaseURL:  "https://api.example.com/v1",
		Enabled:            true,
		VerificationStatus: apihealth.VerificationVerified,
		VerifiedAt:         &verifiedAt,
		ProbeModel:         apihealth.DefaultGPTProbeModel,
		ProbeProtocol:      apihealth.ProtocolResponsesV1,
		AvailableModels:    []string{apihealth.DefaultGPTProbeModel},
		ProbeEnvironment:   apihealth.ProbeEnvironmentUSWestV1,
		MeasurementVersion: 1,
		Version:            1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, "probe-secret-v1", apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditCreated, RequestID: "probe-integration-create"})
	if appErr != nil {
		t.Fatalf("create probe connection: %v", appErr)
	}
	connectionID = connection.ID
	if connection.Version != 1 || !connection.CredentialConfigured {
		t.Fatalf("unexpected created connection: %+v", connection)
	}
	var auditAction, auditRequestID, toVerificationStatus string
	var changedFields []string
	if err := store.pool.QueryRow(ctx, `
		SELECT action, request_id, changed_fields, COALESCE(to_verification_status, '')
		FROM api_probe_connection_events
		WHERE target_connection_id = $1
		ORDER BY occurred_at, id
		LIMIT 1
	`, connection.ID).Scan(&auditAction, &auditRequestID, &changedFields, &toVerificationStatus); err != nil {
		t.Fatalf("read created probe audit event: %v", err)
	}
	if auditAction != apihealth.ProbeAuditCreated || auditRequestID != "probe-integration-create" || toVerificationStatus != apihealth.VerificationVerified || len(changedFields) == 0 {
		t.Fatalf("unexpected created probe audit action=%q request=%q fields=%v status=%q", auditAction, auditRequestID, changedFields, toVerificationStatus)
	}
	if _, found, readErr := store.GetOwnerProbeConnection(ctx, buyerID, connection.ID); readErr != nil || found {
		t.Fatalf("cross-owner connection read was not isolated: found=%t error=%v", found, readErr)
	}
	stored, credential, found, readErr := store.GetOwnerProbeConnectionCredential(ctx, sellerID, connection.ID)
	if readErr != nil || !found || credential != "probe-secret-v1" || stored.ID != connection.ID {
		t.Fatalf("read connection credential: connection=%+v credential=%q found=%t error=%v", stored, credential, found, readErr)
	}

	updated := connection
	updated.Name = "主 Sub2API 已更新"
	updated.Version = 2
	updated.UpdatedAt = now.Add(time.Minute)
	rotatedCredential := "probe-secret-v2"
	if _, staleErr := store.UpdateOwnerProbeConnection(ctx, updated, &rotatedCredential, connection.Version+1, apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditUpdated, RequestID: "probe-integration-stale"}); staleErr == nil || staleErr.Status != http.StatusPreconditionFailed || staleErr.Code != domain.CodeVersionConflict {
		t.Fatalf("stale update did not return version conflict: %+v", staleErr)
	}
	connection, appErr = store.UpdateOwnerProbeConnection(ctx, updated, &rotatedCredential, connection.Version, apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditUpdated, RequestID: "probe-integration-update"})
	if appErr != nil || connection.Version != 2 || connection.Name != updated.Name {
		t.Fatalf("update probe connection: connection=%+v error=%v", connection, appErr)
	}
	_, credential, found, readErr = store.GetOwnerProbeConnectionCredential(ctx, sellerID, connection.ID)
	if readErr != nil || !found || credential != rotatedCredential {
		t.Fatalf("rotated credential was not readable: credential=%q found=%t error=%v", credential, found, readErr)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT action, changed_fields
		FROM api_probe_connection_events
		WHERE target_connection_id = $1 AND request_id = 'probe-integration-update'
	`, connection.ID).Scan(&auditAction, &changedFields); err != nil {
		t.Fatalf("read updated probe audit event: %v", err)
	}
	if auditAction != apihealth.ProbeAuditUpdated || !containsString(changedFields, "name") || !containsString(changedFields, "credential") {
		t.Fatalf("unexpected updated probe audit action=%q fields=%v", auditAction, changedFields)
	}

	if _, err := store.pool.Exec(ctx, `UPDATE api_services SET probe_connection_id = $2 WHERE id = $1`, serviceID, connection.ID); err != nil {
		t.Fatalf("bind service to probe connection: %v", err)
	}
	connection, found, appErr = store.GetOwnerProbeConnection(ctx, sellerID, connection.ID)
	if appErr != nil || !found || len(connection.References) != 1 || connection.References[0].ID != serviceID {
		t.Fatalf("connection reference projection: connection=%+v found=%t error=%v", connection, found, appErr)
	}
	if deleteErr := store.DeleteOwnerProbeConnection(ctx, sellerID, connection.ID, connection.Version, apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditDeleted, RequestID: "probe-integration-blocked-delete"}); deleteErr == nil || deleteErr.Status != http.StatusConflict || deleteErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("referenced connection delete was not rejected: %+v", deleteErr)
	}

	slot := apihealth.SlotStart(now.Add(apihealth.ProbeSlotDuration))
	type claimResult struct {
		jobs []apihealth.ProbeJob
		err  *domain.AppError
	}
	results := make(chan claimResult, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			jobs, claimErr := store.ClaimDueProbes(ctx, slot, slot, 10, 10*time.Second)
			results <- claimResult{jobs: jobs, err: claimErr}
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	jobs := make([]apihealth.ProbeJob, 0, 1)
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent claim failed: %v", result.err)
		}
		jobs = append(jobs, result.jobs...)
	}
	if len(jobs) != 1 || jobs[0].Connection.ID != connection.ID || jobs[0].Credential != rotatedCredential || jobs[0].CredentialError {
		t.Fatalf("same-slot claim was not deduplicated: %+v", jobs)
	}
	ttft := 120
	firstTextAt := slot.Add(120 * time.Millisecond)
	firstDuration := 321
	finalized, appErr := store.FinalizeProbe(ctx, jobs[0].Sample.ID, apihealth.ProbeResult{
		Outcome: apihealth.OutcomeFirstSuccess,
		Attempts: []apihealth.ProbeAttempt{{
			AttemptNumber: 1, StartedAt: slot, FirstTextAt: &firstTextAt, FinishedAt: slot.Add(321 * time.Millisecond),
			HTTPStatus: http.StatusOK, TTFTMS: &ttft, TotalDurationMS: firstDuration, Succeeded: true,
		}},
		TotalDurationMS: firstDuration, HTTPStatus: http.StatusOK, HTTPStatusClass: 2,
		FirstAttemptTTFTMS: &ttft, FirstAttemptTotalDurationMS: &firstDuration,
	}, slot.Add(time.Second))
	if appErr != nil || !finalized {
		t.Fatalf("finalize successful probe: finalized=%t error=%v", finalized, appErr)
	}
	inputs, appErr := store.LoadProbeSummaryInputs(ctx, []string{serviceID}, slot.Add(-time.Hour))
	if appErr != nil || inputs[serviceID].Connection == nil || inputs[serviceID].Connection.ID != connection.ID || len(inputs[serviceID].Samples) != 1 {
		t.Fatalf("shared service summary input: %+v error=%v", inputs, appErr)
	}

	timeoutSlot := slot.Add(apihealth.ProbeSlotDuration)
	timeoutJobs, appErr := store.ClaimDueProbes(ctx, timeoutSlot, timeoutSlot, 10, 10*time.Second)
	if appErr != nil || len(timeoutJobs) != 1 {
		t.Fatalf("claim timeout probe: jobs=%+v error=%v", timeoutJobs, appErr)
	}
	if duplicate, claimErr := store.ClaimDueProbes(ctx, timeoutSlot, timeoutSlot.Add(11*time.Second), 10, 10*time.Second); claimErr != nil || len(duplicate) != 0 {
		t.Fatalf("timeout convergence reclaimed same slot: jobs=%+v error=%v", duplicate, claimErr)
	}
	var timeoutStatus, timeoutCode string
	var timeoutDuration int
	if err := store.pool.QueryRow(ctx, `
		SELECT status, COALESCE(error_code, ''), COALESCE(total_duration_ms, -1)
		FROM api_probe_connection_samples WHERE id = $1
	`, timeoutJobs[0].Sample.ID).Scan(&timeoutStatus, &timeoutCode, &timeoutDuration); err != nil {
		t.Fatalf("read timed-out sample: %v", err)
	}
	if timeoutStatus != apihealth.SampleStatusFailed || timeoutCode != apihealth.ErrorInternalTimeout || timeoutDuration != 10000 {
		t.Fatalf("unexpected timeout state: status=%s code=%s duration=%d", timeoutStatus, timeoutCode, timeoutDuration)
	}

	if _, err := store.pool.Exec(ctx, `UPDATE api_probe_connections SET credential_ciphertext = $2 WHERE id = $1`, connection.ID, []byte{0}); err != nil {
		t.Fatalf("corrupt probe credential: %v", err)
	}
	decryptSlot := timeoutSlot.Add(apihealth.ProbeSlotDuration)
	decryptJobs, appErr := store.ClaimDueProbes(ctx, decryptSlot, decryptSlot, 10, 10*time.Second)
	if appErr != nil || len(decryptJobs) != 1 || !decryptJobs[0].CredentialError || decryptJobs[0].Credential != "" {
		t.Fatalf("credential decryption failure was not surfaced: jobs=%+v error=%v", decryptJobs, appErr)
	}
	if finalized, finalizeErr := store.FinalizeProbe(ctx, decryptJobs[0].Sample.ID, apihealth.ProbeResult{
		TotalDurationMS: 0,
		ErrorCode:       apihealth.ErrorDecryptFailed,
	}, decryptSlot.Add(time.Second)); finalizeErr != nil || !finalized {
		t.Fatalf("finalize decrypt failure: finalized=%t error=%v", finalized, finalizeErr)
	}

	lifecycleAt := decryptSlot.Add(8 * 24 * time.Hour)
	lifecycleResult, appErr := store.RunDataLifecycle(ctx, lifecycleAt, 10, maintenance.Policy{
		SessionRetention:               365 * 24 * time.Hour,
		EmailVerificationRetention:     365 * 24 * time.Hour,
		ReadNotificationRetention:      365 * 24 * time.Hour,
		UnreadNotificationRetention:    365 * 24 * time.Hour,
		DomainEventRetention:           365 * 24 * time.Hour,
		APIDeliveryCredentialRetention: 365 * 24 * time.Hour,
		APIProbeSampleRetention:        7 * 24 * time.Hour,
	})
	if appErr != nil || lifecycleResult.APIProbeSamplesDeleted != 3 {
		t.Fatalf("probe sample retention: result=%+v error=%v", lifecycleResult, appErr)
	}

	cascadeSlot := apihealth.SlotStart(lifecycleAt.Add(apihealth.ProbeSlotDuration))
	cascadeJobs, appErr := store.ClaimDueProbes(ctx, cascadeSlot, cascadeSlot, 10, 10*time.Second)
	if appErr != nil || len(cascadeJobs) != 1 {
		t.Fatalf("claim cascade sample: jobs=%+v error=%v", cascadeJobs, appErr)
	}
	if finalized, finalizeErr := store.FinalizeProbe(ctx, cascadeJobs[0].Sample.ID, apihealth.ProbeResult{TotalDurationMS: 0, ErrorCode: apihealth.ErrorDecryptFailed}, cascadeSlot.Add(time.Second)); finalizeErr != nil || !finalized {
		t.Fatalf("finalize cascade sample: finalized=%t error=%v", finalized, finalizeErr)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE api_services SET probe_connection_id = NULL WHERE id = $1`, serviceID); err != nil {
		t.Fatalf("unbind service: %v", err)
	}
	if deleteErr := store.DeleteOwnerProbeConnection(ctx, sellerID, connection.ID, connection.Version, apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditDeleted, RequestID: "probe-integration-delete"}); deleteErr != nil {
		t.Fatalf("delete unreferenced connection: %v", deleteErr)
	}
	var preservedTargetID string
	if err := store.pool.QueryRow(ctx, `
		SELECT action, target_connection_id::text
		FROM api_probe_connection_events
		WHERE request_id = 'probe-integration-delete'
	`).Scan(&auditAction, &preservedTargetID); err != nil {
		t.Fatalf("read deleted probe audit event: %v", err)
	}
	if auditAction != apihealth.ProbeAuditDeleted || preservedTargetID != connection.ID {
		t.Fatalf("unexpected deleted probe audit action=%q target=%q", auditAction, preservedTargetID)
	}
	connectionID = ""
	var remainingSamples int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM api_probe_connection_samples WHERE connection_id = $1`, connection.ID).Scan(&remainingSamples); err != nil || remainingSamples != 0 {
		t.Fatalf("connection samples did not cascade: count=%d error=%v", remainingSamples, err)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
