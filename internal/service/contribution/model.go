package contribution

import (
	"context"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/submission"
)

// Summary 是投稿列表的公开只读投影。
type Summary struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Title  string `json:"title"`
	Date   string `json:"date"`
	URL    string `json:"detail_url"`
	IsPage bool   `json:"is_page"`
	Status string `json:"status"`
	Review string `json:"review"`
}

// Repository 定义用户投稿的存储能力。
type Repository interface {
	Submit(context.Context, submission.Submission) error
	Submissions(context.Context, int64) ([]Summary, error)
}
