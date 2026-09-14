package schedule

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// scheduleFixture 记录调度调用并提供确定性设置与进度。
type scheduleFixture struct{ days []string }

// Settings 返回每天零点十分总结前一日的测试设置。
func (fixture *scheduleFixture) Settings(ctx context.Context) (map[string]string, error) {
	values := setting.Defaults()
	values["daily_enabled"] = "true"
	return values, nil
}

// JobSuccess 表示测试环境尚未成功运行爬虫。
func (fixture *scheduleFixture) JobSuccess(ctx context.Context, name string) (time.Time, error) {
	return time.Time{}, nil
}

// MarkJobSuccess 接受测试调度进度更新。
func (fixture *scheduleFixture) MarkJobSuccess(ctx context.Context, name string) error { return nil }

// Run 返回空爬虫结果。
func (fixture *scheduleFixture) Run(ctx context.Context) (int, error) { return 0, nil }

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
