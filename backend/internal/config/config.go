package config

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/database"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

type Config struct {
	Port                    string
	AppEnv                  string
	DatabaseURL             string
	Database                database.PostgresOptions
	DatabaseSlowQueryAfter  time.Duration
	EnableDevAuth           bool
	FrontendOrigin          string
	AllowedOrigins          []string
	OAuthProviderMode       string
	OAuthClientID           string
	OAuthClientSecret       string
	OAuthAuthorizeURL       string
	OAuthTokenURL           string
	OAuthUserInfoURL        string
	OAuthRedirectURL        string
	OAuthScopes             string
	ContactEncryptionKey    string
	ContactFingerprintKey   string
	ContactKeyVersion       string
	ContactEncryptionKeys   map[string]string
	ContactFingerprintKeys  map[string]string
	BootstrapAdminUsername  string
	BootstrapAdminPassword  string
	TrustXForwardedFor      bool
	TrustedProxies          []string
	ModelAuditAllowedHosts  []string
	EmailVerificationPepper string
	EmailProvider           string
	SMTP                    SMTPConfig
	Maintenance             MaintenanceConfig
	APIHealth               APIHealthConfig
	MetricsBearerToken      string
	TurnstileSecret         string
	TurnstileHostnames      []string
	Sentry                  SentryConfig
}

type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

type MaintenanceConfig struct {
	Interval                       time.Duration
	BatchSize                      int
	SessionRetention               time.Duration
	EmailVerificationRetention     time.Duration
	ReadNotificationRetention      time.Duration
	UnreadNotificationRetention    time.Duration
	DomainEventRetention           time.Duration
	APIDeliveryCredentialRetention time.Duration
}

type APIHealthConfig struct {
	RunnerEnabled bool
	ScanInterval  time.Duration
	Timeout       time.Duration
	Concurrency   int
	BatchSize     int
	Retention     time.Duration
}

type SentryConfig struct {
	Enabled          bool
	DSN              string
	Environment      string
	Release          string
	TracesSampleRate float64
}

const (
	localContactEncryptionKey    = "c2cmarket-local-contact-encryption-key-v1"
	localContactFingerprintKey   = "c2cmarket-local-contact-fingerprint-key-v1"
	localContactKeyVersion       = "local-dev-v1"
	localEmailVerificationPepper = "c2cmarket-local-email-verification-pepper-v1"

	defaultMaintenanceInterval            = 15 * time.Minute
	defaultMaintenanceBatchSize           = 500
	defaultSessionRetention               = 7 * 24 * time.Hour
	defaultEmailVerificationRetention     = 24 * time.Hour
	defaultReadNotificationRetention      = 90 * 24 * time.Hour
	defaultUnreadNotificationRetention    = 365 * 24 * time.Hour
	defaultDomainEventRetention           = 365 * 24 * time.Hour
	defaultAPIDeliveryCredentialRetention = 30 * 24 * time.Hour
	defaultDatabaseSlowQueryAfter         = time.Second
	defaultAPIHealthScanInterval          = time.Minute
	defaultAPIHealthTimeout               = 30 * time.Second
	defaultAPIHealthConcurrency           = 4
	defaultAPIHealthBatchSize             = 50
	defaultAPIHealthRetention             = 8 * 24 * time.Hour
)

func Load() (Config, error) {
	cfg := Config{
		Port:               strings.TrimSpace(os.Getenv("PORT")),
		AppEnv:             strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))),
		DatabaseURL:        strings.TrimSpace(os.Getenv("DATABASE_URL")),
		MetricsBearerToken: strings.TrimSpace(os.Getenv("METRICS_BEARER_TOKEN")),
		TurnstileSecret:    strings.TrimSpace(os.Getenv("TURNSTILE_SECRET")),
		TurnstileHostnames: parseTurnstileHostnames(os.Getenv("TURNSTILE_HOSTNAMES")),
		Sentry: SentryConfig{
			DSN:         strings.TrimSpace(os.Getenv("SENTRY_DSN")),
			Environment: strings.TrimSpace(os.Getenv("SENTRY_ENVIRONMENT")),
			Release:     strings.TrimSpace(os.Getenv("SENTRY_RELEASE")),
		},
		FrontendOrigin:          strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN")),
		AllowedOrigins:          parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS")),
		OAuthProviderMode:       strings.ToLower(strings.TrimSpace(os.Getenv("OAUTH_PROVIDER_MODE"))),
		OAuthClientID:           strings.TrimSpace(os.Getenv("OAUTH_CLIENT_ID")),
		OAuthClientSecret:       strings.TrimSpace(os.Getenv("OAUTH_CLIENT_SECRET")),
		OAuthAuthorizeURL:       strings.TrimSpace(os.Getenv("OAUTH_AUTHORIZE_URL")),
		OAuthTokenURL:           strings.TrimSpace(os.Getenv("OAUTH_TOKEN_URL")),
		OAuthUserInfoURL:        strings.TrimSpace(os.Getenv("OAUTH_USERINFO_URL")),
		OAuthRedirectURL:        strings.TrimSpace(os.Getenv("OAUTH_REDIRECT_URL")),
		OAuthScopes:             strings.TrimSpace(os.Getenv("OAUTH_SCOPES")),
		ContactEncryptionKey:    strings.TrimSpace(os.Getenv("CONTACT_ENCRYPTION_KEY")),
		ContactFingerprintKey:   strings.TrimSpace(os.Getenv("CONTACT_FINGERPRINT_KEY")),
		ContactKeyVersion:       strings.TrimSpace(os.Getenv("CONTACT_KEY_VERSION")),
		BootstrapAdminUsername:  strings.TrimSpace(os.Getenv("C2C_BOOTSTRAP_ADMIN_USERNAME")),
		BootstrapAdminPassword:  strings.TrimSpace(os.Getenv("C2C_BOOTSTRAP_ADMIN_PASSWORD")),
		TrustedProxies:          parseCommaSeparated(os.Getenv("TRUSTED_PROXIES")),
		ModelAuditAllowedHosts:  parseCommaSeparated(os.Getenv("MODEL_AUDIT_ALLOWED_HOSTS")),
		EmailVerificationPepper: strings.TrimSpace(os.Getenv("EMAIL_VERIFICATION_PEPPER")),
		EmailProvider:           strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER"))),
		SMTP: SMTPConfig{
			Host:        strings.TrimSpace(os.Getenv("SMTP_HOST")),
			Username:    strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
			Password:    strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
			FromAddress: strings.TrimSpace(os.Getenv("MAIL_FROM_ADDRESS")),
			FromName:    strings.TrimSpace(os.Getenv("MAIL_FROM_NAME")),
		},
	}
	var err error
	cfg.Database, err = loadPostgresOptions()
	if err != nil {
		return Config{}, err
	}
	cfg.DatabaseSlowQueryAfter, err = parseDurationEnv(
		"DB_SLOW_QUERY_THRESHOLD",
		os.Getenv("DB_SLOW_QUERY_THRESHOLD"),
		defaultDatabaseSlowQueryAfter,
	)
	if err != nil {
		return Config{}, err
	}
	if cfg.DatabaseSlowQueryAfter < 100*time.Millisecond || cfg.DatabaseSlowQueryAfter > 5*time.Minute {
		return Config{}, fmt.Errorf("DB_SLOW_QUERY_THRESHOLD must be between 100ms and 5m")
	}
	cfg.ContactEncryptionKeys, err = parseSecretKeyring("CONTACT_ENCRYPTION_KEYRING", os.Getenv("CONTACT_ENCRYPTION_KEYRING"))
	if err != nil {
		return Config{}, err
	}
	cfg.ContactFingerprintKeys, err = parseSecretKeyring("CONTACT_FINGERPRINT_KEYRING", os.Getenv("CONTACT_FINGERPRINT_KEYRING"))
	if err != nil {
		return Config{}, err
	}
	cfg.Maintenance.Interval, err = parseDurationEnv("MAINTENANCE_INTERVAL", os.Getenv("MAINTENANCE_INTERVAL"), defaultMaintenanceInterval)
	if err != nil {
		return Config{}, err
	}
	if cfg.Maintenance.Interval < time.Minute || cfg.Maintenance.Interval > 24*time.Hour {
		return Config{}, fmt.Errorf("MAINTENANCE_INTERVAL must be between 1m and 24h")
	}
	cfg.Maintenance.BatchSize, err = parseIntEnv("MAINTENANCE_BATCH_SIZE", os.Getenv("MAINTENANCE_BATCH_SIZE"), defaultMaintenanceBatchSize)
	if err != nil {
		return Config{}, err
	}
	if cfg.Maintenance.BatchSize < 1 || cfg.Maintenance.BatchSize > 5000 {
		return Config{}, fmt.Errorf("MAINTENANCE_BATCH_SIZE must be between 1 and 5000")
	}
	cfg.Maintenance.SessionRetention, err = parseDurationEnv("SESSION_RETENTION", os.Getenv("SESSION_RETENTION"), defaultSessionRetention)
	if err != nil {
		return Config{}, err
	}
	cfg.Maintenance.EmailVerificationRetention, err = parseDurationEnv("EMAIL_VERIFICATION_RETENTION", os.Getenv("EMAIL_VERIFICATION_RETENTION"), defaultEmailVerificationRetention)
	if err != nil {
		return Config{}, err
	}
	cfg.Maintenance.ReadNotificationRetention, err = parseDurationEnv("READ_NOTIFICATION_RETENTION", os.Getenv("READ_NOTIFICATION_RETENTION"), defaultReadNotificationRetention)
	if err != nil {
		return Config{}, err
	}
	cfg.Maintenance.UnreadNotificationRetention, err = parseDurationEnv("UNREAD_NOTIFICATION_RETENTION", os.Getenv("UNREAD_NOTIFICATION_RETENTION"), defaultUnreadNotificationRetention)
	if err != nil {
		return Config{}, err
	}
	cfg.Maintenance.DomainEventRetention, err = parseDurationEnv("DOMAIN_EVENT_RETENTION", os.Getenv("DOMAIN_EVENT_RETENTION"), defaultDomainEventRetention)
	if err != nil {
		return Config{}, err
	}
	cfg.Maintenance.APIDeliveryCredentialRetention, err = parseDurationEnv("API_DELIVERY_CREDENTIAL_RETENTION", os.Getenv("API_DELIVERY_CREDENTIAL_RETENTION"), defaultAPIDeliveryCredentialRetention)
	if err != nil {
		return Config{}, err
	}
	if cfg.Maintenance.UnreadNotificationRetention < cfg.Maintenance.ReadNotificationRetention {
		return Config{}, fmt.Errorf("UNREAD_NOTIFICATION_RETENTION must not be shorter than READ_NOTIFICATION_RETENTION")
	}
	cfg.APIHealth.RunnerEnabled, err = parseBoolEnv("API_HEALTH_RUNNER_ENABLED", os.Getenv("API_HEALTH_RUNNER_ENABLED"), true)
	if err != nil {
		return Config{}, err
	}
	cfg.APIHealth.ScanInterval, err = parseDurationEnv("API_HEALTH_SCAN_INTERVAL", os.Getenv("API_HEALTH_SCAN_INTERVAL"), defaultAPIHealthScanInterval)
	if err != nil {
		return Config{}, err
	}
	if cfg.APIHealth.ScanInterval < 30*time.Second || cfg.APIHealth.ScanInterval > 5*time.Minute {
		return Config{}, fmt.Errorf("API_HEALTH_SCAN_INTERVAL must be between 30s and 5m")
	}
	cfg.APIHealth.Timeout, err = parseDurationEnv("API_HEALTH_PROBE_TIMEOUT", os.Getenv("API_HEALTH_PROBE_TIMEOUT"), defaultAPIHealthTimeout)
	if err != nil {
		return Config{}, err
	}
	if cfg.APIHealth.Timeout < time.Second || cfg.APIHealth.Timeout > 30*time.Second {
		return Config{}, fmt.Errorf("API_HEALTH_PROBE_TIMEOUT must be between 1s and 30s")
	}
	cfg.APIHealth.Concurrency, err = parseIntEnv("API_HEALTH_MAX_CONCURRENCY", os.Getenv("API_HEALTH_MAX_CONCURRENCY"), defaultAPIHealthConcurrency)
	if err != nil {
		return Config{}, err
	}
	if cfg.APIHealth.Concurrency < 1 || cfg.APIHealth.Concurrency > 32 {
		return Config{}, fmt.Errorf("API_HEALTH_MAX_CONCURRENCY must be between 1 and 32")
	}
	cfg.APIHealth.BatchSize, err = parseIntEnv("API_HEALTH_CLAIM_BATCH_SIZE", os.Getenv("API_HEALTH_CLAIM_BATCH_SIZE"), defaultAPIHealthBatchSize)
	if err != nil {
		return Config{}, err
	}
	if cfg.APIHealth.BatchSize < 1 || cfg.APIHealth.BatchSize > 200 {
		return Config{}, fmt.Errorf("API_HEALTH_CLAIM_BATCH_SIZE must be between 1 and 200")
	}
	cfg.APIHealth.Retention, err = parseDurationEnv("API_HEALTH_SAMPLE_RETENTION", os.Getenv("API_HEALTH_SAMPLE_RETENTION"), defaultAPIHealthRetention)
	if err != nil {
		return Config{}, err
	}
	if cfg.APIHealth.Retention < 24*time.Hour || cfg.APIHealth.Retention > 30*24*time.Hour {
		return Config{}, fmt.Errorf("API_HEALTH_SAMPLE_RETENTION must be between 24h and 720h")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = EnvDevelopment
	}
	cfg.Sentry.Enabled, err = parseBoolEnv(
		"SENTRY_ENABLED",
		os.Getenv("SENTRY_ENABLED"),
		cfg.AppEnv == EnvProduction && cfg.Sentry.DSN != "",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.Sentry.TracesSampleRate, err = parseFloatEnv(
		"SENTRY_TRACES_SAMPLE_RATE",
		os.Getenv("SENTRY_TRACES_SAMPLE_RATE"),
		0.1,
	)
	if err != nil {
		return Config{}, err
	}
	if cfg.Sentry.TracesSampleRate < 0 || cfg.Sentry.TracesSampleRate > 1 {
		return Config{}, fmt.Errorf("SENTRY_TRACES_SAMPLE_RATE must be between 0 and 1")
	}
	if cfg.Sentry.Environment == "" {
		cfg.Sentry.Environment = cfg.AppEnv
	}
	if cfg.Sentry.DSN != "" {
		if err := validateSentryDSN(cfg.Sentry.DSN); err != nil {
			return Config{}, err
		}
	}
	if cfg.Sentry.Enabled && cfg.Sentry.DSN == "" {
		return Config{}, fmt.Errorf("SENTRY_DSN is required when SENTRY_ENABLED=true")
	}
	if len(cfg.TurnstileHostnames) == 0 && cfg.AppEnv != EnvProduction {
		cfg.TurnstileHostnames = []string{"localhost", "127.0.0.1"}
	}
	if cfg.FrontendOrigin == "" && cfg.AppEnv != EnvProduction {
		cfg.FrontendOrigin = "http://127.0.0.1:5173"
	}
	if cfg.FrontendOrigin != "" {
		normalizedOrigin, err := normalizeFrontendOrigin(cfg.FrontendOrigin, cfg.AppEnv == EnvProduction)
		if err != nil {
			return Config{}, err
		}
		cfg.FrontendOrigin = normalizedOrigin
		cfg.AllowedOrigins = parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS"), cfg.FrontendOrigin)
	}
	if cfg.OAuthProviderMode == "" {
		cfg.OAuthProviderMode = "fake"
	}
	if cfg.OAuthScopes == "" {
		cfg.OAuthScopes = "openid profile"
	}
	if cfg.EmailProvider == "" {
		if cfg.AppEnv == EnvProduction {
			cfg.EmailProvider = "aliyun_directmail"
		} else {
			cfg.EmailProvider = "development"
		}
	}
	cfg.SMTP.Port = 465
	if value := strings.TrimSpace(os.Getenv("SMTP_PORT")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("SMTP_PORT must be a positive integer")
		}
		cfg.SMTP.Port = parsed
	}
	if cfg.SMTP.FromAddress == "" {
		cfg.SMTP.FromAddress = "noreply@example.com"
	}
	if cfg.SMTP.FromName == "" {
		cfg.SMTP.FromName = "C2CMarket"
	}
	if len(cfg.AllowedOrigins) == 0 && cfg.AppEnv != EnvProduction {
		cfg.AllowedOrigins = []string{
			"http://127.0.0.1:5173",
			"http://localhost:5173",
			"http://127.0.0.1:4173",
			"http://localhost:4173",
		}
	}
	if cfg.AppEnv == EnvProduction && cfg.OAuthProviderMode == "fake" {
		return Config{}, fmt.Errorf("OAUTH_PROVIDER_MODE=fake cannot be used in production")
	}
	if cfg.OAuthProviderMode != "fake" && cfg.OAuthProviderMode != "oauth2" {
		return Config{}, fmt.Errorf("OAUTH_PROVIDER_MODE must be fake or oauth2")
	}
	if cfg.EmailProvider != "development" && cfg.EmailProvider != "aliyun_directmail" {
		return Config{}, fmt.Errorf("EMAIL_PROVIDER must be development or aliyun_directmail")
	}
	if cfg.BootstrapAdminUsername != "" && cfg.BootstrapAdminPassword == "" {
		return Config{}, fmt.Errorf("C2C_BOOTSTRAP_ADMIN_PASSWORD is required when C2C_BOOTSTRAP_ADMIN_USERNAME is set")
	}

	trustXForwardedFor, err := parseBoolEnv("TRUST_X_FORWARDED_FOR", os.Getenv("TRUST_X_FORWARDED_FOR"), false)
	if err != nil {
		return Config{}, err
	}
	cfg.TrustXForwardedFor = trustXForwardedFor
	if len(cfg.TrustedProxies) > 0 {
		if err := validateTrustedProxies(cfg.TrustedProxies); err != nil {
			return Config{}, err
		}
	}
	if cfg.TrustXForwardedFor && len(cfg.TrustedProxies) == 0 {
		return Config{}, fmt.Errorf("TRUSTED_PROXIES is required when TRUST_X_FORWARDED_FOR=true")
	}
	if err := validateTurnstileHostnames(cfg.TurnstileHostnames); err != nil {
		return Config{}, err
	}

	devAuthRaw := strings.TrimSpace(os.Getenv("ENABLE_DEV_AUTH"))
	switch strings.ToLower(devAuthRaw) {
	case "":
		cfg.EnableDevAuth = cfg.AppEnv == EnvDevelopment || cfg.AppEnv == EnvTest
	case "1", "true", "yes", "on":
		cfg.EnableDevAuth = true
	case "0", "false", "no", "off":
		cfg.EnableDevAuth = false
	default:
		return Config{}, fmt.Errorf("ENABLE_DEV_AUTH must be true or false")
	}

	if cfg.AppEnv == EnvProduction {
		if cfg.DatabaseURL == "" {
			return Config{}, fmt.Errorf("DATABASE_URL is required in production")
		}
		if cfg.EnableDevAuth {
			return Config{}, fmt.Errorf("dev auth cannot be enabled in production")
		}
		if cfg.OAuthProviderMode != "oauth2" {
			return Config{}, fmt.Errorf("OAUTH_PROVIDER_MODE=oauth2 is required in production")
		}
		if cfg.OAuthClientID == "" || cfg.OAuthClientSecret == "" || cfg.OAuthAuthorizeURL == "" || cfg.OAuthTokenURL == "" || cfg.OAuthUserInfoURL == "" || cfg.OAuthRedirectURL == "" {
			return Config{}, fmt.Errorf("OAuth provider configuration is required in production")
		}
		if cfg.FrontendOrigin == "" {
			return Config{}, fmt.Errorf("FRONTEND_ORIGIN is required in production")
		}
		if len(cfg.AllowedOrigins) == 0 {
			return Config{}, fmt.Errorf("ALLOWED_ORIGINS or FRONTEND_ORIGIN is required in production")
		}
		if cfg.ContactEncryptionKey == "" && len(cfg.ContactEncryptionKeys) == 0 {
			return Config{}, fmt.Errorf("CONTACT_ENCRYPTION_KEY or CONTACT_ENCRYPTION_KEYRING is required in production")
		}
		if cfg.ContactFingerprintKey == "" && len(cfg.ContactFingerprintKeys) == 0 {
			return Config{}, fmt.Errorf("CONTACT_FINGERPRINT_KEY or CONTACT_FINGERPRINT_KEYRING is required in production")
		}
		if cfg.ContactKeyVersion == "" {
			return Config{}, fmt.Errorf("CONTACT_KEY_VERSION is required in production")
		}
		if len([]byte(cfg.EmailVerificationPepper)) < 32 {
			return Config{}, fmt.Errorf("EMAIL_VERIFICATION_PEPPER must be at least 32 bytes in production")
		}
		if len([]byte(cfg.MetricsBearerToken)) < 32 {
			return Config{}, fmt.Errorf("METRICS_BEARER_TOKEN must be at least 32 bytes in production")
		}
		if cfg.TurnstileSecret == "" {
			return Config{}, fmt.Errorf("TURNSTILE_SECRET is required in production")
		}
		if len(cfg.TurnstileHostnames) == 0 {
			return Config{}, fmt.Errorf("TURNSTILE_HOSTNAMES is required in production")
		}
		if cfg.EmailProvider != "aliyun_directmail" {
			return Config{}, fmt.Errorf("EMAIL_PROVIDER=aliyun_directmail is required in production")
		}
		if err := validateSMTPConfig(cfg.SMTP); err != nil {
			return Config{}, err
		}
	}
	if cfg.EmailProvider == "aliyun_directmail" {
		if cfg.SMTP.Port != 465 {
			return Config{}, fmt.Errorf("SMTP_PORT must be 465 for aliyun_directmail")
		}
	}

	if cfg.ContactEncryptionKey == "" && len(cfg.ContactEncryptionKeys) == 0 {
		cfg.ContactEncryptionKey = localContactEncryptionKey
	}
	if cfg.ContactFingerprintKey == "" && len(cfg.ContactFingerprintKeys) == 0 {
		cfg.ContactFingerprintKey = localContactFingerprintKey
	}
	if cfg.ContactKeyVersion == "" {
		cfg.ContactKeyVersion = localContactKeyVersion
	}
	if cfg.ContactEncryptionKeys == nil {
		cfg.ContactEncryptionKeys = map[string]string{}
	}
	if _, exists := cfg.ContactEncryptionKeys[cfg.ContactKeyVersion]; !exists && cfg.ContactEncryptionKey != "" {
		cfg.ContactEncryptionKeys[cfg.ContactKeyVersion] = cfg.ContactEncryptionKey
	}
	if cfg.ContactFingerprintKeys == nil {
		cfg.ContactFingerprintKeys = map[string]string{}
	}
	if _, exists := cfg.ContactFingerprintKeys[cfg.ContactKeyVersion]; !exists && cfg.ContactFingerprintKey != "" {
		cfg.ContactFingerprintKeys[cfg.ContactKeyVersion] = cfg.ContactFingerprintKey
	}
	if strings.TrimSpace(cfg.ContactEncryptionKeys[cfg.ContactKeyVersion]) == "" {
		return Config{}, fmt.Errorf("CONTACT_ENCRYPTION_KEYRING must contain CONTACT_KEY_VERSION")
	}
	if strings.TrimSpace(cfg.ContactFingerprintKeys[cfg.ContactKeyVersion]) == "" {
		return Config{}, fmt.Errorf("CONTACT_FINGERPRINT_KEYRING must contain CONTACT_KEY_VERSION")
	}
	if cfg.EmailVerificationPepper == "" {
		cfg.EmailVerificationPepper = localEmailVerificationPepper
	}

	return cfg, nil
}

func loadPostgresOptions() (database.PostgresOptions, error) {
	options := database.DefaultPostgresOptions()
	maxConns, err := parseIntEnv("DB_MAX_CONNS", os.Getenv("DB_MAX_CONNS"), int(options.MaxConns))
	if err != nil {
		return database.PostgresOptions{}, err
	}
	minConns, err := parseIntEnv("DB_MIN_CONNS", os.Getenv("DB_MIN_CONNS"), int(options.MinConns))
	if err != nil {
		return database.PostgresOptions{}, err
	}
	options.MaxConns = int32(maxConns)
	options.MinConns = int32(minConns)
	options.MaxConnLifetime, err = parseDurationEnv("DB_MAX_CONN_LIFETIME", os.Getenv("DB_MAX_CONN_LIFETIME"), options.MaxConnLifetime)
	if err != nil {
		return database.PostgresOptions{}, err
	}
	options.MaxConnIdleTime, err = parseDurationEnv("DB_MAX_CONN_IDLE_TIME", os.Getenv("DB_MAX_CONN_IDLE_TIME"), options.MaxConnIdleTime)
	if err != nil {
		return database.PostgresOptions{}, err
	}
	options.HealthCheckPeriod, err = parseDurationEnv("DB_HEALTH_CHECK_PERIOD", os.Getenv("DB_HEALTH_CHECK_PERIOD"), options.HealthCheckPeriod)
	if err != nil {
		return database.PostgresOptions{}, err
	}
	options.StatementTimeout, err = parseDurationEnv("DB_STATEMENT_TIMEOUT", os.Getenv("DB_STATEMENT_TIMEOUT"), options.StatementTimeout)
	if err != nil {
		return database.PostgresOptions{}, err
	}
	options.LockTimeout, err = parseDurationEnv("DB_LOCK_TIMEOUT", os.Getenv("DB_LOCK_TIMEOUT"), options.LockTimeout)
	if err != nil {
		return database.PostgresOptions{}, err
	}
	options.IdleInTransactionSessionTimeout, err = parseDurationEnv(
		"DB_IDLE_IN_TRANSACTION_SESSION_TIMEOUT",
		os.Getenv("DB_IDLE_IN_TRANSACTION_SESSION_TIMEOUT"),
		options.IdleInTransactionSessionTimeout,
	)
	if err != nil {
		return database.PostgresOptions{}, err
	}
	if err := options.Validate(); err != nil {
		return database.PostgresOptions{}, fmt.Errorf("database pool configuration is invalid: %w", err)
	}
	return options, nil
}

func LoadContactReencrypt() (Config, error) {
	cfg := Config{
		DatabaseURL:           strings.TrimSpace(os.Getenv("DATABASE_URL")),
		ContactEncryptionKey:  strings.TrimSpace(os.Getenv("CONTACT_ENCRYPTION_KEY")),
		ContactFingerprintKey: strings.TrimSpace(os.Getenv("CONTACT_FINGERPRINT_KEY")),
		ContactKeyVersion:     strings.TrimSpace(os.Getenv("CONTACT_KEY_VERSION")),
	}
	var err error
	cfg.Database, err = loadPostgresOptions()
	if err != nil {
		return Config{}, err
	}
	cfg.ContactEncryptionKeys, err = parseSecretKeyring("CONTACT_ENCRYPTION_KEYRING", os.Getenv("CONTACT_ENCRYPTION_KEYRING"))
	if err != nil {
		return Config{}, err
	}
	cfg.ContactFingerprintKeys, err = parseSecretKeyring("CONTACT_FINGERPRINT_KEYRING", os.Getenv("CONTACT_FINGERPRINT_KEYRING"))
	if err != nil {
		return Config{}, err
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.ContactKeyVersion == "" {
		return Config{}, fmt.Errorf("CONTACT_KEY_VERSION is required")
	}
	if cfg.ContactEncryptionKeys == nil {
		cfg.ContactEncryptionKeys = map[string]string{}
	}
	if _, exists := cfg.ContactEncryptionKeys[cfg.ContactKeyVersion]; !exists && cfg.ContactEncryptionKey != "" {
		cfg.ContactEncryptionKeys[cfg.ContactKeyVersion] = cfg.ContactEncryptionKey
	}
	if cfg.ContactFingerprintKeys == nil {
		cfg.ContactFingerprintKeys = map[string]string{}
	}
	if _, exists := cfg.ContactFingerprintKeys[cfg.ContactKeyVersion]; !exists && cfg.ContactFingerprintKey != "" {
		cfg.ContactFingerprintKeys[cfg.ContactKeyVersion] = cfg.ContactFingerprintKey
	}
	if strings.TrimSpace(cfg.ContactEncryptionKeys[cfg.ContactKeyVersion]) == "" {
		return Config{}, fmt.Errorf("CONTACT_ENCRYPTION_KEYRING must contain CONTACT_KEY_VERSION")
	}
	if strings.TrimSpace(cfg.ContactFingerprintKeys[cfg.ContactKeyVersion]) == "" {
		return Config{}, fmt.Errorf("CONTACT_FINGERPRINT_KEYRING must contain CONTACT_KEY_VERSION")
	}
	return cfg, nil
}

func parseSecretKeyring(name, raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var keyring map[string]string
	if err := json.Unmarshal([]byte(raw), &keyring); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object of version-to-key entries", name)
	}
	if len(keyring) == 0 {
		return nil, fmt.Errorf("%s must not be empty", name)
	}
	normalized := make(map[string]string, len(keyring))
	for version, key := range keyring {
		version = strings.TrimSpace(version)
		if version == "" || len(version) > 128 || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s contains an invalid version or empty key", name)
		}
		normalized[version] = key
	}
	return normalized, nil
}

func validateSMTPConfig(cfg SMTPConfig) error {
	if cfg.Host == "" {
		return fmt.Errorf("SMTP_HOST is required in production")
	}
	if cfg.Username == "" {
		return fmt.Errorf("SMTP_USERNAME is required in production")
	}
	if cfg.Password == "" {
		return fmt.Errorf("SMTP_PASSWORD is required in production")
	}
	if cfg.FromAddress == "" {
		return fmt.Errorf("MAIL_FROM_ADDRESS is required in production")
	}
	if cfg.FromName == "" {
		return fmt.Errorf("MAIL_FROM_NAME is required in production")
	}
	if cfg.Port != 465 {
		return fmt.Errorf("SMTP_PORT must be 465 for aliyun_directmail")
	}
	return nil
}

func parseBoolEnv(name, raw string, fallback bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return fallback, nil
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", name)
	}
}

func parseDurationEnv(name, raw string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", name)
	}
	return parsed, nil
}

func parseIntEnv(name, raw string, fallback int) (int, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func parseFloatEnv(name, raw string, fallback float64) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", name)
	}
	return parsed, nil
}

func validateSentryDSN(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User == nil {
		return fmt.Errorf("SENTRY_DSN must be an absolute HTTP(S) Sentry DSN")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("SENTRY_DSN must use http or https")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("SENTRY_DSN must not include a query or fragment")
	}
	return nil
}

func parseAllowedOrigins(values ...string) []string {
	return parseCommaSeparated(values...)
}

func normalizeFrontendOrigin(value string, requireHTTPS bool) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Opaque != "" {
		return "", fmt.Errorf("FRONTEND_ORIGIN must be an absolute HTTP(S) origin")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("FRONTEND_ORIGIN must use http or https")
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return "", fmt.Errorf("FRONTEND_ORIGIN must use https in production")
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("FRONTEND_ORIGIN must not include credentials, path, query, or fragment")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = ""
	return parsed.String(), nil
}

func parseCommaSeparated(values ...string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func parseTurnstileHostnames(values ...string) []string {
	parsed := parseCommaSeparated(values...)
	result := make([]string, 0, len(parsed))
	seen := map[string]struct{}{}
	for _, value := range parsed {
		value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func validateTurnstileHostnames(values []string) error {
	for _, value := range values {
		if _, err := netip.ParseAddr(value); err == nil {
			continue
		}
		if len(value) > 253 || strings.ContainsAny(value, "/:?#[]@") || strings.HasPrefix(value, ".") {
			return fmt.Errorf("TURNSTILE_HOSTNAMES contains invalid hostname %q", value)
		}
		for _, label := range strings.Split(value, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return fmt.Errorf("TURNSTILE_HOSTNAMES contains invalid hostname %q", value)
			}
			for _, char := range label {
				if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
					return fmt.Errorf("TURNSTILE_HOSTNAMES contains invalid hostname %q", value)
				}
			}
		}
	}
	return nil
}

func validateTrustedProxies(values []string) error {
	for _, value := range values {
		if _, err := trustedProxyPrefix(value); err != nil {
			return fmt.Errorf("TRUSTED_PROXIES contains invalid IP or CIDR %q", value)
		}
	}
	return nil
}

func trustedProxyPrefix(value string) (netip.Prefix, error) {
	value = strings.Trim(strings.TrimSpace(value), "[]")
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, err
	}
	addr = addr.Unmap()
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}
