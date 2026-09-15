package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestModelRetryRecoversFromTransientUpstreamFailure 验证瞬时上游 5xx 只重试一次并可恢复。
func TestModelRetryRecoversFromTransientUpstreamFailure(t *testing.T) {
	var finals atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input modelRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if input.ToolChoice == nil {
			_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Role: "assistant", Content: "草稿"}}}})
			return
		}
		if finals.Add(1) == 1 {
			writer.Header().Set("Retry-After", "1")
			http.Error(writer, "busy", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Role: "assistant", Content: "重试后答案"}}}})
	}))
	defer server.Close()
	answer, err := New(testSettings{url: server.URL}, agentLibrary(t)).Answer(context.Background(), Request{Query: "瞬时故障"})
	if err != nil || answer != "重试后答案" {
		t.Fatalf("瞬时故障未恢复: %q %v", answer, err)
	}
	if finals.Load() != 2 {
		t.Fatalf("收束请求重试次数错误: %d", finals.Load())
	}
}

// TestModelRetryStopsAfterSingleRetry 验证持续失败只重试一次并携带脱敏细节。
func TestModelRetryStopsAfterSingleRetry(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		http.Error(writer, "busy", http.StatusBadGateway)
	}))
	defer server.Close()
	_, err := New(testSettings{url: server.URL}, agentLibrary(t)).Answer(context.Background(), Request{Query: "持续故障"})
	var failure *Failure
	if !errors.As(err, &failure) || failure.Code != FailureUnavailable || failure.Detail != "upstream_502" || failure.UpstreamStatus != 502 {
		t.Fatalf("失败分类错误: %#v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("重试次数错误: %d", calls.Load())
	}
}

// TestAuthFailureIsNotRetried 验证鉴权类 4xx 不触发重试。
func TestAuthFailureIsNotRetried(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	if _, err := New(testSettings{url: server.URL}, agentLibrary(t)).Answer(context.Background(), Request{Query: "鉴权失败"}); err == nil {
		t.Fatal("鉴权失败未被拒绝")
	}
	if calls.Load() != 1 {
		t.Fatalf("鉴权失败不应重试: %d", calls.Load())
	}
}

// TestFinishLengthRoutesToFinalizer 验证回答达到长度上限时转入禁用工具的收束轮。
func TestFinishLengthRoutesToFinalizer(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input modelRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		if input.ToolChoice == nil {
			calls.Add(1)
			writer.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"被截断的草稿\"},\"finish_reason\":\"length\"}]}\n\n"))
			writer.Write([]byte("data: [DONE]\n\n"))
			return
		}
		if calls.Add(1); *input.ToolChoice != "none" {
			t.Error("收束请求没有禁用工具")
		}
		writer.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"收束答案\"},\"finish_reason\":\"stop\"}]}\n\n"))
		writer.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	answer, err := New(testSettings{url: server.URL}, agentLibrary(t)).Answer(context.Background(), Request{Query: "长度截断"})
	if err != nil || answer != "收束答案" {
		t.Fatalf("长度截断未转入收束: %q %v", answer, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("请求数量错误: %d", calls.Load())
	}
}

// TestFinalizerDoesNotRetryAfterEmission 验证收束轮已发布文字后不再重试。
func TestFinalizerDoesNotRetryAfterEmission(t *testing.T) {
	var finals atomic.Int64
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
		finals.Add(1)
		writer.Header().Set("Content-Type", "text/event-stream")
		writer.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"部分答案\"}}]}\n\n"))
	}))
	defer server.Close()
	var events []Event
	err := New(budgetSettings{url: server.URL}, agentLibrary(t)).Events(context.Background(), Request{Query: "收束中断"}, func(ctx context.Context, event Event) error {
		events = append(events, event)
		return nil
	})
	if err == nil {
		t.Fatal("收束流截断未被判定失败")
	}
	if finals.Load() != 1 {
		t.Fatalf("已发布文字后不应重试: %d", finals.Load())
	}
	if len(events) < 2 || events[len(events)-1].Type != RunFailed {
		t.Fatalf("缺少失败终止事件: %#v", events)
	}
	found := false
	for _, event := range events {
		if event.Type == AnswerDelta && event.Text == "部分答案" {
			found = true
		}
	}
	if !found {
		t.Fatal("已发布的中断文字未到达客户端")
	}
}
