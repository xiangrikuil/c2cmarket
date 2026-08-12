package postgres

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

func TestPostgresOAuthIdentityOwnershipAndConcurrency(t *testing.T) {
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
	requireAuthIdentityTestDatabase(t, store)

	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	ordinaryHandle := "oauth-user-" + suffix
	adminHandle := "oauth-admin-" + suffix
	ordinary, appErr := store.EnsureUser(ctx, ordinaryHandle, false, now)
	if appErr != nil {
		t.Fatalf("create ordinary collision fixture: %v", appErr)
	}
	if ordinary.LinuxDoBinding == nil || !ordinary.LinuxDoBinding.Bound || !auth.HasCapability(ordinary, auth.CapabilityCarpoolPublish) {
		t.Fatalf("development user must retain its synthetic LinuxDo identity: %+v", ordinary)
	}
	reloadedOrdinary, appErr := store.UserByID(ctx, ordinary.ID)
	if appErr != nil || reloadedOrdinary.LinuxDoBinding == nil || !reloadedOrdinary.LinuxDoBinding.Bound || !auth.HasCapability(reloadedOrdinary, auth.CapabilityAPIProbeManage) {
		t.Fatalf("development LinuxDo identity was not durable: user=%+v error=%v", reloadedOrdinary, appErr)
	}
	admin, appErr := store.EnsureUser(ctx, adminHandle, true, now)
	if appErr != nil {
		t.Fatalf("create admin collision fixture: %v", appErr)
	}
	userIDs := map[string]struct{}{ordinary.ID: {}, admin.ID: {}}
	defer cleanupAuthIdentityTestUsers(t, store, userIDs)

	linuxDo, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider:      "linux_do",
		Subject:       "subject-" + suffix,
		Username:      ordinaryHandle,
		DisplayName:   "First OAuth Name",
		TrustLevel:    3,
		LinuxDoUserID: "linux-" + suffix,
	}, now)
	if appErr != nil {
		t.Fatalf("create OAuth identity: %v", appErr)
	}
	userIDs[linuxDo.User.ID] = struct{}{}
	if linuxDo.User.ID == ordinary.ID || linuxDo.User.Username == ordinary.Username {
		t.Fatalf("OAuth identity reused ordinary user: ordinary=%+v oauth=%+v", ordinary, linuxDo.User)
	}
	if linuxDo.User.IsAdmin {
		t.Fatalf("normal OAuth login granted admin: %+v", linuxDo.User)
	}

	renamed, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider:        "linux_do",
		Subject:         "subject-" + suffix,
		Username:        "renamed-" + suffix,
		DisplayName:     "Renamed OAuth User",
		TrustLevel:      4,
		LinuxDoUserID:   "linux-" + suffix,
		LinuxDoUsername: "renamed-provider-" + suffix,
	}, now.Add(time.Minute))
	if appErr != nil {
		t.Fatalf("repeat renamed OAuth identity: %v", appErr)
	}
	if renamed.User.ID != linuxDo.User.ID || renamed.User.Username != linuxDo.User.Username || renamed.Created {
		t.Fatalf("provider rename changed identity ownership: first=%+v second=%+v", linuxDo, renamed)
	}
	if renamed.User.DisplayName != "Renamed OAuth User" {
		t.Fatalf("expected refreshed display name, got %+v", renamed.User)
	}

	adminCollision, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider: "linux_do",
		Subject:  "admin-subject-" + suffix,
		Username: adminHandle,
	}, now)
	if appErr != nil {
		t.Fatalf("create OAuth identity with admin username collision: %v", appErr)
	}
	userIDs[adminCollision.User.ID] = struct{}{}
	if adminCollision.User.ID == admin.ID || adminCollision.User.Username == admin.Username || adminCollision.User.IsAdmin {
		t.Fatalf("OAuth admin collision must create an independent non-admin user: admin=%+v oauth=%+v", admin, adminCollision.User)
	}

	otherProvider, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider: "github",
		Subject:  "subject-" + suffix,
		Username: ordinaryHandle,
	}, now)
	if appErr != nil {
		t.Fatalf("create other-provider identity: %v", appErr)
	}
	userIDs[otherProvider.User.ID] = struct{}{}
	if otherProvider.User.ID == linuxDo.User.ID || otherProvider.User.LinuxDoBinding != nil {
		t.Fatalf("provider isolation failed: linux_do=%+v github=%+v", linuxDo.User, otherProvider.User)
	}

	rollbackHandle := "oauth-rollback-" + suffix
	_, appErr = store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider:      "linux_do",
		Subject:       "rollback-subject-" + suffix,
		Username:      rollbackHandle,
		LinuxDoUserID: "linux-" + suffix,
	}, now)
	if appErr == nil {
		t.Fatal("expected duplicate linux.do binding to fail")
	}
	var rollbackRows int
	if err := store.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM users WHERE username = $1)
		  + (SELECT count(*) FROM auth_identities WHERE provider = 'linux_do' AND provider_subject = $2)
	`, rollbackHandle, "rollback-subject-"+suffix).Scan(&rollbackRows); err != nil {
		t.Fatalf("inspect OAuth rollback: %v", err)
	}
	if rollbackRows != 0 {
		t.Fatalf("failed OAuth transaction left partial rows: count=%d", rollbackRows)
	}

	raceSubject := "race-subject-" + suffix
	raceHandle := "oauth-race-" + suffix
	var createdCount atomic.Int32
	var waitGroup sync.WaitGroup
	results := make(chan auth.OAuthUserResult, 16)
	failures := make(chan string, 16)
	for range 16 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			result, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
				Provider: "linux_do",
				Subject:  raceSubject,
				Username: raceHandle,
			}, now)
			if appErr != nil {
				failures <- appErr.Error()
				return
			}
			if result.Created {
				createdCount.Add(1)
			}
			results <- result
		}()
	}
	waitGroup.Wait()
	close(results)
	close(failures)
	for failure := range failures {
		t.Fatalf("concurrent OAuth login failed: %s", failure)
	}

	var winningUserID string
	for result := range results {
		userIDs[result.User.ID] = struct{}{}
		if winningUserID == "" {
			winningUserID = result.User.ID
		}
		if result.User.ID != winningUserID {
			t.Fatalf("concurrent OAuth logins returned different users: first=%s next=%s", winningUserID, result.User.ID)
		}
	}
	if createdCount.Load() != 1 {
		t.Fatalf("expected exactly one committed OAuth user, got created=%d", createdCount.Load())
	}
	var raceUserCount int
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM users
		WHERE username = $1 OR username LIKE $1 || '-%'
	`, raceHandle).Scan(&raceUserCount); err != nil {
		t.Fatalf("count concurrent OAuth users: %v", err)
	}
	if raceUserCount != 1 {
		t.Fatalf("concurrent OAuth login left temporary users: count=%d", raceUserCount)
	}
}

func TestPostgresBootstrapAdminIsCreateOnlyAndProvenanced(t *testing.T) {
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
	requireAuthIdentityTestDatabase(t, store)
	var bootstrapTableExists bool
	if err := store.pool.QueryRow(ctx, `SELECT to_regclass('public.admin_bootstrap_runs') IS NOT NULL`).Scan(&bootstrapTableExists); err != nil {
		t.Fatalf("check bootstrap table: %v", err)
	}
	if !bootstrapTableExists {
		t.Fatal("admin_bootstrap_runs is missing; apply migration 62 to C2C_TEST_DATABASE_URL")
	}
	var existingBootstrapOrAdmin bool
	if err := store.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM admin_bootstrap_runs)
		    OR EXISTS(SELECT 1 FROM user_permissions WHERE permission = 'admin')
	`).Scan(&existingBootstrapOrAdmin); err != nil {
		t.Fatalf("inspect bootstrap test state: %v", err)
	}
	if existingBootstrapOrAdmin {
		t.Skip("bootstrap integration test requires a database without existing bootstrap or admin rows")
	}

	now := time.Date(2026, 7, 26, 13, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	credential := auth.PasswordCredential{
		User: auth.User{
			Username:    "bootstrap-" + suffix,
			DisplayName: "Bootstrap Admin",
			IsAdmin:     true,
			Status:      "active",
		},
		Algorithm: auth.PasswordAlgorithmArgon2IDV1,
		Salt:      "bootstrap-salt-first",
		Hash:      "bootstrap-hash-first",
	}
	first, appErr := store.BootstrapAdminPassword(ctx, credential, now)
	if appErr != nil || !first.Created {
		t.Fatalf("first bootstrap result=%+v err=%v", first, appErr)
	}
	defer cleanupAuthIdentityTestUsers(t, store, map[string]struct{}{first.User.ID: {}})

	retryCredential := credential
	retryCredential.Salt = "bootstrap-salt-second"
	retryCredential.Hash = "bootstrap-hash-second"
	second, appErr := store.BootstrapAdminPassword(ctx, retryCredential, now.Add(time.Minute))
	if appErr != nil || second.Created || second.User.ID != first.User.ID {
		t.Fatalf("proven bootstrap rerun result=%+v err=%v", second, appErr)
	}
	var storedSalt, storedHash string
	if err := store.pool.QueryRow(ctx, `
		SELECT password_salt, password_hash
		FROM user_password_credentials
		WHERE user_id = $1
	`, first.User.ID).Scan(&storedSalt, &storedHash); err != nil {
		t.Fatalf("read bootstrap credential: %v", err)
	}
	if storedSalt != credential.Salt || storedHash != credential.Hash {
		t.Fatalf("bootstrap rerun changed credential: salt=%q hash=%q", storedSalt, storedHash)
	}

	if _, err := store.pool.Exec(ctx, `
		DELETE FROM user_permissions
		WHERE user_id = $1 AND permission = 'admin'
	`, first.User.ID); err != nil {
		t.Fatalf("damage bootstrap permission fixture: %v", err)
	}
	inconsistent, appErr := store.BootstrapAdminPassword(ctx, retryCredential, now.Add(2*time.Minute))
	if appErr == nil || appErr.Code != domain.CodeAdminBootstrapInconsistent {
		t.Fatalf("expected inconsistent bootstrap state, result=%+v err=%v", inconsistent, appErr)
	}
}

func TestPostgresBootstrapAdminRejectsConflictsAndRollsBack(t *testing.T) {
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
	requireAuthIdentityTestDatabase(t, store)

	var bootstrapTableExists bool
	if err := store.pool.QueryRow(ctx, `SELECT to_regclass('public.admin_bootstrap_runs') IS NOT NULL`).Scan(&bootstrapTableExists); err != nil {
		t.Fatalf("check bootstrap table: %v", err)
	}
	if !bootstrapTableExists {
		t.Fatal("admin_bootstrap_runs is missing; apply migration 62 to C2C_TEST_DATABASE_URL")
	}
	var existingBootstrapOrAdmin bool
	if err := store.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM admin_bootstrap_runs)
		    OR EXISTS(SELECT 1 FROM user_permissions WHERE permission = 'admin')
	`).Scan(&existingBootstrapOrAdmin); err != nil {
		t.Fatalf("inspect bootstrap test state: %v", err)
	}
	if existingBootstrapOrAdmin {
		t.Skip("bootstrap integration test requires a database without existing bootstrap or admin rows")
	}

	now := time.Date(2026, 7, 26, 14, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	occupiedUsername := "occupied-" + suffix
	occupied, appErr := store.EnsureUser(ctx, occupiedUsername, false, now)
	if appErr != nil {
		t.Fatalf("create occupied username: %v", appErr)
	}
	_, appErr = store.BootstrapAdminPassword(ctx, auth.PasswordCredential{
		User:      auth.User{Username: occupiedUsername, DisplayName: occupiedUsername, IsAdmin: true, Status: "active"},
		Algorithm: auth.PasswordAlgorithmArgon2IDV1,
		Salt:      "occupied-salt",
		Hash:      "occupied-hash",
	}, now)
	if appErr == nil || appErr.Code != domain.CodeAdminBootstrapConflict {
		t.Fatalf("expected occupied username conflict, got %v", appErr)
	}
	var occupiedIsAdmin, occupiedHasCredential bool
	if err := store.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_permissions WHERE user_id = $1 AND permission = 'admin'),
		       EXISTS(SELECT 1 FROM user_password_credentials WHERE user_id = $1)
	`, occupied.ID).Scan(&occupiedIsAdmin, &occupiedHasCredential); err != nil {
		t.Fatalf("inspect occupied user: %v", err)
	}
	if occupiedIsAdmin || occupiedHasCredential {
		t.Fatalf("bootstrap mutated occupied user: admin=%v credential=%v", occupiedIsAdmin, occupiedHasCredential)
	}
	cleanupAuthIdentityTestUsers(t, store, map[string]struct{}{occupied.ID: {}})

	oauthResult, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider:      "linux_do",
		Subject:       "bootstrap-oauth-" + suffix,
		Username:      "oauth-bootstrap-" + suffix,
		LinuxDoUserID: "bootstrap-linux-" + suffix,
	}, now)
	if appErr != nil {
		t.Fatalf("create OAuth bootstrap collision fixture: %v", appErr)
	}
	_, appErr = store.BootstrapAdminPassword(ctx, auth.PasswordCredential{
		User:      auth.User{Username: oauthResult.User.Username, DisplayName: oauthResult.User.Username, IsAdmin: true, Status: "active"},
		Algorithm: auth.PasswordAlgorithmArgon2IDV1,
		Salt:      "oauth-occupied-salt",
		Hash:      "oauth-occupied-hash",
	}, now)
	if appErr == nil || appErr.Code != domain.CodeAdminBootstrapConflict {
		t.Fatalf("expected OAuth username conflict, got %v", appErr)
	}
	cleanupAuthIdentityTestUsers(t, store, map[string]struct{}{oauthResult.User.ID: {}})

	foreignAdmin, appErr := store.EnsureUser(ctx, "foreign-admin-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("create foreign admin fixture: %v", appErr)
	}
	_, appErr = store.BootstrapAdminPassword(ctx, auth.PasswordCredential{
		User:      auth.User{Username: "blocked-bootstrap-" + suffix, DisplayName: "Blocked Bootstrap", IsAdmin: true, Status: "active"},
		Algorithm: auth.PasswordAlgorithmArgon2IDV1,
		Salt:      "foreign-admin-salt",
		Hash:      "foreign-admin-hash",
	}, now)
	if appErr == nil || appErr.Code != domain.CodeAdminBootstrapConflict {
		t.Fatalf("expected foreign administrator conflict, got %v", appErr)
	}
	cleanupAuthIdentityTestUsers(t, store, map[string]struct{}{foreignAdmin.ID: {}})

	rollbackUsername := "rollback-" + suffix
	_, appErr = store.BootstrapAdminPassword(ctx, auth.PasswordCredential{
		User:      auth.User{Username: rollbackUsername, DisplayName: rollbackUsername, IsAdmin: true, Status: "active"},
		Algorithm: "invalid_algorithm",
		Salt:      "rollback-salt",
		Hash:      "rollback-hash",
	}, now)
	if appErr == nil {
		t.Fatal("expected invalid credential write to fail")
	}
	var rollbackRows int
	if err := store.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM users WHERE username = $1)
		  + (SELECT count(*) FROM admin_bootstrap_runs WHERE username_snapshot = $1)
	`, rollbackUsername).Scan(&rollbackRows); err != nil {
		t.Fatalf("inspect rollback rows: %v", err)
	}
	if rollbackRows != 0 {
		t.Fatalf("failed bootstrap left partial rows: count=%d", rollbackRows)
	}
}

func requireAuthIdentityTestDatabase(t *testing.T, store *Store) {
	t.Helper()
	var databaseName string
	if err := store.pool.QueryRow(context.Background(), `SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.Contains(strings.ToLower(databaseName), "test") {
		t.Skipf("refusing to run auth identity integration test against non-test database %q", databaseName)
	}
}

func cleanupAuthIdentityTestUsers(t *testing.T, store *Store, userIDs map[string]struct{}) {
	t.Helper()
	ctx := context.Background()
	for userID := range userIDs {
		for _, statement := range []string{
			`DELETE FROM admin_bootstrap_runs WHERE user_id = $1`,
			`DELETE FROM auth_sessions WHERE user_id = $1`,
			`DELETE FROM linux_do_bindings WHERE user_id = $1`,
			`DELETE FROM auth_identities WHERE user_id = $1`,
			`DELETE FROM user_permissions WHERE user_id = $1`,
			`DELETE FROM user_password_credentials WHERE user_id = $1`,
			`DELETE FROM users WHERE id = $1`,
		} {
			if _, err := store.pool.Exec(ctx, statement, userID); err != nil {
				t.Fatalf("cleanup auth identity user %s: %v", userID, err)
			}
		}
	}
}
