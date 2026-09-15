package crawler

import (
	"context"
	"errors"
)

// NamedRunner 是内置与外部爬虫共享的具名任务接口，名称供按需调度选择。
type NamedRunner interface {
	Name() string
	RunObserved(context.Context, Observer) (int, error)
}

// Group 按配置顺序串行运行爬虫，避免同时冲击学校站点和 SQLite 写入器。
type Group struct {
	runners []NamedRunner
}

// NewGroup 创建不可变的爬虫运行序列。
func NewGroup(runners ...NamedRunner) *Group {
	return &Group{runners: append([]NamedRunner(nil), runners...)}
}

// RunObserved 运行指定名称的爬虫，names 为空时运行全部，取消时不再启动后续程序。
func (group *Group) RunObserved(ctx context.Context, names []string, observe Observer) (int, error) {
	selected := nameSet(names)
	total := 0
	var failures []error
	for _, runner := range group.runners {
		if selected != nil && !selected[runner.Name()] {
			continue
		}
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

// nameSet 将名称切片转为集合，nil 表示运行全部。
func nameSet(names []string) map[string]bool {
	if names == nil {
		return nil
	}
	result := make(map[string]bool, len(names))
	for _, name := range names {
		result[name] = true
	}
	return result
}
