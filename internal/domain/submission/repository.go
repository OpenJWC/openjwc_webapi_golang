package submission

import "context"

// Repository 定义投稿持久化所需的最小能力。
type Repository interface {
	Save(context.Context, Submission) error
	FindByID(context.Context, string) (Submission, bool, error)
	ListBySubmitter(context.Context, string) ([]Submission, error)
}
