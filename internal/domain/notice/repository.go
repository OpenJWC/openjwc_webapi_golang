package notice

import "context"

// Filter 表示通知列表的筛选和分页条件。
type Filter struct {
	Label  string
	Limit  int
	Offset int
}

// Repository 定义通知持久化所需的最小能力。
type Repository interface {
	Save(context.Context, Notice) error
	FindByID(context.Context, string) (Notice, bool, error)
	List(context.Context, Filter) ([]Notice, int, error)
}
