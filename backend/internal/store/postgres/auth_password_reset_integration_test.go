package postgres

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

func TestPostgresPasswordResetIsAtomicAndPurposeBound(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer store.Close()

	current := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	student, email := seedPostgresPasswordResetStudent(t, ctx, store, suffix, auth.AccountStatusActive, false)
	sender := &postgresStudentRegistrationSender{codes: make(map[string]string)}
	service := auth.NewServiceWithRegistrationEmailSender(store, func() time.Time { return current }, sender)

	if appErr := service.SetPassword(ctx, auth.SetPasswordInput{UserID: student.ID, NewPassword: "Old-password-1!"}); appErr != nil {
		t.Fatalf("seed password credential: %v", appErr)
	}
	var oldHash string
	if err := store.pool.QueryRow(ctx, `SELECT password_hash FROM user_password_credentials WHERE user_id = $1`, student.ID).Scan(&oldHash); err != nil {
		t.Fatalf("read original password hash: %v", err)
	}
	if appErr := store.CreateSession(ctx, student.ID, "reset-normal-"+suffix, "reset-normal-csrf-"+suffix, current.Add(24*time.Hour), current.Add(30*24*time.Hour), current.Add(-time.Hour)); appErr != nil {
		t.Fatalf("seed normal session: %v", appErr)
	}
	seedPasswordResetSessionBoundaries(t, ctx, store, student.ID, suffix, current)

	var initialVersion, initialGovernanceVersion int64
	var initialStatus string
	var initialSecurityLockedAt *time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT version, governance_version, account_status, security_locked_at
		FROM users WHERE id = $1
	`, student.ID).Scan(&initialVersion, &initialGovernanceVersion, &initialStatus, &initialSecurityLockedAt); err != nil {
		t.Fatalf("read initial user state: %v", err)
	}

	startPasswordReset(t, ctx, service, email)
	var firstChallengeID string
	if err := store.pool.QueryRow(ctx, `
		SELECT id::text FROM email_verification_codes
		WHERE email = $1 AND purpose = 'password_reset' AND consumed_at IS NULL
	`, email).Scan(&firstChallengeID); err != nil {
		t.Fatalf("read first reset challenge: %v", err)
	}
	current = current.Add(time.Minute)
	startPasswordReset(t, ctx, service, email)
	var activeChallenges, replacedChallenges int
	if err := store.pool.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE consumed_at IS NULL)::int,
		  count(*) FILTER (WHERE id = $2 AND consumed_at IS NOT NULL)::int
		FROM email_verification_codes
		WHERE email = $1 AND purpose = 'password_reset'
	`, email, firstChallengeID).Scan(&activeChallenges, &replacedChallenges); err != nil {
		t.Fatalf("inspect reset challenge replacement: %v", err)
	}
	if activeChallenges != 1 || replacedChallenges != 1 {
		t.Fatalf("reset challenge replacement active=%d replaced=%d", activeChallenges, replacedChallenges)
	}

	sender.mu.Lock()
	code := sender.codes[email]
	sender.mu.Unlock()
	if len(code) != 6 {
		t.Fatalf("captured reset code length=%d", len(code))
	}
	current = current.Add(time.Minute)
	input := auth.PasswordResetConfirmInput{
		Email: email, Code: code, NewPassword: "New-password-2!", RequestID: "reset-confirm-" + suffix,
	}
	results := make(chan *domain.AppError, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- service.ConfirmPasswordReset(ctx, input)
		}()
	}
	close(start)
	group.Wait()
	close(results)

	successes, invalid := 0, 0
	for appErr := range results {
		if appErr == nil {
			successes++
			continue
		}
		if appErr.Code == domain.CodeVerificationCodeInvalid {
			invalid++
			continue
		}
		t.Fatalf("unexpected concurrent reset error: %v", appErr)
	}
	if successes != 1 || invalid != 1 {
		t.Fatalf("concurrent reset successes=%d invalid=%d", successes, invalid)
	}

	assertPostgresPasswordResetMutation(t, ctx, store, student.ID, email, oldHash, input.RequestID, initialVersion, initialGovernanceVersion, initialStatus, initialSecurityLockedAt)
	if _, _, appErr := service.LoginWithPassword(ctx, email, "Old-password-1!"); appErr == nil || appErr.Code != domain.CodeInvalidCredentials {
		t.Fatalf("old password remained valid: %v", appErr)
	}
	if user, _, appErr := service.LoginWithPassword(ctx, email, input.NewPassword); appErr != nil || user.ID != student.ID {
		t.Fatalf("new password login user=%+v error=%v", user, appErr)
	}
}

func TestPostgresPasswordResetEligibilityAndConfirmationRecheck(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer store.Close()

	current := time.Date(2026, 8, 14, 14, 0, 0, 0, time.UTC)
	sender := &postgresStudentRegistrationSender{codes: make(map[string]string)}
	service := auth.NewServiceWithRegistrationEmailSender(store, func() time.Time { return current }, sender)
	for _, status := range []string{auth.AccountStatusActive, auth.AccountStatusSuspended, auth.AccountStatusBanned} {
		suffix := strings.ToLower(uuid.NewString()[:8])
		_, email := seedPostgresPasswordResetStudent(t, ctx, store, suffix, status, false)
		startPasswordReset(t, ctx, service, email)
		sender.mu.Lock()
		code := sender.codes[email]
		sender.mu.Unlock()
		if len(code) != 6 {
			t.Fatalf("eligible status %s did not receive a reset code", status)
		}
	}

	for _, fixture := range []struct {
		status string
		locked bool
	}{
		{status: auth.AccountStatusArchived},
		{status: auth.AccountStatusActive, locked: true},
	} {
		suffix := strings.ToLower(uuid.NewString()[:8])
		_, email := seedPostgresPasswordResetStudent(t, ctx, store, suffix, fixture.status, fixture.locked)
		startPasswordReset(t, ctx, service, email)
		sender.mu.Lock()
		_, delivered := sender.codes[email]
		sender.mu.Unlock()
		if delivered {
			t.Fatalf("ineligible status=%s locked=%t received a reset code", fixture.status, fixture.locked)
		}
	}

	for _, isAdmin := range []bool{false, true} {
		suffix := strings.ToLower(uuid.NewString()[:8])
		user, appErr := store.EnsureUser(ctx, "reset-unclaimed-"+suffix, isAdmin, current)
		if appErr != nil {
			t.Fatalf("seed unclaimed identity: %v", appErr)
		}
		email := "unclaimed-" + suffix + "@example.edu"
		startPasswordReset(t, ctx, service, email)
		sender.mu.Lock()
		_, delivered := sender.codes[email]
		sender.mu.Unlock()
		if delivered {
			t.Fatalf("unclaimed identity %s received a reset code", user.ID)
		}
	}

	suffix := strings.ToLower(uuid.NewString()[:8])
	student, email := seedPostgresPasswordResetStudent(t, ctx, store, suffix, auth.AccountStatusActive, false)
	startPasswordReset(t, ctx, service, email)
	sender.mu.Lock()
	code := sender.codes[email]
	sender.mu.Unlock()
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'archived', updated_at = $2 WHERE id = $1`, student.ID, current); err != nil {
		t.Fatalf("archive reset fixture: %v", err)
	}
	if appErr := service.ConfirmPasswordReset(ctx, auth.PasswordResetConfirmInput{Email: email, Code: code, NewPassword: "New-password-2!"}); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("archived account reused issued challenge: %v", appErr)
	}
	assertNoActivePasswordResetChallenge(t, ctx, store, email)

	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'active', updated_at = $2 WHERE id = $1`, student.ID, current); err != nil {
		t.Fatalf("restore reset fixture: %v", err)
	}
	startPasswordReset(t, ctx, service, email)
	sender.mu.Lock()
	code = sender.codes[email]
	sender.mu.Unlock()
	if _, err := store.pool.Exec(ctx, `UPDATE users SET security_locked_at = $2, security_lock_reason = 'integration test', updated_at = $2 WHERE id = $1`, student.ID, current); err != nil {
		t.Fatalf("security-lock reset fixture: %v", err)
	}
	if appErr := service.ConfirmPasswordReset(ctx, auth.PasswordResetConfirmInput{Email: email, Code: code, NewPassword: "New-password-2!"}); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("security-locked account reused issued challenge: %v", appErr)
	}
	assertNoActivePasswordResetChallenge(t, ctx, store, email)
}

func seedPostgresPasswordResetStudent(t *testing.T, ctx context.Context, store *Store, suffix, status string, locked bool) (auth.User, string) {
	t.Helper()
	now := time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC)
	admin, appErr := store.EnsureUser(ctx, "reset-admin-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("seed reset administrator: %v", appErr)
	}
	student, appErr := store.EnsureUser(ctx, "reset-student-"+suffix, false, now)
	if appErr != nil {
		t.Fatalf("seed reset student: %v", appErr)
	}
	domainID := uuid.NewString()
	domainValue := "reset-" + suffix + ".example.edu"
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO student_institution_domains (
		  id, domain, institution_name, enabled, created_by_admin_id, updated_by_admin_id, created_at, updated_at
		)
		VALUES ($1, $2, 'Password Reset Test University', false, $3, $3, $4, $4)
	`, domainID, domainValue, admin.ID, now); err != nil {
		t.Fatalf("seed disabled reset institution domain: %v", err)
	}
	email := "student@" + domainValue
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO student_email_claims (user_id, normalized_email, institution_domain_id, claimed_at)
		VALUES ($1, $2, $3, $4)
	`, student.ID, email, domainID, now); err != nil {
		t.Fatalf("seed reset student claim: %v", err)
	}
	var lockedAt *time.Time
	var lockReason *string
	if locked {
		lockedAt = &now
		reason := "integration test"
		lockReason = &reason
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE users
		SET account_status = $2, security_locked_at = $3, security_lock_reason = $4, updated_at = $5
		WHERE id = $1
	`, student.ID, status, lockedAt, lockReason, now); err != nil {
		t.Fatalf("set reset eligibility state: %v", err)
	}
	reloaded, appErr := store.UserByID(ctx, student.ID)
	if appErr != nil {
		t.Fatalf("reload reset student: %v", appErr)
	}
	return reloaded, email
}

func seedPasswordResetSessionBoundaries(t *testing.T, ctx context.Context, store *Store, userID, suffix string, now time.Time) {
	t.Helper()
	actionID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
		  id, target_user_id, action_type, status, governance_version, reason_code,
		  public_reason, effective_at, is_indefinite, request_id, created_at, updated_at
		)
		VALUES ($1, $2, 'ban', 'effective', 1, 'password_reset_test',
		        'Password reset session boundary fixture.', $3, false, $4, $3, $3)
	`, actionID, userID, now.Add(-2*time.Hour), "reset-action-"+suffix); err != nil {
		t.Fatalf("seed governance action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO restricted_business_sessions (
		  user_id, session_token_hash, csrf_token_hash, governance_action_id,
		  governance_version, restriction_effective_at, created_at, expires_at, last_seen_at
		)
		VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $6)
	`, userID, "reset-restricted-"+suffix, "reset-restricted-csrf-"+suffix, actionID,
		now.Add(-2*time.Hour), now.Add(-time.Hour), now.Add(23*time.Hour)); err != nil {
		t.Fatalf("seed restricted session: %v", err)
	}
	createdAt := now.Add(-5 * time.Minute)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_appeal_sessions (
		  user_id, session_token_hash, csrf_token_hash, created_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, "reset-appeal-"+suffix, "reset-appeal-csrf-"+suffix, createdAt, createdAt.Add(15*time.Minute)); err != nil {
		t.Fatalf("seed account appeal session: %v", err)
	}
}

func startPasswordReset(t *testing.T, ctx context.Context, service *auth.Service, email string) {
	t.Helper()
	result, appErr := service.StartPasswordReset(ctx, auth.PasswordResetStartInput{Email: email, RequestID: "reset-start-" + uuid.NewString()})
	if appErr != nil || !result.Accepted {
		t.Fatalf("start password reset for %s: result=%+v error=%v", email, result, appErr)
	}
}

func assertPostgresPasswordResetMutation(t *testing.T, ctx context.Context, store *Store, userID, email, oldHash, requestID string, initialVersion, initialGovernanceVersion int64, initialStatus string, initialSecurityLockedAt *time.Time) {
	t.Helper()
	var nextHash, status, eventRequestID string
	var nextVersion, governanceVersion int64
	var activeChallenges, consumedChallenges, normalTotal, normalRevoked, restrictedRevoked, appealLive, eventCount int
	var securityLockedAt *time.Time
	var metadata []byte
	if err := store.pool.QueryRow(ctx, `
		SELECT
		  (SELECT password_hash FROM user_password_credentials WHERE user_id = $1),
		  (SELECT version FROM users WHERE id = $1),
		  (SELECT governance_version FROM users WHERE id = $1),
		  (SELECT account_status FROM users WHERE id = $1),
		  (SELECT security_locked_at FROM users WHERE id = $1),
		  (SELECT count(*)::int FROM email_verification_codes WHERE email = $2 AND purpose = 'password_reset' AND consumed_at IS NULL),
		  (SELECT count(*)::int FROM email_verification_codes WHERE email = $2 AND purpose = 'password_reset' AND consumed_at IS NOT NULL),
		  (SELECT count(*)::int FROM auth_sessions WHERE user_id = $1),
		  (SELECT count(*)::int FROM auth_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL),
		  (SELECT count(*)::int FROM restricted_business_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL),
		  (SELECT count(*)::int FROM account_appeal_sessions WHERE user_id = $1 AND revoked_at IS NULL),
		  (SELECT count(*)::int FROM domain_events WHERE aggregate_type = 'user' AND aggregate_id = $1 AND event_type = 'user.password_reset_completed'),
		  (SELECT request_id FROM domain_events WHERE aggregate_type = 'user' AND aggregate_id = $1 AND event_type = 'user.password_reset_completed'),
		  (SELECT metadata_json FROM domain_events WHERE aggregate_type = 'user' AND aggregate_id = $1 AND event_type = 'user.password_reset_completed')
	`, userID, email).Scan(
		&nextHash, &nextVersion, &governanceVersion, &status, &securityLockedAt,
		&activeChallenges, &consumedChallenges, &normalTotal, &normalRevoked,
		&restrictedRevoked, &appealLive, &eventCount, &eventRequestID, &metadata,
	); err != nil {
		t.Fatalf("inspect atomic password reset: %v", err)
	}
	if nextHash == oldHash || nextVersion != initialVersion+1 || governanceVersion != initialGovernanceVersion || status != initialStatus {
		t.Fatalf("password reset user state hash_changed=%t version=%d governance=%d status=%s", nextHash != oldHash, nextVersion, governanceVersion, status)
	}
	if (securityLockedAt == nil) != (initialSecurityLockedAt == nil) {
		t.Fatalf("password reset changed security lock state: before=%v after=%v", initialSecurityLockedAt, securityLockedAt)
	}
	if activeChallenges != 0 || consumedChallenges != 2 || normalTotal != 1 || normalRevoked != 1 || restrictedRevoked != 1 || appealLive != 1 || eventCount != 1 {
		t.Fatalf("password reset mutation active=%d consumed=%d normal=%d/%d restricted_revoked=%d appeal_live=%d events=%d", activeChallenges, consumedChallenges, normalRevoked, normalTotal, restrictedRevoked, appealLive, eventCount)
	}
	if eventRequestID != requestID {
		t.Fatalf("password reset event request_id=%q want=%q", eventRequestID, requestID)
	}
	var eventMetadata struct {
		CredentialType       string   `json:"credentialType"`
		SessionScopesRevoked []string `json:"sessionScopesRevoked"`
	}
	if err := json.Unmarshal(metadata, &eventMetadata); err != nil {
		t.Fatalf("decode password reset event metadata: %v", err)
	}
	if eventMetadata.CredentialType != "password" || len(eventMetadata.SessionScopesRevoked) != 2 || eventMetadata.SessionScopesRevoked[0] != "normal" || eventMetadata.SessionScopesRevoked[1] != "restricted_business" {
		t.Fatalf("unexpected password reset event metadata: %+v", eventMetadata)
	}
}

func assertNoActivePasswordResetChallenge(t *testing.T, ctx context.Context, store *Store, email string) {
	t.Helper()
	var active int
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int FROM email_verification_codes
		WHERE email = $1 AND purpose = 'password_reset' AND consumed_at IS NULL
	`, email).Scan(&active); err != nil {
		t.Fatalf("inspect invalidated password reset challenge: %v", err)
	}
	if active != 0 {
		t.Fatalf("password reset challenge remained active for %s", email)
	}
}
