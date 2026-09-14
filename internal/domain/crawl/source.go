package crawl

import (
	"strings"
	"time"
)

// State 描述一次来源抓取阶段，不与业务有效期混用。
type State string

const (
	Running   State = "running"
	Completed State = "completed"
	Failed    State = "failed"
	Canceled  State = "canceled"
)

// Source 是抓取结果与进度的显式只读投影，保存数包含幂等更新。
type Source struct {
	Name        string
	State       State
	Scanned     int
	Saved       int
	Skipped     int
	Failed      int
	StartedAt   time.Time
	FinishedAt  time.Time
	LastSuccess time.Time
	Detail      string
}

// BuiltinSourceName 判断名称是否由主二进制内置适配器占用。
func BuiltinSourceName(name string) bool {
	return name == "jwc" || name == "cs" || name == "xsxy"
}

// ValidSourceName 接受适合作为持久化键和界面标签的有限小写 ASCII 名称。
func ValidSourceName(name string) bool {
	if len(name) == 0 || len(name) > 64 || name != strings.ToLower(name) {
		return false
	}
	for index, char := range name {
		letter := char >= 'a' && char <= 'z'
		digit := char >= '0' && char <= '9'
		if !letter && !digit && (index == 0 || char != '-' && char != '_' && char != '.') {
			return false
		}
	}
	return true
}
