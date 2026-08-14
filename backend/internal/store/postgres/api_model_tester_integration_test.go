package postgres

import (
	"context"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/apimarket"

	"github.com/google/uuid"
)

func TestPostgresAPIModelTesterAuthorizesDecryptsAndRejectsDestroyedOrderCredential(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 8, 8, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	t.Cleanup(func() {
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, now)
	fixture := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, now, now.Add(-time.Hour), "", nil)

	const apiKey = "model-tester-order-secret"
	encoded, err := store.contactCodec.encode(apiKey, fixture.CredentialID, contactFieldOrderAPIKey)
	if err != nil {
		t.Fatalf("encode model tester credential: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_order_delivery_credentials
		SET api_base_url = 'https://api.example.com/v1',
		    api_key_ciphertext = $2,
		    api_key_nonce = $3,
		    secret_encryption_key_version = $4,
		    secret_encryption_format = $5
		WHERE id = $1
	`, fixture.CredentialID, encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion, encoded.CipherFormat); err != nil {
		t.Fatalf("update model tester credential: %v", err)
	}

	sources, appErr := store.ListAPIModelTestOrderSources(ctx, buyerID)
	if appErr != nil || len(sources) != 1 || sources[0].OrderID != fixture.OrderID || sources[0].BaseURL != "https://api.example.com/v1" {
		t.Fatalf("unexpected order sources: items=%+v error=%v", sources, appErr)
	}
	credential, appErr := store.GetAPIModelTestOrderCredential(ctx, buyerID, fixture.OrderID)
	if appErr != nil || credential.APIKey != apiKey || credential.BaseURL != sources[0].BaseURL {
		t.Fatalf("unexpected order credential: credential=%+v error=%v", credential, appErr)
	}
	_, appErr = store.GetAPIModelTestOrderCredential(ctx, uuid.NewString(), fixture.OrderID)
	if appErr == nil || appErr.Status != http.StatusNotFound || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("expected foreign buyer to receive not found, got %+v", appErr)
	}

	destructionTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin model tester credential destruction: %v", err)
	}
	defer func() { _ = destructionTx.Rollback(context.Background()) }()
	if err := lockAPIOrderCredentialLifecycleInTx(ctx, destructionTx, fixture.OrderID); err != nil {
		t.Fatalf("lock model tester credential destruction: %v", err)
	}
	if _, err := destructionTx.Exec(ctx, `
		UPDATE api_order_delivery_credentials
		SET api_base_url = NULL,
		    instructions = NULL,
		    api_key_ciphertext = NULL,
		    api_key_nonce = NULL,
		    destroyed_at = $2,
		    destroy_reason = 'retention_expired'
		WHERE id = $1
	`, fixture.CredentialID, now.Add(time.Hour)); err != nil {
		t.Fatalf("destroy model tester credential: %v", err)
	}
	type credentialResult struct {
		appErr *domain.AppError
	}
	readResult := make(chan credentialResult, 1)
	go func() {
		_, readErr := store.GetAPIModelTestOrderCredential(context.Background(), buyerID, fixture.OrderID)
		readResult <- credentialResult{appErr: readErr}
	}()
	select {
	case result := <-readResult:
		t.Fatalf("model tester credential read bypassed destruction lock: %+v", result.appErr)
	case <-time.After(100 * time.Millisecond):
	}
	if err := destructionTx.Commit(ctx); err != nil {
		t.Fatalf("commit model tester credential destruction: %v", err)
	}
	select {
	case result := <-readResult:
		appErr = result.appErr
	case <-time.After(5 * time.Second):
		t.Fatal("model tester credential read did not resume after destruction commit")
	}
	if appErr == nil || appErr.Status != http.StatusConflict || appErr.Code != domain.CodeAPIModelTestCredentialUnavailable {
		t.Fatalf("expected destroyed credential to be unavailable, got %+v", appErr)
	}
	sources, appErr = store.ListAPIModelTestOrderSources(ctx, buyerID)
	if appErr != nil || len(sources) != 0 {
		t.Fatalf("destroyed credential remained eligible: items=%+v error=%v", sources, appErr)
	}
}

func TestPostgresAPIServiceProbeConnectionBindingEnforcesOwnerAndReadiness(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	connectionIDs := []string{}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `UPDATE api_services SET probe_connection_id = NULL WHERE owner_user_id = $1`, sellerID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connection_model_changes WHERE changed_by_user_id IN ($1, $2)`, sellerID, buyerID)
		for _, connectionID := range connectionIDs {
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connections WHERE id = $1`, connectionID)
		}
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, now)

	createConnection := func(ownerID, name string, enabled bool, verificationStatus string) apihealth.Connection {
		verifiedAt := (*time.Time)(nil)
		if verificationStatus == apihealth.VerificationVerified {
			value := now
			verifiedAt = &value
		}
		connection, appErr := store.CreateOwnerProbeConnection(ctx, apihealth.Connection{
			OwnerUserID: ownerID, Name: name,
			BaseURL: "https://api.example.com/v1", NormalizedBaseURL: "https://api.example.com/v1",
			CredentialConfigured: true, Enabled: enabled, VerificationStatus: verificationStatus,
			VerifiedAt: verifiedAt, ProbeModel: apihealth.DefaultGPTProbeModel,
			ProbeProtocol: apihealth.ProtocolResponsesV1, AvailableModels: []string{apihealth.DefaultGPTProbeModel},
			ProbeEnvironment:   apihealth.ProbeEnvironmentUSWestV1,
			MeasurementVersion: 1, Version: 1, CreatedAt: now, UpdatedAt: now,
		}, "probe-secret", apihealth.ProbeAuditMutation{Action: apihealth.ProbeAuditCreated, RequestID: "model-tester-create-" + name})
		if appErr != nil {
			t.Fatalf("create %s connection: %v", name, appErr)
		}
		connectionIDs = append(connectionIDs, connection.ID)
		return connection
	}
	ready := createConnection(sellerID, "ready", true, apihealth.VerificationVerified)
	foreign := createConnection(buyerID, "foreign", true, apihealth.VerificationVerified)
	disabled := createConnection(sellerID, "disabled", false, apihealth.VerificationVerified)

	bound, appErr := store.UpdateAPIServiceProbeConnection(ctx, apimarket.UpdateProbeConnectionInput{
		ServiceID: serviceID, OwnerUserID: sellerID, ProbeConnectionID: ready.ID, ExpectedVersion: 1,
	}, now.Add(time.Minute))
	if appErr != nil || bound.ProbeConnectionID != ready.ID || !bound.ProbeReady || bound.Version != 2 {
		t.Fatalf("unexpected ready binding: service=%+v error=%v", bound, appErr)
	}
	for _, connectionID := range []string{foreign.ID, disabled.ID} {
		_, appErr = store.UpdateAPIServiceProbeConnection(ctx, apimarket.UpdateProbeConnectionInput{
			ServiceID: serviceID, OwnerUserID: sellerID, ProbeConnectionID: connectionID, ExpectedVersion: bound.Version,
		}, now.Add(2*time.Minute))
		if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || appErr.Code != domain.CodeValidationFailed ||
			len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "probeConnectionId" {
			t.Fatalf("expected unavailable connection rejection for %s, got %+v", connectionID, appErr)
		}
	}
	unbound, appErr := store.UpdateAPIServiceProbeConnection(ctx, apimarket.UpdateProbeConnectionInput{
		ServiceID: serviceID, OwnerUserID: sellerID, ProbeConnectionID: "", ExpectedVersion: bound.Version,
	}, now.Add(3*time.Minute))
	if appErr != nil || unbound.ProbeConnectionID != "" || unbound.ProbeReady || unbound.Version != 3 {
		t.Fatalf("unexpected unbound service: service=%+v error=%v", unbound, appErr)
	}
}
