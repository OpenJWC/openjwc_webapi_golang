package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/app"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           app.New(logger).Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go serve(server, logger)
	awaitShutdown(server, logger)
}

func serve(server *http.Server, logger *slog.Logger) {
	logger.Info("HTTP 服务已启动", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP 服务异常退出", "error", err)
		os.Exit(1)
	}
}

func awaitShutdown(server *http.Server, logger *slog.Logger) {
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP 服务关闭超时", "error", err)
	}
}
