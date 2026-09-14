package agent

import (
	"context"
	"errors"
	"fmt"
)

// ErrBusy 表示 Agent 并发容量已满。
var ErrBusy = errors.New("智能问答繁忙")

// ErrUnavailable 表示模型服务未配置或暂不可用。
var ErrUnavailable = errors.New("模型服务暂不可用")

// Request 是与旧聊天接口兼容的输入结构。
type Request struct {
	NoticeIDs []string         `json:"notice_ids"`
	Query     string           `json:"user_query"`
	Stream    bool             `json:"stream"`
	History   []HistoryMessage `json:"history"`
}

// HistoryMessage 表示客户端提供的历史消息，不允许伪造系统或工具角色。
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Validate 限制历史、指定资讯及总文本，避免绕过上下文预算。
func (request Request) Validate() error {
	if request.Query == "" || len(request.Query) > 16000 || len(request.History) > 20 || len(request.NoticeIDs) > 10 {
		return fmt.Errorf("问答请求超出长度或数量限制")
	}
	total := len(request.Query)
	for _, message := range request.History {
		if message.Role != "user" && message.Role != "assistant" {
			return fmt.Errorf("历史消息角色只能为 user 或 assistant")
		}
		total += len(message.Content)
	}
	for _, id := range request.NoticeIDs {
		if id == "" || len(id) > 256 {
			return fmt.Errorf("资讯 ID 无效")
		}
	}
	if total > 48000 {
		return fmt.Errorf("历史消息总长度超出限制")
	}
	return nil
}

// Chat 完成一次独立 Agent 会话。
type Chat interface {
	Answer(context.Context, Request) (string, error)
}

// EventChat 定义可取消且受背压约束的公开问答事件流。
type EventChat interface {
	Events(ctx context.Context, request Request, emit Emit) error
}
