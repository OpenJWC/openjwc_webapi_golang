package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
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

// TestCrawlerConfigSeedSaveAndRead 验证爬虫配置只补缺、可更新并可读回最近成功时间。
func TestCrawlerConfigSeedSaveAndRead(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SeedCrawlerConfigs(ctx, []crawl.Config{
		{Name: "jwc", Enabled: true, IntervalMinutes: 480},
		{Name: "ext", Enabled: false, IntervalMinutes: 60},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SeedCrawlerConfigs(ctx, []crawl.Config{{Name: "jwc", Enabled: false, IntervalMinutes: 5}}); err != nil {
		t.Fatal(err)
	}
	configs, err := store.CrawlerConfigs(ctx)
	if err != nil || len(configs) != 2 {
		t.Fatalf("配置读取失败: %v %v", configs, err)
	}
	byName := map[string]crawl.Config{}
	for _, config := range configs {
		byName[config.Name] = config
	}
	if !byName["jwc"].Enabled || byName["jwc"].IntervalMinutes != 480 {
		t.Fatalf("种子不应覆盖已有配置: %+v", byName["jwc"])
	}
	response, err := store.setCrawlerConfig(ctx, admin.Request{Action: admin.ActionSetCrawlerConfig, ID: "ext", Values: map[string]string{"enabled": "true", "interval_minutes": "90"}})
	if err != nil || response.Text == "" {
		t.Fatalf("更新配置失败: %v %v", response, err)
	}
	configs, err = store.CrawlerConfigs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	updated := map[string]crawl.Config{}
	for _, config := range configs {
		updated[config.Name] = config
	}
	if !updated["ext"].Enabled || updated["ext"].IntervalMinutes != 90 {
		t.Fatalf("配置更新未生效: %v", configs)
	}
}
