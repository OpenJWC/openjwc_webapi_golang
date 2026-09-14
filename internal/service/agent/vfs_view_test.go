package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// TestVFSViewsFilterByCategoryDateAndRecentWindow 验证虚拟分类、日期和近期窗口不会改变规范文件身份。
func TestVFSViewsFilterByCategoryDateAndRecentWindow(t *testing.T) {
	store := agentLibrary(t)
	ctx := context.Background()
	for _, entry := range []struct{ id, label, day string }{{"old", "教务", "2025-01-01"}, {"new", "教务", "2026-09-13"}, {"other", "活动", "2026-09-12"}} {
		date, err := time.Parse("2006-01-02", entry.day)
		if err != nil {
			t.Fatal(err)
		}
		item, err := notice.New(notice.CreateInput{ID: entry.id, Label: entry.label, Title: "考试安排", PublishedAt: date, DetailURL: "https://example.com/" + entry.id, Content: "考试规则"})
		if err != nil {
			t.Fatal(err)
		}
		if err = store.Save(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	vfs := NewVFS(store)
	vfs.clock = func() time.Time { return time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC) }
	for _, command := range []string{"ls /by-label/教务 --from 2026-09-01 --to 2026-09-30", "grep '考试' /by-date/2026/09 --label 教务", "ls /recent --label 教务"} {
		text, err := vfs.Execute(ctx, command)
		if err != nil || !strings.Contains(text, FilePath("new")) || strings.Contains(text, FilePath("old")) || strings.Contains(text, FilePath("other")) {
			t.Fatalf("分类或日期过滤错误: %s\n%s\n%v", command, text, err)
		}
	}
	if _, err := vfs.Execute(ctx, "ls /notices --sort random"); err == nil {
		t.Fatal("未知排序未拒绝")
	}
	metadata, err := vfs.Metadata(ctx, "Asia/Shanghai")
	if err != nil || !strings.Contains(metadata, "2026-09-14T08:00:00+08:00") || !strings.Contains(metadata, "尚无完整成功抓取记录") {
		t.Fatalf("检索时间不准确: %s %v", metadata, err)
	}
}

// TestVFSCanContinueLongUnicodeContent 验证长正文尾部可以续读且不会拆坏字符。
func TestVFSCanContinueLongUnicodeContent(t *testing.T) {
	store := agentLibrary(t)
	item, err := notice.New(notice.CreateInput{ID: "long", Title: "长文", PublishedAt: time.Now(), DetailURL: "https://example.com/long", Content: strings.Repeat("中", 18000) + "末尾证据"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	vfs := NewVFS(store)
	first, err := vfs.Execute(context.Background(), "cat "+FilePath("long"))
	if err != nil || !strings.Contains(first, "续读:") || strings.Contains(first, "末尾证据") {
		t.Fatal("第一页缺少准确续读提示")
	}
	second, err := vfs.Execute(context.Background(), "cat "+FilePath("long")+" 12000 12000")
	if err != nil || !strings.Contains(second, "末尾证据") {
		t.Fatalf("不能读取后半段: %v", err)
	}
}
