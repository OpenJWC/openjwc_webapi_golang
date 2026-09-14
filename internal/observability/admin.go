package observability

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// Monitor 为本地管理操作补充当前进程监控和日志摘要。
type Monitor struct {
	next    admin.Executor
	logs    *Buffer
	started time.Time
}

// NewMonitor 组合管理分派器与有界日志缓冲。
func NewMonitor(next admin.Executor, logs *Buffer) *Monitor {
	return &Monitor{next: next, logs: logs, started: time.Now()}
}

// Execute 为监控和日志提供只读投影，其余动作仍由原用例处理。
func (monitor *Monitor) Execute(ctx context.Context, request admin.Request) (admin.Response, error) {
	if err := request.Validate(); err != nil {
		return admin.Response{}, err
	}
	if request.Action == admin.ActionLogs {
		return admin.Response{Columns: []string{"时间", "等级", "摘要"}, Rows: monitor.logs.Rows()}, nil
	}
	response, err := monitor.next.Execute(ctx, request)
	if err != nil {
		return response, err
	}
	if request.Action == admin.ActionStats {
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		response.Text = fmt.Sprintf("运行 %s · goroutine %d · Go 堆 %.1f MiB · CPU 逻辑核 %d", time.Since(monitor.started).Round(time.Second), runtime.NumGoroutine(), float64(memory.HeapAlloc)/(1<<20), runtime.NumCPU())
	}
	return response, nil
}
