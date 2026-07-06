package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"c2c-market/backend/internal/app"
	"c2c-market/backend/internal/config"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("后端服务退出失败: %v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("配置无效: %w", err)
	}

	application, err := app.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("初始化后端应用失败: %w", err)
	}
	defer application.Close()

	addr := ":" + cfg.Port
	log.Printf("后端服务启动，监听地址 %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           application.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return listenAndShutdown(ctx, server, 15*time.Second)
}

func listenAndShutdown(ctx context.Context, server *http.Server, shutdownTimeout time.Duration) error {
	runCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			log.Printf("后端服务已正常关闭")
			return nil
		}
		return fmt.Errorf("监听失败: %w", err)
	case <-runCtx.Done():
		log.Printf("收到关闭信号，开始优雅关闭")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("优雅关闭失败，强制关闭: %v", err)
		_ = server.Close()
		return fmt.Errorf("优雅关闭失败: %w", err)
	}
	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("关闭期间监听失败: %w", err)
	}
	log.Printf("后端服务已正常关闭")
	return nil
}
