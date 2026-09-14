package app

import (
	"context"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/recovery"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/tui"
)

// Recover 在独占数据目录后组装离线恢复 TUI，不尝试打开损坏数据库。
func Recover(ctx context.Context, cfg config.Config) error {
	lock, err := lockDirectory(cfg.DataDir)
	if err != nil {
		return err
	}
	defer lock.Close()
	operations := recovery.New(cfg.DatabasePath, func(ctx context.Context, source string) (string, error) {
		return sqlite.Restore(ctx, source, cfg.DatabasePath)
	})
	return tui.RunRecovery(ctx, operations)
}
