package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// BenchmarkNoticePagesParallel 测量真实 SQLite 上一千条资讯的并发分页读取。
func BenchmarkNoticePagesParallel(benchmark *testing.B) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(benchmark.TempDir(), "db", "bench.db"))
	if err != nil {
		benchmark.Fatal(err)
	}
	defer store.Close()
	for index := 0; index < 1000; index++ {
		item, err := notice.New(notice.CreateInput{ID: fmt.Sprintf("notice-%04d", index), Title: "课程与考试通知", PublishedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), DetailURL: "https://example.com/notice", Content: "请查看课程安排和考试时间。"})
		if err != nil {
			benchmark.Fatal(err)
		}
		if err = store.Save(ctx, item); err != nil {
			benchmark.Fatal(err)
		}
	}
	benchmark.ResetTimer()
	benchmark.RunParallel(func(worker *testing.PB) {
		for worker.Next() {
			items, total, err := store.List(ctx, notice.Filter{Limit: 20})
			if err != nil || len(items) != 20 || total != 1000 {
				benchmark.Errorf("分页失败: %d %d %v", len(items), total, err)
				return
			}
		}
	})
}
