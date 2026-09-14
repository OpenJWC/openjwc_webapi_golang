package crawler

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"golang.org/x/net/html"
)

// fixtureTransport 为爬虫测试提供确定性 HTML，不访问学校生产站点。
type fixtureTransport struct{}

// RoundTrip 返回包含正文、脚本和附件的详情页夹具。
func (fixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body := `<div class="Article_Content"><p>考试在周一举行。</p><script>secretScript()</script><a href="/files/a.pdf">考试附件</a></div>`
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
}

// TestCrawlerPreservesLegacyURLHashesAndExtractsContent 验证旧 URL 标识、中文正文及附件转换。
func TestCrawlerPreservesLegacyURLHashesAndExtractsContent(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "db", "crawler.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	crawler := New(store, store)
	crawler.client = &http.Client{Transport: fixtureTransport{}}
	document, err := html.Parse(strings.NewReader(`<table><tr><td class="main"><a title="考试安排" href="/2026/0901/test/page.htm">考试安排</a></td><td class="main"><div>2026-09-01</div></td></tr></table>`))
	if err != nil {
		t.Fatal(err)
	}
	count, hasNew, err := crawler.ingest(ctx, document, sites["jwc"], category{label: "教务信息"}, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || count != 1 || !hasNew {
		t.Fatalf("抓取失败: %d %v %v", count, hasNew, err)
	}
	id := fmt.Sprintf("%x", sha256.Sum256([]byte("https://jwc.seu.edu.cn/2026/0901/test/page.htm")))
	item, found, err := store.FindByID(ctx, id)
	if err != nil || !found {
		t.Fatalf("旧版资讯 ID 未保留: %v %v", found, err)
	}
	if !strings.Contains(item.Content(), "考试在周一") || strings.Contains(item.Content(), "secretScript") {
		t.Fatal("正文提取错误")
	}
	if len(item.Attachments()) != 1 || item.Attachments()[0].URL != "https://jwc.seu.edu.cn/files/a.pdf" {
		t.Fatalf("附件解析错误: %v", item.Attachments())
	}
}

// TestJWCSelectorIgnoresOuterLayoutRows 验证包含整张列表的外层 tr 不会被重复当作资讯。
func TestJWCSelectorIgnoresOuterLayoutRows(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "db", "nested.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	spider := New(store, store)
	spider.client = &http.Client{Transport: fixtureTransport{}}
	document, err := html.Parse(strings.NewReader(`<table><tr><td><table><tr><td class="main"><a title="考试安排" href="/detail.htm">考试安排</a></td><td class="main">2026-09-01</td></tr></table></td></tr></table>`))
	if err != nil {
		t.Fatal(err)
	}
	progress := crawl.Source{}
	count, _, err := spider.ingestObserved(context.Background(), document, sites["jwc"], category{label: "教务信息"}, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), &progress)
	if err != nil || count != 1 || progress.Scanned != 1 || progress.Skipped != 0 {
		t.Fatalf("外层布局行未排除: count=%d progress=%+v err=%v", count, progress, err)
	}
}

// TestCollegeSelectorIgnoresFooterNewsClass 验证学院页脚复用 news 类名时不会计为无日期资讯。
func TestCollegeSelectorIgnoresFooterNewsClass(t *testing.T) {
	document, err := html.Parse(strings.NewReader(`<ul><li class="news"><span class="news_title"><a title="通知" href="/detail">通知</a></span><span class="news_meta">2026-09-14</span></li><li class="news"><a title="相关链接" href="/links">相关链接</a></li></ul>`))
	if err != nil {
		t.Fatal(err)
	}
	rows := descendants(document, func(node *html.Node) bool { return isNoticeRow(node, sites["cs"]) })
	if len(rows) != 1 || !strings.Contains(nodeText(rows[0]), "通知") {
		t.Fatalf("学院资讯行筛选错误: %d", len(rows))
	}
}

// TestOfficialRedirectUpgradesSameHostOnly 验证学校站点的 HTTP 降级地址被改回 HTTPS，跨站重定向仍拒绝。
func TestOfficialRedirectUpgradesSameHostOnly(t *testing.T) {
	previous, _ := http.NewRequest(http.MethodGet, "https://jwc.seu.edu.cn/detail.htm", nil)
	next, _ := http.NewRequest(http.MethodGet, "http://jwc.seu.edu.cn/detail.psp", nil)
	if err := followOfficialRedirect(next, []*http.Request{previous}); err != nil || next.URL.Scheme != "https" {
		t.Fatalf("同站重定向未安全升级: %s %v", next.URL, err)
	}
	cross, _ := http.NewRequest(http.MethodGet, "https://example.com/detail", nil)
	if err := followOfficialRedirect(cross, []*http.Request{previous}); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatal("跨站重定向未拒绝")
	}
}

// TestCrawlerDetectsUpstreamMarkupChanges 验证站点结构失效不会伪装成抓取成功。
func TestCrawlerDetectsUpstreamMarkupChanges(t *testing.T) {
	document, err := html.Parse(strings.NewReader(`<html><body>站点维护中</body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	crawler := New(nil, nil)
	if _, _, err := crawler.ingest(context.Background(), document, sites["jwc"], category{label: "教务信息"}, time.Now()); err == nil {
		t.Fatal("未识别栏目解析失败")
	}
}
