package crawler

import (
	"context"
	"testing"
)

// fakeRunner 记录自身是否被组执行。
type fakeRunner struct {
	name string
	ran  bool
}

// Name 返回该运行单元名称。
func (runner *fakeRunner) Name() string { return runner.name }

// RunObserved 标记执行并返回固定结果。
func (runner *fakeRunner) RunObserved(ctx context.Context, observe Observer) (int, error) {
	runner.ran = true
	return 1, nil
}

// TestGroupRunsSelectedNames 验证按名称选择只运行匹配的运行单元。
func TestGroupRunsSelectedNames(t *testing.T) {
	a, b, c := &fakeRunner{name: "a"}, &fakeRunner{name: "b"}, &fakeRunner{name: "c"}
	count, err := NewGroup(a, b, c).RunObserved(context.Background(), []string{"a", "c"}, nil)
	if err != nil || count != 2 {
		t.Fatalf("选择运行结果错误: %d %v", count, err)
	}
	if !a.ran || b.ran || !c.ran {
		t.Fatalf("选择集合错误: a=%v b=%v c=%v", a.ran, b.ran, c.ran)
	}
}

// TestGroupRunsAllWhenNamesEmpty 验证空选择运行全部运行单元。
func TestGroupRunsAllWhenNamesEmpty(t *testing.T) {
	a, b := &fakeRunner{name: "a"}, &fakeRunner{name: "b"}
	count, err := NewGroup(a, b).RunObserved(context.Background(), nil, nil)
	if err != nil || count != 2 || !a.ran || !b.ran {
		t.Fatalf("空选择未运行全部: %d %v a=%v b=%v", count, err, a.ran, b.ran)
	}
}
