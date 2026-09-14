package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// EventType 定义公开事件协议，不包含模型隐藏推理。
type EventType string

const (
	RunStarted    EventType = "run.started"
	ToolStarted   EventType = "tool.started"
	ToolCompleted EventType = "tool.completed"
	AnswerDelta   EventType = "answer.delta"
	RunCompleted  EventType = "run.completed"
	RunFailed     EventType = "run.failed"
)

// Event 是客户端可展示的有界事件，正文与系统提示词不出现在工具事件中。
type Event struct {
	Version    int       `json:"version"`
	RunID      string    `json:"run_id"`
	Sequence   int       `json:"sequence"`
	Type       EventType `json:"type"`
	ToolID     string    `json:"tool_id,omitempty"`
	Name       string    `json:"name,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	Status     string    `json:"status,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Text       string    `json:"text,omitempty"`
	Delivery   string    `json:"delivery,omitempty"`
	Code       string    `json:"code,omitempty"`
}

// Emit 是同步且可取消的事件消费端口，调用者必须遵守上下文和背压。
type Emit func(ctx context.Context, event Event) error

// eventWriter 只归属于一次运行，维护递增序号和独立运行标识。
type eventWriter struct {
	id       string
	sequence int
	sink     Emit
}

// newEventWriter 为单次请求创建不可预测的运行标识。
func newEventWriter(sink Emit) (*eventWriter, error) {
	if sink == nil {
		return nil, fmt.Errorf("事件消费端不能为空")
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return nil, err
	}
	return &eventWriter{id: hex.EncodeToString(bytes[:]), sink: sink}, nil
}

// send 在发送前检查取消，并向每个事件附加稳定协议元数据。
func (writer *eventWriter) send(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	writer.sequence++
	event.Version = 1
	event.RunID = writer.id
	event.Sequence = writer.sequence
	return writer.sink(ctx, event)
}
