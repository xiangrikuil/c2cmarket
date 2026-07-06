package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

type Config struct {
	Port                   string
	AppEnv                 string
	DatabaseURL            string
	EnableDevAuth          bool
	AllowedOrigins         []string
	OAuthProviderMode      string
	OAuthClientID          string
	OAuthClientSecret      string
	OAuthAuthorizeURL      string
	OAuthTokenURL          string
	OAuthUserInfoURL       string
	OAuthRedirectURL       string
	OAuthScopes            string
	ContactEncryptionKey   string
	ContactFingerprintKey  string
	ContactKeyVersion      string
	BootstrapAdminUsername string
	BootstrapAdminPassword string
	EmailProvider          string
	SMTP                   SMTPConfig
}

type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

const (
	localContactEncryptionKey  = "c2cmarket-local-contact-encryption-key-v1"
	localContactFingerprintKey = "c2cmarket-local-contact-fingerprint-key-v1"
	localContactKeyVersion     = "local-dev-v1"
)

func Load() (Config, error) {
	cfg := Config{
		Port:                   strings.TrimSpace(os.Getenv("PORT")),
		AppEnv:                 strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))),
		DatabaseURL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AllowedOrigins:         parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS"), os.Getenv("FRONTEND_ORIGIN")),
		OAuthProviderMode:      strings.ToLower(strings.TrimSpace(os.Getenv("OAUTH_PROVIDER_MODE"))),
		OAuthClientID:          strings.TrimSpace(os.Getenv("OAUTH_CLIENT_ID")),
		OAuthClientSecret:      strings.TrimSpace(os.Getenv("OAUTH_CLIENT_SECRET")),
		OAuthAuthorizeURL:      strings.TrimSpace(os.Getenv("OAUTH_AUTHORIZE_URL")),
		OAuthTokenURL:          strings.TrimSpace(os.Getenv("OAUTH_TOKEN_URL")),
		OAuthUserInfoURL:       strings.TrimSpace(os.Getenv("OAUTH_USERINFO_URL")),
		OAuthRedirectURL:       strings.TrimSpace(os.Getenv("OAUTH_REDIRECT_URL")),
		OAuthScopes:            strings.TrimSpace(os.Getenv("OAUTH_SCOPES")),
		ContactEncryptionKey:   strings.TrimSpace(os.Getenv("CONTACT_ENCRYPTION_KEY")),
		ContactFingerprintKey:  strings.TrimSpace(os.Getenv("CONTACT_FINGERPRINT_KEY")),
		ContactKeyVersion:      strings.TrimSpace(os.Getenv("CONTACT_KEY_VERSION")),
		BootstrapAdminUsername: strings.TrimSpace(os.Getenv("C2C_BOOTSTRAP_ADMIN_USERNAME")),
		BootstrapAdminPassword: strings.TrimSpace(os.Getenv("C2C_BOOTSTRAP_ADMIN_PASSWORD")),
		EmailProvider:          strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER"))),
		SMTP: SMTPConfig{
			Host:        strings.TrimSpace(os.Getenv("SMTP_HOST")),
			Username:    strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
			Password:    strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
			FromAddress: strings.TrimSpace(os.Getenv("MAIL_FROM_ADDRESS")),
			FromName:    strings.TrimSpace(os.Getenv("MAIL_FROM_NAME")),
		},
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = EnvDevelopment
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
		if len(cfg.AllowedOrigins) == 0 {
			return Config{}, fmt.Errorf("ALLOWED_ORIGINS or FRONTEND_ORIGIN is required in production")
		}
		if cfg.ContactEncryptionKey == "" {
			return Config{}, fmt.Errorf("CONTACT_ENCRYPTION_KEY is required in production")
		}
		if cfg.ContactFingerprintKey == "" {
			return Config{}, fmt.Errorf("CONTACT_FINGERPRINT_KEY is required in production")
		}
		if cfg.ContactKeyVersion == "" {
			return Config{}, fmt.Errorf("CONTACT_KEY_VERSION is required in production")
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

	if cfg.ContactEncryptionKey == "" {
		cfg.ContactEncryptionKey = localContactEncryptionKey
	}
	if cfg.ContactFingerprintKey == "" {
		cfg.ContactFingerprintKey = localContactFingerprintKey
	}
	if cfg.ContactKeyVersion == "" {
		cfg.ContactKeyVersion = localContactKeyVersion
	}

	return cfg, nil
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

func parseAllowedOrigins(values ...string) []string {
	seen := map[string]struct{}{}
	origins := []string{}
	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			origin := strings.TrimSpace(part)
			if origin == "" {
				continue
			}
			if _, ok := seen[origin]; ok {
				continue
			}
			seen[origin] = struct{}{}
			origins = append(origins, origin)
		}
	}
	return origins
}
