package config

import (
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/database"
)

func TestLoadDefaultsToDevelopmentDevAuth(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ENABLE_DEV_AUTH", "")
	t.Setenv("EMAIL_VERIFICATION_PEPPER", "")
	t.Setenv("MAINTENANCE_INTERVAL", "")
	t.Setenv("MAINTENANCE_BATCH_SIZE", "")
	clearDatabaseOptionEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %q", cfg.Port)
	}
	if cfg.AppEnv != EnvDevelopment {
		t.Fatalf("expected development env, got %q", cfg.AppEnv)
	}
	if !cfg.EnableDevAuth {
		t.Fatalf("expected dev auth enabled in development")
	}
	if cfg.ContactEncryptionKey == "" || cfg.ContactFingerprintKey == "" || cfg.ContactKeyVersion == "" {
		t.Fatalf("expected local contact crypto defaults")
	}
	if cfg.EmailVerificationPepper != localEmailVerificationPepper {
		t.Fatalf("expected explicit local verification pepper")
	}
	if cfg.Maintenance.Interval != 15*time.Minute || cfg.Maintenance.BatchSize != 500 {
		t.Fatalf("unexpected maintenance defaults: %+v", cfg.Maintenance)
	}
	if cfg.Database != database.DefaultPostgresOptions() {
		t.Fatalf("unexpected database defaults: %+v", cfg.Database)
	}
	if cfg.DatabaseSlowQueryAfter != time.Second {
		t.Fatalf("unexpected slow query threshold: %s", cfg.DatabaseSlowQueryAfter)
	}
}

func TestLoadParsesDatabaseOptions(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("DB_MAX_CONNS", "40")
	t.Setenv("DB_MIN_CONNS", "4")
	t.Setenv("DB_MAX_CONN_LIFETIME", "2h")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "10m")
	t.Setenv("DB_HEALTH_CHECK_PERIOD", "30s")
	t.Setenv("DB_STATEMENT_TIMEOUT", "20s")
	t.Setenv("DB_LOCK_TIMEOUT", "3s")
	t.Setenv("DB_IDLE_IN_TRANSACTION_SESSION_TIMEOUT", "45s")
	t.Setenv("DB_SLOW_QUERY_THRESHOLD", "1500ms")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load database options: %v", err)
	}
	expected := database.PostgresOptions{
		MaxConns:                        40,
		MinConns:                        4,
		MaxConnLifetime:                 2 * time.Hour,
		MaxConnIdleTime:                 10 * time.Minute,
		HealthCheckPeriod:               30 * time.Second,
		StatementTimeout:                20 * time.Second,
		LockTimeout:                     3 * time.Second,
		IdleInTransactionSessionTimeout: 45 * time.Second,
	}
	if cfg.Database != expected {
		t.Fatalf("unexpected database options: %+v", cfg.Database)
	}
	if cfg.DatabaseSlowQueryAfter != 1500*time.Millisecond {
		t.Fatalf("unexpected slow query threshold: %s", cfg.DatabaseSlowQueryAfter)
	}
}

func TestLoadRejectsInvalidDatabaseOptions(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("DB_MAX_CONNS", "4")
	t.Setenv("DB_MIN_CONNS", "5")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "database pool configuration is invalid") {
		t.Fatalf("expected invalid database options error, got %v", err)
	}
}

func TestLoadParsesModelAuditAllowedHosts(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("MODEL_AUDIT_ALLOWED_HOSTS", "api.openai.com, relay.example.com,api.openai.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.ModelAuditAllowedHosts) != 2 ||
		cfg.ModelAuditAllowedHosts[0] != "api.openai.com" ||
		cfg.ModelAuditAllowedHosts[1] != "relay.example.com" {
		t.Fatalf("unexpected model audit allowlist: %v", cfg.ModelAuditAllowedHosts)
	}
}

func TestLoadParsesContactKeyringsAndSelectsCurrentVersion(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("CONTACT_KEY_VERSION", "v2")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "")
	t.Setenv("CONTACT_ENCRYPTION_KEYRING", `{"v1":"test-encryption-v1","v2":"test-encryption-v2"}`)
	t.Setenv("CONTACT_FINGERPRINT_KEYRING", `{"v1":"test-fingerprint-v1","v2":"test-fingerprint-v2"}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load keyring config: %v", err)
	}
	if cfg.ContactEncryptionKeys["v1"] != "test-encryption-v1" ||
		cfg.ContactEncryptionKeys["v2"] != "test-encryption-v2" ||
		cfg.ContactFingerprintKeys["v2"] != "test-fingerprint-v2" {
		t.Fatal("unexpected contact keyring entries")
	}
}

func TestLoadRejectsContactKeyringWithoutCurrentVersion(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("CONTACT_KEY_VERSION", "v2")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "")
	t.Setenv("CONTACT_ENCRYPTION_KEYRING", `{"v1":"test-encryption-v1"}`)
	t.Setenv("CONTACT_FINGERPRINT_KEYRING", `{"v1":"test-fingerprint-v1"}`)

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CONTACT_KEY_VERSION") {
		t.Fatalf("expected missing current keyring version error, got %v", err)
	}
}

func TestLoadRejectsMalformedContactKeyring(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("CONTACT_ENCRYPTION_KEYRING", "not-json")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "CONTACT_ENCRYPTION_KEYRING") {
		t.Fatalf("expected malformed keyring error, got %v", err)
	}
}

func clearDatabaseOptionEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"DB_MAX_CONNS",
		"DB_MIN_CONNS",
		"DB_MAX_CONN_LIFETIME",
		"DB_MAX_CONN_IDLE_TIME",
		"DB_HEALTH_CHECK_PERIOD",
		"DB_STATEMENT_TIMEOUT",
		"DB_LOCK_TIMEOUT",
		"DB_IDLE_IN_TRANSACTION_SESSION_TIMEOUT",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadRejectsProductionWithoutDatabase(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ENABLE_DEV_AUTH", "false")

	if _, err := Load(); err == nil {
		t.Fatalf("expected production without database to fail")
	}
}

func TestLoadRejectsProductionDevAuth(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ENABLE_DEV_AUTH", "true")
	t.Setenv("OAUTH_PROVIDER_MODE", "oauth2")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("OAUTH_AUTHORIZE_URL", "https://linux.do/oauth/authorize")
	t.Setenv("OAUTH_TOKEN_URL", "https://linux.do/oauth/token")
	t.Setenv("OAUTH_USERINFO_URL", "https://linux.do/api/user")
	t.Setenv("OAUTH_REDIRECT_URL", "https://c2cmarket.local/api/v1/auth/oauth/callback")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "production-encryption-key")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "production-fingerprint-key")
	t.Setenv("CONTACT_KEY_VERSION", "prod-v1")

	if _, err := Load(); err == nil {
		t.Fatalf("expected production dev auth to fail")
	}
}

func TestLoadRejectsProductionFakeOAuth(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ENABLE_DEV_AUTH", "false")
	t.Setenv("OAUTH_PROVIDER_MODE", "fake")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "production-encryption-key")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "production-fingerprint-key")
	t.Setenv("CONTACT_KEY_VERSION", "prod-v1")

	if _, err := Load(); err == nil {
		t.Fatalf("expected production fake OAuth to fail")
	}
}

func TestLoadAllowsProductionWhenPersistentConfigIsComplete(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ENABLE_DEV_AUTH", "false")
	t.Setenv("OAUTH_PROVIDER_MODE", "oauth2")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("OAUTH_AUTHORIZE_URL", "https://linux.do/oauth/authorize")
	t.Setenv("OAUTH_TOKEN_URL", "https://linux.do/oauth/token")
	t.Setenv("OAUTH_USERINFO_URL", "https://linux.do/api/user")
	t.Setenv("OAUTH_REDIRECT_URL", "https://c2cmarket.local/api/v1/auth/oauth/callback")
	t.Setenv("FRONTEND_ORIGIN", "https://c2cmarket.example/")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "production-encryption-key")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "production-fingerprint-key")
	t.Setenv("CONTACT_KEY_VERSION", "prod-v1")
	t.Setenv("EMAIL_VERIFICATION_PEPPER", "production-email-verification-pepper-value")
	t.Setenv("ALLOWED_ORIGINS", "https://c2cmarket.example")
	t.Setenv("EMAIL_PROVIDER", "aliyun_directmail")
	t.Setenv("SMTP_HOST", "smtpdm.aliyun.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "noreply@example.com")
	t.Setenv("SMTP_PASSWORD", "smtp-password")
	t.Setenv("MAIL_FROM_ADDRESS", "noreply@example.com")
	t.Setenv("MAIL_FROM_NAME", "C2CMarket")
	t.Setenv("METRICS_BEARER_TOKEN", "test-only-metrics-token-at-least-32-bytes")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load production config: %v", err)
	}
	if cfg.EnableDevAuth {
		t.Fatalf("expected production dev auth disabled")
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "https://c2cmarket.example" {
		t.Fatalf("unexpected allowed origins: %+v", cfg.AllowedOrigins)
	}
	if cfg.FrontendOrigin != "https://c2cmarket.example" {
		t.Fatalf("unexpected frontend origin: %q", cfg.FrontendOrigin)
	}
	if cfg.EmailProvider != "aliyun_directmail" || cfg.SMTP.Host != "smtpdm.aliyun.com" || cfg.SMTP.Port != 465 || cfg.SMTP.FromAddress != "noreply@example.com" {
		t.Fatalf("unexpected SMTP config: provider=%s smtp=%+v", cfg.EmailProvider, cfg.SMTP)
	}
	t.Setenv("METRICS_BEARER_TOKEN", "too-short")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "METRICS_BEARER_TOKEN") {
		t.Fatalf("expected short production metrics token to fail, got %v", err)
	}
}

func TestLoadRejectsProductionMissingEmailVerificationPepper(t *testing.T) {
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ENABLE_DEV_AUTH", "false")
	t.Setenv("OAUTH_PROVIDER_MODE", "oauth2")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("OAUTH_AUTHORIZE_URL", "https://linux.do/oauth/authorize")
	t.Setenv("OAUTH_TOKEN_URL", "https://linux.do/oauth/token")
	t.Setenv("OAUTH_USERINFO_URL", "https://linux.do/api/user")
	t.Setenv("OAUTH_REDIRECT_URL", "https://c2cmarket.example/api/v1/auth/oauth/callback")
	t.Setenv("FRONTEND_ORIGIN", "https://c2cmarket.example")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "production-encryption-key")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "production-fingerprint-key")
	t.Setenv("CONTACT_KEY_VERSION", "prod-v1")
	t.Setenv("EMAIL_VERIFICATION_PEPPER", "")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "EMAIL_VERIFICATION_PEPPER") {
		t.Fatalf("expected production verification pepper failure, got %v", err)
	}
}

func TestLoadValidatesMaintenanceBounds(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("MAINTENANCE_INTERVAL", "30s")
	if _, err := Load(); err == nil {
		t.Fatal("expected too-short maintenance interval to fail")
	}

	t.Setenv("MAINTENANCE_INTERVAL", "15m")
	t.Setenv("MAINTENANCE_BATCH_SIZE", "5001")
	if _, err := Load(); err == nil {
		t.Fatal("expected oversized maintenance batch to fail")
	}

	t.Setenv("MAINTENANCE_BATCH_SIZE", "500")
	t.Setenv("READ_NOTIFICATION_RETENTION", "365h")
	t.Setenv("UNREAD_NOTIFICATION_RETENTION", "24h")
	if _, err := Load(); err == nil {
		t.Fatal("expected unread retention shorter than read retention to fail")
	}
}

func TestNormalizeFrontendOrigin(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		requireHTTPS bool
		want         string
		wantError    bool
	}{
		{name: "production HTTPS origin", value: "https://C2CMarket.Shop/", requireHTTPS: true, want: "https://c2cmarket.shop"},
		{name: "development HTTP origin", value: "http://127.0.0.1:5173", want: "http://127.0.0.1:5173"},
		{name: "production rejects HTTP", value: "http://c2cmarket.shop", requireHTTPS: true, wantError: true},
		{name: "rejects path", value: "https://c2cmarket.shop/app", requireHTTPS: true, wantError: true},
		{name: "rejects query", value: "https://c2cmarket.shop?preview=1", requireHTTPS: true, wantError: true},
		{name: "rejects relative value", value: "c2cmarket.shop", requireHTTPS: true, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeFrontendOrigin(tt.value, tt.requireHTTPS)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected %q to fail, got %q", tt.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalize origin: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestLoadRejectsProductionMissingDirectMailConfig(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ENABLE_DEV_AUTH", "false")
	t.Setenv("OAUTH_PROVIDER_MODE", "oauth2")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("OAUTH_AUTHORIZE_URL", "https://linux.do/oauth/authorize")
	t.Setenv("OAUTH_TOKEN_URL", "https://linux.do/oauth/token")
	t.Setenv("OAUTH_USERINFO_URL", "https://linux.do/api/user")
	t.Setenv("OAUTH_REDIRECT_URL", "https://c2cmarket.local/api/v1/auth/oauth/callback")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "production-encryption-key")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "production-fingerprint-key")
	t.Setenv("CONTACT_KEY_VERSION", "prod-v1")
	t.Setenv("ALLOWED_ORIGINS", "https://c2cmarket.example")
	t.Setenv("EMAIL_PROVIDER", "aliyun_directmail")

	if _, err := Load(); err == nil {
		t.Fatalf("expected production startup to require DirectMail config")
	}
}

func TestLoadRejectsProductionDirectMailNonImplicitTLSPort(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ENABLE_DEV_AUTH", "false")
	t.Setenv("OAUTH_PROVIDER_MODE", "oauth2")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("OAUTH_AUTHORIZE_URL", "https://linux.do/oauth/authorize")
	t.Setenv("OAUTH_TOKEN_URL", "https://linux.do/oauth/token")
	t.Setenv("OAUTH_USERINFO_URL", "https://linux.do/api/user")
	t.Setenv("OAUTH_REDIRECT_URL", "https://c2cmarket.local/api/v1/auth/oauth/callback")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "production-encryption-key")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "production-fingerprint-key")
	t.Setenv("CONTACT_KEY_VERSION", "prod-v1")
	t.Setenv("ALLOWED_ORIGINS", "https://c2cmarket.example")
	t.Setenv("EMAIL_PROVIDER", "aliyun_directmail")
	t.Setenv("SMTP_HOST", "smtpdm.aliyun.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "noreply@example.com")
	t.Setenv("SMTP_PASSWORD", "smtp-password")
	t.Setenv("MAIL_FROM_ADDRESS", "noreply@example.com")
	t.Setenv("MAIL_FROM_NAME", "C2CMarket")

	if _, err := Load(); err == nil {
		t.Fatalf("expected production DirectMail SMTP to require port 465")
	}
}

func TestLoadRejectsProductionMissingFrontendOrigin(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ENABLE_DEV_AUTH", "false")
	t.Setenv("OAUTH_PROVIDER_MODE", "oauth2")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("OAUTH_AUTHORIZE_URL", "https://linux.do/oauth/authorize")
	t.Setenv("OAUTH_TOKEN_URL", "https://linux.do/oauth/token")
	t.Setenv("OAUTH_USERINFO_URL", "https://linux.do/api/user")
	t.Setenv("OAUTH_REDIRECT_URL", "https://c2cmarket.local/api/v1/auth/oauth/callback")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "production-encryption-key")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "production-fingerprint-key")
	t.Setenv("CONTACT_KEY_VERSION", "prod-v1")

	if _, err := Load(); err == nil {
		t.Fatalf("expected production startup to require allowed origins")
	}
}

func TestLoadRejectsProductionMissingContactKeys(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", EnvProduction)
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("ENABLE_DEV_AUTH", "false")
	t.Setenv("OAUTH_PROVIDER_MODE", "oauth2")
	t.Setenv("OAUTH_CLIENT_ID", "client-id")
	t.Setenv("OAUTH_CLIENT_SECRET", "client-secret")
	t.Setenv("OAUTH_AUTHORIZE_URL", "https://linux.do/oauth/authorize")
	t.Setenv("OAUTH_TOKEN_URL", "https://linux.do/oauth/token")
	t.Setenv("OAUTH_USERINFO_URL", "https://linux.do/api/user")
	t.Setenv("OAUTH_REDIRECT_URL", "https://c2cmarket.local/api/v1/auth/oauth/callback")
	t.Setenv("CONTACT_ENCRYPTION_KEY", "")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "")
	t.Setenv("CONTACT_KEY_VERSION", "")

	if _, err := Load(); err == nil {
		t.Fatalf("expected production startup to require contact crypto keys")
	}
}

func TestLoadAllowsExplicitNonProductionDevAuthDisable(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ENABLE_DEV_AUTH", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Port != "9090" {
		t.Fatalf("expected configured port, got %q", cfg.Port)
	}
	if cfg.EnableDevAuth {
		t.Fatalf("expected dev auth disabled")
	}
}

func TestLoadRejectsBootstrapUsernameWithoutPassword(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("C2C_BOOTSTRAP_ADMIN_USERNAME", "admin")
	t.Setenv("C2C_BOOTSTRAP_ADMIN_PASSWORD", "")

	if _, err := Load(); err == nil {
		t.Fatalf("expected bootstrap username without password to fail")
	}
}

func TestLoadTrustedProxyDefaultsDisabled(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("TRUST_X_FORWARDED_FOR", "")
	t.Setenv("TRUSTED_PROXIES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.TrustXForwardedFor {
		t.Fatalf("expected X-Forwarded-For trust disabled by default")
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Fatalf("expected no trusted proxies by default, got %+v", cfg.TrustedProxies)
	}
}

func TestLoadTrustedProxyRequiresTrustedProxies(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("TRUST_X_FORWARDED_FOR", "true")
	t.Setenv("TRUSTED_PROXIES", "")

	if _, err := Load(); err == nil {
		t.Fatalf("expected forwarding trust without trusted proxies to fail")
	}
}

func TestLoadTrustedProxyRejectsInvalidEntry(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("TRUST_X_FORWARDED_FOR", "true")
	t.Setenv("TRUSTED_PROXIES", "not-an-ip")

	if _, err := Load(); err == nil {
		t.Fatalf("expected invalid trusted proxy entry to fail")
	}
}

func TestLoadTrustedProxyParsesIPAndCIDR(t *testing.T) {
	t.Setenv("APP_ENV", EnvDevelopment)
	t.Setenv("TRUST_X_FORWARDED_FOR", "true")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 192.168.0.0/24")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !cfg.TrustXForwardedFor {
		t.Fatalf("expected X-Forwarded-For trust enabled")
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "10.0.0.1" || cfg.TrustedProxies[1] != "192.168.0.0/24" {
		t.Fatalf("unexpected trusted proxies: %+v", cfg.TrustedProxies)
	}
}
