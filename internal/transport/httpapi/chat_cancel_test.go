package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
)

// floodingEvents 模拟持续产生事件且正确响应背压取消的模型。
type floodingEvents struct {
	count    int
	finished bool
}

// Events 发送有界事件直到客户端断连，结束标记用于检查回收路径。
func (source *floodingEvents) Events(ctx context.Context, input agent.Request, emit agent.Emit) error {
	defer func() { source.finished = true }()
	for index := 0; index < 1000; index++ {
		source.count++
		if err := emit(ctx, agent.Event{Version: 1, RunID: "test", Sequence: index + 1, Type: agent.ToolStarted}); err != nil {
			return err
		}
	}
	return nil
}

// brokenWriter 模拟连接立即失效，不通过任意延时触发取消。
type brokenWriter struct{ header http.Header }

// Header 返回模拟响应头。
func (writer brokenWriter) Header() http.Header { return writer.header }

// WriteHeader 接受模拟状态码。
func (writer brokenWriter) WriteHeader(status int) {}

// Write 明确报告连接写入失败。
func (writer brokenWriter) Write(data []byte) (int, error) {
	return 0, errors.New("测试连接已断开")
}

// TestEventDisconnectCancelsProducerAndBoundsQueue 验证断连会回收生产者且不会积累全部事件。
func TestEventDisconnectCancelsProducerAndBoundsQueue(t *testing.T) {
	source := &floodingEvents{}
	client := &Client{dependencies: ClientDependencies{Events: source}}
	request := httptest.NewRequest(http.MethodPost, "/api/v2/client/chat", strings.NewReader("{}"))
	client.serveEvents(brokenWriter{header: make(http.Header)}, request, agent.Request{Query: "问题"})
	if !source.finished || source.count > 11 {
		t.Fatalf("未回收或队列无界: finished=%v count=%d", source.finished, source.count)
	}
}
