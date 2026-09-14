package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// openTestStore 在私有临时目录创建数据库，不接触仓库运行数据。
func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "db", "openjwc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return store
}

// testNotice 创建用于事务及检索验证的资讯。
func testNotice(t *testing.T, id string, title string) notice.Notice {
	t.Helper()
	item, err := notice.New(notice.CreateInput{ID: id, Title: title, PublishedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), DetailURL: "https://example.com/notice", IsPage: true, Content: "课程考试安排与奖学金申请说明", Attachments: []notice.Attachment{{Name: "附件", URL: "https://example.com/file"}}})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

// TestStoreRoundTripAndIndexMutation 验证资讯与附件原子更新及全文索引随删除清理。
func TestStoreRoundTripAndIndexMutation(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.Save(ctx, testNotice(t, "one", "课程考试安排")); err != nil {
		t.Fatal(err)
	}
	item, found, err := store.FindByID(ctx, "one")
	if err != nil || !found || len(item.Attachments()) != 1 {
		t.Fatalf("读取资讯: %v %v", found, err)
	}
	for _, query := range []string{"课程考试", "考试", `" OR 1=1`} {
		results, err := store.Search(ctx, query, 10)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if query == `" OR 1=1` {
			want = 0
		}
		if len(results) != want {
			t.Fatalf("检索 %q 返回 %d，预期 %d", query, len(results), want)
		}
	}
	if err := store.Save(ctx, testNotice(t, "one", "毕业典礼通知")); err != nil {
		t.Fatal(err)
	}
	results, err := store.Search(ctx, "毕业典礼", 10)
	if err != nil || len(results) != 1 {
		t.Fatalf("更新索引: %v %v", results, err)
	}
	if deleted, err := store.DeleteNotice(ctx, "one"); err != nil || !deleted {
		t.Fatalf("删除: %v %v", deleted, err)
	}
	results, err = store.Search(ctx, "毕业典礼", 10)
	if err != nil || len(results) != 0 {
		t.Fatalf("索引未清理: %v %v", results, err)
	}
	if err := store.Check(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestBackupRefusesOverwriteAndRestoresSnapshot 验证在线备份可独立恢复且不能覆盖已有文件。
func TestBackupRefusesOverwriteAndRestoresSnapshot(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.Save(ctx, testNotice(t, "one", "原始资讯")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "backup", "snapshot.db")
	if err := store.Backup(ctx, path); err != nil {
		t.Fatal(err)
	}
	if err := store.Backup(ctx, path); err == nil {
		t.Fatal("允许覆盖已有备份")
	}
	restored, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if _, found, err := restored.FindByID(ctx, "one"); err != nil || !found {
		t.Fatalf("恢复快照: %v %v", found, err)
	}
	if err := restored.Check(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestConcurrentReadsAndWrites 验证有界读池和单写连接不会丢失提交。
func TestConcurrentReadsAndWrites(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	var workers sync.WaitGroup
	for index := 0; index < 32; index++ {
		item := testNotice(t, time.Unix(int64(index), 0).Format(time.RFC3339), "并发课程资讯")
		workers.Go(func() {
			if err := store.Save(ctx, item); err != nil {
				t.Error(err)
				return
			}
			if _, _, err := store.List(ctx, notice.Filter{Limit: 10}); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	_, total, err := store.List(ctx, notice.Filter{Limit: 100})
	if err != nil || total != 32 {
		t.Fatalf("并发写入: total=%d err=%v", total, err)
	}
}

// TestRejectsCorruptDatabase 验证损坏数据不会被自动重建覆盖。
func TestRejectsCorruptDatabase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "db")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "broken.db")
	if err := os.WriteFile(path, []byte("not a database"), 0600); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(context.Background(), path); err == nil {
		store.Close()
		t.Fatal("损坏数据库被接受")
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "not a database" {
		t.Fatal("损坏数据被覆盖")
	}
}
