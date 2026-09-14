package crawler

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
	"golang.org/x/net/html"
)

// Observer 接收每页及来源结束时的值快照，返回错误可以中止任务。
type Observer func(context.Context, crawl.Source) error

// RunObserved 在唯一爬虫任务中逐来源报告进度，保留成功写入的部分结果。
func (crawler *Crawler) RunObserved(ctx context.Context, observe Observer) (int, error) {
	select {
	case crawler.slot <- struct{}{}:
	default:
		return 0, fmt.Errorf("爬虫任务已在运行")
	}
	defer func() { <-crawler.slot }()
	settings, err := crawler.settings.Settings(ctx)
	if err != nil {
		return 0, err
	}
	for _, key := range []string{"crawler_max_pages", "crawler_days_gap", "crawler_sites"} {
		if err := setting.Validate(key, settings[key]); err != nil {
			return 0, err
		}
	}
	maxPages, _ := strconv.Atoi(settings["crawler_max_pages"])
	days, _ := strconv.Atoi(settings["crawler_days_gap"])
	cutoff := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 1-days)
	total := 0
	var failures []error
	for _, name := range strings.Split(settings["crawler_sites"], ",") {
		source, ok := sites[strings.TrimSpace(name)]
		if !ok {
			return total, fmt.Errorf("未知爬虫站点")
		}
		progress := crawl.Source{Name: name, State: crawl.Running, StartedAt: time.Now().UTC()}
		if observe != nil {
			if err = observe(ctx, progress); err != nil {
				return total, err
			}
		}
		sourceErr := crawler.crawlSource(ctx, source, maxPages, cutoff, &progress, observe)
		total += progress.Saved
		progress.FinishedAt = time.Now().UTC()
		progress.State = crawl.Completed
		if sourceErr != nil {
			progress.State = crawl.Failed
			progress.Detail = "来源抓取或解析失败，请检查站点与网络"
			failures = recordFailure(failures, sourceErr)
		}
		if ctx.Err() != nil {
			progress.State = crawl.Canceled
			progress.Detail = "任务已取消"
		}
		if observe != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			err = observe(cleanup, progress)
			cancel()
			if err != nil {
				return total, errors.Join(sourceErr, err)
			}
		}
		if ctx.Err() != nil {
			return total, ctx.Err()
		}
	}
	return total, errors.Join(failures...)
}

// crawlSource 逐栏目扫描页面，不因旧置顶条目而提前停止仍包含新资讯的页面。
func (crawler *Crawler) crawlSource(ctx context.Context, source site, maxPages int, cutoff time.Time, progress *crawl.Source, observe Observer) error {
	var failures []error
	for _, category := range source.categories {
		for page := 1; page <= maxPages; page++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			path := category.path
			if page > 1 {
				path = strings.Replace(path, "list", "list"+strconv.Itoa(page), 1)
			}
			document, err := crawler.fetch(ctx, "https://"+source.host+path)
			if err != nil {
				progress.Failed++
				failures = recordFailure(failures, fmt.Errorf("栏目 %s: %w", category.label, err))
				break
			}
			_, hasNew, err := crawler.ingestObserved(ctx, document, source, category, cutoff, progress)
			if err != nil {
				var partial partialError
				if errors.As(err, &partial) {
					progress.Detail = fmt.Sprintf("%d 条资讯详情不可访问或无法解析，其他资讯已保存", progress.Failed)
				} else {
					failures = recordFailure(failures, err)
				}
			}
			if observe != nil {
				if err = observe(ctx, *progress); err != nil {
					return err
				}
			}
			pages := descendants(document, func(node *html.Node) bool { return hasClass(node, "all_pages") })
			last := 1
			if len(pages) > 0 {
				last, _ = strconv.Atoi(nodeText(pages[0]))
			}
			if !hasNew || page >= last {
				break
			}
		}
	}
	return errors.Join(failures...)
}

// recordFailure 保留有限错误样本，完整失败数量由来源统计记录。
func recordFailure(failures []error, err error) []error {
	if len(failures) < 20 {
		return append(failures, err)
	}
	return failures
}
