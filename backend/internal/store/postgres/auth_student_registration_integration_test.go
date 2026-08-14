package postgres

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

type postgresStudentRegistrationSender struct {
	mu    sync.Mutex
	codes map[string]string
}

func (sender *postgresStudentRegistrationSender) SendVerificationCode(_ context.Context, email, code string, _ time.Time) *domain.AppError {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.codes[email] = code
	return nil
}

func (*postgresStudentRegistrationSender) SendRegistrationSuccess(context.Context, string, string, string, time.Time) *domain.AppError {
	return nil
}

func (sender *postgresStudentRegistrationSender) SendPasswordResetCode(_ context.Context, email, code string, _ time.Time) *domain.AppError {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.codes[email] = code
	return nil
}

func (*postgresStudentRegistrationSender) ExposeDevCode() bool { return true }

func TestPostgresStudentRegistrationLifecycleAndConcurrencyAreAtomic(t *testing.T) {
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

	now := time.Date(2026, 8, 13, 2, 0, 0, 0, time.UTC)
	sender := &postgresStudentRegistrationSender{codes: make(map[string]string)}
	authService := auth.NewServiceWithRegistrationEmailSenderAndIdempotency(
		store,
		func() time.Time { return now },
		sender,
		idempotency.NewService(store, func() time.Time { return now }),
	)
	suffix := strings.ToLower(uuid.NewString()[:8])
	domainValue := "students-" + suffix + ".example.edu"

	config, appErr := authService.StudentRegistrationConfig(ctx)
	if appErr != nil {
		t.Fatalf("read default student registration config: %v", appErr)
	}
	if config.Enabled || config.Version != 1 {
		t.Fatalf("migration must seed registration disabled at version 1: %+v", config)
	}
	disabledEmail := "disabled@" + domainValue
	if _, appErr := authService.StartEmailRegistration(ctx, auth.EmailRegistrationStartInput{Email: disabledEmail}); appErr == nil || appErr.Code != domain.CodeEmailRegistrationDisabled {
		t.Fatalf("disabled registration start error = %v", appErr)
	}
	assertStudentRegistrationCounts(t, store, disabledEmail, "", 0, 0, 0)

	admin, appErr := store.EnsureUser(ctx, "student-admin-"+suffix, true, now)
	if appErr != nil || !admin.IsAdmin {
		t.Fatalf("create student registration administrator: user=%+v error=%v", admin, appErr)
	}
	domainCompletion, appErr := authService.CreateStudentInstitutionDomainWithIdempotency(
		ctx,
		admin,
		"POST /api/v1/admin/student-institution-domains",
		"student-domain-"+suffix,
		"student-domain-hash-"+suffix,
		auth.StudentInstitutionDomainCreateInput{
			Domain:          domainValue,
			InstitutionName: "PostgreSQL Test University",
			Enabled:         true,
			Reason:          "验证学生注册真实数据库契约",
			RequestID:       "student-domain-request-" + suffix,
		},
		func(item auth.StudentInstitutionDomain) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{Status: http.StatusCreated, ContentType: "application/json", Body: []byte(`{}`), ResourceType: "student_institution_domain", ResourceID: item.ID}, nil
		},
	)
	if appErr != nil || domainCompletion.Status != http.StatusCreated {
		t.Fatalf("create exact institution domain: completion=%+v error=%v", domainCompletion, appErr)
	}
	settingCompletion, appErr := authService.UpdateAdminStudentRegistrationWithIdempotency(
		ctx,
		admin,
		"PATCH /api/v1/admin/student-registration",
		"student-enable-"+suffix,
		"student-enable-hash-"+suffix,
		auth.StudentRegistrationSettingUpdate{
			Enabled:         true,
			ExpectedVersion: 1,
			Reason:          "开启学生注册 PostgreSQL 验收",
			RequestID:       "student-enable-request-" + suffix,
		},
		func(config auth.StudentRegistrationConfig) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`), ResourceType: "student_registration_setting", ResourceID: "00000000-0000-0000-0000-000000000091"}, nil
		},
	)
	if appErr != nil || settingCompletion.Status != http.StatusOK {
		t.Fatalf("enable student registration: completion=%+v error=%v", settingCompletion, appErr)
	}

	primaryEmail := "primary@" + domainValue
	primaryChallenge := startPostgresStudentRegistration(t, ctx, authService, primaryEmail)
	primaryInput := auth.EmailRegistrationConfirmInput{
		Email: primaryEmail, Code: primaryChallenge.DevCode,
		Username: "primary-" + suffix, Password: "CorrectHorse1!",
	}
	primaryUser, primarySession, appErr := authService.ConfirmEmailRegistration(ctx, primaryInput)
	if appErr != nil {
		t.Fatalf("confirm atomic student registration: %v", appErr)
	}
	if primaryUser.StudentClaim == nil || primaryUser.StudentClaim.NormalizedEmail != primaryEmail || primarySession.UserID != primaryUser.ID || !primarySession.NewRegistration {
		t.Fatalf("unexpected registered user/session: user=%+v session=%+v", primaryUser, primarySession)
	}
	if got := auth.ProjectCapabilities(primaryUser); len(got) != 1 || got[0] != auth.CapabilityAPIOrderCreate {
		t.Fatalf("unexpected student capabilities: %v", got)
	}
	assertAtomicStudentRegistration(t, store, primaryUser.ID, primaryEmail)
	if _, _, appErr := authService.ConfirmEmailRegistration(ctx, primaryInput); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("consumed challenge was reusable: %v", appErr)
	}
	if _, appErr := authService.StartEmailRegistration(ctx, auth.EmailRegistrationStartInput{Email: primaryEmail}); appErr == nil || appErr.Code != domain.CodeStudentEmailClaimed {
		t.Fatalf("durable claim was reusable: %v", appErr)
	}

	firstRaceEmail := "race-a@" + domainValue
	secondRaceEmail := "race-b@" + domainValue
	firstRaceChallenge := startPostgresStudentRegistration(t, ctx, authService, firstRaceEmail)
	secondRaceChallenge := startPostgresStudentRegistration(t, ctx, authService, secondRaceEmail)
	sharedUsername := "shared-" + suffix
	type confirmResult struct {
		email  string
		code   string
		user   auth.User
		appErr *domain.AppError
	}
	results := make(chan confirmResult, 2)
	start := make(chan struct{})
	var group sync.WaitGroup
	for _, candidate := range []struct {
		email string
		code  string
	}{{firstRaceEmail, firstRaceChallenge.DevCode}, {secondRaceEmail, secondRaceChallenge.DevCode}} {
		candidate := candidate
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			user, _, appErr := authService.ConfirmEmailRegistration(ctx, auth.EmailRegistrationConfirmInput{
				Email: candidate.email, Code: candidate.code, Username: sharedUsername, Password: "CorrectHorse1!",
			})
			results <- confirmResult{email: candidate.email, code: candidate.code, user: user, appErr: appErr}
		}()
	}
	close(start)
	group.Wait()
	close(results)

	var usernameWinner, usernameLoser confirmResult
	for result := range results {
		if result.appErr == nil {
			usernameWinner = result
		} else {
			usernameLoser = result
		}
	}
	if usernameWinner.user.ID == "" || usernameLoser.appErr == nil || usernameLoser.appErr.Code != domain.CodeUsernameUnavailable {
		t.Fatalf("concurrent username race did not have one atomic winner: winner=%+v loser_error=%v", usernameWinner.user, usernameLoser.appErr)
	}
	assertStudentRegistrationCounts(t, store, usernameLoser.email, "", 0, 0, 1)
	retryUser, _, appErr := authService.ConfirmEmailRegistration(ctx, auth.EmailRegistrationConfirmInput{
		Email: usernameLoser.email, Code: usernameLoser.code, Username: "retry-" + suffix, Password: "CorrectHorse1!",
	})
	if appErr != nil || retryUser.StudentClaim == nil {
		t.Fatalf("username-conflict rollback did not preserve retryable challenge: user=%+v error=%v", retryUser, appErr)
	}

	claimRaceEmail := "claim-race@" + domainValue
	claimRaceChallenge := startPostgresStudentRegistration(t, ctx, authService, claimRaceEmail)
	claimResults := make(chan confirmResult, 2)
	claimStart := make(chan struct{})
	for _, username := range []string{"claim-a-" + suffix, "claim-b-" + suffix} {
		username := username
		group.Add(1)
		go func() {
			defer group.Done()
			<-claimStart
			user, _, appErr := authService.ConfirmEmailRegistration(ctx, auth.EmailRegistrationConfirmInput{
				Email: claimRaceEmail, Code: claimRaceChallenge.DevCode, Username: username, Password: "CorrectHorse1!",
			})
			claimResults <- confirmResult{email: claimRaceEmail, code: username, user: user, appErr: appErr}
		}()
	}
	close(claimStart)
	group.Wait()
	close(claimResults)
	claimSuccesses := 0
	claimFailures := 0
	for result := range claimResults {
		if result.appErr == nil {
			claimSuccesses++
			continue
		}
		if result.appErr.Code != domain.CodeVerificationCodeInvalid && result.appErr.Code != domain.CodeStudentEmailClaimed {
			t.Fatalf("unexpected concurrent claim error: %v", result.appErr)
		}
		claimFailures++
	}
	if claimSuccesses != 1 || claimFailures != 1 {
		t.Fatalf("concurrent claim race success=%d failure=%d", claimSuccesses, claimFailures)
	}
	assertStudentRegistrationCounts(t, store, claimRaceEmail, "", 1, 1, 0)
	var claimRaceUsers int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE username IN ($1, $2)`, "claim-a-"+suffix, "claim-b-"+suffix).Scan(&claimRaceUsers); err != nil {
		t.Fatalf("inspect claim-race usernames: %v", err)
	}
	if claimRaceUsers != 1 {
		t.Fatalf("concurrent claim race left partial users: count=%d", claimRaceUsers)
	}

	if _, appErr := authService.UpdateAdminStudentRegistrationWithIdempotency(
		ctx,
		admin,
		"PATCH /api/v1/admin/student-registration",
		"student-disable-"+suffix,
		"student-disable-hash-"+suffix,
		auth.StudentRegistrationSettingUpdate{Enabled: false, ExpectedVersion: 2, Reason: "完成验收后恢复关闭", RequestID: "student-disable-request-" + suffix},
		func(config auth.StudentRegistrationConfig) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`), ResourceType: "student_registration_setting", ResourceID: "00000000-0000-0000-0000-000000000091"}, nil
		},
	); appErr != nil {
		t.Fatalf("restore registration switch to disabled: %v", appErr)
	}
}

func startPostgresStudentRegistration(t *testing.T, ctx context.Context, service *auth.Service, email string) auth.EmailRegistrationChallenge {
	t.Helper()
	challenge, appErr := service.StartEmailRegistration(ctx, auth.EmailRegistrationStartInput{Email: email})
	if appErr != nil {
		t.Fatalf("start student registration for %s: %v", email, appErr)
	}
	if len(challenge.DevCode) != 6 || challenge.ExpiresAt.IsZero() {
		t.Fatalf("unexpected registration challenge for %s: %+v", email, challenge)
	}
	return challenge
}

func assertAtomicStudentRegistration(t *testing.T, store *Store, userID, email string) {
	t.Helper()
	var users, claims, credentials, attributions, sessions, events, consumedChallenges int
	if err := store.pool.QueryRow(context.Background(), `
		SELECT
		  (SELECT count(*) FROM users WHERE id = $1 AND email = $2),
		  (SELECT count(*) FROM student_email_claims WHERE user_id = $1 AND normalized_email = $2),
		  (SELECT count(*) FROM user_password_credentials WHERE user_id = $1),
		  (SELECT count(*) FROM user_registration_attributions WHERE user_id = $1),
		  (SELECT count(*) FROM auth_sessions WHERE user_id = $1),
		  (SELECT count(*) FROM domain_events WHERE aggregate_type = 'user' AND aggregate_id = $1 AND event_type = 'user.student_identity_assigned'),
		  (SELECT count(*) FROM email_verification_codes WHERE email = $2 AND purpose = 'email_registration' AND consumed_at IS NOT NULL)
	`, userID, email).Scan(&users, &claims, &credentials, &attributions, &sessions, &events, &consumedChallenges); err != nil {
		t.Fatalf("inspect atomic student registration: %v", err)
	}
	if users != 1 || claims != 1 || credentials != 1 || attributions != 1 || sessions != 1 || events != 1 || consumedChallenges != 1 {
		t.Fatalf("student registration was not atomic: users=%d claims=%d credentials=%d attributions=%d sessions=%d events=%d consumedChallenges=%d", users, claims, credentials, attributions, sessions, events, consumedChallenges)
	}
}

func assertStudentRegistrationCounts(t *testing.T, store *Store, email, username string, wantUsers, wantClaims, wantActiveChallenges int) {
	t.Helper()
	var users, claims, activeChallenges int
	if err := store.pool.QueryRow(context.Background(), `
		SELECT
		  (SELECT count(*) FROM users WHERE email = $1 OR ($2 <> '' AND username = $2)),
		  (SELECT count(*) FROM student_email_claims WHERE normalized_email = $1),
		  (SELECT count(*) FROM email_verification_codes WHERE email = $1 AND purpose = 'email_registration' AND consumed_at IS NULL)
	`, email, username).Scan(&users, &claims, &activeChallenges); err != nil {
		t.Fatalf("inspect student registration counts for %s: %v", email, err)
	}
	if users != wantUsers || claims != wantClaims || activeChallenges != wantActiveChallenges {
		t.Fatalf("student registration counts for %s: users=%d claims=%d activeChallenges=%d, want %d/%d/%d", email, users, claims, activeChallenges, wantUsers, wantClaims, wantActiveChallenges)
	}
}
