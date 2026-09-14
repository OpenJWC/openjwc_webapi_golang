package digest

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
)

// fakeAgent 为日报重试测试记录调用次数与故障状态。
type fakeAgent struct {
	calls int
	fail  bool
}

// Answer 返回可切换故障的确定性日报文本。
func (model *fakeAgent) Answer(ctx context.Context, request agent.Request) (string, error) {
	model.calls++
	if model.fail {
		return "", errors.New("模拟模型故障")
	}
	return "考试安排有用，请及时查看。来源 notice。", nil
}

// TestDailyReportRetriesFailureAndPublishesOnce 验证失败不会发布半成品且成功后不重复计费。
func TestDailyReportRetriesFailureAndPublishesOnce(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "db", "daily.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	item, err := notice.New(notice.CreateInput{ID: "notice", Title: "考试通知", PublishedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), DetailURL: "https://example.com/notice"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(ctx, item); err != nil {
		t.Fatal(err)
	}
	model := &fakeAgent{fail: true}
	service := New(store, store, model)
	if err = service.Generate(ctx, "2026-09-01"); err == nil {
		t.Fatal("模型故障未上报")
	}
	if _, found, err := store.DailyContent(ctx, "2026-09-01"); err != nil || found {
		t.Fatalf("半成品被发布: %v %v", found, err)
	}
	model.fail = false
	if err = service.Generate(ctx, "2026-09-01"); err != nil {
		t.Fatal(err)
	}
	if err = service.Generate(ctx, "2026-09-01"); err != nil {
		t.Fatal(err)
	}
	if model.calls != 2 {
		t.Fatalf("日报重复生成: %d", model.calls)
	}
	if _, found, err := store.DailyContent(ctx, "2026-09-01"); err != nil || !found {
		t.Fatalf("日报未发布: %v %v", found, err)
	}
}
