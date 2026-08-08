package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const apiProbeConnectionColumns = `
	c.id::text, c.owner_user_id::text, c.name, c.base_url, c.normalized_base_url,
	(c.credential_ciphertext IS NOT NULL), c.enabled, c.verification_status,
	c.verified_at, COALESCE(c.last_verification_error_code, ''), c.measurement_version,
	c.version, c.created_at, c.updated_at
`

func (store *Store) ListOwnerProbeConnections(ctx context.Context, ownerUserID string) ([]apihealth.Connection, *domain.AppError) {
	if store == nil || store.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := store.pool.Query(ctx, `
		SELECT `+apiProbeConnectionColumns+`
		FROM api_probe_connections c
		WHERE c.owner_user_id = $1
		ORDER BY c.updated_at DESC, c.id DESC
	`, ownerUserID)
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
	err := scanAPIProbeConnection(store.pool.QueryRow(ctx, `
		SELECT `+apiProbeConnectionColumns+`
		FROM api_probe_connections c
		WHERE c.owner_user_id = $1 AND c.id = $2
	`, ownerUserID, connectionID), &connection)
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
	err := store.pool.QueryRow(ctx, `
		SELECT `+apiProbeConnectionColumns+`, c.credential_ciphertext, c.credential_nonce,
		       c.credential_key_version, c.credential_cipher_format
		FROM api_probe_connections c
		WHERE c.owner_user_id = $1 AND c.id = $2
	`, ownerUserID, connectionID).Scan(destinations...)
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

func (store *Store) CreateOwnerProbeConnection(ctx context.Context, connection apihealth.Connection, credential string) (apihealth.Connection, *domain.AppError) {
	if store == nil || store.pool == nil || store.contactCodec == nil {
		return apihealth.Connection{}, internalStoreError()
	}
	connection.ID = uuid.NewString()
	encoded, err := store.contactCodec.encode(strings.TrimSpace(credential), connection.ID, contactFieldProbeAPIKey)
	if err != nil {
		return apihealth.Connection{}, internalStoreError()
	}
	row := store.pool.QueryRow(ctx, `
		INSERT INTO api_probe_connections AS c (
			id, owner_user_id, name, base_url, normalized_base_url,
			credential_ciphertext, credential_nonce, credential_key_version,
			credential_cipher_format, credential_fingerprint,
			enabled, verification_status, verified_at, last_verification_error_code,
			measurement_version, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18
		)
		RETURNING `+apiProbeConnectionColumns,
		connection.ID, connection.OwnerUserID, connection.Name, connection.BaseURL, connection.NormalizedBaseURL,
		encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion, encoded.CipherFormat, []byte(encoded.Fingerprint),
		connection.Enabled, connection.VerificationStatus, connection.VerifiedAt, nullText(connection.LastVerificationErrorCode),
		connection.MeasurementVersion, connection.Version, connection.CreatedAt, connection.UpdatedAt)
	if err := scanAPIProbeConnection(row, &connection); err != nil {
		if isForeignKeyViolation(err) {
			return apihealth.Connection{}, apiHealthConflict("当前用户不存在，无法创建探针连接。")
		}
		return apihealth.Connection{}, internalStoreError()
	}
	return connection, nil
}

func (store *Store) UpdateOwnerProbeConnection(ctx context.Context, connection apihealth.Connection, credential *string, expectedVersion int64) (apihealth.Connection, *domain.AppError) {
	if store == nil || store.pool == nil || store.contactCodec == nil {
		return apihealth.Connection{}, internalStoreError()
	}
	credentialProvided := credential != nil
	var encoded encodedContactValue
	if credentialProvided {
		var err error
		encoded, err = store.contactCodec.encode(strings.TrimSpace(*credential), connection.ID, contactFieldProbeAPIKey)
		if err != nil {
			return apihealth.Connection{}, internalStoreError()
		}
	}
	row := store.pool.QueryRow(ctx, `
		UPDATE api_probe_connections c
		SET name = $4, base_url = $5, normalized_base_url = $6,
		    credential_ciphertext = CASE WHEN $7 THEN $8 ELSE c.credential_ciphertext END,
		    credential_nonce = CASE WHEN $7 THEN $9 ELSE c.credential_nonce END,
		    credential_key_version = CASE WHEN $7 THEN $10 ELSE c.credential_key_version END,
		    credential_cipher_format = CASE WHEN $7 THEN $11 ELSE c.credential_cipher_format END,
		    credential_fingerprint = CASE WHEN $7 THEN $12 ELSE c.credential_fingerprint END,
		    enabled = $13, verification_status = $14, verified_at = $15,
		    last_verification_error_code = $16, measurement_version = $17,
		    version = $18, updated_at = $19
		WHERE c.id = $1 AND c.owner_user_id = $2 AND c.version = $3
		RETURNING `+apiProbeConnectionColumns,
		connection.ID, connection.OwnerUserID, expectedVersion,
		connection.Name, connection.BaseURL, connection.NormalizedBaseURL,
		credentialProvided, nullBytes(encoded.Ciphertext), nullBytes(encoded.Nonce), nullText(encoded.EncryptionKeyVersion),
		nullText(encoded.CipherFormat), nullBytes([]byte(encoded.Fingerprint)), connection.Enabled,
		connection.VerificationStatus, connection.VerifiedAt, nullText(connection.LastVerificationErrorCode),
		connection.MeasurementVersion, connection.Version, connection.UpdatedAt)
	if err := scanAPIProbeConnection(row, &connection); errors.Is(err, pgx.ErrNoRows) {
		return apihealth.Connection{}, apiHealthVersionConflict()
	} else if err != nil {
		return apihealth.Connection{}, internalStoreError()
	}
	return connection, nil
}

func (store *Store) DeleteOwnerProbeConnection(ctx context.Context, ownerUserID, connectionID string, expectedVersion int64) *domain.AppError {
	if store == nil || store.pool == nil {
		return internalStoreError()
	}
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	var version int64
	if err := tx.QueryRow(ctx, `
		SELECT version FROM api_probe_connections
		WHERE owner_user_id = $1 AND id = $2 FOR UPDATE
	`, ownerUserID, connectionID).Scan(&version); errors.Is(err, pgx.ErrNoRows) {
		return apiHealthNotFound()
	} else if err != nil {
		return internalStoreError()
	}
	if version != expectedVersion {
		return apiHealthVersionConflict()
	}
	rows, err := tx.Query(ctx, `
		SELECT id::text, title FROM api_services
		WHERE probe_connection_id = $1 ORDER BY updated_at DESC, id
	`, connectionID)
	if err != nil {
		return internalStoreError()
	}
	references := make([]apihealth.ServiceReference, 0)
	for rows.Next() {
		var reference apihealth.ServiceReference
		if err := rows.Scan(&reference.ID, &reference.Title); err != nil {
			rows.Close()
			return internalStoreError()
		}
		references = append(references, reference)
	}
	if rows.Err() != nil {
		rows.Close()
		return internalStoreError()
	}
	rows.Close()
	if len(references) > 0 {
		titles := make([]string, 0, len(references))
		for _, reference := range references {
			titles = append(titles, reference.Title+" ("+reference.ID+")")
		}
		return apiHealthConflict("该连接仍被以下服务使用，请先改绑或解绑：" + strings.Join(titles, "、"))
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_probe_connections WHERE id = $1`, connectionID); err != nil {
		return internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (store *Store) LoadOwnerProbeConnectionSamples(ctx context.Context, ownerUserID string, connectionIDs []string, since time.Time) (map[string][]apihealth.Sample, *domain.AppError) {
	result := make(map[string][]apihealth.Sample, len(connectionIDs))
	if len(connectionIDs) == 0 {
		return result, nil
	}
	rows, err := store.pool.Query(ctx, `
		SELECT sample.connection_id::text, sample.id::text, sample.connection_id::text,
		       sample.measurement_version, sample.slot_started_at, sample.status,
		       sample.total_duration_ms, sample.http_status_class, COALESCE(sample.error_code, ''),
		       sample.started_at, sample.finished_at, sample.created_at
		FROM api_probe_connection_samples sample
		JOIN api_probe_connections connection ON connection.id = sample.connection_id
		WHERE connection.owner_user_id = $1
		  AND sample.connection_id = ANY($2::uuid[])
		  AND sample.slot_started_at >= $3
		  AND sample.status IN ('succeeded', 'failed')
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
	rows, err := store.pool.Query(ctx, `
		SELECT service.id::text, `+apiProbeConnectionColumns+`
		FROM api_services service
		JOIN api_probe_connections c ON c.id = service.probe_connection_id
		WHERE service.id = ANY($1::uuid[])
	`, serviceIDs)
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
		SELECT service.id::text,
		       sample.id::text, sample.connection_id::text, sample.measurement_version,
		       sample.slot_started_at, sample.status, sample.total_duration_ms,
		       sample.http_status_class, COALESCE(sample.error_code, ''), sample.started_at,
		       sample.finished_at, sample.created_at
		FROM api_services service
		JOIN api_probe_connection_samples sample ON sample.connection_id = service.probe_connection_id
		WHERE service.id = ANY($1::uuid[]) AND sample.slot_started_at >= $2
		  AND sample.status IN ('succeeded', 'failed')
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
		SET status = 'failed', total_duration_ms = $2,
		    error_code = 'internal_timeout', finished_at = $1
		WHERE status = 'running'
		  AND started_at <= $1::timestamptz - ($3::bigint * interval '1 millisecond')
	`, now, timeoutMS, timeoutMS); err != nil {
		return nil, internalStoreError()
	}
	rows, err := tx.Query(ctx, `
		SELECT `+apiProbeConnectionColumns+`,
		       c.credential_ciphertext, c.credential_nonce, c.credential_key_version, c.credential_cipher_format
		FROM api_probe_connections c
		WHERE c.enabled = true AND c.verification_status = 'verified'
		  AND c.credential_ciphertext IS NOT NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM api_probe_connection_samples sample
		    WHERE sample.connection_id = c.id AND sample.slot_started_at = $2
		  )
		ORDER BY c.updated_at, c.id
		FOR UPDATE SKIP LOCKED LIMIT $1
	`, limit, slotStartedAt)
	if err != nil {
		return nil, internalStoreError()
	}
	type claimedConnection struct {
		connection   apihealth.Connection
		ciphertext   []byte
		nonce        []byte
		keyVersion   string
		cipherFormat string
	}
	claimed := make([]claimedConnection, 0, limit)
	for rows.Next() {
		var item claimedConnection
		destinations := apiProbeConnectionScanDestinations(&item.connection)
		destinations = append(destinations, &item.ciphertext, &item.nonce, &item.keyVersion, &item.cipherFormat)
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
			ID: uuid.NewString(), ConnectionID: item.connection.ID,
			MeasurementVersion: item.connection.MeasurementVersion,
			SlotStartedAt:      slotStartedAt, Status: apihealth.SampleStatusRunning,
			StartedAt: now, CreatedAt: now,
		}
		command, err := tx.Exec(ctx, `
			INSERT INTO api_probe_connection_samples (
				id, connection_id, measurement_version, slot_started_at, status, started_at, created_at
			) VALUES ($1, $2, $3, $4, 'running', $5, $5)
			ON CONFLICT (connection_id, slot_started_at) DO NOTHING
		`, sample.ID, sample.ConnectionID, sample.MeasurementVersion, sample.SlotStartedAt, sample.StartedAt)
		if err != nil {
			return nil, internalStoreError()
		}
		if command.RowsAffected() == 0 {
			continue
		}
		credential, err := store.contactCodec.decode(item.ciphertext, item.nonce, item.keyVersion, item.cipherFormat, item.connection.ID, contactFieldProbeAPIKey)
		if err != nil {
			jobs = append(jobs, apihealth.ProbeJob{Sample: sample, Connection: item.connection, CredentialError: true})
			continue
		}
		jobs = append(jobs, apihealth.ProbeJob{Sample: sample, Connection: item.connection, Credential: credential})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, internalStoreError()
	}
	return jobs, nil
}

func (store *Store) FinalizeProbe(ctx context.Context, sampleID string, result apihealth.ProbeResult, finishedAt time.Time) (bool, *domain.AppError) {
	status := apihealth.SampleStatusSucceeded
	var errorCode any
	if result.ErrorCode != "" {
		status = apihealth.SampleStatusFailed
		errorCode = result.ErrorCode
	}
	command, err := store.pool.Exec(ctx, `
		UPDATE api_probe_connection_samples
		SET status = $2, total_duration_ms = $3,
		    http_status_class = $4, error_code = $5, finished_at = $6
		WHERE id = $1 AND status = 'running'
	`, sampleID, status, result.TotalDurationMS, nullInt(result.HTTPStatusClass), errorCode, finishedAt)
	if err != nil {
		return false, internalStoreError()
	}
	return command.RowsAffected() == 1, nil
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
		DELETE FROM api_probe_connection_samples sample USING expired
		WHERE sample.id = expired.id
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
	rows, err := store.pool.Query(ctx, `
		SELECT probe_connection_id::text, id::text, title
		FROM api_services
		WHERE probe_connection_id = ANY($1::uuid[])
		ORDER BY updated_at DESC, id
	`, connectionIDs)
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
		&connection.MeasurementVersion, &connection.Version, &connection.CreatedAt, &connection.UpdatedAt,
	}
}

func apiProbeSampleScanDestinations(sample *apihealth.Sample) []any {
	return []any{
		&sample.ID, &sample.ConnectionID, &sample.MeasurementVersion, &sample.SlotStartedAt,
		&sample.Status, &sample.TotalDurationMS, &sample.HTTPStatusClass, &sample.ErrorCode,
		&sample.StartedAt, &sample.FinishedAt, &sample.CreatedAt,
	}
}

func nullBytes(value []byte) any {
	if len(value) == 0 {
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
