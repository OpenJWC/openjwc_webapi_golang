package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestToolFailurePublishesSafeCodeAndLetsModelRecover 验证工具失败不泄露命令细节且模型可继续回答。
func TestToolFailurePublishesSafeCodeAndLetsModelRecover(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input modelRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if input.Messages[len(input.Messages)-1].Role == "tool" {
			_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Content: "已根据可用资讯回答"}}}})
			return
		}
		call := toolCall{ID: "bad", Type: "function", Function: toolFunction{Name: "bash", Arguments: `{"command":"rm -rf /"}`}}
		_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Calls: []toolCall{call}}}}})
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
		t.Fatalf("工具细节泄露或模型未恢复: %#v", events)
	}
}
