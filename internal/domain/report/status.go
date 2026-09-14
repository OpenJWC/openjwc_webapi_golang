package report

// Status 定义日报持久化阶段，不与正文或日期字符串混用。
type Status string

const (
	Running   Status = "running"
	Failed    Status = "failed"
	Completed Status = "completed"
)

// Valid 判断状态是否属于允许持久化的日报阶段。
func (status Status) Valid() bool {
	return status == Running || status == Failed || status == Completed
}
