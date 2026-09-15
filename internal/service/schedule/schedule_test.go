package schedule

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// scheduleFixture 记录调度调用并提供确定性设置、配置与进度。
type scheduleFixture struct {
	days      []string
	runs      [][]string
	configs   []crawl.Config
	crawlerOn bool
}

// Settings 返回每天零点十分总结前一日的测试设置。
func (fixture *scheduleFixture) Settings(ctx context.Context) (map[string]string, error) {
	values := setting.Defaults()
	values["daily_enabled"] = "true"
	if fixture.crawlerOn {
		values["crawler_enabled"] = "true"
	}
	return values, nil
}

// CrawlerConfigs 返回测试提供的爬虫配置。
func (fixture *scheduleFixture) CrawlerConfigs(ctx context.Context) ([]crawl.Config, error) {
	return fixture.configs, nil
}

// Run 记录调度器选择的爬虫名称。
func (fixture *scheduleFixture) Run(ctx context.Context, names []string) (int, error) {
	fixture.runs = append(fixture.runs, append([]string(nil), names...))
	return 0, nil
}

// Generate 记录调度器要求生成的日期。
func (fixture *scheduleFixture) Generate(ctx context.Context, day string) error {
	fixture.days = append(fixture.days, day)
	return nil
}

// TestDailyScheduleUsesPreviousLocalCalendarDay 验证日报覆盖完整本地日且不会提前生成。
func TestDailyScheduleUsesPreviousLocalCalendarDay(t *testing.T) {
	fixture := &scheduleFixture{}
	scheduler := New(fixture, fixture, fixture, fixture, slog.New(slog.NewTextHandler(io.Discard, nil)))
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	scheduler.tick(context.Background(), time.Date(2026, 9, 2, 0, 9, 0, 0, zone))
	if len(fixture.days) != 0 {
		t.Fatal("计划时间前生成了日报")
	}
	scheduler.tick(context.Background(), time.Date(2026, 9, 2, 0, 11, 0, 0, zone))
	if len(fixture.days) != 1 || fixture.days[0] != "2026-09-01" {
		t.Fatalf("日报日期错误: %v", fixture.days)
	}
	scheduler.tick(context.Background(), time.Date(2026, 9, 2, 0, 12, 0, 0, zone))
	if len(fixture.days) != 1 {
		t.Fatal("冷却时间内重复调用")
	}
}

// TestCrawlerScheduleSelectsDueCrawlers 验证只触发启用且达到间隔的爬虫，并在冷却内去重。
func TestCrawlerScheduleSelectsDueCrawlers(t *testing.T) {
	now := time.Now()
	fixture := &scheduleFixture{crawlerOn: true, configs: []crawl.Config{
		{Name: "jwc", Enabled: true, IntervalMinutes: 60, LastSuccess: now.Add(-2 * time.Hour)},
		{Name: "cs", Enabled: true, IntervalMinutes: 480, LastSuccess: now},
		{Name: "ext", Enabled: false, IntervalMinutes: 60},
	}}
	scheduler := New(fixture, fixture, fixture, fixture, slog.New(slog.NewTextHandler(io.Discard, nil)))
	scheduler.tick(context.Background(), now)
	if len(fixture.runs) != 1 || len(fixture.runs[0]) != 1 || fixture.runs[0][0] != "jwc" {
		t.Fatalf("到期爬虫选择错误: %v", fixture.runs)
	}
	fixture.runs = nil
	scheduler.tick(context.Background(), now)
	if len(fixture.runs) != 0 {
		t.Fatalf("冷却内重复触发: %v", fixture.runs)
	}
}
