package notice

import (
	"context"
	"time"
)

// Filter 表示通知列表的筛选和分页条件。
type Filter struct {
	Query  string
	Since  time.Time
	Before time.Time
	Order  Order
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

// Order 定义唯一允许的资讯排序策略。
type Order string

const (
	Newest    Order = "newest"
	Relevance Order = "relevance"
)

// Catalog 是资讯覆盖范围与抓取时间的只读统计投影。
type Catalog struct {
	Total     int
	First     time.Time
	Latest    time.Time
	LastCrawl time.Time
}
