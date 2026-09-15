package crawljob

import (
	"context"
	"errors"
	"time"
)

// ticket 关联单个任务身份、名称选择与独立的完成通知。
type ticket struct {
	id        string
	selection []string
	done      chan result
}

// result 只传给该任务的等待者，不读取可能已经切换的全局状态。
type result struct {
	count int
	err   error
}

// Serve 拥有唯一任务执行循环，服务退出时清理运行中及已接纳的排队任务。
func (manager *Manager) Serve(ctx context.Context) {
	defer func() {
		manager.mutex.Lock()
		manager.stopped = true
		manager.active = false
		manager.state = "stopped"
		manager.cancel = nil
		manager.mutex.Unlock()
		select {
		case job := <-manager.queue:
			job.done <- result{err: context.Canceled}
		default:
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-manager.queue:
			manager.execute(ctx, job)
		}
	}
}

// execute 使用服务拥有的限时上下文，统一更新手动和计划任务的成功时间。
func (manager *Manager) execute(parent context.Context, job ticket) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	manager.mutex.Lock()
	manager.cancel = cancel
	manager.state = "running"
	if manager.cancelPending {
		cancel()
	}
	manager.mutex.Unlock()
	count, err := manager.runner.RunObserved(ctx, job.selection, manager.store.SaveSource)
	if err == nil {
		err = manager.store.MarkJobSuccess(ctx, "crawler")
	}
	cancel()
	state := "completed"
	if err != nil {
		state = "failed"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		state = "canceled"
	}
	manager.mutex.Lock()
	manager.cancel = nil
	manager.active = false
	manager.state = state
	if err != nil {
		reason := []rune(err.Error())
		manager.detail = "爬虫任务未完成：" + string(reason[:min(512, len(reason))])
	}
	manager.mutex.Unlock()
	job.done <- result{count: count, err: err}
}
