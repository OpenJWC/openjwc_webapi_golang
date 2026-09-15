package schedule

import (
	"context"
	"log/slog"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// Configs 提供每个爬虫的启停、间隔与最近成功时间。
type Configs interface {
	CrawlerConfigs(context.Context) ([]crawl.Config, error)
}

// Crawler 定义按名称选择运行的有界可取消爬虫入口。
type Crawler interface {
	Run(context.Context, []string) (int, error)
}

// Digest 定义可重试且幂等的日报生成入口。
type Digest interface {
	Generate(context.Context, string) error
}

// Scheduler 的运行状态由单个后台循环独占。
type Scheduler struct {
	settings         setting.Reader
	configs          Configs
	crawler          Crawler
	digest           Digest
	logger           *slog.Logger
	lastCrawlAttempt time.Time
	lastDailyAttempt time.Time
}

// New 组装调度依赖，不在构造函数中启动 goroutine。
func New(settings setting.Reader, configs Configs, crawler Crawler, digest Digest, logger *slog.Logger) *Scheduler {
	return &Scheduler{settings: settings, configs: configs, crawler: crawler, digest: digest, logger: logger}
}

// Run 按分钟检查计划，取消后结束且不遗留后台任务。
func (scheduler *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		scheduler.tick(ctx, time.Now())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// tick 按每爬虫间隔调度，日报按本地前一日生成，失败不忙循环重试。
func (scheduler *Scheduler) tick(ctx context.Context, current time.Time) {
	values, err := scheduler.settings.Settings(ctx)
	if err != nil {
		scheduler.logger.Error("读取调度设置失败", "error", err)
		return
	}
	if values["crawler_enabled"] == "true" && current.Sub(scheduler.lastCrawlAttempt) >= 5*time.Minute {
		due := scheduler.dueCrawlers(ctx, current)
		if len(due) > 0 {
			scheduler.lastCrawlAttempt = current
			job, cancel := context.WithTimeout(ctx, 10*time.Minute)
			count, err := scheduler.crawler.Run(job, due)
			cancel()
			if err != nil {
				scheduler.logger.Error("爬虫计划执行失败", "imported", count, "error", err)
			} else {
				scheduler.logger.Info("爬虫计划完成", "imported", count)
			}
		}
	}
	if values["daily_enabled"] != "true" || current.Sub(scheduler.lastDailyAttempt) < 5*time.Minute {
		return
	}
	location, err := time.LoadLocation(values["timezone"])
	if err != nil {
		scheduler.logger.Error("日报时区配置无效")
		return
	}
	local := current.In(location)
	if local.Format("15:04") < values["daily_time"] {
		return
	}
	day := local.AddDate(0, 0, -1)
	scheduler.lastDailyAttempt = current
	job, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := scheduler.digest.Generate(job, day.Format("2006-01-02")); err != nil {
		scheduler.logger.Error("日报计划执行失败", "date", day.Format("2006-01-02"), "error", err)
	}
}

// dueCrawlers 返回启用且距上次成功达到间隔的爬虫名称。
func (scheduler *Scheduler) dueCrawlers(ctx context.Context, current time.Time) []string {
	configs, err := scheduler.configs.CrawlerConfigs(ctx)
	if err != nil {
		scheduler.logger.Error("读取爬虫配置失败", "error", err)
		return nil
	}
	due := make([]string, 0, len(configs))
	for _, config := range configs {
		if !config.Enabled {
			continue
		}
		interval := time.Duration(config.IntervalMinutes) * time.Minute
		if current.Sub(config.LastSuccess) >= interval {
			due = append(due, config.Name)
		}
	}
	return due
}
