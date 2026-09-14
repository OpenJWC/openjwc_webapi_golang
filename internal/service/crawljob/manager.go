package crawljob

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/crawler"
)

// Runner 提供单次抓取及值快照观察能力。
type Runner interface {
	RunObserved(context.Context, crawler.Observer) (int, error)
}

// Store 保存进度和全任务成功时间。
type Store interface {
	SaveSource(context.Context, crawl.Source) error
	CrawlSources(context.Context) ([]crawl.Source, error)
	MarkJobSuccess(context.Context, string) error
}

// Manager 拥有有界任务队列，互斥锁保护任务身份、取消函数与状态。
type Manager struct {
	runner        Runner
	store         Store
	mutex         sync.Mutex
	queue         chan ticket
	id            string
	state         string
	detail        string
	active        bool
	stopped       bool
	cancel        context.CancelFunc
	cancelPending bool
}

// New 创建尚未启动协程的任务管理器，Serve 由应用生命周期拥有。
func New(runner Runner, store Store) *Manager {
	return &Manager{runner: runner, store: store, queue: make(chan ticket, 1), state: "idle"}
}

// enqueue 原子接纳唯一任务，调用者上下文不成为任务的父上下文。
func (manager *Manager) enqueue(ctx context.Context) (ticket, error) {
	if err := ctx.Err(); err != nil {
		return ticket{}, err
	}
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if manager.stopped {
		return ticket{}, fmt.Errorf("服务任务调度已停止")
	}
	if manager.active {
		return ticket{}, fmt.Errorf("爬虫任务已在运行")
	}
	job := ticket{id: fmt.Sprintf("%d", time.Now().UnixNano()), done: make(chan result, 1)}
	manager.id = job.id
	manager.state = "queued"
	manager.detail = ""
	manager.active = true
	manager.cancelPending = false
	manager.queue <- job
	return job, nil
}

// Start 接纳手动任务后立即返回，关闭 TUI 不会取消该任务。
func (manager *Manager) Start(ctx context.Context) (string, error) {
	job, err := manager.enqueue(ctx)
	return job.id, err
}

// Run 为调度器提供同步等待入口，超时仅取消自己发起的任务。
func (manager *Manager) Run(ctx context.Context) (int, error) {
	job, err := manager.enqueue(ctx)
	if err != nil {
		return 0, err
	}
	select {
	case result := <-job.done:
		return result.count, result.err
	case <-ctx.Done():
		_ = manager.Cancel(context.Background(), job.id)
		return 0, ctx.Err()
	}
}

// Cancel 校验任务身份后取消当前任务，不会误伤后来创建的任务。
func (manager *Manager) Cancel(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if !manager.active || id == "" || id != manager.id {
		return fmt.Errorf("任务已结束或标识不匹配")
	}
	manager.cancelPending = true
	manager.state = "canceling"
	if manager.cancel != nil {
		manager.cancel()
	}
	return nil
}
