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
