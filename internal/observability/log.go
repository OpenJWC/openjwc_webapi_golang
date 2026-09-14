package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Buffer 保存有界日志摘要，不保留结构化属性中的凭据或请求数据。
type Buffer struct {
	mutex sync.Mutex
	rows  [][]string
}

// handler 将日志转发到原处理器并记录不含属性的管理摘要。
type handler struct {
	next   slog.Handler
	buffer *Buffer
}

// NewLogger 为现有日志目标附加最近两百条摘要缓冲。
func NewLogger(logger *slog.Logger) (*slog.Logger, *Buffer) {
	buffer := &Buffer{rows: make([][]string, 0, 200)}
	return slog.New(&handler{next: logger.Handler(), buffer: buffer}), buffer
}

// Rows 返回最新在前的日志副本，不暴露内部可变切片。
func (buffer *Buffer) Rows() [][]string {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	rows := make([][]string, 0, len(buffer.rows))
	for index := len(buffer.rows) - 1; index >= 0; index-- {
		rows = append(rows, append([]string(nil), buffer.rows[index]...))
	}
	return rows
}

// Enabled 复用原日志处理器的等级过滤。
func (recording *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return recording.next.Enabled(ctx, level)
}

// Handle 记录有界安全摘要后转发原始结构化日志。
func (recording *handler) Handle(ctx context.Context, record slog.Record) error {
	recording.buffer.mutex.Lock()
	if len(recording.buffer.rows) == 200 {
		copy(recording.buffer.rows, recording.buffer.rows[1:])
		recording.buffer.rows = recording.buffer.rows[:199]
	}
	recording.buffer.rows = append(recording.buffer.rows, []string{record.Time.UTC().Format(time.RFC3339), record.Level.String(), record.Message})
	recording.buffer.mutex.Unlock()
	return recording.next.Handle(ctx, record)
}

// WithAttrs 共享摘要缓冲并保留底层处理器的属性语义。
func (recording *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{next: recording.next.WithAttrs(attrs), buffer: recording.buffer}
}

// WithGroup 共享摘要缓冲并保留底层处理器的分组语义。
func (recording *handler) WithGroup(name string) slog.Handler {
	return &handler{next: recording.next.WithGroup(name), buffer: recording.buffer}
}
