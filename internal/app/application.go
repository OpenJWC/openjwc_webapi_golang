package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/observability"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/crawler"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/crawljob"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/digest"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/schedule"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/control"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/httpapi"
)

// Run 组装所有依赖，拥有监听器、数据库与后台任务的完整生命周期。
func Run(parent context.Context, cfg config.Config, logger *slog.Logger) error {
	logger, logs := observability.NewLogger(logger)
	lock, err := lockDirectory(cfg.DataDir)
	if err != nil {
		return err
	}
	defer lock.Close()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	store, err := sqlite.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()
	chat := agent.New(store, store)
	if err := store.InterruptCrawls(ctx); err != nil {
		return err
	}
	spider := crawler.New(store, store)
	crawlRunners := []crawler.ObservedRunner{spider}
	for _, program := range cfg.CrawlerPrograms {
		crawlRunners = append(crawlRunners, crawler.NewExternal(crawler.Program{Name: program.Name, Path: program.Path, Args: program.Args}, store, store))
	}
	crawlTasks := crawljob.New(crawler.NewGroup(crawlRunners...), store)
	daily := digest.New(store, store, chat)
	scheduler := schedule.New(store, store, crawlTasks, daily, logger)
	publicListener, err := net.Listen("tcp", cfg.Server.Address)
	if err != nil {
		return fmt.Errorf("监听客户端 HTTP: %w", err)
	}
	defer publicListener.Close()
	localListener, err := control.Listen(cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("监听本地管理接口: %w", err)
	}
	defer localListener.Close()
	clientRouter := httpapi.NewClientRouter(logger, httpapi.ClientDependencies{
		Notices:     store,
		Identity:    store,
		Settings:    store,
		Chat:        chat,
		Events:      chat,
		Submissions: store,
		Keys:        store,
		Reports:     store,
	})
	management := admin.NewDispatcher(store, crawlTasks, daily)
	monitoredManagement := observability.NewMonitor(management, logs)
	public := newServer(ctx, clientRouter)
	local := newServer(ctx, control.Handler(monitoredManagement))
	var workers sync.WaitGroup
	results := make(chan error, 2)
	workers.Go(func() {
		if cfg.Server.TLSCert != "" {
			results <- public.ServeTLS(publicListener, cfg.Server.TLSCert, cfg.Server.TLSKey)
			return
		}
		results <- public.Serve(publicListener)
	})
	workers.Go(func() { results <- local.Serve(localListener) })
	workers.Go(func() { crawlTasks.Serve(ctx) })
	workers.Go(func() { scheduler.Run(ctx) })
	logger.Info("OpenJWC 服务已启动", "address", publicListener.Addr().String(), "admin_socket", cfg.SocketPath)
	select {
	case <-ctx.Done():
	case err = <-results:
	}
	cancel()
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if shutdownErr := public.Shutdown(shutdown); shutdownErr != nil {
		_ = public.Close()
		logger.Error("客户端 HTTP 关闭超时", "error", shutdownErr)
	}
	if shutdownErr := local.Shutdown(shutdown); shutdownErr != nil {
		_ = local.Close()
		logger.Error("管理接口关闭超时", "error", shutdownErr)
	}
	workers.Wait()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// newServer 为公开和本地监听器配置有界网络资源。
func newServer(ctx context.Context, handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      4 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		BaseContext:       func(listener net.Listener) context.Context { return ctx },
	}
}
