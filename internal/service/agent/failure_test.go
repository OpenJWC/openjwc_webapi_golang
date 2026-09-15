package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestToolFailurePublishesSafeCodeAndLetsModelRecover 验证工具失败不泄露命令细节且最终仍经流式轮回答。
func TestToolFailurePublishesSafeCodeAndLetsModelRecover(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input modelRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		calls++
		switch {
		case input.ToolChoice == nil && calls == 1:
			call := toolCall{ID: "bad", Type: "function", Function: toolFunction{Name: "bash", Arguments: `{"command":"rm -rf /"}`}}
			_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Calls: []toolCall{call}}}}})
		case input.ToolChoice == nil:
			_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Content: "已根据可用资讯回答"}}}})
		default:
			if input.ToolChoice == nil || *input.ToolChoice != "none" {
				t.Error("最终回答请求没有禁用工具")
			}
			writer.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(writer, `data: {"choices":[{"index":0,"delta":{"content":"已根据可用资讯回答"}}]}`+"\n\n")
			fmt.Fprint(writer, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\n")
			fmt.Fprint(writer, "data: [DONE]\n\n")
		}
	}))
	defer server.Close()
	var events []Event
	err := New(testSettings{url: server.URL}, agentLibrary(t)).Events(context.Background(), Request{Query: "测试失败恢复"}, func(ctx context.Context, event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 || events[2].Type != ToolCompleted || events[2].Status != "failed" || events[2].Code != string(ToolUnsupported) {
		t.Fatalf("工具失败码错误: %#v", events)
	}
	if events[1].Summary != "参数校验未通过" || events[4].Type != RunCompleted {
		t.Fatalf("工具细节泄露或运行未完成: %#v", events)
	}
	for _, event := range events {
		if data, marshalErr := json.Marshal(event); marshalErr == nil && strings.Contains(string(data), "rm -rf") {
			t.Fatal("工具命令泄露到客户端")
		}
	}
	if events[3].Delivery != "streaming-final" {
		t.Fatalf("最终回答未流式: %+v", events[3])
	}
}
