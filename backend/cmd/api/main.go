package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"c2c-market/backend/internal/app"
	"c2c-market/backend/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置无效: %v", err)
	}

	application, err := app.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("初始化后端应用失败: %v", err)
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
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("后端服务退出: %v", err)
	}
}
