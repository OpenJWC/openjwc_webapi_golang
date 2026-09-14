package crawler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
)

const maxExternalEvents = 20000
const maxExternalNotices = 10000
const maxExternalCounter = 10000000

// readEvents 消费完整 NDJSON 流，要求恰好一个终止事件且其后没有数据。
func (external *External) readEvents(ctx context.Context, scanner *bufio.Scanner, source *crawl.Source, observe Observer) (int, string, error) {
	events, notices := 0, 0
	terminal, failed := false, false
	detail := ""
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return notices, detail, err
		}
		line := scanner.Text()
		if terminal {
			return notices, detail, fmt.Errorf("外部爬虫终止事件后仍有输出")
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		events++
		if events > maxExternalEvents {
			return notices, detail, fmt.Errorf("外部爬虫事件过多或终止事件后仍有输出")
		}
		event, err := decodeProtocolEvent(line)
		if err != nil {
			return notices, detail, err
		}
		switch event.Type {
		case "progress":
			if event.Notice != nil {
				return notices, detail, fmt.Errorf("外部爬虫进度事件包含资讯")
			}
			if err = applyProgress(source, event); err != nil {
				return notices, detail, err
			}
			detail = event.Detail
			source.Detail = safeDetail(detail, "")
		case "notice":
			if event.Notice == nil || notices >= maxExternalNotices {
				return notices, detail, fmt.Errorf("外部爬虫资讯事件无效或过多")
			}
			if err = external.saveExternalNotice(ctx, *event.Notice); err != nil {
				return notices, detail, fmt.Errorf("外部爬虫资讯无效: %w", err)
			}
			notices++
			source.Saved = notices
			source.Scanned = max(source.Scanned, source.Saved+source.Skipped+source.Failed)
		case "completed", "failed":
			if event.Notice != nil || applyProgress(source, event) != nil {
				return notices, detail, fmt.Errorf("外部爬虫终止事件无效")
			}
			terminal = true
			failed = event.Type == "failed"
			detail = event.Detail
		default:
			return notices, detail, fmt.Errorf("外部爬虫事件类型无效")
		}
		if observe != nil && !terminal {
			if err = observe(ctx, *source); err != nil {
				return notices, detail, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return notices, detail, fmt.Errorf("读取外部爬虫协议: %w", err)
	}
	if !terminal {
		return notices, detail, fmt.Errorf("外部爬虫缺少终止事件")
	}
	if failed {
		return notices, detail, fmt.Errorf("外部爬虫报告失败")
	}
	return notices, detail, nil
}

// decodeProtocolEvent 严格解析单行事件并拒绝未知字段和尾随 JSON。
func decodeProtocolEvent(line string) (protocolEvent, error) {
	var event protocolEvent
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return event, fmt.Errorf("外部爬虫事件 JSON 无效")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || event.Version != 1 {
		return event, fmt.Errorf("外部爬虫事件版本或边界无效")
	}
	return event, nil
}

// applyProgress 接受单调累计计数，防止回退或整数资源滥用。
func applyProgress(source *crawl.Source, event protocolEvent) error {
	updates := []struct {
		value  *int
		target *int
	}{{value: event.Scanned, target: &source.Scanned}, {value: event.Skipped, target: &source.Skipped}, {value: event.Failed, target: &source.Failed}}
	for _, update := range updates {
		if update.value == nil {
			continue
		}
		if *update.value < *update.target || *update.value > maxExternalCounter {
			return fmt.Errorf("外部爬虫进度超出范围或发生回退")
		}
		*update.target = *update.value
	}
	if len([]rune(event.Detail)) > 512 {
		return fmt.Errorf("外部爬虫进度说明过长")
	}
	return nil
}
