package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// budgetSettings 为收束测试提供单轮工具预算与模拟模型地址。
type budgetSettings struct{ url string }

// Settings 返回每次独立的有效预算设置。
func (settings budgetSettings) Settings(ctx context.Context) (map[string]string, error) {
	values := setting.Defaults()
	values["llm_base_url"] = settings.url
	values["llm_api_key"] = "test-only"
	values["agent_max_model_rounds"] = "1"
	return values, nil
}

// TestBudgetFinalizerStreamsBeforeUpstreamCompletion 验证预算收束时最终模型分片会在上游完成前发给客户端。
func TestBudgetFinalizerStreamsBeforeUpstreamCompletion(t *testing.T) {
	release := make(chan struct{})
	finalStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input modelRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if input.ToolChoice == nil {
			call := toolCall{ID: "list", Type: "function", Function: toolFunction{Name: "bash", Arguments: `{"command":"ls /notices"}`}}
			_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Role: "assistant", Calls: []toolCall{call}}}}})
			return
		}
		if *input.ToolChoice != "none" {
			t.Error("收束请求没有禁用工具")
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(writer, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"首段\"}}]}\n\n")
		writer.(http.Flusher).Flush()
		close(finalStarted)
		<-release
		fmt.Fprint(writer, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"结论\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	loop := New(budgetSettings{url: server.URL}, agentLibrary(t))
	events := make(chan Event, 8)
	done := make(chan error, 1)
	go func() {
		done <- loop.Events(context.Background(), Request{Query: "需要收束"}, func(ctx context.Context, event Event) error {
			events <- event
			return nil
		})
	}()
	<-finalStarted
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-events:
			if event.Type == AnswerDelta {
				if event.Text != "首段" || event.Delivery != "streaming-final" {
					t.Fatalf("最终流式首段错误: %+v", event)
				}
				select {
				case err := <-done:
					t.Fatalf("上游未完成时运行提前结束: %v", err)
				default:
				}
				close(release)
				for {
					select {
					case next := <-events:
						if next.Type == RunCompleted {
							if err := <-done; err != nil {
								t.Fatal(err)
							}
							return
						}
					case <-deadline.C:
						t.Fatal("最终流式回答没有完成")
					}
				}
			}
		case <-deadline.C:
			t.Fatal("未收到最终流式首段")
		}
	}
}
