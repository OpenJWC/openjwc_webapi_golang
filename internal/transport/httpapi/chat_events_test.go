package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
)

// Events 提供固定的新事件协议夹具，与旧 Answer 使用相同文本。
func (fixedChat) Events(ctx context.Context, request agent.Request, emit agent.Emit) error {
	for index, kind := range []agent.EventType{agent.RunStarted, agent.AnswerDelta, agent.RunCompleted} {
		event := agent.Event{Version: 1, RunID: "fixture", Sequence: index + 1, Type: kind}
		if kind == agent.AnswerDelta {
			event.Text = "第一行\n第二行"
			event.Delivery = "buffered-final"
		}
		if err := emit(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// TestChatEventStreamKeepsJSONFramesAndPermissions 验证新接口权限及标准 JSON 事件帧不会破坏换行。
func TestChatEventStreamKeepsJSONFramesAndPermissions(t *testing.T) {
	router, store, token := testClient(t)
	if response := callClient(router, "POST", "/api/v2/client/chat", `{"user_query":"问题"}`, token); response.Code != 403 {
		t.Fatalf("未授权问答仍可调用: %d", response.Code)
	}
	if _, err := store.Execute(context.Background(), admin.Request{Action: admin.ActionPermissions, ID: "1", Values: map[string]string{"active": "true", "browse": "true", "chat": "true", "submit": "true", "max_devices": "3"}}); err != nil {
		t.Fatal(err)
	}
	response := callClient(router, "POST", "/api/v2/client/chat", `{"user_query":"问题"}`, token)
	if response.Code != 200 || response.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("事件接口失败: %d", response.Code)
	}
	count := 0
	for _, line := range strings.Split(response.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event agent.Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		count++
		if event.Sequence != count {
			t.Fatal("事件序号不连续")
		}
		if event.Type == agent.AnswerDelta && event.Text != "第一行\n第二行" {
			t.Fatal("JSON 帧损坏了换行")
		}
	}
	if count != 3 {
		t.Fatalf("事件未完整发送: %d", count)
	}
}
