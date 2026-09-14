package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// crawlerSettings 只开启单个夹具站点，不调用生产来源。
type crawlerSettings struct{}

// Settings 返回两页抓取配置。
func (crawlerSettings) Settings(context.Context) (map[string]string, error) {
	values := setting.Defaults()
	values["crawler_sites"] = "jwc"
	values["crawler_max_pages"] = "2"
	return values, nil
}

// pageTransport 返回旧置顶、重复链接和第二页的确定性页面。
type pageTransport struct{ broken bool }

// RoundTrip 根据路径提供栏目和正文，损坏模式只破坏第二篇正文。
func (transport pageTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body := ""
	if strings.HasPrefix(request.URL.Path, "/detail") {
		body = `<div class="Article_Content">真实正文</div>`
		if transport.broken && request.URL.Path == "/detail2.htm" {
			body = `<div>结构已经变化</div>`
		}
	} else {
		number := 1
		if strings.Contains(request.URL.Path, "list2") {
			number = 2
		}
		row := fmt.Sprintf(`<tr><td class="main"><a title="新资讯" href="/detail%d.htm">新资讯</a></td><td class="main">%s</td></tr>`, number, time.Now().UTC().Format("2006-01-02"))
		body = `<table><tr><td class="main"><a title="旧置顶" href="/old.htm">旧置顶</a></td><td class="main">2000-01-01</td></tr>` + row + row + `</table><span class="all_pages">2</span>`
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

// TestCrawlerProgressHandlesPinnedOldRowsAndPartialFailures 验证旧置顶不阻断新数据，多页统计及部分失败可见。
func TestCrawlerProgressHandlesPinnedOldRowsAndPartialFailures(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(fmt.Sprint(broken), func(t *testing.T) {
			store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "db", "progress.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			spider := New(crawlerSettings{}, store)
			spider.client = &http.Client{Transport: pageTransport{broken: broken}}
			var last crawl.Source
			count, runErr := spider.RunObserved(context.Background(), func(ctx context.Context, source crawl.Source) error {
				last = source
				return store.SaveSource(ctx, source)
			})
			if last.Scanned != 42 || last.Skipped != 28 {
				t.Fatalf("统计错误: %+v", last)
			}
			if broken {
				if runErr != nil || count != 7 || last.State != crawl.Completed || last.Failed != 7 || !strings.Contains(last.Detail, "其他资讯已保存") {
					t.Fatalf("部分失败未作为可见完成状态保留: %+v %v", last, runErr)
				}
			} else if runErr != nil || count != 14 || last.State != crawl.Completed {
				t.Fatalf("多页抓取失败: %+v %v", last, runErr)
			}
		})
	}
}
