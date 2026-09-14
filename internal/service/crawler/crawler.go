package crawler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
	"golang.org/x/net/html"
)

// Repository 定义爬虫只需的资讯幂等保存能力。
type Repository interface {
	Save(context.Context, notice.Notice) error
}

// Crawler 以单任务和串行网络请求限制对学校站点及本地写库的压力。
type Crawler struct {
	settings   setting.Reader
	repository Repository
	client     *http.Client
	slot       chan struct{}
}

// New 创建不依赖浏览器、Python 或 Rust 二进制的爬虫。
func New(settings setting.Reader, repository Repository) *Crawler {
	return &Crawler{settings: settings, repository: repository, slot: make(chan struct{}, 1), client: &http.Client{Timeout: 20 * time.Second, CheckRedirect: followOfficialRedirect}}
}

// Run 保留同步爬虫入口，任务管理器使用带进度的入口。
func (crawler *Crawler) Run(ctx context.Context) (int, error) { return crawler.RunObserved(ctx, nil) }

// fetch 只接收成功且有界的 HTML，不跟随重定向。
func (crawler *Crawler) fetch(ctx context.Context, address string) (*html.Node, error) {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "https" {
		return nil, fmt.Errorf("爬虫地址无效")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "OpenJWC/1.0 (+https://github.com/OpenJWC)")
	response, err := crawler.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("抓取页面失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("来源页面 %s 返回状态 %d", safeAddress(parsed), response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(content) > 4<<20 {
		return nil, fmt.Errorf("来源页面超过 4 MiB")
	}
	return html.Parse(strings.NewReader(string(content)))
}

// followOfficialRedirect 仅跟随已知学校主机内的有限重定向，并把站点错误降级的 HTTP 地址改回 HTTPS。
func followOfficialRedirect(request *http.Request, via []*http.Request) error {
	if len(via) == 0 || len(via) > 3 || request.URL.User != nil || !officialHost(request.URL.Hostname()) || request.URL.Hostname() != via[len(via)-1].URL.Hostname() {
		return http.ErrUseLastResponse
	}
	if request.URL.Scheme == "http" {
		request.URL.Scheme = "https"
	}
	if request.URL.Scheme != "https" {
		return http.ErrUseLastResponse
	}
	return nil
}

// officialHost 判断重定向目标是否仍属于编译期固定的学校来源。
func officialHost(host string) bool {
	for _, source := range sites {
		if host == source.host {
			return true
		}
	}
	return false
}

// safeAddress 只输出无查询和片段的来源地址，避免错误信息携带意外参数。
func safeAddress(address *url.URL) string {
	return address.Scheme + "://" + address.Host + address.EscapedPath()
}
