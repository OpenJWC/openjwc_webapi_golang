package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
)

// answerResult 是单个问答工作协程返回的结果。
type answerResult struct {
	text string
	err  error
}

// chatStream 在工具推理期间发送空数据心跳，兼容 Android 的非标准原始文本解析器。
func (client *Client) chatStream(writer http.ResponseWriter, request *http.Request, input agent.Request) {
	ctx, cancel := context.WithCancel(request.Context())
	result := make(chan answerResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		text, err := client.dependencies.Chat.Answer(ctx, input)
		result <- answerResult{text: text, err: err}
	}()
	defer func() { cancel(); <-done }()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	started := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !started {
				startStream(writer)
				started = true
			}
			if err := streamFrame(writer, ""); err != nil {
				return
			}
		case answer := <-result:
			if answer.err != nil && !started {
				client.chatError(writer, answer.err)
				return
			}
			if !started {
				startStream(writer)
			}
			if answer.err != nil {
				answer.text = "AI 服务暂时不可用，请稍后重试。"
			}
			if err := streamFrame(writer, answer.text); err != nil {
				return
			}
			_ = streamFrame(writer, "[DONE]")
			return
		}
	}
}

// startStream 写入禁用缓存和代理缓冲的 SSE 响应头。
func startStream(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("X-Accel-Buffering", "no")
}

// streamFrame 保留旧版 data 原始文本帧，不额外给正文换行添加 data 前缀。
func streamFrame(writer http.ResponseWriter, text string) error {
	return writeSSE(writer, fmt.Sprintf("data: %s\n\n", text))
}
