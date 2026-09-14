package schedule

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// Jobs 定义持久化调度进度的存储能力。
type Jobs interface {
	JobSuccess(context.Context, string) (time.Time, error)
}

// Crawler 定义有界可取消的爬虫入口。
type Crawler interface {
	Run(context.Context) (int, error)
}

// Digest 定义可重试且幂等的日报生成入口。
type Digest interface {
	Generate(context.Context, string) error
}

// Scheduler 的运行状态由单个后台循环独占。
type Scheduler struct {
	settings         setting.Reader
	jobs             Jobs
	crawler          Crawler
	digest           Digest
	logger           *slog.Logger
	lastCrawlAttempt time.Time
	lastDailyAttempt time.Time
}

// New 组装调度依赖，不在构造函数中启动 goroutine。
func New(settings setting.Reader, jobs Jobs, crawler Crawler, digest Digest, logger *slog.Logger) *Scheduler {
	return &Scheduler{settings: settings, jobs: jobs, crawler: crawler, digest: digest, logger: logger}
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

// tick 根据持久化成功时间及本轮冷却时间调度，不对失败任务忙循环重试。
func (scheduler *Scheduler) tick(ctx context.Context, current time.Time) {
	values, err := scheduler.settings.Settings(ctx)
	if err != nil {
		scheduler.logger.Error("读取调度设置失败", "error", err)
		return
	}
	if values["crawler_enabled"] == "true" && current.Sub(scheduler.lastCrawlAttempt) >= 5*time.Minute {
		lastSuccess, err := scheduler.jobs.JobSuccess(ctx, "crawler")
		minutes, _ := strconv.Atoi(values["crawler_interval_minutes"])
		if err != nil {
			scheduler.logger.Error("读取爬虫最近成功时间失败", "error", err)
		} else if current.Sub(lastSuccess) >= time.Duration(minutes)*time.Minute {
			scheduler.lastCrawlAttempt = current
			job, cancel := context.WithTimeout(ctx, 10*time.Minute)
			count, err := scheduler.crawler.Run(job)
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
