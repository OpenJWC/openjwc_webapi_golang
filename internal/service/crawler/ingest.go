package crawler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
	"golang.org/x/net/html"
)

// datePattern 匹配旧爬虫兼容的日期文本。
var datePattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// partialError 表示列表有效但部分详情不可访问或不可解析，不阻断整个来源完成。
type partialError struct{ count int }

// Error 返回不包含页面正文或内部错误的部分失败摘要。
func (failure partialError) Error() string {
	return fmt.Sprintf("%d 条资讯详情未导入", failure.count)
}

// ingest 从栏目条目提取资讯，对解析失败显式报错而不写空正文覆盖旧值。
func (crawler *Crawler) ingest(ctx context.Context, document *html.Node, source site, category category, cutoff time.Time) (int, bool, error) {
	progress := crawl.Source{}
	return crawler.ingestObserved(ctx, document, source, category, cutoff, &progress)
}

// ingestObserved 统计已解析候选的扫描、保存、跳过和失败，不把重复保存视为新增。
func (crawler *Crawler) ingestObserved(ctx context.Context, document *html.Node, source site, category category, cutoff time.Time, progress *crawl.Source) (int, bool, error) {
	rows := descendants(document, func(node *html.Node) bool { return isNoticeRow(node, source) })
	count, matched := 0, 0
	hasNew := false
	var failures []error
	seen := make(map[string]bool)
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return count, hasNew, err
		}
		links := descendants(row, func(node *html.Node) bool { return node.Data == "a" && attribute(node, "title") != "" })
		if len(links) == 0 {
			continue
		}
		progress.Scanned++
		date := datePattern.FindString(nodeText(row))
		published, err := time.Parse("2006-01-02", date)
		if err != nil {
			progress.Failed++
			failures = recordFailure(failures, fmt.Errorf("栏目 %s 存在无有效日期的资讯行", category.label))
			continue
		}
		matched++
		if published.Before(cutoff) {
			progress.Skipped++
			continue
		}
		hasNew = true
		link := links[0]
		base, _ := url.Parse("https://" + source.host)
		reference, err := url.Parse(attribute(link, "href"))
		if err != nil {
			continue
		}
		address := base.ResolveReference(reference)
		if address.Scheme != "http" && address.Scheme != "https" {
			continue
		}
		if seen[address.String()] {
			progress.Skipped++
			continue
		}
		seen[address.String()] = true
		digest := sha256.Sum256([]byte(address.String()))
		input := notice.CreateInput{ID: hex.EncodeToString(digest[:]), Label: category.label, Title: attribute(link, "title"), PublishedAt: published, DetailURL: address.String(), IsPage: !isAttachment(address.Path)}
		if input.IsPage && address.Host == source.host && address.Scheme == "https" {
			body, err := crawler.fetch(ctx, address.String())
			if err != nil {
				progress.Failed++
				failures = recordFailure(failures, fmt.Errorf("资讯 %s: %w", safeAddress(address), err))
				continue
			}
			parts := descendants(body, func(node *html.Node) bool { return hasClass(node, source.bodyClass) })
			if len(parts) == 0 {
				progress.Failed++
				failures = recordFailure(failures, fmt.Errorf("资讯正文选择器失效: %s", input.ID))
				continue
			}
			input.Content = nodeText(parts[0])
			for _, attachment := range descendants(parts[0], func(node *html.Node) bool { return node.Data == "a" }) {
				ref, err := url.Parse(attribute(attachment, "href"))
				if err != nil {
					continue
				}
				full := address.ResolveReference(ref)
				if (full.Scheme == "http" || full.Scheme == "https") && isAttachment(full.Path) {
					input.Attachments = append(input.Attachments, notice.Attachment{Name: nodeText(attachment), URL: full.String()})
				}
			}
		}
		item, err := notice.New(input)
		if err != nil {
			progress.Failed++
			failures = recordFailure(failures, err)
			continue
		}
		if err = crawler.repository.Save(ctx, item); err != nil {
			return count, hasNew, fmt.Errorf("保存爬取资讯: %w", err)
		}
		count++
		progress.Saved++
	}
	if matched == 0 {
		progress.Failed++
		return count, hasNew, fmt.Errorf("栏目 %s 未找到可解析资讯，请检查站点结构", category.label)
	}
	if len(failures) > 0 {
		return count, hasNew, partialError{count: len(failures)}
	}
	return count, hasNew, nil
}

// isAttachment 识别旧爬虫支持的附件类型，只保存链接而不下载文件。
func isAttachment(address string) bool {
	switch strings.ToLower(path.Ext(address)) {
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".zip", ".rar":
		return true
	default:
		return false
	}
}

// isNoticeRow 使用站点最小结构特征排除包含整张列表的外层布局节点。
func isNoticeRow(node *html.Node, source site) bool {
	if node.Type != html.ElementNode {
		return false
	}
	if source.host != "jwc.seu.edu.cn" {
		if node.Data != "li" || !hasClass(node, "news") {
			return false
		}
		title, metadata := false, false
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			title = title || child.Type == html.ElementNode && hasClass(child, "news_title")
			metadata = metadata || child.Type == html.ElementNode && hasClass(child, "news_meta")
		}
		return title && metadata
	}
	if node.Data != "tr" {
		return false
	}
	mainCells := 0
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == "td" && hasClass(child, "main") {
			mainCells++
		}
	}
	return mainCells >= 2
}
