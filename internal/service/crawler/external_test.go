package crawler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// externalSettings 返回外部协议所需的有效抓取边界。
type externalSettings struct{}

// Settings 返回固定页数和回溯天数。
func (externalSettings) Settings(context.Context) (map[string]string, error) {
	values := setting.Defaults()
	values["crawler_max_pages"] = "3"
	values["crawler_days_gap"] = "30"
	return values, nil
}

// TestExternalProcessHelper 充当仅由当前测试二进制启动的协议程序。
func TestExternalProcessHelper(t *testing.T) {
	mode := helperMode()
	if mode == "" {
		return
	}
	var request protocolRequest
	if err := json.NewDecoder(bufio.NewReader(os.Stdin)).Decode(&request); err != nil || request.Version != 1 || request.Source != "college-a" {
		os.Exit(20)
	}
	fmt.Println(`{"version":1,"type":"progress","scanned":1}`)
	if mode == "missing" {
		os.Exit(0)
	}
	if mode == "block" {
		time.Sleep(time.Hour)
		return
	}
	fmt.Println(`{"version":1,"type":"notice","notice":{"label":"学院通知","title":"外部资讯","published_at":"2026-09-14T00:00:00Z","detail_url":"https://example.edu/notice/1","is_page":true,"content":"正文","attachments":[]}}`)
	if mode == "invalid" {
		fmt.Println(`{"version":1,"type":"completed","scanned":0}`)
		os.Exit(0)
	}
	if mode == "omit" {
		fmt.Println(`{"version":1,"type":"completed"}`)
		os.Exit(0)
	}
	fmt.Println(`{"version":1,"type":"completed","scanned":1}`)
	os.Exit(0)
}

// helperMode 读取测试分隔符后的模式，普通 go test 不进入子进程逻辑。
func helperMode() string {
	for index, argument := range os.Args {
		if argument == "--" && index+1 < len(os.Args) {
			return os.Args[index+1]
		}
	}
	return ""
}

// TestExternalCrawlerImportsValidatedNotice 验证子进程不接触数据库即可通过协议导入并报告进度。
func TestExternalCrawlerImportsValidatedNotice(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "db", "external.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	external := testExternal(t, "success", store)
	var snapshots []crawl.Source
	count, err := external.RunObserved(context.Background(), func(ctx context.Context, source crawl.Source) error {
		snapshots = append(snapshots, source)
		return store.SaveSource(ctx, source)
	})
	if err != nil || count != 1 || snapshots[len(snapshots)-1].State != crawl.Completed {
		t.Fatalf("外部爬虫失败: %d %+v %v", count, snapshots, err)
	}
	items, err := store.Search(context.Background(), "外部资讯", 10)
	if err != nil || len(items) != 1 || items[0].Content() != "正文" {
		t.Fatalf("外部资讯未导入: %+v %v", items, err)
	}
}

// TestExternalCrawlerTerminalMayOmitUnchangedCounters 验证终止事件无需机械重复未变化的累计字段。
func TestExternalCrawlerTerminalMayOmitUnchangedCounters(t *testing.T) {
	external := testExternal(t, "omit", discardRepository{})
	if count, err := external.RunObserved(context.Background(), nil); err != nil || count != 1 {
		t.Fatalf("省略未变化计数的终止事件失败: %d %v", count, err)
	}
}

// TestExternalCrawlerRejectsRegressingTerminalProgress 验证进度回退不会被伪装成完成。
func TestExternalCrawlerRejectsRegressingTerminalProgress(t *testing.T) {
	external := testExternal(t, "invalid", discardRepository{})
	if _, err := external.RunObserved(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "终止事件无效") {
		t.Fatalf("无效终止进度未拒绝: %v", err)
	}
}

// TestExternalCrawlerRequiresTerminalEvent 验证正常退出但缺少终止事件仍视为协议失败。
func TestExternalCrawlerRequiresTerminalEvent(t *testing.T) {
	external := testExternal(t, "missing", discardRepository{})
	if _, err := external.RunObserved(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "缺少终止事件") {
		t.Fatalf("缺失终止事件未拒绝: %v", err)
	}
}

// TestExternalCrawlerCancellationKillsProcess 验证任务取消会结束等待中的独立外部进程。
func TestExternalCrawlerCancellationKillsProcess(t *testing.T) {
	external := testExternal(t, "block", discardRepository{})
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := external.RunObserved(ctx, func(_ context.Context, source crawl.Source) error {
			if source.Scanned > 0 {
				select {
				case <-started:
				default:
					close(started)
				}
			}
			return nil
		})
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; err == nil {
		t.Fatal("取消未结束外部进程")
	}
}

// discardRepository 接受外部资讯但不持久化。
type discardRepository struct{}

// Save 接受已校验资讯。
func (discardRepository) Save(context.Context, notice.Notice) error { return nil }

// testExternal 构造以测试二进制作为外部程序的适配器。
func testExternal(t *testing.T, mode string, repository Repository) *External {
	t.Helper()
	return NewExternal(Program{Name: "college-a", Path: os.Args[0], Args: []string{"-test.run=^TestExternalProcessHelper$", "--", mode}}, externalSettings{}, repository)
}
