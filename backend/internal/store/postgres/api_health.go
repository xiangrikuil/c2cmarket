package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const apiProbeConnectionColumns = `
	c.id::text, c.owner_user_id::text, c.name, c.base_url, c.normalized_base_url,
	(c.credential_ciphertext IS NOT NULL), c.enabled, c.verification_status,
	c.verified_at, COALESCE(c.last_verification_error_code, ''),
	COALESCE(c.probe_model, ''), COALESCE(c.probe_protocol, ''), c.probe_models_snapshot,
	c.probe_environment, c.probe_model_changed_at, COALESCE(c.probe_price_version_id::text, ''),
	COALESCE(c.probe_input_price_per_million::text, ''), COALESCE(c.probe_cached_input_price_per_million::text, ''),
	COALESCE(c.probe_output_price_per_million::text, ''), COALESCE(c.probe_price_currency, ''),
	c.measurement_version, c.version, c.created_at, c.updated_at
`

const apiProbeSampleColumns = `
	sample.id::text, sample.connection_id::text, sample.measurement_version, sample.slot_started_at,
	sample.status, sample.probe_model, sample.probe_protocol, sample.probe_environment,
	COALESCE(sample.latency_rule_version_id::text, ''), COALESCE(sample.outcome, ''), sample.attempt_count,
	sample.first_attempt_ttft_ms, sample.first_attempt_total_duration_ms, sample.recovery_duration_ms,
	sample.total_duration_ms, sample.http_status_class, sample.final_http_status, COALESCE(sample.error_code, ''),
	sample.input_tokens, sample.cached_input_tokens, sample.output_tokens, sample.reasoning_tokens,
	sample.usage_complete, COALESCE(sample.base_cost_usd::text, ''), COALESCE(sample.retry_cost_usd::text, ''),
	sample.started_at, sample.finished_at, sample.created_at
`

type probeConnectionAuditEvent struct {
	TargetConnectionID     string
	OwnerUserID            string
	Action                 string
	ChangedFields          []string
	RequestID              string
	OccurredAt             time.Time
	FromVerificationStatus string
	ToVerificationStatus   string
	OldMeasurementVersion  *int64
	NewMeasurementVersion  *int64
	OldModel               *string
	NewModel               *string
	OldProtocol            *string
	NewProtocol            *string
	Environment            *string
}

func (store *Store) ListOwnerProbeConnections(ctx context.Context, ownerUserID string) ([]apihealth.Connection, *domain.AppError) {
	if store == nil || store.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := store.pool.Query(ctx, `SELECT `+apiProbeConnectionColumns+` FROM api_probe_connections c WHERE c.owner_user_id = $1 ORDER BY c.updated_at DESC, c.id DESC`, ownerUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	connections := make([]apihealth.Connection, 0)
	connectionIDs := make([]string, 0)
	for rows.Next() {
		var connection apihealth.Connection
		if err := scanAPIProbeConnection(rows, &connection); err != nil {
			return nil, internalStoreError()
		}
		connections = append(connections, connection)
		connectionIDs = append(connectionIDs, connection.ID)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	if appErr := store.loadProbeConnectionReferences(ctx, connectionIDs, connections); appErr != nil {
		return nil, appErr
	}
	return connections, nil
}

func (store *Store) GetOwnerProbeConnection(ctx context.Context, ownerUserID, connectionID string) (apihealth.Connection, bool, *domain.AppError) {
	if store == nil || store.pool == nil {
		return apihealth.Connection{}, false, internalStoreError()
	}
	var connection apihealth.Connection
	err := scanAPIProbeConnection(store.pool.QueryRow(ctx, `SELECT `+apiProbeConnectionColumns+` FROM api_probe_connections c WHERE c.owner_user_id = $1 AND c.id = $2`, ownerUserID, connectionID), &connection)
	if errors.Is(err, pgx.ErrNoRows) {
		return apihealth.Connection{}, false, nil
	}
	if err != nil {
		return apihealth.Connection{}, false, internalStoreError()
	}
	connections := []apihealth.Connection{connection}
	if appErr := store.loadProbeConnectionReferences(ctx, []string{connection.ID}, connections); appErr != nil {
		return apihealth.Connection{}, false, appErr
	}
	return connections[0], true, nil
}

func (store *Store) GetOwnerProbeConnectionCredential(ctx context.Context, ownerUserID, connectionID string) (apihealth.Connection, string, bool, *domain.AppError) {
	if store == nil || store.pool == nil || store.contactCodec == nil {
		return apihealth.Connection{}, "", false, internalStoreError()
	}
	var connection apihealth.Connection
	var ciphertext, nonce []byte
	var keyVersion, cipherFormat string
	destinations := apiProbeConnectionScanDestinations(&connection)
	destinations = append(destinations, &ciphertext, &nonce, &keyVersion, &cipherFormat)
	err := store.pool.QueryRow(ctx, `SELECT `+apiProbeConnectionColumns+`, c.credential_ciphertext, c.credential_nonce, c.credential_key_version, c.credential_cipher_format FROM api_probe_connections c WHERE c.owner_user_id = $1 AND c.id = $2`, ownerUserID, connectionID).Scan(destinations...)
	if errors.Is(err, pgx.ErrNoRows) {
		return apihealth.Connection{}, "", false, nil
	}
	if err != nil {
		return apihealth.Connection{}, "", false, internalStoreError()
	}
	credential, err := store.contactCodec.decode(ciphertext, nonce, keyVersion, cipherFormat, connection.ID, contactFieldProbeAPIKey)
	if err != nil {
		return apihealth.Connection{}, "", false, internalStoreError()
	}
	return connection, credential, true, nil
}

func (store *Store) LookupProbeModelPrice(ctx context.Context, model string) (apihealth.PriceSnapshot, bool, *domain.AppError) {
	if store == nil || store.pool == nil {
		return apihealth.PriceSnapshot{}, false, internalStoreError()
	}
	var price apihealth.PriceSnapshot
	err := store.pool.QueryRow(ctx, `
		SELECT version.id::text,
		       COALESCE(version.input_price_per_million::text, ''),
		       COALESCE(version.cached_input_price_per_million::text, ''),
		       COALESCE(version.output_price_per_million::text, ''),
		       'USD'
		FROM api_model_catalog model
		JOIN LATERAL (
		  SELECT * FROM api_model_price_versions value
		  WHERE value.model_catalog_id = model.id AND value.valid_to IS NULL
		  ORDER BY value.valid_from DESC LIMIT 1
		) version ON true
		WHERE model.model_key = $1
	`, strings.TrimSpace(model)).Scan(&price.VersionID, &price.InputPricePerMillion, &price.CachedInputPricePerMillion, &price.OutputPricePerMillion, &price.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return apihealth.PriceSnapshot{}, false, nil
	}
	if err != nil {
		return apihealth.PriceSnapshot{}, false, internalStoreError()
	}
	return price, true, nil
}

func (store *Store) CreateOwnerProbeConnection(ctx context.Context, connection apihealth.Connection, credential string, audit apihealth.ProbeAuditMutation) (apihealth.Connection, *domain.AppError) {
	connection, _, appErr := store.createOwnerProbeConnection(ctx, nil, connection, credential, audit, nil)
	return connection, appErr
}

func (store *Store) CreateOwnerProbeConnectionWithIdempotency(
	ctx context.Context,
	entry idempotency.Entry,
	connection apihealth.Connection,
	credential string,
	audit apihealth.ProbeAuditMutation,
	buildCompletion apihealth.MutationCompletionBuilder,
) (apihealth.Connection, idempotency.Completion, *domain.AppError) {
	return store.createOwnerProbeConnection(ctx, &entry, connection, credential, audit, buildCompletion)
}

func (store *Store) createOwnerProbeConnection(
	ctx context.Context,
	entry *idempotency.Entry,
	connection apihealth.Connection,
	credential string,
	audit apihealth.ProbeAuditMutation,
	buildCompletion apihealth.MutationCompletionBuilder,
) (apihealth.Connection, idempotency.Completion, *domain.AppError) {
	if store == nil || store.pool == nil || store.contactCodec == nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	connection.ID = uuid.NewString()
	encoded, err := store.contactCodec.encode(strings.TrimSpace(credential), connection.ID, contactFieldProbeAPIKey)
	if err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	var lockedEntry idempotency.Entry
	if entry != nil {
		var appErr *domain.AppError
		lockedEntry, appErr = lockProcessingIdempotencyInTx(ctx, tx, *entry)
		if appErr != nil {
			return apihealth.Connection{}, idempotency.Completion{}, appErr
		}
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO api_probe_connections AS c (
			id, owner_user_id, name, base_url, normalized_base_url,
			credential_ciphertext, credential_nonce, credential_key_version, credential_cipher_format, credential_fingerprint,
			enabled, verification_status, verified_at, last_verification_error_code,
			probe_model, probe_protocol, probe_models_snapshot, probe_environment, probe_model_changed_at,
			probe_price_version_id, probe_input_price_per_million, probe_cached_input_price_per_million,
			probe_output_price_per_million, probe_price_currency,
			measurement_version, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24, $25, $26, $27, $28
		) RETURNING `+apiProbeConnectionColumns,
		connection.ID, connection.OwnerUserID, connection.Name, connection.BaseURL, connection.NormalizedBaseURL,
		encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion, encoded.CipherFormat, []byte(encoded.Fingerprint),
		connection.Enabled, connection.VerificationStatus, connection.VerifiedAt, nullText(connection.LastVerificationErrorCode),
		nullText(connection.ProbeModel), nullText(connection.ProbeProtocol), connection.AvailableModels, connection.ProbeEnvironment,
		connection.ProbeModelChangedAt, nullUUID(connection.Price.VersionID), nullDecimal(connection.Price.InputPricePerMillion),
		nullDecimal(connection.Price.CachedInputPricePerMillion), nullDecimal(connection.Price.OutputPricePerMillion), nullText(connection.Price.Currency),
		connection.MeasurementVersion, connection.Version, connection.CreatedAt, connection.UpdatedAt)
	if err := scanAPIProbeConnection(row, &connection); err != nil {
		if isForeignKeyViolation(err) {
			return apihealth.Connection{}, idempotency.Completion{}, apiHealthConflict("当前用户不存在，无法创建探针连接。")
		}
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	newMeasurementVersion := connection.MeasurementVersion
	newModel, newProtocol, environment := connection.ProbeModel, connection.ProbeProtocol, connection.ProbeEnvironment
	if err := appendProbeConnectionAuditEvent(ctx, tx, probeConnectionAuditEvent{
		TargetConnectionID: connection.ID, OwnerUserID: connection.OwnerUserID,
		Action: apihealth.ProbeAuditCreated, RequestID: audit.RequestID, OccurredAt: connection.CreatedAt,
		ChangedFields:         []string{"name", "base_url", "credential", "probe_model", "probe_protocol", "environment", "enabled"},
		ToVerificationStatus:  connection.VerificationStatus,
		NewMeasurementVersion: &newMeasurementVersion, NewModel: &newModel, NewProtocol: &newProtocol, Environment: &environment,
	}); err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	var completion idempotency.Completion
	if entry != nil {
		if buildCompletion == nil {
			return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
		}
		var appErr *domain.AppError
		completion, appErr = buildCompletion(connection)
		if appErr != nil {
			return apihealth.Connection{}, idempotency.Completion{}, appErr
		}
		if appErr := completeIdempotencyInTx(ctx, tx, lockedEntry, completion, probeAuditOccurredAt(audit)); appErr != nil {
			return apihealth.Connection{}, idempotency.Completion{}, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	return connection, completion, nil
}

func (store *Store) UpdateOwnerProbeConnection(ctx context.Context, connection apihealth.Connection, credential *string, expectedVersion int64, audit apihealth.ProbeAuditMutation) (apihealth.Connection, *domain.AppError) {
	connection, _, appErr := store.updateOwnerProbeConnection(ctx, nil, connection, credential, expectedVersion, audit, nil)
	return connection, appErr
}

func (store *Store) UpdateOwnerProbeConnectionWithIdempotency(
	ctx context.Context,
	entry idempotency.Entry,
	connection apihealth.Connection,
	credential *string,
	expectedVersion int64,
	audit apihealth.ProbeAuditMutation,
	buildCompletion apihealth.MutationCompletionBuilder,
) (apihealth.Connection, idempotency.Completion, *domain.AppError) {
	return store.updateOwnerProbeConnection(ctx, &entry, connection, credential, expectedVersion, audit, buildCompletion)
}

func (store *Store) updateOwnerProbeConnection(
	ctx context.Context,
	entry *idempotency.Entry,
	connection apihealth.Connection,
	credential *string,
	expectedVersion int64,
	audit apihealth.ProbeAuditMutation,
	buildCompletion apihealth.MutationCompletionBuilder,
) (apihealth.Connection, idempotency.Completion, *domain.AppError) {
	if store == nil || store.pool == nil || store.contactCodec == nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	credentialProvided := credential != nil
	var encoded encodedContactValue
	if credentialProvided {
		var err error
		encoded, err = store.contactCodec.encode(strings.TrimSpace(*credential), connection.ID, contactFieldProbeAPIKey)
		if err != nil {
			return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
		}
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	var lockedEntry idempotency.Entry
	if entry != nil {
		var appErr *domain.AppError
		lockedEntry, appErr = lockProcessingIdempotencyInTx(ctx, tx, *entry)
		if appErr != nil {
			return apihealth.Connection{}, idempotency.Completion{}, appErr
		}
	}
	var oldName, oldBaseURL, oldVerificationStatus, oldModel, oldProtocol, oldEnvironment string
	var oldEnabled bool
	var oldMeasurementVersion, actualVersion int64
	if err := tx.QueryRow(ctx, `
		SELECT name, base_url, enabled, verification_status, COALESCE(probe_model, ''),
		       COALESCE(probe_protocol, ''), probe_environment, measurement_version, version
		FROM api_probe_connections
		WHERE id = $1 AND owner_user_id = $2
		FOR UPDATE
	`, connection.ID, connection.OwnerUserID).Scan(
		&oldName, &oldBaseURL, &oldEnabled, &oldVerificationStatus, &oldModel,
		&oldProtocol, &oldEnvironment, &oldMeasurementVersion, &actualVersion,
	); errors.Is(err, pgx.ErrNoRows) {
		return apihealth.Connection{}, idempotency.Completion{}, apiHealthNotFound()
	} else if err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	if actualVersion != expectedVersion {
		return apihealth.Connection{}, idempotency.Completion{}, apiHealthVersionConflict()
	}
	row := tx.QueryRow(ctx, `
		UPDATE api_probe_connections c
		SET name = $4, base_url = $5, normalized_base_url = $6,
		    credential_ciphertext = CASE WHEN $7 THEN $8 ELSE c.credential_ciphertext END,
		    credential_nonce = CASE WHEN $7 THEN $9 ELSE c.credential_nonce END,
		    credential_key_version = CASE WHEN $7 THEN $10 ELSE c.credential_key_version END,
		    credential_cipher_format = CASE WHEN $7 THEN $11 ELSE c.credential_cipher_format END,
		    credential_fingerprint = CASE WHEN $7 THEN $12 ELSE c.credential_fingerprint END,
		    enabled = $13, verification_status = $14, verified_at = $15, last_verification_error_code = $16,
		    probe_model = $17, probe_protocol = $18, probe_models_snapshot = $19,
		    probe_environment = $20, probe_model_changed_at = $21,
		    probe_price_version_id = $22, probe_input_price_per_million = $23,
		    probe_cached_input_price_per_million = $24, probe_output_price_per_million = $25,
		    probe_price_currency = $26, measurement_version = $27, version = $28, updated_at = $29
		WHERE c.id = $1 AND c.owner_user_id = $2 AND c.version = $3
		RETURNING `+apiProbeConnectionColumns,
		connection.ID, connection.OwnerUserID, expectedVersion,
		connection.Name, connection.BaseURL, connection.NormalizedBaseURL,
		credentialProvided, nullBytes(encoded.Ciphertext), nullBytes(encoded.Nonce), nullText(encoded.EncryptionKeyVersion),
		nullText(encoded.CipherFormat), nullBytes([]byte(encoded.Fingerprint)), connection.Enabled,
		connection.VerificationStatus, connection.VerifiedAt, nullText(connection.LastVerificationErrorCode),
		nullText(connection.ProbeModel), nullText(connection.ProbeProtocol), connection.AvailableModels,
		connection.ProbeEnvironment, connection.ProbeModelChangedAt, nullUUID(connection.Price.VersionID),
		nullDecimal(connection.Price.InputPricePerMillion), nullDecimal(connection.Price.CachedInputPricePerMillion),
		nullDecimal(connection.Price.OutputPricePerMillion), nullText(connection.Price.Currency),
		connection.MeasurementVersion, connection.Version, connection.UpdatedAt)
	if err := scanAPIProbeConnection(row, &connection); errors.Is(err, pgx.ErrNoRows) {
		return apihealth.Connection{}, idempotency.Completion{}, apiHealthVersionConflict()
	} else if err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	changedFields := probeConnectionChangedFields(
		oldName, oldBaseURL, oldModel, oldProtocol, oldEnvironment, oldEnabled,
		connection, credentialProvided,
	)
	modelChanged := oldModel != connection.ProbeModel || oldProtocol != connection.ProbeProtocol
	action := probeConnectionAuditAction(audit.Action, modelChanged, oldEnabled, connection.Enabled)
	event := probeConnectionAuditEvent{
		TargetConnectionID: connection.ID, OwnerUserID: connection.OwnerUserID,
		Action: action, ChangedFields: changedFields, RequestID: audit.RequestID, OccurredAt: connection.UpdatedAt,
		FromVerificationStatus: oldVerificationStatus, ToVerificationStatus: connection.VerificationStatus,
	}
	if action == apihealth.ProbeAuditModelChanged {
		newMeasurementVersion := connection.MeasurementVersion
		newModel, newProtocol, environment := connection.ProbeModel, connection.ProbeProtocol, connection.ProbeEnvironment
		event.OldMeasurementVersion = &oldMeasurementVersion
		event.NewMeasurementVersion = &newMeasurementVersion
		event.OldModel = &oldModel
		event.NewModel = &newModel
		event.OldProtocol = &oldProtocol
		event.NewProtocol = &newProtocol
		event.Environment = &environment
	}
	if err := appendProbeConnectionAuditEvent(ctx, tx, event); err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	var completion idempotency.Completion
	if entry != nil {
		if buildCompletion == nil {
			return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
		}
		var appErr *domain.AppError
		completion, appErr = buildCompletion(connection)
		if appErr != nil {
			return apihealth.Connection{}, idempotency.Completion{}, appErr
		}
		if appErr := completeIdempotencyInTx(ctx, tx, lockedEntry, completion, probeAuditOccurredAt(audit)); appErr != nil {
			return apihealth.Connection{}, idempotency.Completion{}, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return apihealth.Connection{}, idempotency.Completion{}, internalStoreError()
	}
	return connection, completion, nil
}

func probeConnectionAuditAction(requested string, modelChanged, oldEnabled, newEnabled bool) string {
	requested = strings.TrimSpace(requested)
	if requested != "" && requested != apihealth.ProbeAuditUpdated {
		return requested
	}
	switch {
	case modelChanged:
		return apihealth.ProbeAuditModelChanged
	case oldEnabled != newEnabled && newEnabled:
		return apihealth.ProbeAuditEnabled
	case oldEnabled != newEnabled:
		return apihealth.ProbeAuditDisabled
	default:
		return apihealth.ProbeAuditUpdated
	}
}

func (store *Store) DeleteOwnerProbeConnection(ctx context.Context, ownerUserID, connectionID string, expectedVersion int64, audit apihealth.ProbeAuditMutation) *domain.AppError {
	_, appErr := store.deleteOwnerProbeConnection(ctx, nil, ownerUserID, connectionID, expectedVersion, audit, nil)
	return appErr
}

func (store *Store) DeleteOwnerProbeConnectionWithIdempotency(
	ctx context.Context,
	entry idempotency.Entry,
	ownerUserID, connectionID string,
	expectedVersion int64,
	audit apihealth.ProbeAuditMutation,
	buildCompletion apihealth.MutationCompletionBuilder,
) (idempotency.Completion, *domain.AppError) {
	return store.deleteOwnerProbeConnection(ctx, &entry, ownerUserID, connectionID, expectedVersion, audit, buildCompletion)
}

func (store *Store) deleteOwnerProbeConnection(
	ctx context.Context,
	entry *idempotency.Entry,
	ownerUserID, connectionID string,
	expectedVersion int64,
	audit apihealth.ProbeAuditMutation,
	buildCompletion apihealth.MutationCompletionBuilder,
) (idempotency.Completion, *domain.AppError) {
	if store == nil || store.pool == nil {
		return idempotency.Completion{}, internalStoreError()
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	var lockedEntry idempotency.Entry
	if entry != nil {
		var appErr *domain.AppError
		lockedEntry, appErr = lockProcessingIdempotencyInTx(ctx, tx, *entry)
		if appErr != nil {
			return idempotency.Completion{}, appErr
		}
	}
	var version int64
	var verificationStatus string
	if err := tx.QueryRow(ctx, `SELECT version, verification_status FROM api_probe_connections WHERE owner_user_id = $1 AND id = $2 FOR UPDATE`, ownerUserID, connectionID).Scan(&version, &verificationStatus); errors.Is(err, pgx.ErrNoRows) {
		return idempotency.Completion{}, apiHealthNotFound()
	} else if err != nil {
		return idempotency.Completion{}, internalStoreError()
	}
	if version != expectedVersion {
		return idempotency.Completion{}, apiHealthVersionConflict()
	}
	rows, err := tx.Query(ctx, `SELECT id::text, title FROM api_services WHERE probe_connection_id = $1 ORDER BY updated_at DESC, id`, connectionID)
	if err != nil {
		return idempotency.Completion{}, internalStoreError()
	}
	references := make([]apihealth.ServiceReference, 0)
	for rows.Next() {
		var reference apihealth.ServiceReference
		if err := rows.Scan(&reference.ID, &reference.Title); err != nil {
			rows.Close()
			return idempotency.Completion{}, internalStoreError()
		}
		references = append(references, reference)
	}
	if rows.Err() != nil {
		rows.Close()
		return idempotency.Completion{}, internalStoreError()
	}
	rows.Close()
	if len(references) > 0 {
		titles := make([]string, 0, len(references))
		for _, reference := range references {
			titles = append(titles, reference.Title+" ("+reference.ID+")")
		}
		return idempotency.Completion{}, apiHealthConflict("该连接仍被以下服务使用，请先改绑或解绑：" + strings.Join(titles, "、"))
	}
	if err := appendProbeConnectionAuditEvent(ctx, tx, probeConnectionAuditEvent{
		TargetConnectionID: connectionID, OwnerUserID: ownerUserID,
		Action: apihealth.ProbeAuditDeleted, RequestID: audit.RequestID, OccurredAt: audit.OccurredAt,
		FromVerificationStatus: verificationStatus,
	}); err != nil {
		return idempotency.Completion{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_probe_connections WHERE id = $1`, connectionID); err != nil {
		return idempotency.Completion{}, internalStoreError()
	}
	var completion idempotency.Completion
	if entry != nil {
		if buildCompletion == nil {
			return idempotency.Completion{}, internalStoreError()
		}
		var appErr *domain.AppError
		completion, appErr = buildCompletion(apihealth.Connection{ID: connectionID, OwnerUserID: ownerUserID})
		if appErr != nil {
			return idempotency.Completion{}, appErr
		}
		if appErr := completeIdempotencyInTx(ctx, tx, lockedEntry, completion, probeAuditOccurredAt(audit)); appErr != nil {
			return idempotency.Completion{}, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return idempotency.Completion{}, internalStoreError()
	}
	return completion, nil
}

func probeAuditOccurredAt(audit apihealth.ProbeAuditMutation) time.Time {
	occurredAt := audit.OccurredAt.UTC()
	if occurredAt.IsZero() {
		return time.Now().UTC()
	}
	return occurredAt
}

func appendProbeConnectionAuditEvent(ctx context.Context, tx pgx.Tx, event probeConnectionAuditEvent) error {
	requestID := strings.TrimSpace(event.RequestID)
	if requestID == "" {
		requestID = "probe-event-" + uuid.NewString()
	}
	occurredAt := event.OccurredAt.UTC()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	changedFields := event.ChangedFields
	if changedFields == nil {
		changedFields = []string{}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO api_probe_connection_events (
		  target_connection_id, owner_user_id, actor_user_id, actor_kind, action,
		  old_measurement_version, new_measurement_version, old_model, new_model,
		  old_protocol, new_protocol, environment,
		  from_verification_status, to_verification_status, changed_fields,
		  request_id, occurred_at, created_at
		) VALUES (
		  $1, $2, $2, 'user', $3,
		  $4, $5, $6, $7, $8, $9, $10,
		  $11, $12, $13, $14, $15, $15
		)
	`, event.TargetConnectionID, event.OwnerUserID, event.Action,
		event.OldMeasurementVersion, event.NewMeasurementVersion, event.OldModel, event.NewModel,
		event.OldProtocol, event.NewProtocol, event.Environment,
		nullText(event.FromVerificationStatus), nullText(event.ToVerificationStatus), changedFields,
		requestID, occurredAt)
	return err
}

func probeConnectionChangedFields(
	oldName, oldBaseURL, oldModel, oldProtocol, oldEnvironment string,
	oldEnabled bool,
	connection apihealth.Connection,
	credentialProvided bool,
) []string {
	changed := make([]string, 0, 7)
	if oldName != connection.Name {
		changed = append(changed, "name")
	}
	if oldBaseURL != connection.BaseURL {
		changed = append(changed, "base_url")
	}
	if credentialProvided {
		changed = append(changed, "credential")
	}
	if oldModel != connection.ProbeModel {
		changed = append(changed, "probe_model")
	}
	if oldProtocol != connection.ProbeProtocol {
		changed = append(changed, "probe_protocol")
	}
	if oldEnvironment != connection.ProbeEnvironment {
		changed = append(changed, "environment")
	}
	if oldEnabled != connection.Enabled {
		changed = append(changed, "enabled")
	}
	return changed
}

func (store *Store) LoadOwnerProbeConnectionSamples(ctx context.Context, ownerUserID string, connectionIDs []string, since time.Time) (map[string][]apihealth.Sample, *domain.AppError) {
	result := make(map[string][]apihealth.Sample, len(connectionIDs))
	if len(connectionIDs) == 0 {
		return result, nil
	}
	rows, err := store.pool.Query(ctx, `
		SELECT sample.connection_id::text, `+apiProbeSampleColumns+`
		FROM api_probe_connection_samples sample
		JOIN api_probe_connections connection ON connection.id = sample.connection_id
		WHERE connection.owner_user_id = $1 AND sample.connection_id = ANY($2::uuid[])
		  AND sample.measurement_version = connection.measurement_version
		  AND sample.status IN ('succeeded', 'failed')
		  AND (
		    sample.slot_started_at >= $3
		    OR sample.id = (
		      SELECT previous.id
		      FROM api_probe_connection_samples previous
		      WHERE previous.connection_id = sample.connection_id
		        AND previous.measurement_version = connection.measurement_version
		        AND previous.status IN ('succeeded', 'failed')
		        AND previous.slot_started_at < $3
		      ORDER BY previous.slot_started_at DESC, previous.created_at DESC, previous.id DESC
		      LIMIT 1
		    )
		  )
		ORDER BY sample.connection_id, sample.slot_started_at
	`, ownerUserID, connectionIDs, since)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	for rows.Next() {
		var connectionID string
		var sample apihealth.Sample
		destinations := []any{&connectionID}
		destinations = append(destinations, apiProbeSampleScanDestinations(&sample)...)
		if err := rows.Scan(destinations...); err != nil {
			return nil, internalStoreError()
		}
		result[connectionID] = append(result[connectionID], sample)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func (store *Store) LoadProbeSummaryInputs(ctx context.Context, serviceIDs []string, since time.Time) (map[string]apihealth.SummaryInput, *domain.AppError) {
	result := make(map[string]apihealth.SummaryInput, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return result, nil
	}
	rows, err := store.pool.Query(ctx, `SELECT service.id::text, `+apiProbeConnectionColumns+` FROM api_services service JOIN api_probe_connections c ON c.id = service.probe_connection_id WHERE service.id = ANY($1::uuid[])`, serviceIDs)
	if err != nil {
		return nil, internalStoreError()
	}
	for rows.Next() {
		var serviceID string
		var connection apihealth.Connection
		destinations := []any{&serviceID}
		destinations = append(destinations, apiProbeConnectionScanDestinations(&connection)...)
		if err := rows.Scan(destinations...); err != nil {
			rows.Close()
			return nil, internalStoreError()
		}
		result[serviceID] = apihealth.SummaryInput{Connection: &connection}
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, internalStoreError()
	}
	rows.Close()

	sampleRows, err := store.pool.Query(ctx, `
		SELECT service.id::text, `+apiProbeSampleColumns+`
		FROM api_services service
		JOIN api_probe_connections connection ON connection.id = service.probe_connection_id
		JOIN api_probe_connection_samples sample ON sample.connection_id = service.probe_connection_id
		WHERE service.id = ANY($1::uuid[])
		  AND sample.measurement_version = connection.measurement_version
		  AND sample.status IN ('succeeded', 'failed')
		  AND (
		    sample.slot_started_at >= $2
		    OR sample.id = (
		      SELECT previous.id
		      FROM api_probe_connection_samples previous
		      WHERE previous.connection_id = sample.connection_id
		        AND previous.measurement_version = connection.measurement_version
		        AND previous.status IN ('succeeded', 'failed')
		        AND previous.slot_started_at < $2
		      ORDER BY previous.slot_started_at DESC, previous.created_at DESC, previous.id DESC
		      LIMIT 1
		    )
		  )
		ORDER BY service.id, sample.slot_started_at
	`, serviceIDs, since)
	if err != nil {
		return nil, internalStoreError()
	}
	defer sampleRows.Close()
	for sampleRows.Next() {
		var serviceID string
		var sample apihealth.Sample
		destinations := []any{&serviceID}
		destinations = append(destinations, apiProbeSampleScanDestinations(&sample)...)
		if err := sampleRows.Scan(destinations...); err != nil {
			return nil, internalStoreError()
		}
		input := result[serviceID]
		input.Samples = append(input.Samples, sample)
		result[serviceID] = input
	}
	if sampleRows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func (store *Store) ClaimDueProbes(ctx context.Context, slotStartedAt, now time.Time, limit int, runningTimeout time.Duration) ([]apihealth.ProbeJob, *domain.AppError) {
	if limit < 1 {
		return []apihealth.ProbeJob{}, nil
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rollback(ctx, tx)
	timeoutMS := int(runningTimeout.Milliseconds())
	if timeoutMS < 0 {
		timeoutMS = 0
	}
	if _, err := tx.Exec(ctx, `
		UPDATE api_probe_connection_samples
		SET status = 'failed', outcome = 'final_failure', attempt_count = 1,
		    total_duration_ms = $2, first_attempt_total_duration_ms = $2,
		    error_code = 'internal_timeout', finished_at = $1
		WHERE status = 'running' AND started_at <= $1::timestamptz - ($3::bigint * interval '1 millisecond')
	`, now, timeoutMS, timeoutMS); err != nil {
		return nil, internalStoreError()
	}
	rows, err := tx.Query(ctx, `
		SELECT `+apiProbeConnectionColumns+`,
		       c.credential_ciphertext, c.credential_nonce, c.credential_key_version, c.credential_cipher_format,
		       COALESCE(rule.id::text, ''), COALESCE(rule.version, 0), COALESCE(rule.slow_ttft_ms, 0),
		       COALESCE(rule.hard_timeout_ms, 0), rule.published_at
		FROM api_probe_connections c
		LEFT JOIN LATERAL (
		  SELECT * FROM api_probe_latency_rules value
		  WHERE value.model = c.probe_model AND value.protocol = c.probe_protocol
		    AND value.environment = c.probe_environment AND value.status = 'active'
		  ORDER BY value.version DESC LIMIT 1
		) rule ON true
		WHERE c.enabled = true AND c.verification_status = 'verified'
		  AND c.credential_ciphertext IS NOT NULL AND c.probe_model IS NOT NULL AND c.probe_protocol IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM api_probe_connection_samples sample
		    WHERE sample.connection_id = c.id AND sample.slot_started_at = $2
		  )
		ORDER BY c.updated_at, c.id FOR UPDATE OF c SKIP LOCKED LIMIT $1
	`, limit, slotStartedAt)
	if err != nil {
		return nil, internalStoreError()
	}
	type claimedConnection struct {
		connection                apihealth.Connection
		ciphertext, nonce         []byte
		keyVersion, cipherFormat  string
		ruleID                    string
		ruleVersion               int64
		slowTTFTMS, hardTimeoutMS int
		rulePublishedAt           *time.Time
	}
	claimed := make([]claimedConnection, 0, limit)
	for rows.Next() {
		var item claimedConnection
		destinations := apiProbeConnectionScanDestinations(&item.connection)
		destinations = append(destinations, &item.ciphertext, &item.nonce, &item.keyVersion, &item.cipherFormat,
			&item.ruleID, &item.ruleVersion, &item.slowTTFTMS, &item.hardTimeoutMS, &item.rulePublishedAt)
		if err := rows.Scan(destinations...); err != nil {
			rows.Close()
			return nil, internalStoreError()
		}
		claimed = append(claimed, item)
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, internalStoreError()
	}
	rows.Close()

	jobs := make([]apihealth.ProbeJob, 0, len(claimed))
	for _, item := range claimed {
		sample := apihealth.Sample{
			ID: uuid.NewString(), ConnectionID: item.connection.ID, MeasurementVersion: item.connection.MeasurementVersion,
			SlotStartedAt: slotStartedAt, Status: apihealth.SampleStatusRunning, ProbeModel: item.connection.ProbeModel,
			ProbeProtocol: item.connection.ProbeProtocol, ProbeEnvironment: item.connection.ProbeEnvironment,
			LatencyRuleVersionID: item.ruleID, StartedAt: now, CreatedAt: now,
		}
		command, err := tx.Exec(ctx, `
			INSERT INTO api_probe_connection_samples (
			  id, connection_id, measurement_version, slot_started_at, status,
			  probe_model, probe_protocol, probe_environment, latency_rule_version_id, started_at, created_at
			) VALUES ($1, $2, $3, $4, 'running', $5, $6, $7, $8, $9, $9)
			ON CONFLICT (connection_id, slot_started_at) DO NOTHING
		`, sample.ID, sample.ConnectionID, sample.MeasurementVersion, sample.SlotStartedAt,
			sample.ProbeModel, sample.ProbeProtocol, sample.ProbeEnvironment, nullUUID(sample.LatencyRuleVersionID), sample.StartedAt)
		if err != nil {
			return nil, internalStoreError()
		}
		if command.RowsAffected() == 0 {
			continue
		}
		job := apihealth.ProbeJob{Sample: sample, Connection: item.connection}
		if item.ruleID != "" {
			job.LatencyRule = &apihealth.LatencyRule{ID: item.ruleID, Model: item.connection.ProbeModel, Protocol: item.connection.ProbeProtocol, Environment: item.connection.ProbeEnvironment, Version: item.ruleVersion, SlowTTFTMS: item.slowTTFTMS, HardTimeoutMS: item.hardTimeoutMS}
		}
		credential, err := store.contactCodec.decode(item.ciphertext, item.nonce, item.keyVersion, item.cipherFormat, item.connection.ID, contactFieldProbeAPIKey)
		if err != nil {
			job.CredentialError = true
		} else {
			job.Credential = credential
		}
		jobs = append(jobs, job)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, internalStoreError()
	}
	return jobs, nil
}

func (store *Store) FinalizeProbe(ctx context.Context, sampleID string, result apihealth.ProbeResult, finishedAt time.Time) (bool, *domain.AppError) {
	status := apihealth.SampleStatusSucceeded
	if result.Outcome == apihealth.OutcomeFinalFailure || result.ErrorCode != "" {
		status = apihealth.SampleStatusFailed
		result.Outcome = apihealth.OutcomeFinalFailure
	}
	attemptCount := len(result.Attempts)
	if attemptCount == 0 {
		attemptCount = 1
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return false, internalStoreError()
	}
	defer rollback(ctx, tx)
	command, err := tx.Exec(ctx, `
		UPDATE api_probe_connection_samples
		SET status = $2, outcome = $3, attempt_count = $4,
		    first_attempt_ttft_ms = $5, first_attempt_total_duration_ms = $6,
		    recovery_duration_ms = $7, total_duration_ms = $8,
		    http_status_class = $9, final_http_status = $10, error_code = $11,
		    input_tokens = $12, cached_input_tokens = $13, output_tokens = $14, reasoning_tokens = $15,
		    usage_complete = $16, base_cost_usd = $17, retry_cost_usd = $18, finished_at = $19
		WHERE id = $1 AND status = 'running'
	`, sampleID, status, result.Outcome, attemptCount,
		result.FirstAttemptTTFTMS, result.FirstAttemptTotalDurationMS, result.RecoveryDurationMS,
		result.TotalDurationMS, nullInt(result.HTTPStatusClass), nullInt(result.HTTPStatus), nullText(result.ErrorCode),
		result.Usage.InputTokens, result.Usage.CachedInputTokens, result.Usage.OutputTokens, result.Usage.ReasoningTokens,
		result.UsageComplete, nullDecimal(result.BaseCostUSD), nullDecimal(result.RetryCostUSD), finishedAt)
	if err != nil {
		return false, internalStoreError()
	}
	if command.RowsAffected() == 0 {
		return false, nil
	}
	for _, attempt := range result.Attempts {
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_probe_connection_attempts (
			  sample_id, attempt_number, started_at, first_text_at, finished_at,
			  http_status, ttft_ms, total_duration_ms, succeeded, retryable, error_code, retry_after_ms,
			  input_tokens, cached_input_tokens, output_tokens, reasoning_tokens, usage_complete, cost_usd
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		`, sampleID, attempt.AttemptNumber, attempt.StartedAt, attempt.FirstTextAt, attempt.FinishedAt,
			nullInt(attempt.HTTPStatus), attempt.TTFTMS, attempt.TotalDurationMS, attempt.Succeeded, attempt.Retryable,
			nullText(attempt.ErrorCode), nullInt(attempt.RetryAfterMS), attempt.Usage.InputTokens,
			attempt.Usage.CachedInputTokens, attempt.Usage.OutputTokens, attempt.Usage.ReasoningTokens,
			attempt.Usage.Complete(), nullDecimal(attempt.CostUSD)); err != nil {
			return false, internalStoreError()
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, internalStoreError()
	}
	return true, nil
}

func (store *Store) DeleteFinalProbeSamplesBefore(ctx context.Context, cutoff time.Time, limit int) (int, *domain.AppError) {
	if limit < 1 {
		return 0, nil
	}
	command, err := store.pool.Exec(ctx, `
		WITH expired AS (
		  SELECT id FROM api_probe_connection_samples
		  WHERE status IN ('succeeded', 'failed') AND finished_at < $1
		  ORDER BY finished_at, id LIMIT $2
		)
		DELETE FROM api_probe_connection_samples sample USING expired WHERE sample.id = expired.id
	`, cutoff, limit)
	if err != nil {
		return 0, internalStoreError()
	}
	return int(command.RowsAffected()), nil
}

func (store *Store) loadProbeConnectionReferences(ctx context.Context, connectionIDs []string, connections []apihealth.Connection) *domain.AppError {
	if len(connectionIDs) == 0 {
		return nil
	}
	byID := make(map[string]int, len(connections))
	for index := range connections {
		byID[connections[index].ID] = index
		connections[index].References = []apihealth.ServiceReference{}
	}
	rows, err := store.pool.Query(ctx, `SELECT probe_connection_id::text, id::text, title FROM api_services WHERE probe_connection_id = ANY($1::uuid[]) ORDER BY updated_at DESC, id`, connectionIDs)
	if err != nil {
		return internalStoreError()
	}
	defer rows.Close()
	for rows.Next() {
		var connectionID string
		var reference apihealth.ServiceReference
		if err := rows.Scan(&connectionID, &reference.ID, &reference.Title); err != nil {
			return internalStoreError()
		}
		if index, exists := byID[connectionID]; exists {
			connections[index].References = append(connections[index].References, reference)
		}
	}
	if rows.Err() != nil {
		return internalStoreError()
	}
	return nil
}

func scanAPIProbeConnection(row scanner, connection *apihealth.Connection) error {
	return row.Scan(apiProbeConnectionScanDestinations(connection)...)
}

func apiProbeConnectionScanDestinations(connection *apihealth.Connection) []any {
	return []any{
		&connection.ID, &connection.OwnerUserID, &connection.Name, &connection.BaseURL,
		&connection.NormalizedBaseURL, &connection.CredentialConfigured, &connection.Enabled,
		&connection.VerificationStatus, &connection.VerifiedAt, &connection.LastVerificationErrorCode,
		&connection.ProbeModel, &connection.ProbeProtocol, &connection.AvailableModels,
		&connection.ProbeEnvironment, &connection.ProbeModelChangedAt, &connection.Price.VersionID,
		&connection.Price.InputPricePerMillion, &connection.Price.CachedInputPricePerMillion,
		&connection.Price.OutputPricePerMillion, &connection.Price.Currency,
		&connection.MeasurementVersion, &connection.Version, &connection.CreatedAt, &connection.UpdatedAt,
	}
}

func apiProbeSampleScanDestinations(sample *apihealth.Sample) []any {
	return []any{
		&sample.ID, &sample.ConnectionID, &sample.MeasurementVersion, &sample.SlotStartedAt,
		&sample.Status, &sample.ProbeModel, &sample.ProbeProtocol, &sample.ProbeEnvironment,
		&sample.LatencyRuleVersionID, &sample.Outcome, &sample.AttemptCount,
		&sample.FirstAttemptTTFTMS, &sample.FirstAttemptTotalDurationMS, &sample.RecoveryDurationMS,
		&sample.TotalDurationMS, &sample.HTTPStatusClass, &sample.FinalHTTPStatus, &sample.ErrorCode,
		&sample.Usage.InputTokens, &sample.Usage.CachedInputTokens, &sample.Usage.OutputTokens,
		&sample.Usage.ReasoningTokens, &sample.UsageComplete, &sample.BaseCostUSD, &sample.RetryCostUSD,
		&sample.StartedAt, &sample.FinishedAt, &sample.CreatedAt,
	}
}

func nullBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullDecimal(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func apiHealthNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Probe connection not found", "探针连接不存在。")
}

func apiHealthVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "探针连接已更新，请刷新后重试。")
}

func apiHealthConflict(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Probe connection conflict", detail)
}
