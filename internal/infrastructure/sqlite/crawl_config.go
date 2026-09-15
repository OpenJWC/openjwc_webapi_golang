package sqlite

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// crawlerConfigsTable 生成爬虫配置页表格，最近成功时间为空表示尚未成功运行。
func (store *Store) crawlerConfigsTable(ctx context.Context) (admin.Response, error) {
	configs, err := store.CrawlerConfigs(ctx)
	if err != nil {
		return admin.Response{}, err
	}
	columns := []string{"名称", "启用", "间隔(分钟)", "最近成功"}
	rows := make([][]string, 0, len(configs))
	for _, config := range configs {
		success := ""
		if !config.LastSuccess.IsZero() {
			success = config.LastSuccess.UTC().Format(time.RFC3339)
		}
		rows = append(rows, []string{config.Name, strconv.FormatBool(config.Enabled), strconv.Itoa(config.IntervalMinutes), success})
	}
	return admin.Response{Columns: columns, Rows: rows}, nil
}

// setCrawlerConfig 校验后事务更新单个爬虫的启停与间隔并记录审计。
func (store *Store) setCrawlerConfig(ctx context.Context, request admin.Request) (admin.Response, error) {
	name := strings.TrimSpace(request.ID)
	if !crawl.ValidSourceName(name) {
		return admin.Response{}, fmt.Errorf("爬虫名称无效")
	}
	enabled, err := strconv.ParseBool(request.Values["enabled"])
	if err != nil {
		return admin.Response{}, fmt.Errorf("启用状态必须为 true 或 false")
	}
	interval, err := strconv.Atoi(request.Values["interval_minutes"])
	if err != nil || interval < 5 || interval > 10080 {
		return admin.Response{}, fmt.Errorf("自动间隔必须为 5～10080 分钟")
	}
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	value := 0
	if enabled {
		value = 1
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO crawler_config(name,enabled,interval_minutes,updated_at) VALUES(?,?,?,?) ON CONFLICT(name) DO UPDATE SET enabled=excluded.enabled,interval_minutes=excluded.interval_minutes,updated_at=excluded.updated_at", name, value, interval, now()); err != nil {
		return admin.Response{}, err
	}
	return commitAdmin(ctx, tx, request)
}
