package config

import "testing"

func TestLoadDefaultsToDevelopmentDevAuth(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("ENABLE_DEV_AUTH", "")

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
	t.Setenv("CONTACT_ENCRYPTION_KEY", "production-encryption-key")
	t.Setenv("CONTACT_FINGERPRINT_KEY", "production-fingerprint-key")
	t.Setenv("CONTACT_KEY_VERSION", "prod-v1")
	t.Setenv("ALLOWED_ORIGINS", "https://c2cmarket.example")
	t.Setenv("EMAIL_PROVIDER", "aliyun_directmail")
	t.Setenv("SMTP_HOST", "smtpdm.aliyun.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "noreply@example.com")
	t.Setenv("SMTP_PASSWORD", "smtp-password")
	t.Setenv("MAIL_FROM_ADDRESS", "noreply@example.com")
	t.Setenv("MAIL_FROM_NAME", "C2CMarket")

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
	if cfg.EmailProvider != "aliyun_directmail" || cfg.SMTP.Host != "smtpdm.aliyun.com" || cfg.SMTP.Port != 465 || cfg.SMTP.FromAddress != "noreply@example.com" {
		t.Fatalf("unexpected SMTP config: provider=%s smtp=%+v", cfg.EmailProvider, cfg.SMTP)
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

func TestLoadRejectsProductionMissingAllowedOrigins(t *testing.T) {
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
