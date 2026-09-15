package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
)

// SaveSource 保存来源进度，失败或取消不能覆盖最近一次成功时间。
func (store *Store) SaveSource(ctx context.Context, source crawl.Source) error {
	if !crawl.ValidSourceName(source.Name) {
		return fmt.Errorf("爬虫来源无效")
	}
	if source.Scanned < 0 || source.Saved < 0 || source.Skipped < 0 || source.Failed < 0 || len(source.Detail) > 2048 {
		return fmt.Errorf("爬虫统计范围无效")
	}
	switch source.State {
	case crawl.Running, crawl.Completed, crawl.Failed, crawl.Canceled:
	default:
		return fmt.Errorf("爬虫状态无效")
	}
	success := ""
	if source.State == crawl.Completed {
		success = source.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	_, err := store.writer.ExecContext(ctx, `INSERT INTO crawl_sources(name,state,scanned,saved,skipped,failed,started_at,finished_at,last_success,detail)
 VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(name) DO UPDATE SET state=excluded.state,scanned=excluded.scanned,saved=excluded.saved,
 skipped=excluded.skipped,failed=excluded.failed,started_at=excluded.started_at,finished_at=excluded.finished_at,detail=excluded.detail,
 last_success=CASE WHEN excluded.last_success<>'' THEN excluded.last_success ELSE crawl_sources.last_success END`, source.Name, source.State, source.Scanned, source.Saved, source.Skipped, source.Failed, source.StartedAt.UTC().Format(time.RFC3339Nano), source.FinishedAt.UTC().Format(time.RFC3339Nano), success, source.Detail)
	return err
}

// CrawlSources 返回持久化来源状态，使重启后仍可查看上次执行结果。
func (store *Store) CrawlSources(ctx context.Context) ([]crawl.Source, error) {
	rows, err := store.reader.QueryContext(ctx, "SELECT name,state,scanned,saved,skipped,failed,started_at,finished_at,last_success,detail FROM crawl_sources ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]crawl.Source, 0)
	for rows.Next() {
		var source crawl.Source
		var started, finished, success string
		if err = rows.Scan(&source.Name, &source.State, &source.Scanned, &source.Saved, &source.Skipped, &source.Failed, &started, &finished, &success, &source.Detail); err != nil {
			return nil, err
		}
		for _, entry := range []struct {
			value  string
			target *time.Time
		}{{value: started, target: &source.StartedAt}, {value: finished, target: &source.FinishedAt}, {value: success, target: &source.LastSuccess}} {
			if entry.value != "" {
				*entry.target, err = time.Parse(time.RFC3339Nano, entry.value)
				if err != nil {
					return nil, err
				}
			}
		}
		result = append(result, source)
	}
	return result, rows.Err()
}

// InterruptCrawls 将上次进程遗留的运行状态标记中断，而不是假装仍在执行。
func (store *Store) InterruptCrawls(ctx context.Context) error {
	_, err := store.writer.ExecContext(ctx, "UPDATE crawl_sources SET state='canceled',finished_at=?,detail='服务进程中断，任务未完成' WHERE state='running'", now())
	return err
}

// CrawlerConfigs 返回所有已配置爬虫及其最近成功时间，供调度与界面读取。
func (store *Store) CrawlerConfigs(ctx context.Context) ([]crawl.Config, error) {
	rows, err := store.reader.QueryContext(ctx, "SELECT c.name,c.enabled,c.interval_minutes,coalesce(s.last_success,'') FROM crawler_config c LEFT JOIN crawl_sources s ON s.name=c.name ORDER BY c.name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]crawl.Config, 0)
	for rows.Next() {
		var config crawl.Config
		var enabled int
		var success string
		if err = rows.Scan(&config.Name, &enabled, &config.IntervalMinutes, &success); err != nil {
			return nil, err
		}
		config.Enabled = enabled != 0
		if success != "" {
			config.LastSuccess, err = time.Parse(time.RFC3339Nano, success)
			if err != nil {
				return nil, err
			}
		}
		result = append(result, config)
	}
	return result, rows.Err()
}

// SeedCrawlerConfigs 仅补齐缺失的爬虫配置，不覆盖管理员已有的启停与间隔编辑。
func (store *Store) SeedCrawlerConfigs(ctx context.Context, specs []crawl.Config) error {
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, spec := range specs {
		if !crawl.ValidSourceName(spec.Name) || spec.IntervalMinutes < 5 || spec.IntervalMinutes > 10080 {
			return fmt.Errorf("爬虫配置 %s 无效", spec.Name)
		}
		enabled := 0
		if spec.Enabled {
			enabled = 1
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO crawler_config(name,enabled,interval_minutes,updated_at) VALUES(?,?,?,?) ON CONFLICT(name) DO NOTHING", spec.Name, enabled, spec.IntervalMinutes, now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
