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

const apiProbeConfigColumns = `
	c.id::text, c.api_service_id::text, c.owner_user_id::text, c.protocol,
	c.base_url, c.normalized_origin, c.model, (c.credential_ciphertext IS NOT NULL), c.enabled,
	c.authorization_status, COALESCE(c.authorization_method, ''), COALESCE(c.verified_origin, ''),
	c.verified_at, COALESCE(c.approved_by_admin_id::text, ''), c.approved_at,
	COALESCE(c.rejection_reason, ''), c.challenge_expires_at, c.measurement_version,
	COALESCE(c.last_config_error_code, ''), c.version, c.created_at, c.updated_at
`

func (s *Store) GetOwnerProbeConfig(ctx context.Context, ownerUserID, serviceID string) (apihealth.Config, bool, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apihealth.Config{}, false, internalStoreError()
	}
	var config apihealth.Config
	err := scanAPIProbeConfig(s.pool.QueryRow(ctx, `
		SELECT `+apiProbeConfigColumns+`
		FROM api_service_probe_configs c
		WHERE c.owner_user_id = $1 AND c.api_service_id = $2
	`, ownerUserID, serviceID), &config)
	if errors.Is(err, pgx.ErrNoRows) {
		return apihealth.Config{}, false, nil
	}
	if err != nil {
		return apihealth.Config{}, false, internalStoreError()
	}
	return config, true, nil
}

func (s *Store) UpsertOwnerProbeConfig(ctx context.Context, mutation apihealth.ConfigMutation, credential *string, expectedVersion int64) (apihealth.Config, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return apihealth.Config{}, internalStoreError()
	}
	config := mutation.Config
	if expectedVersion == 0 {
		config.ID = uuid.NewString()
		var encoded encodedContactValue
		var err error
		if credential != nil {
			encoded, err = s.contactCodec.encode(strings.TrimSpace(*credential), config.ID, contactFieldProbeAPIKey)
			if err != nil {
				return apihealth.Config{}, internalStoreError()
			}
		}
		row := s.pool.QueryRow(ctx, `
			INSERT INTO api_service_probe_configs AS c (
				id, api_service_id, owner_user_id, protocol, base_url, normalized_origin, model,
				credential_ciphertext, credential_nonce, credential_key_version,
				credential_cipher_format, credential_fingerprint,
				enabled, authorization_status, measurement_version, version, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7,
				$8, $9, $10, $11, $12,
				$13, $14, $15, $16, $17, $18
			)
			RETURNING `+apiProbeConfigColumns,
			config.ID, config.APIServiceID, config.OwnerUserID, config.Protocol,
			config.BaseURL, config.NormalizedOrigin, config.Model,
			nullBytes(encoded.Ciphertext), nullBytes(encoded.Nonce), nullText(encoded.EncryptionKeyVersion),
			nullText(encoded.CipherFormat), nullBytes([]byte(encoded.Fingerprint)),
			config.Enabled, config.AuthorizationStatus, config.MeasurementVersion, config.Version,
			config.CreatedAt, config.UpdatedAt)
		if err := scanAPIProbeConfig(row, &config); err != nil {
			if isUniqueViolation(err) || isForeignKeyViolation(err) {
				return apihealth.Config{}, apiHealthConflict("服务不存在、无权配置或已经存在探针配置。")
			}
			return apihealth.Config{}, internalStoreError()
		}
		return config, nil
	}

	credentialProvided := credential != nil
	var encoded encodedContactValue
	if credentialProvided {
		var err error
		encoded, err = s.contactCodec.encode(strings.TrimSpace(*credential), config.ID, contactFieldProbeAPIKey)
		if err != nil {
			return apihealth.Config{}, internalStoreError()
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	row := tx.QueryRow(ctx, `
		UPDATE api_service_probe_configs c
		SET protocol = $4, base_url = $5, normalized_origin = $6, model = $7,
		    credential_ciphertext = CASE WHEN $8 THEN $9 ELSE c.credential_ciphertext END,
		    credential_nonce = CASE WHEN $8 THEN $10 ELSE c.credential_nonce END,
		    credential_key_version = CASE WHEN $8 THEN $11 ELSE c.credential_key_version END,
		    credential_cipher_format = CASE WHEN $8 THEN $12 ELSE c.credential_cipher_format END,
		    credential_fingerprint = CASE WHEN $8 THEN $13 ELSE c.credential_fingerprint END,
		    enabled = $14,
		    authorization_status = $15, authorization_method = $16,
		    verified_origin = $17, verified_at = $18,
		    approved_by_admin_id = $19, approved_at = $20, rejection_reason = $21,
		    challenge_token_hash = CASE WHEN c.measurement_version <> $22 THEN NULL ELSE c.challenge_token_hash END,
		    challenge_expires_at = CASE WHEN c.measurement_version <> $22 THEN NULL ELSE c.challenge_expires_at END,
		    measurement_version = $22, last_config_error_code = $23,
		    version = $24, updated_at = $25
		WHERE c.id = $1 AND c.api_service_id = $2 AND c.owner_user_id = $3 AND c.version = $26
		RETURNING `+apiProbeConfigColumns,
		config.ID, config.APIServiceID, config.OwnerUserID,
		config.Protocol, config.BaseURL, config.NormalizedOrigin, config.Model,
		credentialProvided, nullBytes(encoded.Ciphertext), nullBytes(encoded.Nonce), nullText(encoded.EncryptionKeyVersion),
		nullText(encoded.CipherFormat), nullBytes([]byte(encoded.Fingerprint)), config.Enabled,
		config.AuthorizationStatus, nullText(config.AuthorizationMethod), nullText(config.VerifiedOrigin), config.VerifiedAt,
		nullUUID(config.ApprovedByAdminID), config.ApprovedAt, nullText(config.RejectionReason),
		config.MeasurementVersion, nullText(config.LastConfigErrorCode), config.Version, config.UpdatedAt, expectedVersion)
	if err := scanAPIProbeConfig(row, &config); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apihealth.Config{}, apiHealthVersionConflict()
		}
		return apihealth.Config{}, internalStoreError()
	}
	if mutation.AuthorizationInvalidated {
		if appErr := insertProbeAuthorizationEvent(
			ctx,
			tx,
			config,
			config.OwnerUserID,
			apihealth.AuthorizationActionOriginInvalidated,
			"",
			apihealth.AuthorizationReasonMeasurementChanged,
			config.UpdatedAt,
		); appErr != nil {
			return apihealth.Config{}, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	return config, nil
}

func (s *Store) DeleteOwnerProbeConfig(ctx context.Context, ownerUserID, serviceID string, expectedVersion int64, now time.Time) *domain.AppError {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	var config apihealth.Config
	if err := scanAPIProbeConfig(tx.QueryRow(ctx, `
		SELECT `+apiProbeConfigColumns+` FROM api_service_probe_configs c
		WHERE c.owner_user_id = $1 AND c.api_service_id = $2 FOR UPDATE
	`, ownerUserID, serviceID), &config); errors.Is(err, pgx.ErrNoRows) {
		return apiHealthNotFound()
	} else if err != nil {
		return internalStoreError()
	}
	if config.Version != expectedVersion {
		return apiHealthVersionConflict()
	}
	if appErr := insertProbeAuthorizationEvent(ctx, tx, config, ownerUserID, apihealth.AuthorizationActionConfigDeleted, config.AuthorizationMethod, "", now); appErr != nil {
		return appErr
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_service_probe_configs WHERE id = $1`, config.ID); err != nil {
		return internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) CreateProbeChallenge(ctx context.Context, ownerUserID, serviceID, method string, tokenHash []byte, expiresAt time.Time, expectedVersion int64, now time.Time) (apihealth.Config, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	var config apihealth.Config
	row := tx.QueryRow(ctx, `
		UPDATE api_service_probe_configs c
		SET authorization_status = 'pending', authorization_method = $3,
		    verified_origin = NULL, verified_at = NULL,
		    approved_by_admin_id = NULL, approved_at = NULL, rejection_reason = NULL,
		    challenge_token_hash = $4, challenge_expires_at = $5,
		    version = version + 1, updated_at = $6
		WHERE owner_user_id = $1 AND api_service_id = $2 AND version = $7
		RETURNING `+apiProbeConfigColumns,
		ownerUserID, serviceID, method, tokenHash, expiresAt, now, expectedVersion)
	if err := scanAPIProbeConfig(row, &config); errors.Is(err, pgx.ErrNoRows) {
		return apihealth.Config{}, apiHealthVersionConflict()
	} else if err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	if appErr := insertProbeAuthorizationEvent(ctx, tx, config, ownerUserID, apihealth.AuthorizationActionChallengeCreated, method, "", now); appErr != nil {
		return apihealth.Config{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	return config, nil
}

func (s *Store) GetProbeChallenge(ctx context.Context, ownerUserID, serviceID string) (apihealth.StoredChallenge, *domain.AppError) {
	var challenge apihealth.StoredChallenge
	row := s.pool.QueryRow(ctx, `
		SELECT `+apiProbeConfigColumns+`, COALESCE(c.authorization_method, ''),
		       c.challenge_token_hash, c.challenge_expires_at
		FROM api_service_probe_configs c
		WHERE c.owner_user_id = $1 AND c.api_service_id = $2
	`, ownerUserID, serviceID)
	destinations := apiProbeConfigScanDestinations(&challenge.Config)
	destinations = append(destinations, &challenge.Method, &challenge.TokenHash, &challenge.ExpiresAt)
	if err := row.Scan(destinations...); errors.Is(err, pgx.ErrNoRows) {
		return apihealth.StoredChallenge{}, apiHealthNotFound()
	} else if err != nil {
		return apihealth.StoredChallenge{}, internalStoreError()
	}
	return challenge, nil
}

func (s *Store) CompleteProbeVerification(ctx context.Context, ownerUserID, serviceID, method string, expectedVersion int64, succeeded bool, reason string, now time.Time) (apihealth.Config, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	status := apihealth.AuthorizationPending
	action := apihealth.AuthorizationActionVerificationFailed
	verifiedAt := any(nil)
	if succeeded {
		status = apihealth.AuthorizationVerified
		action = apihealth.AuthorizationActionVerificationSucceeded
		verifiedAt = now
	}
	var config apihealth.Config
	row := tx.QueryRow(ctx, `
		UPDATE api_service_probe_configs c
		SET authorization_status = $3, authorization_method = $4,
		    verified_origin = CASE WHEN $5 THEN normalized_origin ELSE NULL END,
		    verified_at = $6, approved_by_admin_id = NULL, approved_at = NULL,
		    rejection_reason = NULL, challenge_token_hash = NULL, challenge_expires_at = NULL,
		    last_config_error_code = CASE WHEN $5 THEN NULL ELSE $7 END,
		    version = version + 1, updated_at = $8
		WHERE owner_user_id = $1 AND api_service_id = $2 AND version = $9
		RETURNING `+apiProbeConfigColumns,
		ownerUserID, serviceID, status, method, succeeded, verifiedAt, nullText(reason), now, expectedVersion)
	if err := scanAPIProbeConfig(row, &config); errors.Is(err, pgx.ErrNoRows) {
		return apihealth.Config{}, apiHealthVersionConflict()
	} else if err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	if appErr := insertProbeAuthorizationEvent(ctx, tx, config, ownerUserID, action, method, reason, now); appErr != nil {
		return apihealth.Config{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	return config, nil
}

func (s *Store) ListAdminProbeConfigs(ctx context.Context, status string, page domain.PageRequest) (domain.Page[apihealth.Config], *domain.AppError) {
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[apihealth.Config]{}, appErr
	}
	if status == "" {
		status = apihealth.AuthorizationPending
	}
	if status != apihealth.AuthorizationPending && status != apihealth.AuthorizationRejected && status != apihealth.AuthorizationApproved && status != apihealth.AuthorizationVerified {
		return domain.Page[apihealth.Config]{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Status invalid", "授权状态不正确。", "status", "invalid", "授权状态不正确。")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiProbeConfigColumns+`, service.title, owner_user.username, owner_user.display_name
		FROM api_service_probe_configs c
		JOIN api_services service ON service.id = c.api_service_id
		JOIN users owner_user ON owner_user.id = c.owner_user_id
		WHERE c.authorization_status = $1
		  AND ($2::timestamptz IS NULL OR (c.updated_at, c.id) < ($2::timestamptz, $3::uuid))
		ORDER BY c.updated_at DESC, c.id DESC LIMIT $4
	`, status, nullTime(position.Time), nullUUID(position.ID), page.Limit+1)
	if err != nil {
		return domain.Page[apihealth.Config]{}, internalStoreError()
	}
	defer rows.Close()
	items := make([]apihealth.Config, 0, page.Limit+1)
	for rows.Next() {
		var config apihealth.Config
		if err := scanAdminAPIProbeConfig(rows, &config); err != nil {
			return domain.Page[apihealth.Config]{}, internalStoreError()
		}
		items = append(items, config)
	}
	if rows.Err() != nil {
		return domain.Page[apihealth.Config]{}, internalStoreError()
	}
	return pageFromItems(items, page, func(config apihealth.Config) (time.Time, string) { return config.UpdatedAt, config.ID }), nil
}

func (s *Store) AdminDecideProbeConfig(ctx context.Context, adminUserID, configID string, expectedVersion int64, approve bool, reason string, now time.Time) (apihealth.Config, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	status := apihealth.AuthorizationRejected
	action := apihealth.AuthorizationActionAdminRejected
	if approve {
		status = apihealth.AuthorizationApproved
		action = apihealth.AuthorizationActionAdminApproved
	}
	var config apihealth.Config
	row := tx.QueryRow(ctx, `
		UPDATE api_service_probe_configs c
		SET authorization_status = $2, authorization_method = 'admin_approval',
		    verified_origin = CASE WHEN $3 THEN normalized_origin ELSE NULL END,
		    verified_at = CASE WHEN $3 THEN $4::timestamptz ELSE NULL::timestamptz END,
		    approved_by_admin_id = $5, approved_at = $4,
		    rejection_reason = CASE WHEN $3 THEN NULL ELSE $6 END,
		    challenge_token_hash = NULL, challenge_expires_at = NULL,
		    last_config_error_code = NULL, version = c.version + 1, updated_at = $4
		FROM api_services service, users owner_user
		WHERE c.id = $1 AND c.version = $7
		  AND service.id = c.api_service_id AND owner_user.id = c.owner_user_id
		RETURNING `+apiProbeConfigColumns+`, service.title, owner_user.username, owner_user.display_name`,
		configID, status, approve, now, adminUserID, reason, expectedVersion)
	if err := scanAdminAPIProbeConfig(row, &config); errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM api_service_probe_configs WHERE id = $1)`, configID).Scan(&exists); err != nil {
			return apihealth.Config{}, internalStoreError()
		}
		if !exists {
			return apihealth.Config{}, apiHealthNotFound()
		}
		return apihealth.Config{}, apiHealthVersionConflict()
	} else if err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	if appErr := insertProbeAuthorizationEvent(ctx, tx, config, adminUserID, action, apihealth.AuthorizationMethodAdminApproval, reason, now); appErr != nil {
		return apihealth.Config{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apihealth.Config{}, internalStoreError()
	}
	return config, nil
}

func (s *Store) LoadProbeSummaryInputs(ctx context.Context, serviceIDs []string, since time.Time) (map[string]apihealth.SummaryInput, *domain.AppError) {
	result := make(map[string]apihealth.SummaryInput, len(serviceIDs))
	if len(serviceIDs) == 0 {
		return result, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiProbeConfigColumns+` FROM api_service_probe_configs c
		WHERE c.api_service_id = ANY($1::uuid[])
	`, serviceIDs)
	if err != nil {
		return nil, internalStoreError()
	}
	for rows.Next() {
		var config apihealth.Config
		if err := scanAPIProbeConfig(rows, &config); err != nil {
			rows.Close()
			return nil, internalStoreError()
		}
		copy := config
		result[config.APIServiceID] = apihealth.SummaryInput{Config: &copy}
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, internalStoreError()
	}
	rows.Close()
	sampleRows, err := s.pool.Query(ctx, `
		SELECT id::text, api_service_id::text, probe_config_id::text, measurement_version,
		       probe_model_snapshot, slot_started_at, status, ttft_ms, total_duration_ms,
		       http_status_class, COALESCE(error_code, ''), started_at, finished_at, created_at
		FROM api_service_probe_samples
		WHERE api_service_id = ANY($1::uuid[]) AND slot_started_at >= $2
		  AND status IN ('succeeded', 'failed')
		ORDER BY slot_started_at
	`, serviceIDs, since)
	if err != nil {
		return nil, internalStoreError()
	}
	defer sampleRows.Close()
	for sampleRows.Next() {
		var sample apihealth.Sample
		if err := scanAPIProbeSample(sampleRows, &sample); err != nil {
			return nil, internalStoreError()
		}
		input := result[sample.APIServiceID]
		input.Samples = append(input.Samples, sample)
		result[sample.APIServiceID] = input
	}
	if sampleRows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func (s *Store) ClaimDueProbes(ctx context.Context, slotStartedAt, now time.Time, limit int, runningTimeout time.Duration) ([]apihealth.ProbeJob, *domain.AppError) {
	if limit < 1 {
		return []apihealth.ProbeJob{}, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rollback(ctx, tx)
	timeoutMS := int(runningTimeout.Milliseconds())
	if timeoutMS < 0 {
		timeoutMS = 0
	}
	if _, err := tx.Exec(ctx, `
		UPDATE api_service_probe_samples
		SET status = 'failed', ttft_ms = NULL, total_duration_ms = $2,
		    error_code = 'internal_timeout', finished_at = $1
		WHERE status = 'running'
		  AND started_at <= $1::timestamptz - ($3::bigint * interval '1 millisecond')
	`, now, timeoutMS, timeoutMS); err != nil {
		return nil, internalStoreError()
	}
	rows, err := tx.Query(ctx, `
		SELECT c.id::text, c.api_service_id::text, c.owner_user_id::text, c.protocol,
		       c.base_url, c.normalized_origin, c.model, true, c.enabled,
		       c.authorization_status, COALESCE(c.authorization_method, ''), COALESCE(c.verified_origin, ''),
		       c.verified_at, COALESCE(c.approved_by_admin_id::text, ''), c.approved_at,
		       COALESCE(c.rejection_reason, ''), c.challenge_expires_at, c.measurement_version,
		       COALESCE(c.last_config_error_code, ''), c.version, c.created_at, c.updated_at,
		       c.credential_ciphertext, c.credential_nonce, c.credential_key_version, c.credential_cipher_format
		FROM api_service_probe_configs c
		WHERE c.enabled = true AND c.credential_ciphertext IS NOT NULL
		  AND c.authorization_status IN ('verified', 'approved')
		  AND c.verified_origin = c.normalized_origin
		  AND NOT EXISTS (
		    SELECT 1 FROM api_service_probe_samples sample
		    WHERE sample.api_service_id = c.api_service_id AND sample.slot_started_at = $2
		  )
		ORDER BY c.updated_at, c.id
		FOR UPDATE SKIP LOCKED LIMIT $1
	`, limit, slotStartedAt)
	if err != nil {
		return nil, internalStoreError()
	}
	type claimedProbeConfig struct {
		config       apihealth.Config
		ciphertext   []byte
		nonce        []byte
		keyVersion   string
		cipherFormat string
	}
	claimedConfigs := make([]claimedProbeConfig, 0, limit)
	for rows.Next() {
		var claimed claimedProbeConfig
		destinations := apiProbeConfigScanDestinations(&claimed.config)
		destinations = append(destinations, &claimed.ciphertext, &claimed.nonce, &claimed.keyVersion, &claimed.cipherFormat)
		if err := rows.Scan(destinations...); err != nil {
			rows.Close()
			return nil, internalStoreError()
		}
		claimedConfigs = append(claimedConfigs, claimed)
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, internalStoreError()
	}
	rows.Close()

	jobs := make([]apihealth.ProbeJob, 0, len(claimedConfigs))
	for _, claimed := range claimedConfigs {
		config := claimed.config
		sample := apihealth.Sample{
			ID: uuid.NewString(), APIServiceID: config.APIServiceID, ProbeConfigID: config.ID,
			MeasurementVersion: config.MeasurementVersion, ProbeModelSnapshot: config.Model,
			SlotStartedAt: slotStartedAt, Status: apihealth.SampleStatusRunning,
			StartedAt: now, CreatedAt: now,
		}
		command, err := tx.Exec(ctx, `
			INSERT INTO api_service_probe_samples (
				id, api_service_id, probe_config_id, measurement_version, probe_model_snapshot,
				slot_started_at, status, started_at, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'running', $7, $7)
			ON CONFLICT (api_service_id, slot_started_at) DO NOTHING
		`, sample.ID, sample.APIServiceID, sample.ProbeConfigID, sample.MeasurementVersion,
			sample.ProbeModelSnapshot, sample.SlotStartedAt, sample.StartedAt)
		if err != nil {
			return nil, internalStoreError()
		}
		if command.RowsAffected() == 0 {
			continue
		}
		credential, err := s.contactCodec.decode(claimed.ciphertext, claimed.nonce, claimed.keyVersion, claimed.cipherFormat, config.ID, contactFieldProbeAPIKey)
		if err != nil {
			jobs = append(jobs, apihealth.ProbeJob{Sample: sample, Config: config, CredentialError: true})
			continue
		}
		jobs = append(jobs, apihealth.ProbeJob{Sample: sample, Config: config, Credential: credential})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, internalStoreError()
	}
	return jobs, nil
}

func (s *Store) FinalizeProbe(ctx context.Context, sampleID string, result apihealth.ProbeResult, finishedAt time.Time) (bool, *domain.AppError) {
	status := apihealth.SampleStatusSucceeded
	var ttft any = result.TTFTMS
	var errorCode any
	if result.ErrorCode != "" {
		status = apihealth.SampleStatusFailed
		ttft = nil
		errorCode = result.ErrorCode
	}
	command, err := s.pool.Exec(ctx, `
		UPDATE api_service_probe_samples
		SET status = $2, ttft_ms = $3, total_duration_ms = $4,
		    http_status_class = $5, error_code = $6, finished_at = $7
		WHERE id = $1 AND status = 'running'
	`, sampleID, status, ttft, result.TotalDurationMS, nullInt(result.HTTPStatusClass), errorCode, finishedAt)
	if err != nil {
		return false, internalStoreError()
	}
	return command.RowsAffected() == 1, nil
}

func (s *Store) DeleteFinalProbeSamplesBefore(ctx context.Context, cutoff time.Time, limit int) (int, *domain.AppError) {
	if limit < 1 {
		return 0, nil
	}
	command, err := s.pool.Exec(ctx, `
		WITH expired AS (
			SELECT id FROM api_service_probe_samples
			WHERE status IN ('succeeded', 'failed') AND finished_at < $1
			ORDER BY finished_at, id LIMIT $2
		)
		DELETE FROM api_service_probe_samples sample USING expired
		WHERE sample.id = expired.id
	`, cutoff, limit)
	if err != nil {
		return 0, internalStoreError()
	}
	return int(command.RowsAffected()), nil
}

func insertProbeAuthorizationEvent(ctx context.Context, tx pgx.Tx, config apihealth.Config, actorUserID, action, method, reason string, now time.Time) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO api_service_probe_authorization_events (
			id, probe_config_id, api_service_id, actor_user_id, action, method,
			origin_snapshot, reason, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, uuid.NewString(), config.ID, config.APIServiceID, nullUUID(actorUserID), action,
		nullText(method), config.NormalizedOrigin, nullText(reason), now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func scanAPIProbeConfig(row scanner, config *apihealth.Config) error {
	return row.Scan(apiProbeConfigScanDestinations(config)...)
}

func scanAdminAPIProbeConfig(row scanner, config *apihealth.Config) error {
	destinations := apiProbeConfigScanDestinations(config)
	destinations = append(destinations, &config.ServiceTitle, &config.OwnerUsername, &config.OwnerDisplayName)
	return row.Scan(destinations...)
}

func apiProbeConfigScanDestinations(config *apihealth.Config) []any {
	return []any{
		&config.ID, &config.APIServiceID, &config.OwnerUserID, &config.Protocol,
		&config.BaseURL, &config.NormalizedOrigin, &config.Model, &config.CredentialConfigured,
		&config.Enabled, &config.AuthorizationStatus, &config.AuthorizationMethod,
		&config.VerifiedOrigin, &config.VerifiedAt, &config.ApprovedByAdminID,
		&config.ApprovedAt, &config.RejectionReason, &config.ChallengeExpiresAt,
		&config.MeasurementVersion, &config.LastConfigErrorCode, &config.Version,
		&config.CreatedAt, &config.UpdatedAt,
	}
}

func scanAPIProbeSample(row scanner, sample *apihealth.Sample) error {
	return row.Scan(
		&sample.ID, &sample.APIServiceID, &sample.ProbeConfigID, &sample.MeasurementVersion,
		&sample.ProbeModelSnapshot, &sample.SlotStartedAt, &sample.Status, &sample.TTFTMS,
		&sample.TotalDurationMS, &sample.HTTPStatusClass, &sample.ErrorCode,
		&sample.StartedAt, &sample.FinishedAt, &sample.CreatedAt,
	)
}

func nullBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func apiHealthNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Probe config not found", "探针配置不存在。")
}

func apiHealthVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "探针配置已更新，请刷新后重试。")
}

func apiHealthConflict(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Probe config conflict", detail)
}
