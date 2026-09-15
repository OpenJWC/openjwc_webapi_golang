package crawljob

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/crawler"
)

// controlledRunner 通过握手通道检查任务上下文，不依赖任意延时。
type controlledRunner struct {
	started  chan struct{}
	check    chan struct{}
	observed chan error
}

// RunObserved 等待显式取消并报告该来源的取消状态。
func (runner controlledRunner) RunObserved(ctx context.Context, names []string, observe crawler.Observer) (int, error) {
	source := crawl.Source{Name: "jwc", State: crawl.Running, StartedAt: time.Now().UTC()}
	if err := observe(ctx, source); err != nil {
		return 0, err
	}
	close(runner.started)
	select {
	case <-runner.check:
		runner.observed <- ctx.Err()
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	<-ctx.Done()
	source.State = crawl.Canceled
	source.FinishedAt = time.Now().UTC()
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	if err := observe(cleanup, source); err != nil {
		return 0, err
	}
	return 0, ctx.Err()
}

// TestManualJobSurvivesCallerCancellationAndStopsExplicitly 验证任务属于服务而非 TUI，重复启动和过期取消均被拒绝。
func TestManualJobSurvivesCallerCancellationAndStopsExplicitly(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "db", "job.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	runner := controlledRunner{started: make(chan struct{}), check: make(chan struct{}), observed: make(chan error, 1)}
	manager := New(runner, store, nil)
	owner, stop := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() { manager.Serve(owner); close(finished) }()
	defer func() { stop(); <-finished }()
	caller, cancelCaller := context.WithCancel(context.Background())
	id, err := manager.Start(caller)
	if err != nil {
		t.Fatal(err)
	}
	<-runner.started
	cancelCaller()
	close(runner.check)
	if err := <-runner.observed; err != nil {
		t.Fatalf("TUI 取消影响了后台任务: %v", err)
	}
	if _, err = manager.Start(context.Background()); err == nil {
		t.Fatal("并发启动未拒绝")
	}
	if err = manager.Cancel(context.Background(), "stale-id"); err == nil {
		t.Fatal("过期标识取消了当前任务")
	}
	if err = manager.Cancel(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	stop()
}

// TestQueuedJobCanBeCanceledBeforeWorkerStarts 验证尚未执行的任务也能被显式取消。
func TestQueuedJobCanBeCanceledBeforeWorkerStarts(t *testing.T) {
	manager := New(canceledRunner{}, discardStore{}, nil)
	job, err := manager.enqueue(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Cancel(context.Background(), job.id); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { manager.Serve(ctx); close(done) }()
	result := <-job.done
	cancel()
	<-done
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("排队取消未传播: %v", result.err)
	}
}

// canceledRunner 验证执行上下文的取消状态。
type canceledRunner struct{}

// RunObserved 返回传入的取消状态。
func (canceledRunner) RunObserved(ctx context.Context, names []string, observe crawler.Observer) (int, error) {
	return 0, ctx.Err()
}

// discardStore 接受测试进度但不连接真实数据库。
type discardStore struct{}

// SaveSource 接受测试来源快照。
func (discardStore) SaveSource(context.Context, crawl.Source) error { return nil }

// CrawlSources 返回空的持久化来源列表。
func (discardStore) CrawlSources(context.Context) ([]crawl.Source, error) { return nil, nil }

// MarkJobSuccess 接受测试成功时间。
func (discardStore) MarkJobSuccess(context.Context, string) error { return nil }
