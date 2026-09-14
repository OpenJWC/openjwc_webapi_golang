package admin

import (
	"context"
	"fmt"
)

// Crawler 定义管理员可触发的爬虫任务。
type Crawler interface {
	Start(context.Context) (string, error)
	Progress(context.Context) (Response, error)
	Cancel(context.Context, string) error
}

// Digest 定义管理员可触发的日报任务。
type Digest interface {
	Generate(context.Context, string) error
}

// Dispatcher 将任务操作与持久化管理操作路由到对应服务。
type Dispatcher struct {
	base    Executor
	crawler Crawler
	digest  Digest
}

// NewDispatcher 创建本地管理用例分派器。
func NewDispatcher(base Executor, crawler Crawler, digest Digest) *Dispatcher {
	return &Dispatcher{base: base, crawler: crawler, digest: digest}
}

// Execute 校验动作后调用目标服务，不授予任意命令或 SQL 能力。
func (dispatcher *Dispatcher) Execute(ctx context.Context, request Request) (Response, error) {
	if err := request.Validate(); err != nil {
		return Response{}, err
	}
	switch request.Action {
	case ActionCrawl:
		id, err := dispatcher.crawler.Start(ctx)
		return Response{Text: fmt.Sprintf("爬虫任务 %s 已接纳；退出 TUI 不会取消。请在爬虫任务页面查看进度。", id)}, err
	case ActionCrawlProgress:
		return dispatcher.crawler.Progress(ctx)
	case ActionCrawlCancel:
		err := dispatcher.crawler.Cancel(ctx, request.ID)
		return Response{Text: "已请求取消任务"}, err
	case ActionDigest:
		err := dispatcher.digest.Generate(ctx, request.ID)
		return Response{Text: "日报已生成，可在日报页面查看"}, err
	default:
		return dispatcher.base.Execute(ctx, request)
	}
}
