package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/config"
	core "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/module/navigationbadge"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/realtime"
	"c2c-market/backend/internal/server"
	"c2c-market/backend/internal/store/postgres"
)

type App struct {
	Config           config.Config
	Store            *postgres.Store
	Service          *core.Service
	NavigationBadges *navigationbadge.Service
	RealtimeHub      *realtime.Hub
	RealtimeListener *realtime.PostgresListener
	Handler          http.Handler
	shutdownOnce     sync.Once
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	var store *postgres.Store
	if cfg.DatabaseURL != "" {
		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		connectedStore, err := postgres.ConnectWithContactCrypto(connectCtx, cfg.DatabaseURL, postgres.ContactCryptoConfig{
			EncryptionKey:         cfg.ContactEncryptionKey,
			FingerprintKey:        cfg.ContactFingerprintKey,
			EncryptionKeyVersion:  cfg.ContactKeyVersion,
			FingerprintKeyVersion: cfg.ContactKeyVersion,
		})
		if err != nil {
			return nil, err
		}
		store = connectedStore
		log.Printf("PostgreSQL 已连接")
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
		if appErr := store.CleanupExpiredIdempotency(cleanupCtx, time.Now().Add(-24*time.Hour)); appErr != nil {
			cleanupCancel()
			store.Close()
			return nil, fmt.Errorf("清理过期幂等记录失败: %w", appErr)
		}
		cleanupCancel()
	} else {
		log.Printf("未配置 DATABASE_URL，当前仅启用内存运行切片")
	}

	emailSender, err := buildEmailSender(cfg)
	if err != nil {
		if store != nil {
			store.Close()
		}
		return nil, err
	}
	service := core.NewServiceWithRepositoriesAndEmailSender(core.Repositories{}, emailSender)
	if store != nil {
		service = core.NewServiceWithRepositoriesAndEmailSender(core.Repositories{
			Auth:              store,
			Idempotency:       store,
			OfficialPrice:     store,
			Catalog:           store,
			APIService:        store,
			APIPurchaseIntent: store,
			APIOrder:          store,
			Announcement:      store,
			Notification:      store,
			Carpool:           store,
			Contact:           store,
			Profile:           store,
			Demand:            store,
			Feedback:          store,
			Favorite:          store,
			Review:            store,
			Search:            store,
			Report:            store,
			ModelAudit:        store,
		}, emailSender)
	}
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
			log.Printf("管理员 bootstrap 已完成 username=%s", result.User.Username)
		} else {
			log.Printf("管理员 bootstrap 已跳过，已有管理员密码凭证 username=%s", username)
		}
	}
	navigationBadges := navigationbadge.NewService(store, time.Now)
	realtimeHub := realtime.NewHub()
	var realtimeListener *realtime.PostgresListener
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
	}

	handler := server.NewServer(service, server.ServerOptions{
		EnableDevAuth:      cfg.EnableDevAuth,
		ReadinessChecker:   store,
		NavigationBadges:   navigationBadges,
		RealtimeHub:        realtimeHub,
		AppEnv:             cfg.AppEnv,
		AllowedOrigins:     cfg.AllowedOrigins,
		TrustXForwardedFor: cfg.TrustXForwardedFor,
		TrustedProxies:     cfg.TrustedProxies,
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
		Handler:          handler,
	}, nil
}

func buildEmailSender(cfg config.Config) (profile.EmailSender, error) {
	switch cfg.EmailProvider {
	case "", "development":
		return profile.NewDevelopmentEmailSender(), nil
	case "aliyun_directmail":
		return profile.NewSMTPEmailSender(profile.SMTPConfig{
			Host:        cfg.SMTP.Host,
			Port:        cfg.SMTP.Port,
			Username:    cfg.SMTP.Username,
			Password:    cfg.SMTP.Password,
			FromAddress: cfg.SMTP.FromAddress,
			FromName:    cfg.SMTP.FromName,
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
		if a.RealtimeListener != nil {
			a.RealtimeListener.Close()
		}
		if a.RealtimeHub != nil {
			a.RealtimeHub.Close()
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
