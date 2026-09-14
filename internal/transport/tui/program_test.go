package tui

import (
	"context"
	"errors"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"testing"
)

// waitingExecutor 模拟等待会话取消的本地管理请求。
type waitingExecutor struct{ started chan struct{} }

// Execute 等待取消并返回原始上下文错误。
func (executor waitingExecutor) Execute(ctx context.Context, request admin.Request) (admin.Response, error) {
	close(executor.started)
	<-ctx.Done()
	return admin.Response{}, ctx.Err()
}

// TestCommandBuilderUsesSessionCancellation 验证会话所有者取消后异步管理命令可以退出。
func TestCommandBuilderUsesSessionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executor := waitingExecutor{started: make(chan struct{})}
	scope := &requestScope{next: executor}
	command := newCommandBuilder(ctx, scope)(admin.Request{Action: admin.ActionStats})
	result := make(chan resultMessage, 1)
	go func() { result <- command().(resultMessage) }()
	<-executor.started
	cancel()
	scope.closeAndWait()
	if message := <-result; !errors.Is(message.err, context.Canceled) {
		t.Fatalf("会话取消未传播: %v", message.err)
	}
	if _, err := scope.Execute(context.Background(), admin.Request{Action: admin.ActionStats}); !errors.Is(err, context.Canceled) {
		t.Fatal("关闭后的会话仍接受新请求")
	}
}
