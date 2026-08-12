package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const studentRegistrationSettingTargetID = "00000000-0000-0000-0000-000000000091"

func (s *Store) StudentRegistrationConfig(ctx context.Context) (auth.StudentRegistrationConfig, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.StudentRegistrationConfig{}, internalStoreError()
	}
	var config auth.StudentRegistrationConfig
	if err := s.pool.QueryRow(ctx, `
		SELECT enabled, version
		FROM student_registration_settings
		WHERE singleton_key = 'global'
	`).Scan(&config.Enabled, &config.Version); err != nil {
		return auth.StudentRegistrationConfig{}, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, domain, institution_name, enabled, version, created_at, updated_at
		FROM student_institution_domains
		WHERE enabled = true
		ORDER BY institution_name, domain
	`)
	if err != nil {
		return auth.StudentRegistrationConfig{}, internalStoreError()
	}
	defer rows.Close()
	config.Institutions = []auth.StudentInstitutionDomain{}
	for rows.Next() {
		var item auth.StudentInstitutionDomain
		if err := rows.Scan(&item.ID, &item.Domain, &item.InstitutionName, &item.Enabled, &item.Version, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return auth.StudentRegistrationConfig{}, internalStoreError()
		}
		config.Institutions = append(config.Institutions, item)
	}
	if rows.Err() != nil {
		return auth.StudentRegistrationConfig{}, internalStoreError()
	}
	return config, nil
}

func (s *Store) AdminStudentRegistration(ctx context.Context) (auth.StudentRegistrationConfig, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.StudentRegistrationConfig{}, internalStoreError()
	}
	var config auth.StudentRegistrationConfig
	if err := s.pool.QueryRow(ctx, `
		SELECT enabled, version
		FROM student_registration_settings
		WHERE singleton_key = 'global'
	`).Scan(&config.Enabled, &config.Version); err != nil {
		return auth.StudentRegistrationConfig{}, internalStoreError()
	}
	config.Institutions = []auth.StudentInstitutionDomain{}
	return config, nil
}

func (s *Store) AdminStudentInstitutionDomains(ctx context.Context) ([]auth.StudentInstitutionDomain, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, domain, institution_name, enabled, version, created_at, updated_at
		FROM student_institution_domains
		ORDER BY domain, id
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := []auth.StudentInstitutionDomain{}
	for rows.Next() {
		var item auth.StudentInstitutionDomain
		if err := rows.Scan(&item.ID, &item.Domain, &item.InstitutionName, &item.Enabled, &item.Version, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) UpdateAdminStudentRegistrationWithIdempotency(ctx context.Context, entry idempotency.Entry, input auth.StudentRegistrationSettingUpdate, now time.Time, build auth.StudentRegistrationCompletionBuilder) (auth.StudentRegistrationConfig, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	lockedEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, appErr
	}
	var current auth.StudentRegistrationConfig
	if err := tx.QueryRow(ctx, `
		SELECT enabled, version
		FROM student_registration_settings
		WHERE singleton_key = 'global'
		FOR UPDATE
	`).Scan(&current.Enabled, &current.Version); err != nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, internalStoreError()
	}
	if current.Version != input.ExpectedVersion {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, studentRegistrationStoreVersionConflict()
	}
	beforeEnabled := current.Enabled
	if err := tx.QueryRow(ctx, `
		UPDATE student_registration_settings
		SET enabled = $1,
		    version = version + 1,
		    updated_by_admin_id = $2,
		    updated_at = $3
		WHERE singleton_key = 'global'
		RETURNING enabled, version
	`, input.Enabled, input.AdminUserID, now).Scan(&current.Enabled, &current.Version); err != nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, internalStoreError()
	}
	current.Institutions = []auth.StudentInstitutionDomain{}
	if appErr := insertStudentAdminAudit(ctx, tx, input.AdminUserID, "student_registration.updated", "student_registration_setting", studentRegistrationSettingTargetID, input.Reason, input.RequestID, map[string]any{"enabled": beforeEnabled}, map[string]any{"enabled": current.Enabled}, now); appErr != nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, appErr
	}
	completion, appErr := build(current)
	if appErr != nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, lockedEntry, completion, now); appErr != nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.StudentRegistrationConfig{}, idempotency.Completion{}, internalStoreError()
	}
	return current, completion, nil
}

func (s *Store) CreateStudentInstitutionDomainWithIdempotency(ctx context.Context, entry idempotency.Entry, input auth.StudentInstitutionDomainCreateInput, now time.Time, build auth.StudentInstitutionDomainCompletionBuilder) (auth.StudentInstitutionDomain, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	lockedEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, appErr
	}
	var item auth.StudentInstitutionDomain
	err = tx.QueryRow(ctx, `
		INSERT INTO student_institution_domains (
		  domain, institution_name, enabled, version,
		  created_by_admin_id, updated_by_admin_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, 1, $4, $4, $5, $5)
		RETURNING id::text, domain, institution_name, enabled, version, created_at, updated_at
	`, input.Domain, input.InstitutionName, input.Enabled, input.AdminUserID, now).Scan(
		&item.ID, &item.Domain, &item.InstitutionName, &item.Enabled, &item.Version, &item.CreatedAt, &item.UpdatedAt,
	)
	if isUniqueViolation(err) {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, studentInstitutionDomainStoreConflict()
	}
	if err != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := insertStudentAdminAudit(ctx, tx, input.AdminUserID, "student_institution_domain.created", "student_institution_domain", item.ID, input.Reason, input.RequestID, map[string]any{}, studentInstitutionAuditProjection(item), now); appErr != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, appErr
	}
	completion, appErr := build(item)
	if appErr != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, lockedEntry, completion, now); appErr != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) UpdateStudentInstitutionDomainWithIdempotency(ctx context.Context, entry idempotency.Entry, input auth.StudentInstitutionDomainUpdateInput, now time.Time, build auth.StudentInstitutionDomainCompletionBuilder) (auth.StudentInstitutionDomain, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	lockedEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, appErr
	}
	var item auth.StudentInstitutionDomain
	err = tx.QueryRow(ctx, `
		SELECT id::text, domain, institution_name, enabled, version, created_at, updated_at
		FROM student_institution_domains
		WHERE id = $1
		FOR UPDATE
	`, input.ID).Scan(&item.ID, &item.Domain, &item.InstitutionName, &item.Enabled, &item.Version, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, studentInstitutionDomainStoreNotFound()
	}
	if err != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	if item.Version != input.ExpectedVersion {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, studentRegistrationStoreVersionConflict()
	}
	before := studentInstitutionAuditProjection(item)
	previouslyEnabled := item.Enabled
	err = tx.QueryRow(ctx, `
		UPDATE student_institution_domains
		SET institution_name = $2,
		    enabled = $3,
		    version = version + 1,
		    updated_by_admin_id = $4,
		    updated_at = $5
		WHERE id = $1
		RETURNING id::text, domain, institution_name, enabled, version, created_at, updated_at
	`, input.ID, input.InstitutionName, input.Enabled, input.AdminUserID, now).Scan(
		&item.ID, &item.Domain, &item.InstitutionName, &item.Enabled, &item.Version, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	action := "student_institution_domain.updated"
	if previouslyEnabled != item.Enabled {
		if item.Enabled {
			action = "student_institution_domain.enabled"
		} else {
			action = "student_institution_domain.disabled"
		}
	}
	if appErr := insertStudentAdminAudit(ctx, tx, input.AdminUserID, action, "student_institution_domain", item.ID, input.Reason, input.RequestID, before, studentInstitutionAuditProjection(item), now); appErr != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, appErr
	}
	completion, appErr := build(item)
	if appErr != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, lockedEntry, completion, now); appErr != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.StudentInstitutionDomain{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) StartStudentEmailRegistration(ctx context.Context, input auth.EmailRegistrationStartInput, codeHash string, expiresAt, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	email := strings.TrimSpace(strings.ToLower(input.Email))
	domainValue := emailDomain(email)
	if email == "" || domainValue == "" {
		return auth.StudentEmailNotEligibleError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`, "student_registration", email); err != nil {
		return internalStoreError()
	}

	var enabled bool
	if err := tx.QueryRow(ctx, `
		SELECT enabled
		FROM student_registration_settings
		WHERE singleton_key = 'global'
		FOR SHARE
	`).Scan(&enabled); err != nil {
		return internalStoreError()
	}
	if !enabled {
		return domain.NewError(http.StatusForbidden, domain.CodeEmailRegistrationDisabled, "Email registration disabled", "学生邮箱注册当前未开放。")
	}
	var institutionID string
	if err := tx.QueryRow(ctx, `
		SELECT id::text
		FROM student_institution_domains
		WHERE domain = $1 AND enabled = true
		FOR SHARE
	`, domainValue).Scan(&institutionID); errors.Is(err, pgx.ErrNoRows) {
		return auth.StudentEmailNotEligibleError()
	} else if err != nil {
		return internalStoreError()
	}
	var claimed bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM student_email_claims WHERE normalized_email = $1)
	`, email).Scan(&claimed); err != nil {
		return internalStoreError()
	}
	if claimed {
		return auth.StudentEmailClaimedError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = $2
		WHERE user_id IS NULL
		  AND email = $1
		  AND purpose = 'email_registration'
		  AND consumed_at IS NULL
	`, email, now); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO email_verification_codes (
		  user_id, email, purpose, code_hash, expires_at, attempt_count, created_at
		)
		VALUES (NULL, $1, 'email_registration', $2, $3, 0, $4)
	`, email, strings.TrimSpace(codeHash), expiresAt, now); err != nil {
		return internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ConfirmStudentEmailRegistration(
	ctx context.Context,
	input auth.EmailRegistrationConfirmInput,
	codeHash string,
	credential auth.PasswordCredential,
	sessionTokenHash, csrfTokenHash string,
	sessionExpiresAt, sessionAbsoluteExpiresAt, now time.Time,
) (auth.User, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, internalStoreError()
	}
	email := strings.TrimSpace(strings.ToLower(input.Email))
	domainValue := emailDomain(email)
	if appErr := auth.ValidatePublicUsername(input.Username); appErr != nil {
		return auth.User{}, appErr
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var enabled bool
	if err := tx.QueryRow(ctx, `
		SELECT enabled
		FROM student_registration_settings
		WHERE singleton_key = 'global'
		FOR SHARE
	`).Scan(&enabled); err != nil {
		return auth.User{}, internalStoreError()
	}
	if !enabled {
		return auth.User{}, domain.NewError(http.StatusForbidden, domain.CodeEmailRegistrationDisabled, "Email registration disabled", "学生邮箱注册当前未开放。")
	}

	var codeID string
	var storedHash string
	var expiresAt time.Time
	var attempts int
	err = tx.QueryRow(ctx, `
		SELECT id::text, code_hash, expires_at, attempt_count
		FROM email_verification_codes
		WHERE user_id IS NULL
		  AND email = $1
		  AND purpose = 'email_registration'
		  AND consumed_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1
		FOR UPDATE
	`, email).Scan(&codeID, &storedHash, &expiresAt, &attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.VerificationCodeInvalidError()
	}
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	if !now.Before(expiresAt) || attempts >= auth.EmailRegistrationMaxAttempts {
		return auth.User{}, auth.VerificationCodeInvalidError()
	}
	if !constantTimeStringEqual(storedHash, codeHash) {
		attempts++
		_, err = tx.Exec(ctx, `
			UPDATE email_verification_codes
			SET attempt_count = $2,
			    consumed_at = CASE WHEN $2 >= $3 THEN $4 ELSE NULL END
			WHERE id = $1
		`, codeID, attempts, auth.EmailRegistrationMaxAttempts, now)
		if err != nil {
			return auth.User{}, internalStoreError()
		}
		if err := tx.Commit(ctx); err != nil {
			return auth.User{}, internalStoreError()
		}
		return auth.User{}, auth.VerificationCodeInvalidError()
	}

	var institution auth.StudentInstitutionDomain
	err = tx.QueryRow(ctx, `
		SELECT id::text, domain, institution_name, enabled, version, created_at, updated_at
		FROM student_institution_domains
		WHERE domain = $1 AND enabled = true
		FOR SHARE
	`, domainValue).Scan(
		&institution.ID, &institution.Domain, &institution.InstitutionName,
		&institution.Enabled, &institution.Version, &institution.CreatedAt, &institution.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.StudentEmailNotEligibleError()
	}
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	var claimed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM student_email_claims WHERE normalized_email = $1)`, email).Scan(&claimed); err != nil {
		return auth.User{}, internalStoreError()
	}
	if claimed {
		return auth.User{}, auth.StudentEmailClaimedError()
	}

	var user auth.User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (
		  username, display_name, email, email_verified_at, account_status,
		  created_at, updated_at, last_active_at
		)
		VALUES ($1, $1, $2, $3, 'active', $3, $3, $3)
		RETURNING id::text, analytics_user_id::text, username, display_name, account_status
	`, input.Username, email, now).Scan(&user.ID, &user.AnalyticsUserID, &user.Username, &user.DisplayName, &user.Status)
	if isUniqueViolationOnConstraint(err, "users_username_key") {
		return auth.User{}, auth.UsernameUnavailableError()
	}
	if isUniqueViolation(err) {
		return auth.User{}, auth.StudentEmailClaimedError()
	}
	if err != nil {
		return auth.User{}, internalStoreError()
	}

	claim := auth.StudentEmailClaim{
		ID: uuid.NewString(), UserID: user.ID, NormalizedEmail: email,
		InstitutionDomainID: institution.ID, InstitutionDomain: institution.Domain,
		InstitutionName: institution.InstitutionName, ClaimedAt: now,
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO student_email_claims (
		  id, user_id, normalized_email, institution_domain_id, claimed_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, claim.ID, claim.UserID, claim.NormalizedEmail, claim.InstitutionDomainID, claim.ClaimedAt)
	if isUniqueViolation(err) {
		return auth.User{}, auth.StudentEmailClaimedError()
	}
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO user_password_credentials (
		  user_id, password_algorithm, password_salt, password_hash,
		  created_at, password_updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, user.ID, credential.Algorithm, credential.Salt, credential.Hash, now)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	if err := insertRegistrationAttribution(ctx, tx, user.ID, "email", auth.NormalizeRegistrationAttribution(input.Attribution), now); err != nil {
		return auth.User{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = $2
		WHERE id = $1 AND consumed_at IS NULL
	`, codeID, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	metadata, err := json.Marshal(map[string]any{
		"identityType":        "student_email_claim",
		"institutionDomainId": institution.ID,
		"capabilities":        []string{auth.CapabilityAPIOrderCreate},
	})
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (
		  aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
		  aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ('user', $1, 'user.student_identity_assigned', $1, 'user', 1,
		        'student-registration', $2, $3)
	`, user.ID, metadata, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_sessions (
		  user_id, session_token_hash, csrf_token_hash, expires_at,
		  renewed_at, absolute_expires_at, created_at, updated_at, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $6, $5, $6, $6, $6)
	`, user.ID, sessionTokenHash, csrfTokenHash, sessionExpiresAt, sessionAbsoluteExpiresAt, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, internalStoreError()
	}
	user.StudentClaim = &claim
	return auth.HydrateCapabilities(user), nil
}

func loadStudentClaim(ctx context.Context, q queryer, userID string) (*auth.StudentEmailClaim, error) {
	var claim auth.StudentEmailClaim
	err := q.QueryRow(ctx, `
		SELECT claim.id::text, claim.user_id::text, claim.normalized_email,
		       claim.institution_domain_id::text, institution.domain,
		       institution.institution_name, claim.claimed_at
		FROM student_email_claims claim
		JOIN student_institution_domains institution ON institution.id = claim.institution_domain_id
		WHERE claim.user_id = $1
	`, userID).Scan(
		&claim.ID, &claim.UserID, &claim.NormalizedEmail,
		&claim.InstitutionDomainID, &claim.InstitutionDomain,
		&claim.InstitutionName, &claim.ClaimedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &claim, nil
}

func hydrateAuthStudentClaim(ctx context.Context, q queryer, user *auth.User) error {
	claim, err := loadStudentClaim(ctx, q, user.ID)
	if err != nil {
		return err
	}
	user.StudentClaim = claim
	user.Capabilities = auth.ProjectCapabilities(*user)
	return nil
}

func emailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func constantTimeStringEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	var different byte
	for index := range left {
		different |= left[index] ^ right[index]
	}
	return different == 0
}

func studentInstitutionAuditProjection(item auth.StudentInstitutionDomain) map[string]any {
	return map[string]any{
		"domain":          item.Domain,
		"institutionName": item.InstitutionName,
		"enabled":         item.Enabled,
		"version":         item.Version,
	}
}

func insertStudentAdminAudit(ctx context.Context, tx pgx.Tx, adminUserID, action, targetType, targetID, reason, requestID string, before, after map[string]any, now time.Time) *domain.AppError {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return internalStoreError()
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
		  admin_user_id, action, target_type, target_id, reason,
		  before_json, after_json, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, adminUserID, action, targetType, targetID, reason, beforeJSON, afterJSON, requestID, now); err != nil {
		return internalStoreError()
	}
	return nil
}

func studentRegistrationStoreVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "学生注册配置已更新，请刷新后重试。")
}

func studentInstitutionDomainStoreConflict() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Institution domain already exists", "该院校精确域名已存在。")
}

func studentInstitutionDomainStoreNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Institution domain not found", "院校域名不存在。")
}

func (s *Store) MarkSessionPasswordReauthenticated(ctx context.Context, sessionTokenHash string, reauthenticatedAt time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	command, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET password_reauthenticated_at = $2,
		    updated_at = $2,
		    last_seen_at = $2
		WHERE session_token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > $2
		  AND absolute_expires_at > $2
	`, sessionTokenHash, reauthenticatedAt)
	if err != nil {
		return internalStoreError()
	}
	if command.RowsAffected() != 1 {
		return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session expired", "当前会话已过期。")
	}
	return nil
}

func (s *Store) StartOAuthLink(ctx context.Context, sessionTokenHash, stateHash, purpose string, expiresAt, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	command, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET oauth_link_state_hash = $2,
		    oauth_link_state_purpose = $3,
		    oauth_link_state_expires_at = $4,
		    oauth_link_state_consumed_at = NULL,
		    updated_at = $5
		WHERE session_token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > $5
		  AND absolute_expires_at > $5
		  AND password_reauthenticated_at >= $5 - interval '10 minutes'
	`, sessionTokenHash, stateHash, purpose, expiresAt, now)
	if err != nil {
		return internalStoreError()
	}
	if command.RowsAffected() != 1 {
		return auth.RecentReauthenticationRequiredError()
	}
	return nil
}

func (s *Store) CompleteOAuthLink(
	ctx context.Context,
	sessionTokenHash, stateHash string,
	profile auth.OAuthProfile,
	replacementSessionTokenHash, replacementCSRFTokenHash string,
	replacementExpiresAt, replacementAbsoluteExpiresAt, now time.Time,
) (auth.User, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, internalStoreError()
	}
	provider := auth.CanonicalOAuthProvider(profile.Provider)
	subject := auth.CanonicalOAuthSubject(profile.Subject)
	if !auth.IsLinuxDoProvider(provider) || subject == "" {
		return auth.User{}, auth.OAuthLinkStateInvalidError()
	}
	profile.Provider = provider
	profile.Subject = subject
	linuxDoUserID := strings.TrimSpace(profile.LinuxDoUserID)
	if linuxDoUserID == "" {
		linuxDoUserID = subject
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var userID string
	var recentAuth *time.Time
	err = tx.QueryRow(ctx, `
		SELECT user_id::text, password_reauthenticated_at
		FROM auth_sessions
		WHERE session_token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > $3
		  AND absolute_expires_at > $3
		  AND oauth_link_state_hash = $2
		  AND oauth_link_state_purpose = 'link_linuxdo'
		  AND oauth_link_state_consumed_at IS NULL
		  AND oauth_link_state_expires_at > $3
		FOR UPDATE
	`, sessionTokenHash, stateHash, now).Scan(&userID, &recentAuth)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.OAuthLinkStateInvalidError()
	}
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	if recentAuth == nil || recentAuth.Before(now.Add(-auth.RecentPasswordReauthenticationWindow)) {
		return auth.User{}, auth.RecentReauthenticationRequiredError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE auth_sessions
		SET oauth_link_state_consumed_at = $2, updated_at = $2
		WHERE session_token_hash = $1
	`, sessionTokenHash, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	lockKeys := [][]string{{"linux_do_binding", linuxDoUserID}, {"oauth_identity", provider, subject}}
	sort.Slice(lockKeys, func(i, j int) bool { return strings.Join(lockKeys[i], "\x00") < strings.Join(lockKeys[j], "\x00") })
	for _, lockKey := range lockKeys {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(array_to_string($1::text[], ':'), 0))`, lockKey); err != nil {
			return auth.User{}, internalStoreError()
		}
	}

	var ownerID string
	err = tx.QueryRow(ctx, `
		SELECT user_id::text
		FROM auth_identities
		WHERE provider = $1 AND provider_subject = $2
		FOR UPDATE
	`, provider, subject).Scan(&ownerID)
	if err == nil && ownerID != userID {
		return commitOAuthLinkConflict(ctx, tx)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, internalStoreError()
	}
	identityMissing := errors.Is(err, pgx.ErrNoRows)
	var bindingOwnerID string
	err = tx.QueryRow(ctx, `
		SELECT user_id::text
		FROM linux_do_bindings
		WHERE linux_do_user_id = $1
		FOR UPDATE
	`, linuxDoUserID).Scan(&bindingOwnerID)
	if err == nil && bindingOwnerID != userID {
		return commitOAuthLinkConflict(ctx, tx)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, internalStoreError()
	}

	var user auth.User
	var existingBinding authLinuxDoBindingScan
	err = tx.QueryRow(ctx, `
		SELECT u.id::text, u.analytics_user_id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin'),
		       l.linux_do_user_id, l.linux_do_username, l.trust_level, l.avatar_url, l.bound_at, l.last_synced_at
		FROM users u
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.id = $1
		FOR UPDATE OF u
	`, userID).Scan(
		&user.ID, &user.AnalyticsUserID, &user.Username, &user.DisplayName, &user.Status, &user.IsAdmin,
		&existingBinding.userID, &existingBinding.username, &existingBinding.trustLevel,
		&existingBinding.avatarURL, &existingBinding.boundAt, &existingBinding.lastSyncedAt,
	)
	if err != nil || user.Status != auth.AccountStatusActive {
		return auth.User{}, auth.OAuthLinkStateInvalidError()
	}
	applyAuthLinuxDoBinding(&user, existingBinding)
	if err := hydrateAuthStudentClaim(ctx, tx, &user); err != nil {
		return auth.User{}, internalStoreError()
	}
	before := auth.ProjectCapabilities(user)

	if identityMissing {
		_, err = tx.Exec(ctx, `
			INSERT INTO auth_identities (user_id, provider, provider_subject, created_at, last_login_at)
			VALUES ($1, $2, $3, $4, $4)
		`, userID, provider, subject, now)
		if err != nil {
			return auth.User{}, internalStoreError()
		}
	}

	binding, err := syncLinuxDoBinding(ctx, tx, userID, profile, now)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	user.LinuxDoBinding = binding
	after := auth.ProjectCapabilities(user)
	metadata, err := json.Marshal(map[string]any{
		"identityType":       "linux_do",
		"capabilitiesBefore": before,
		"capabilitiesAfter":  after,
	})
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	var nextVersion int64
	if err := tx.QueryRow(ctx, `UPDATE users SET version = version + 1, updated_at = $2 WHERE id = $1 RETURNING version`, user.ID, now).Scan(&nextVersion); err != nil {
		return auth.User{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (
		  aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
		  aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ('user', $1, 'user.linuxdo_linked', $1, 'user', $2,
		        'linuxdo-link', $3, $4)
	`, user.ID, nextVersion, metadata, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = $2, updated_at = $2
		WHERE session_token_hash = $1
	`, sessionTokenHash, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO auth_sessions (
		  user_id, session_token_hash, csrf_token_hash, expires_at,
		  renewed_at, absolute_expires_at, created_at, updated_at, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $6, $5, $6, $6, $6)
	`, user.ID, replacementSessionTokenHash, replacementCSRFTokenHash, replacementExpiresAt, replacementAbsoluteExpiresAt, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, internalStoreError()
	}
	return auth.HydrateCapabilities(user), nil
}

func commitOAuthLinkConflict(ctx context.Context, tx pgx.Tx) (auth.User, *domain.AppError) {
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, internalStoreError()
	}
	return auth.User{}, auth.OAuthIdentityConflictError()
}
