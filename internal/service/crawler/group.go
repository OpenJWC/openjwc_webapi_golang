package crawler

import (
	"context"
	"errors"
)

// ObservedRunner 是内置与外部爬虫共享的任务接口。
type ObservedRunner interface {
	RunObserved(context.Context, Observer) (int, error)
}

// Group 按配置顺序串行运行爬虫，避免同时冲击学校站点和 SQLite 写入器。
type Group struct {
	runners []ObservedRunner
}

// NewGroup 创建不可变的爬虫运行序列。
func NewGroup(runners ...ObservedRunner) *Group {
	return &Group{runners: append([]ObservedRunner(nil), runners...)}
}

// RunObserved 保留每个来源的部分成果并汇总失败，取消时不再启动后续程序。
func (group *Group) RunObserved(ctx context.Context, observe Observer) (int, error) {
	total := 0
	var failures []error
	for _, runner := range group.runners {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		count, err := runner.RunObserved(ctx, observe)
		total += count
		if err != nil {
			failures = append(failures, err)
		}
	}
	return total, errors.Join(failures...)
}
