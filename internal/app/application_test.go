package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
)

// TestDataDirectoryCannotHaveTwoServiceOwners 验证进程锁阻止两个服务同时拥有同一数据目录。
func TestDataDirectoryCannotHaveTwoServiceOwners(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	first, err := lockDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if second, err := lockDirectory(dir); err == nil {
		second.Close()
		t.Fatal("重复服务未被拒绝")
	}
}

// TestCanceledStartupReleasesResources 验证取消后服务退出且进程锁可再次取得。
func TestCanceledStartupReleasesResources(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "s")
	cfg := config.Config{Server: config.ServerConfig{Address: "127.0.0.1:0"}, DataDir: dir, DatabasePath: filepath.Join(dir, "state.db"), SocketPath: filepath.Join(dir, "admin.sock")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = Run(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	lock, err := lockDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
}

// TestServerDefaultsBoundNetworkResources 验证慢连接与空闲连接不会无限占用资源。
func TestServerDefaultsBoundNetworkResources(t *testing.T) {
	server := newServer(context.Background(), nil)
	if server.ReadHeaderTimeout != 5*time.Second || server.ReadTimeout == 0 || server.IdleTimeout == 0 || server.MaxHeaderBytes == 0 {
		t.Fatal("网络资源限制缺失")
	}
}
