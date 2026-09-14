package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
)

// TestCrawlFailureAndRestartPreserveLastSuccess 验证失败与进程中断不会覆盖最近一次成功时间。
func TestCrawlFailureAndRestartPreserveLastSuccess(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	source := crawl.Source{Name: "jwc", State: crawl.Completed, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), Scanned: 1, Saved: 1}
	if err := store.SaveSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	source.State = crawl.Failed
	source.Failed = 1
	if err := store.SaveSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	source.State = crawl.Running
	if err := store.SaveSource(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := store.InterruptCrawls(ctx); err != nil {
		t.Fatal(err)
	}
	results, err := store.CrawlSources(ctx)
	if err != nil || len(results) != 1 {
		t.Fatalf("来源记录读取失败: %v", err)
	}
	if results[0].State != crawl.Canceled || !results[0].LastSuccess.Equal(source.FinishedAt) {
		t.Fatalf("成功时间或中断状态错误: %+v", results[0])
	}
}
