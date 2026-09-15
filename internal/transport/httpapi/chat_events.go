package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
)

// chatEvents 在新版本接口发送标准 SSE，旧接口的原始文本帧不受影响。
func (client *Client) chatEvents(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	var input agent.Request
	if !decodeBody(writer, request, &input) {
		return
	}
	if err := input.Validate(); err != nil {
		invalidInput(writer, err.Error())
		return
	}
	if client.dependencies.Events == nil {
		client.chatError(writer, agent.ErrUnavailable)
		return
	}
	client.chatMutex.Lock()
	if client.chatUsers[principal.ID()] {
		client.chatMutex.Unlock()
		client.chatError(writer, agent.ErrBusy)
		return
	}
	client.chatUsers[principal.ID()] = true
	client.chatMutex.Unlock()
	defer func() { client.chatMutex.Lock(); delete(client.chatUsers, principal.ID()); client.chatMutex.Unlock() }()
	client.serveEvents(writer, request, input)
}

// serveEvents 使用有界队列隔离模型和网络写入，断连后取消并回收模型请求。
func (client *Client) serveEvents(writer http.ResponseWriter, request *http.Request, input agent.Request) {
	ctx, cancel := context.WithCancel(request.Context())
	events := make(chan agent.Event, 8)
	finished := make(chan struct{})
	var runErr error
	go func() {
		runErr = client.dependencies.Events.Events(ctx, input, func(ctx context.Context, event agent.Event) error {
			select {
			case events <- event:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		close(events)
		close(finished)
	}()
	defer func() { cancel(); <-finished }()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	started := false
	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-events:
			if !open {
				client.logChatFailure(runErr, started)
				if !started && runErr != nil {
					client.chatError(writer, runErr)
				}
				return
			}
			if !started {
				startStream(writer)
				started = true
			}
			if err := writeEvent(writer, event); err != nil {
				return
			}
		case <-ticker.C:
			if !started {
				startStream(writer)
				started = true
			}
			if err := writeSSE(writer, ": ping\n\n"); err != nil {
				return
			}
		}
	}
}

// logChatFailure 仅记录经过分类的计数与运行标识，不记录请求或模型内容。
func (client *Client) logChatFailure(err error, started bool) {
	if err == nil {
		return
	}
	var failure *agent.Failure
	if errors.As(err, &failure) {
		client.logger.Warn("Agent 运行未完成", "run_id", failure.RunID, "code", failure.Code, "detail", failure.Detail, "upstream_status", failure.UpstreamStatus, "streamed", started, "model_rounds", failure.ModelRounds, "tool_calls", failure.ToolCalls, "tool_result_bytes", failure.ToolResultBytes)
		return
	}
	client.logger.Warn("Agent 事件流未完成", "code", "agent_event_delivery_failed", "streamed", started)
}

// writeEvent 将换行和特殊字符编码进 JSON，事件编号可以定位但不支持断点重放。
func writeEvent(writer http.ResponseWriter, event agent.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return writeSSE(writer, fmt.Sprintf("id: %s:%d\nevent: %s\ndata: %s\n\n", event.RunID, event.Sequence, event.Type, data))
}

// writeSSE 为单次网络写入设置期限，防止慢消费者无限占用问答容量。
func writeSSE(writer http.ResponseWriter, frame string) error {
	controller := http.NewResponseController(writer)
	if err := controller.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	defer controller.SetWriteDeadline(time.Time{})
	if _, err := fmt.Fprint(writer, frame); err != nil {
		return err
	}
	return controller.Flush()
}
