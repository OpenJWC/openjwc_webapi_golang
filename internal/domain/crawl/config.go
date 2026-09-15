package crawl

import "time"

// DefaultIntervalMinutes 是未显式配置时单个爬虫的自动间隔。
const DefaultIntervalMinutes = 480

// Config 表示单个爬虫的启停与自动间隔，LastSuccess 供调度判断是否到期。
type Config struct {
	Name            string
	Enabled         bool
	IntervalMinutes int
	LastSuccess     time.Time
}
