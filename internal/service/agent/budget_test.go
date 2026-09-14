package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestAgentCancellationStopsModelRequest 验证客户端取消会传递给正在等待的模型连接。
func TestAgentCancellationStopsModelRequest(t *testing.T) {
	store := agentLibrary(t)
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		close(started)
		<-request.Context().Done()
	}))
	defer server.Close()
	loop := New(testSettings{url: server.URL}, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := loop.Answer(ctx, Request{Query: "需要取消的问题"}); result <- err }()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("取消未传递: %v", err)
	}
	if len(loop.slots) != 0 {
		t.Fatal("取消后没有释放 Agent 容量")
	}
}

// TestAgentTerminatesRepeatedToolLoop 验证模型持续索要工具时仍会在轮数预算内结束。
func TestAgentTerminatesRepeatedToolLoop(t *testing.T) {
	store := agentLibrary(t)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		call := toolCall{ID: "list", Type: "function", Function: toolFunction{Name: "bash", Arguments: `{"command":"ls /notices"}`}}
		message := modelMessage{Role: "assistant", Calls: []toolCall{call}}
		_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: message}}})
	}))
	defer server.Close()
	if _, err := New(testSettings{url: server.URL}, store).Answer(context.Background(), Request{Query: "持续检索"}); err == nil {
		t.Fatal("无限工具回环没有被拒绝")
	}
	if calls.Load() != 8 {
		t.Fatalf("模型轮数预算错误: %d", calls.Load())
	}
}
