package postgres

import (
	"context"
	"crypto/sha256"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/module/apihealth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAPIHealthPostgresVersionConflictUsesPreconditionFailed(t *testing.T) {
	t.Parallel()
	appErr := apiHealthVersionConflict()
	if appErr.Status != http.StatusPreconditionFailed || appErr.Code != domain.CodeVersionConflict {
		t.Fatalf("unexpected version conflict: %+v", appErr)
	}
}

func TestAPIHealthPostgresConfigAuthorizationAndProbeLifecycle(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_quota_test") {
		t.Fatalf("refusing to run probe integration test against non-dedicated database %q", databaseName)
	}

	now := time.Date(2026, 8, 4, 4, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	contactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	adminID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, contactID, buyerID, buyerContactID, serviceID, now)
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, '探针管理员', 'active', $3, $3)
	`, adminID, "probe-admin-"+adminID[:8], now); err != nil {
		t.Fatalf("seed probe admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_permissions (user_id, permission) VALUES ($1, 'admin')
	`, adminID); err != nil {
		t.Fatalf("seed probe admin permission: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM api_service_probe_authorization_events WHERE api_service_id = $1`, serviceID)
		_, _ = pool.Exec(ctx, `DELETE FROM user_permissions WHERE user_id = $1`, adminID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, adminID)
		cleanupQuotaServiceForTest(t, ctx, pool, sellerID, buyerID)
	})

	codec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKey: "probe-integration-encryption", FingerprintKey: "probe-integration-fingerprint",
		EncryptionKeyVersion: "probe-v1", FingerprintKeyVersion: "probe-v1",
	})
	if err != nil {
		t.Fatalf("create probe codec: %v", err)
	}
	store := &Store{pool: pool, contactCodec: codec}
	credential := "sk-probe-dedicated"
	mutation, err := apihealth.BuildConfigMutation(nil, serviceID, sellerID, apihealth.ConfigInput{
		BaseURL: "https://example.com/v1", Model: "gpt-5", Credential: &credential, Enabled: true,
	}, now)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	config, appErr := store.UpsertOwnerProbeConfig(ctx, mutation, &credential, 0)
	if appErr != nil {
		t.Fatalf("create config: %v", appErr)
	}
	if config.Version != 1 || config.MeasurementVersion != 1 || !config.CredentialConfigured {
		t.Fatalf("unexpected created config: %+v", config)
	}
	if _, appErr := store.AdminDecideProbeConfig(ctx, adminID, uuid.NewString(), 1, true, "", now); appErr == nil || appErr.Status != http.StatusNotFound || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("missing admin probe decision did not return not found: %v", appErr)
	}
	if _, appErr := store.AdminDecideProbeConfig(ctx, adminID, config.ID, config.Version+1, true, "", now); appErr == nil || appErr.Status != http.StatusPreconditionFailed || appErr.Code != domain.CodeVersionConflict {
		t.Fatalf("stale admin probe decision did not return version conflict: %v", appErr)
	}
	if _, found, appErr := store.GetOwnerProbeConfig(ctx, buyerID, serviceID); appErr != nil || found {
		t.Fatalf("cross-owner config read was not isolated: found=%t err=%v", found, appErr)
	}
	pending, appErr := store.ListAdminProbeConfigs(ctx, apihealth.AuthorizationPending, domain.PageRequest{Limit: 10})
	if appErr != nil || len(pending.Items) != 1 {
		t.Fatalf("list pending probe configs: %+v %v", pending, appErr)
	}
	if item := pending.Items[0]; item.ServiceTitle != "Sub2API 短期额度" || item.OwnerDisplayName != "额度卖家" || !strings.HasPrefix(item.OwnerUsername, "quota-seller-") {
		t.Fatalf("admin probe projection missing service/owner labels: %+v", item)
	}
	token := "one-time-challenge-token"
	hash := apiHealthChallengeHash(token)
	if _, appErr := store.CreateProbeChallenge(ctx, sellerID, serviceID, apihealth.AuthorizationMethodDNSTXT, hash, now.Add(15*time.Minute), config.Version+1, now); appErr == nil || appErr.Status != http.StatusPreconditionFailed || appErr.Code != domain.CodeVersionConflict {
		t.Fatalf("stale challenge version did not fail with precondition: %v", appErr)
	}
	config, appErr = store.CreateProbeChallenge(ctx, sellerID, serviceID, apihealth.AuthorizationMethodDNSTXT, hash, now.Add(15*time.Minute), config.Version, now)
	if appErr != nil {
		t.Fatalf("create challenge: %v", appErr)
	}
	challenge, appErr := store.GetProbeChallenge(ctx, sellerID, serviceID)
	if appErr != nil || string(challenge.TokenHash) != string(hash) {
		t.Fatalf("read challenge: %+v %v", challenge, appErr)
	}
	config, appErr = store.CompleteProbeVerification(ctx, sellerID, serviceID, apihealth.AuthorizationMethodDNSTXT, config.Version, true, "", now.Add(time.Minute))
	if appErr != nil || !apihealth.IsAuthorized(config) {
		t.Fatalf("verify config: %+v %v", config, appErr)
	}
	rotatedCredential := "sk-probe-rotated"
	metadataMutation, err := apihealth.BuildConfigMutation(&config, serviceID, sellerID, apihealth.ConfigInput{
		BaseURL: config.BaseURL, Model: config.Model, Credential: &rotatedCredential, Enabled: true,
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("build metadata update: %v", err)
	}
	if metadataMutation.MeasurementInvalidated || metadataMutation.AuthorizationInvalidated {
		t.Fatalf("metadata update unexpectedly invalidated measurement identity: %+v", metadataMutation)
	}
	config, appErr = store.UpsertOwnerProbeConfig(ctx, metadataMutation, &rotatedCredential, config.Version)
	if appErr != nil || !apihealth.IsAuthorized(config) || config.MeasurementVersion != 1 {
		t.Fatalf("update probe credential: %+v %v", config, appErr)
	}
	var invalidationEventCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM api_service_probe_authorization_events
		WHERE probe_config_id = $1 AND action = $2
	`, config.ID, apihealth.AuthorizationActionOriginInvalidated).Scan(&invalidationEventCount); err != nil || invalidationEventCount != 0 {
		t.Fatalf("metadata update appended invalidation event: count=%d err=%v", invalidationEventCount, err)
	}

	slot := apihealth.SlotStart(now.Add(5 * time.Minute))
	type claimResult struct {
		jobs []apihealth.ProbeJob
		err  *domain.AppError
	}
	claimResults := make(chan claimResult, 2)
	startClaims := make(chan struct{})
	var claimWait sync.WaitGroup
	for range 2 {
		claimWait.Add(1)
		go func() {
			defer claimWait.Done()
			<-startClaims
			jobs, claimErr := store.ClaimDueProbes(ctx, slot, slot, 10, 10*time.Second)
			claimResults <- claimResult{jobs: jobs, err: claimErr}
		}()
	}
	close(startClaims)
	claimWait.Wait()
	close(claimResults)
	var jobs []apihealth.ProbeJob
	for result := range claimResults {
		if result.err != nil {
			t.Fatalf("concurrent claim failed: %v", result.err)
		}
		jobs = append(jobs, result.jobs...)
	}
	if len(jobs) != 1 || jobs[0].Credential != rotatedCredential {
		t.Fatalf("concurrent same-slot claim did not produce exactly one job: %+v", jobs)
	}
	duplicate, appErr := store.ClaimDueProbes(ctx, slot, slot.Add(time.Second), 10, 10*time.Second)
	if appErr != nil || len(duplicate) != 0 {
		t.Fatalf("duplicate slot was claimed: %+v %v", duplicate, appErr)
	}
	finalized, appErr := store.FinalizeProbe(ctx, jobs[0].Sample.ID, apihealth.ProbeResult{
		TTFTMS: 750, TotalDurationMS: 900, HTTPStatusClass: 2,
	}, slot.Add(2*time.Second))
	if appErr != nil || !finalized {
		t.Fatalf("finalize probe: finalized=%v err=%v", finalized, appErr)
	}
	inputs, appErr := store.LoadProbeSummaryInputs(ctx, []string{serviceID}, slot.Add(-time.Hour))
	if appErr != nil || len(inputs[serviceID].Samples) != 1 {
		t.Fatalf("load summary inputs: %+v %v", inputs, appErr)
	}

	updatedMutation, err := apihealth.BuildConfigMutation(&config, serviceID, sellerID, apihealth.ConfigInput{
		BaseURL: config.BaseURL, Model: "gpt-5.1", Enabled: true,
	}, slot.Add(time.Minute))
	if err != nil {
		t.Fatalf("build model update: %v", err)
	}
	config, appErr = store.UpsertOwnerProbeConfig(ctx, updatedMutation, nil, config.Version)
	if appErr != nil || config.MeasurementVersion != 2 || config.AuthorizationStatus != apihealth.AuthorizationPending {
		t.Fatalf("update measurement identity: %+v %v", config, appErr)
	}
	var eventAction, eventActorUserID, eventMethod, eventOrigin, eventReason, persistedAuthorizationStatus string
	var persistedMeasurementVersion int64
	if err := pool.QueryRow(ctx, `
		SELECT c.authorization_status, c.measurement_version, e.action,
		       COALESCE(e.actor_user_id::text, ''), COALESCE(e.method, ''),
		       e.origin_snapshot, COALESCE(e.reason, '')
		FROM api_service_probe_configs c
		JOIN api_service_probe_authorization_events e ON e.probe_config_id = c.id
		WHERE c.id = $1 AND e.action = $2
	`, config.ID, apihealth.AuthorizationActionOriginInvalidated).Scan(
		&persistedAuthorizationStatus, &persistedMeasurementVersion, &eventAction,
		&eventActorUserID, &eventMethod, &eventOrigin, &eventReason,
	); err != nil {
		t.Fatalf("read updated config with invalidation event: %v", err)
	}
	if persistedAuthorizationStatus != apihealth.AuthorizationPending || persistedMeasurementVersion != 2 ||
		eventAction != apihealth.AuthorizationActionOriginInvalidated || eventActorUserID != sellerID || eventMethod != "" ||
		eventOrigin != config.NormalizedOrigin || eventReason != apihealth.AuthorizationReasonMeasurementChanged {
		t.Fatalf("unexpected persisted invalidation transition: status=%s measurement=%d action=%s actor=%s method=%s origin=%s reason=%s",
			persistedAuthorizationStatus, persistedMeasurementVersion, eventAction, eventActorUserID, eventMethod, eventOrigin, eventReason)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM api_service_probe_authorization_events
		WHERE probe_config_id = $1 AND action = $2
	`, config.ID, apihealth.AuthorizationActionOriginInvalidated).Scan(&invalidationEventCount); err != nil || invalidationEventCount != 1 {
		t.Fatalf("measurement update invalidation event count: count=%d err=%v", invalidationEventCount, err)
	}
	jobs, appErr = store.ClaimDueProbes(ctx, slot.Add(apihealth.ProbeSlotDuration), slot.Add(apihealth.ProbeSlotDuration), 10, 10*time.Second)
	if appErr != nil || len(jobs) != 0 {
		t.Fatalf("unauthorized config was claimed: %+v %v", jobs, appErr)
	}
	config, appErr = store.AdminDecideProbeConfig(ctx, adminID, config.ID, config.Version, false, "未能确认当前精确公网 origin。", slot.Add(2*time.Minute))
	if appErr != nil || config.AuthorizationStatus != apihealth.AuthorizationRejected || apihealth.IsAuthorized(config) || config.RejectionReason == "" {
		t.Fatalf("reject config: %+v %v", config, appErr)
	}
	nextSlot := slot.Add(apihealth.ProbeSlotDuration)
	jobs, appErr = store.ClaimDueProbes(ctx, nextSlot, nextSlot, 10, 10*time.Second)
	if appErr != nil || len(jobs) != 0 {
		t.Fatalf("rejected config was claimed: %+v %v", jobs, appErr)
	}
	config, appErr = store.AdminDecideProbeConfig(ctx, adminID, config.ID, config.Version, true, "已核对精确公网 origin。", slot.Add(3*time.Minute))
	if appErr != nil || config.AuthorizationStatus != apihealth.AuthorizationApproved || !apihealth.IsAuthorized(config) {
		t.Fatalf("approve config: %+v %v", config, appErr)
	}
	jobs, appErr = store.ClaimDueProbes(ctx, nextSlot, nextSlot, 10, 10*time.Second)
	if appErr != nil || len(jobs) != 1 || jobs[0].Sample.MeasurementVersion != 2 {
		t.Fatalf("claim approved measurement: %+v %v", jobs, appErr)
	}
	timeoutSweepAt := nextSlot.Add(11 * time.Second)
	if sameSlot, sweepErr := store.ClaimDueProbes(ctx, nextSlot, timeoutSweepAt, 10, 10*time.Second); sweepErr != nil || len(sameSlot) != 0 {
		t.Fatalf("timeout sweep reclaimed the same slot: %+v %v", sameSlot, sweepErr)
	}
	var timedOutStatus, timedOutCode string
	var timedOutDuration int
	if err := pool.QueryRow(ctx, `
		SELECT status, COALESCE(error_code, ''), COALESCE(total_duration_ms, -1)
		FROM api_service_probe_samples WHERE id = $1
	`, jobs[0].Sample.ID).Scan(&timedOutStatus, &timedOutCode, &timedOutDuration); err != nil {
		t.Fatalf("read timed-out sample: %v", err)
	}
	if timedOutStatus != apihealth.SampleStatusFailed || timedOutCode != apihealth.ErrorInternalTimeout || timedOutDuration != 10000 {
		t.Fatalf("unexpected timeout convergence: status=%s code=%s duration=%d", timedOutStatus, timedOutCode, timedOutDuration)
	}
	if _, err := pool.Exec(ctx, `UPDATE api_service_probe_configs SET credential_ciphertext = $2 WHERE id = $1`, config.ID, []byte{0}); err != nil {
		t.Fatalf("corrupt probe credential for integration boundary: %v", err)
	}
	decryptSlot := nextSlot.Add(apihealth.ProbeSlotDuration)
	decryptJobs, decryptErr := store.ClaimDueProbes(ctx, decryptSlot, decryptSlot, 10, 10*time.Second)
	if decryptErr != nil || len(decryptJobs) != 1 || !decryptJobs[0].CredentialError || decryptJobs[0].Credential != "" {
		t.Fatalf("corrupt credential was not surfaced as a claim error: %+v %v", decryptJobs, decryptErr)
	}
	if finalized, finalizeErr := store.FinalizeProbe(ctx, decryptJobs[0].Sample.ID, apihealth.ProbeResult{ErrorCode: apihealth.ErrorDecryptFailed}, decryptSlot.Add(time.Second)); finalizeErr != nil || !finalized {
		t.Fatalf("finalize decrypt failure: finalized=%t err=%v", finalized, finalizeErr)
	}
	maintenanceResult, appErr := store.RunDataLifecycle(ctx, nextSlot.Add(8*24*time.Hour), 10, maintenance.Policy{
		SessionRetention:               365 * 24 * time.Hour,
		EmailVerificationRetention:     365 * 24 * time.Hour,
		ReadNotificationRetention:      365 * 24 * time.Hour,
		UnreadNotificationRetention:    365 * 24 * time.Hour,
		DomainEventRetention:           365 * 24 * time.Hour,
		APIDeliveryCredentialRetention: 365 * 24 * time.Hour,
		APIProbeSampleRetention:        7 * 24 * time.Hour,
	})
	if appErr != nil || maintenanceResult.APIProbeSamplesDeleted != 3 {
		t.Fatalf("delete retained probe samples: %+v %v", maintenanceResult, appErr)
	}
	var remainingRunning int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM api_service_probe_samples
		WHERE api_service_id = $1 AND status = 'running'
	`, serviceID).Scan(&remainingRunning); err != nil || remainingRunning != 0 {
		t.Fatalf("maintenance changed running sample: count=%d err=%v", remainingRunning, err)
	}
	if appErr := store.DeleteOwnerProbeConfig(ctx, sellerID, serviceID, config.Version, nextSlot.Add(time.Minute)); appErr != nil {
		t.Fatalf("delete probe config: %v", appErr)
	}
	var sampleCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_service_probe_samples WHERE api_service_id = $1`, serviceID).Scan(&sampleCount); err != nil || sampleCount != 0 {
		t.Fatalf("probe samples did not cascade: count=%d err=%v", sampleCount, err)
	}
}

func apiHealthChallengeHash(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}
