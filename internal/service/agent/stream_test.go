package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// TestStreamAssemblesToolsWithoutExposingIntermediateText 验证分片参数、事件顺序和隐藏字段不会出现在最终答案中。
func TestStreamAssemblesToolsWithoutExposingIntermediateText(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input modelRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if !input.Stream {
			t.Error("未请求模型流式响应")
		}
		calls++
		last := input.Messages[len(input.Messages)-1]
		switch {
		case input.ToolChoice == nil && last.Role == "user":
			writer.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(writer, `data: {"choices":[{"index":0,"delta":{"content":"不得展示的中间正文","reasoning_content":"隐藏思考","tool_calls":[{"index":0,"id":"a","type":"function","function":{"name":"bash","arguments":"{\"command\":"}}]}}]}`+"\n\n")
			fmt.Fprint(writer, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ls /notices\"}"}}]},"finish_reason":"tool_calls"}]}`+"\n\n")
			fmt.Fprint(writer, "data: [DONE]\n\n")
		case input.ToolChoice == nil && last.Role == "tool":
			_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Role: "assistant", Content: "已找到资讯"}}}})
		default:
			if input.ToolChoice == nil || *input.ToolChoice != "none" {
				t.Error("最终回答请求没有禁用工具")
			}
			writer.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(writer, `data: {"choices":[{"index":0,"delta":{"content":"已找到资讯，"}}]}`+"\n\n")
			fmt.Fprint(writer, `data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\n")
			fmt.Fprint(writer, "data: [DONE]\n\n")
		}
	}))
	defer server.Close()
	loop := New(testSettings{url: server.URL}, agentLibrary(t))
	var events []Event
	err := loop.Events(context.Background(), Request{Query: "查找资讯"}, func(ctx context.Context, event Event) error { events = append(events, event); return nil })
	if err != nil {
		t.Fatal(err)
	}
	var types []EventType
	for index, event := range events {
		types = append(types, event.Type)
		if event.Sequence != index+1 || event.RunID != events[0].RunID || event.Version != 1 {
			t.Fatalf("元数据错误: %+v", event)
		}
		data, _ := json.Marshal(event)
		if strings.Contains(string(data), "中间正文") || strings.Contains(string(data), "隐藏思考") {
			t.Fatal("泄露模型中间内容")
		}
	}
	if !reflect.DeepEqual(types, []EventType{RunStarted, ToolStarted, ToolCompleted, AnswerDelta, RunCompleted}) {
		t.Fatalf("事件顺序错误: %v", types)
	}
	if events[3].Text != "已找到资讯，" || events[3].Delivery != "streaming-final" {
		t.Fatalf("最终答案未真正流式: %+v", events[3])
	}
	if events[1].ToolID != events[2].ToolID {
		t.Fatal("工具事件未配对")
	}
	if calls != 3 {
		t.Fatalf("模型请求次数错误: %d", calls)
	}
}

// TestTruncatedOrOversizedModelStreamFailsClosed 验证提前 EOF、未知结束原因和越界工具索引不会返回部分答案。
func TestTruncatedOrOversizedModelStreamFailsClosed(t *testing.T) {
	for _, text := range []string{
		"data: [DONE]\n\n",
		`data: {"choices":[{"index":0,"delta":{"content":"partial"}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"length"}]}` + "\n\ndata: [DONE]\n\n",
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":9}]}}]}` + "\n\n",
	} {
		if _, err := readModelStream(strings.NewReader(text)); err == nil {
			t.Fatal("接受了不完整模型流")
		}
	}
	call := toolCall{Type: "function", Function: toolFunction{Name: "bash", Arguments: `{"command":"ls /notices"} {}`}}
	if _, err := toolCommand(call); err == nil {
		t.Fatal("工具参数接受尾随 JSON")
	}
}
