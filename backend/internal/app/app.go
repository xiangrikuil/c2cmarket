package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/apihealthrunner"
	"c2c-market/backend/internal/config"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/apimodeltest"
	core "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/module/evidence"
	"c2c-market/backend/internal/module/navigationbadge"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/observability"
	"c2c-market/backend/internal/platform/outboundhttp"
	"c2c-market/backend/internal/platform/turnstile"
	"c2c-market/backend/internal/realtime"
	"c2c-market/backend/internal/server"
	"c2c-market/backend/internal/store/postgres"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type App struct {
	Config           config.Config
	Store            *postgres.Store
	Service          *core.Service
	NavigationBadges *navigationbadge.Service
	RealtimeHub      *realtime.Hub
	RealtimeListener *realtime.PostgresListener
	Maintenance      *maintenance.Runner
	APIHealth        *apihealth.Service
	APIModelTester   *apimodeltest.Service
	APIHealthRunner  *apihealthrunner.Runner
	Evidence         *evidence.Service
	EvidenceCleanup  *evidence.CleanupRunner
	RateLimiter      *middleware.RateLimiter
	Metrics          *observability.Metrics
	Handler          http.Handler
	shutdownOnce     sync.Once
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	var turnstileVerifier turnstile.Verifier
	if strings.TrimSpace(cfg.TurnstileSecret) != "" {
		configuredVerifier, err := turnstile.New(cfg.TurnstileSecret, cfg.TurnstileHostnames, turnstile.Options{})
		if err != nil {
			return nil, fmt.Errorf("初始化 Turnstile 验证器失败: %w", err)
		}
		turnstileVerifier = configuredVerifier
	}
	modelAuditPolicy, err := outboundhttp.NewPolicy(cfg.ModelAuditAllowedHosts)
	if err != nil {
		return nil, fmt.Errorf("初始化模型审计安全出站策略失败: %w", err)
	}
	var store *postgres.Store
	if cfg.DatabaseURL != "" {
		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		connectedStore, err := postgres.ConnectWithContactCryptoAndOptions(
			connectCtx,
			cfg.DatabaseURL,
			postgres.ContactCryptoConfig{
				EncryptionKey:         cfg.ContactEncryptionKey,
				FingerprintKey:        cfg.ContactFingerprintKey,
				EncryptionKeyVersion:  cfg.ContactKeyVersion,
				FingerprintKeyVersion: cfg.ContactKeyVersion,
				EncryptionKeys:        cfg.ContactEncryptionKeys,
				FingerprintKeys:       cfg.ContactFingerprintKeys,
			},
			cfg.Database,
		)
		if err != nil {
			return nil, err
		}
		store = connectedStore
		log.Printf("PostgreSQL 已连接")
	} else {
		log.Printf("未配置 DATABASE_URL，当前仅启用内存运行切片")
	}
	evidenceService, err := buildEvidenceService(ctx, cfg.Evidence, store)
	if err != nil {
		if store != nil {
			store.Close()
		}
		return nil, err
	}

	emailSender, err := buildEmailSender(cfg)
	if err != nil {
		if store != nil {
			store.Close()
		}
		return nil, err
	}
	serviceOptions := core.ServiceOptions{EmailVerificationPepper: cfg.EmailVerificationPepper}
	service := core.NewServiceWithRepositoriesEmailSenderAndOptions(core.Repositories{}, emailSender, serviceOptions)
	if store != nil {
		service = core.NewServiceWithRepositoriesEmailSenderAndOptions(core.RepositoriesFromPersistence(store), emailSender, serviceOptions)
	}
	service.ConfigureModelAuditOutbound(modelAuditPolicy)
	service.ConfigureAPIOrderDeliveryVerifier(cfg.APIHealth.Timeout)
	if strings.TrimSpace(cfg.BootstrapAdminPassword) != "" {
		result, appErr := service.BootstrapAdmin(ctx, core.BootstrapAdminInput{
			Username: cfg.BootstrapAdminUsername,
			Password: cfg.BootstrapAdminPassword,
		})
		if appErr != nil {
			if store != nil {
				store.Close()
			}
			return nil, fmt.Errorf("bootstrap admin failed: %w", appErr)
		}
		username := strings.TrimSpace(cfg.BootstrapAdminUsername)
		if username == "" {
			username = "admin"
		}
		if result.Created {
			log.Printf("管理员 bootstrap 已完成 user_id=%s username=%s", result.User.ID, result.User.Username)
		} else {
			log.Printf("管理员 bootstrap 来源已确认 user_id=%s username=%s", result.User.ID, username)
		}
	}
	navigationBadges := navigationbadge.NewService(store, time.Now)
	realtimeHub := realtime.NewHub()
	var realtimeListener *realtime.PostgresListener
	var maintenanceRunner *maintenance.Runner
	var apiHealthService *apihealth.Service
	var apiHealthRunner *apihealthrunner.Runner
	var evidenceCleanup *evidence.CleanupRunner
	if cfg.DatabaseURL != "" {
		realtimeListener, err = realtime.NewPostgresListener(cfg.DatabaseURL, realtimeHub, log.Default())
		if err != nil {
			realtimeHub.Close()
			if store != nil {
				store.Close()
			}
			return nil, fmt.Errorf("初始化 PostgreSQL 实时监听失败: %w", err)
		}
		if err := realtimeListener.Start(ctx); err != nil {
			realtimeListener.Close()
			realtimeHub.Close()
			if store != nil {
				store.Close()
			}
			return nil, fmt.Errorf("启动 PostgreSQL 实时监听失败: %w", err)
		}
		maintenanceRunner, err = maintenance.NewRunner(store, maintenance.Config{
			Interval:  cfg.Maintenance.Interval,
			BatchSize: cfg.Maintenance.BatchSize,
			Policy: maintenance.Policy{
				SessionRetention:               cfg.Maintenance.SessionRetention,
				EmailVerificationRetention:     cfg.Maintenance.EmailVerificationRetention,
				ReadNotificationRetention:      cfg.Maintenance.ReadNotificationRetention,
				UnreadNotificationRetention:    cfg.Maintenance.UnreadNotificationRetention,
				DomainEventRetention:           cfg.Maintenance.DomainEventRetention,
				APIDeliveryCredentialRetention: cfg.Maintenance.APIDeliveryCredentialRetention,
				APIProbeSampleRetention:        cfg.APIHealth.Retention,
			},
		}, time.Now, log.Default())
		if err != nil {
			realtimeListener.Close()
			realtimeListener.Wait()
			realtimeHub.Close()
			store.Close()
			return nil, fmt.Errorf("初始化数据维护任务失败: %w", err)
		}
		maintenanceRunner.Start()
		if evidenceService != nil {
			evidenceCleanup = evidence.NewCleanupRunner(evidenceService, cfg.Maintenance.Interval, cfg.Maintenance.BatchSize)
			evidenceCleanup.Start()
		}
		apiHealthProber := apihealth.NewOpenAIRealModelProber(cfg.APIHealth.Timeout)
		apiHealthService = apihealth.NewService(store, apiHealthProber, time.Now)
		apiHealthRunner = apihealthrunner.New(
			store,
			apiHealthProber,
			apihealthrunner.Options{
				Enabled: cfg.APIHealth.RunnerEnabled, ScanInterval: cfg.APIHealth.ScanInterval,
				Timeout: cfg.APIHealth.Timeout, Concurrency: cfg.APIHealth.Concurrency,
				BatchSize: cfg.APIHealth.BatchSize,
			},
			time.Now,
			log.Default(),
		)
		apiHealthService.SetRunnerStatusProvider(apiHealthRunner)
		apiHealthRunner.Start(ctx)
	}
	apiModelTesterService := apimodeltest.NewService(store, cfg.APIHealth.Timeout, time.Now)

	rateLimiter := middleware.NewRateLimiter(time.Minute)
	rateLimiter.Start(ctx)
	runtimeMetrics := observability.New(observability.Sources{
		Database:         store,
		RateLimiter:      rateLimiter,
		Maintenance:      maintenanceRunner,
		APIHealthRunner:  apiHealthRunner,
		OutboundPolicy:   modelAuditPolicy,
		RealtimeHub:      realtimeHub,
		RealtimeListener: realtimeListener,
		SlowQueryAfter:   cfg.DatabaseSlowQueryAfter,
	})
	service.ConfigurePasswordResetDeliveryRecorder(runtimeMetrics)
	handler := server.NewServer(service, server.ServerOptions{
		EnableDevAuth:      cfg.EnableDevAuth,
		ReadinessChecker:   store,
		APIHealth:          apiHealthService,
		AdminAPIHealth:     apiHealthService,
		APIModelTester:     apiModelTesterService,
		NavigationBadges:   navigationBadges,
		RealtimeHub:        realtimeHub,
		AppEnv:             cfg.AppEnv,
		FrontendOrigin:     cfg.FrontendOrigin,
		AllowedOrigins:     cfg.AllowedOrigins,
		TrustXForwardedFor: cfg.TrustXForwardedFor,
		TrustedProxies:     cfg.TrustedProxies,
		RateLimiter:        rateLimiter,
		Metrics:            runtimeMetrics,
		MetricsBearerToken: cfg.MetricsBearerToken,
		TurnstileVerifier:  turnstileVerifier,
		SentryEnabled:      cfg.Sentry.Enabled,
		Evidence:           evidenceService,
		OAuth: server.OAuthOptions{
			ProviderMode: cfg.OAuthProviderMode,
			ClientID:     cfg.OAuthClientID,
			ClientSecret: cfg.OAuthClientSecret,
			AuthorizeURL: cfg.OAuthAuthorizeURL,
			TokenURL:     cfg.OAuthTokenURL,
			UserInfoURL:  cfg.OAuthUserInfoURL,
			RedirectURL:  cfg.OAuthRedirectURL,
			Scopes:       cfg.OAuthScopes,
		},
	})

	return &App{
		Config:           cfg,
		Store:            store,
		Service:          service,
		NavigationBadges: navigationBadges,
		RealtimeHub:      realtimeHub,
		RealtimeListener: realtimeListener,
		Maintenance:      maintenanceRunner,
		APIHealth:        apiHealthService,
		APIModelTester:   apiModelTesterService,
		APIHealthRunner:  apiHealthRunner,
		Evidence:         evidenceService,
		EvidenceCleanup:  evidenceCleanup,
		RateLimiter:      rateLimiter,
		Metrics:          runtimeMetrics,
		Handler:          handler,
	}, nil
}

func buildEvidenceService(ctx context.Context, cfg config.EvidenceConfig, repo evidence.Repository) (*evidence.Service, error) {
	if repo == nil || (!cfg.Enabled && !cfg.StorageConfigured()) {
		return nil, nil
	}
	awsConfig, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithBaseEndpoint(cfg.Endpoint),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize evidence object storage configuration: %w", err)
	}
	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
	})
	objectStore, err := evidence.NewS3ObjectStore(client, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("initialize evidence object storage: %w", err)
	}
	return evidence.NewServiceWithUploadCapability(repo, objectStore, time.Now, cfg.Enabled), nil
}

func buildEmailSender(cfg config.Config) (profile.EmailSender, error) {
	switch cfg.EmailProvider {
	case "", "development":
		return profile.NewDevelopmentEmailSender(), nil
	case "aliyun_directmail":
		return profile.NewSMTPEmailSender(profile.SMTPConfig{
			Host:           cfg.SMTP.Host,
			Port:           cfg.SMTP.Port,
			Username:       cfg.SMTP.Username,
			Password:       cfg.SMTP.Password,
			FromAddress:    cfg.SMTP.FromAddress,
			FromName:       cfg.SMTP.FromName,
			FrontendOrigin: cfg.FrontendOrigin,
		})
	default:
		return nil, fmt.Errorf("unsupported EMAIL_PROVIDER %q", cfg.EmailProvider)
	}
}

func (a *App) BeginShutdown() {
	if a == nil {
		return
	}
	a.shutdownOnce.Do(func() {
		if a.APIHealthRunner != nil {
			a.APIHealthRunner.Close()
		}
		if a.RealtimeListener != nil {
			a.RealtimeListener.Close()
		}
		if a.RealtimeHub != nil {
			a.RealtimeHub.Close()
		}
		if a.Maintenance != nil {
			a.Maintenance.Close()
		}
		if a.EvidenceCleanup != nil {
			a.EvidenceCleanup.Close()
		}
		if a.RateLimiter != nil {
			a.RateLimiter.Close()
		}
	})
}

func (a *App) Close() {
	if a == nil {
		return
	}
	a.BeginShutdown()
	if a.RealtimeListener != nil {
		a.RealtimeListener.Wait()
	}
	if a.Store != nil {
		a.Store.Close()
	}
}
