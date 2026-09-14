package tui

import (
	"context"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"sync"
)

// requestScope 跟踪终端会话发起的请求，退出时禁止新请求并等待清理完成。
type requestScope struct {
	next    admin.Executor
	mutex   sync.Mutex
	workers sync.WaitGroup
	closed  bool
}

// Execute 在受保护的会话范围内登记请求，避免关闭与迟到命令竞态。
func (scope *requestScope) Execute(ctx context.Context, request admin.Request) (admin.Response, error) {
	scope.mutex.Lock()
	if scope.closed {
		scope.mutex.Unlock()
		return admin.Response{}, context.Canceled
	}
	scope.workers.Add(1)
	scope.mutex.Unlock()
	defer scope.workers.Done()
	return scope.next.Execute(ctx, request)
}

// closeAndWait 禁止迟到命令进入后等待现有请求结束，离线恢复不会提前释放进程锁。
func (scope *requestScope) closeAndWait() {
	scope.mutex.Lock()
	scope.closed = true
	scope.mutex.Unlock()
	scope.workers.Wait()
}
