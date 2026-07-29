package submission_test

import (
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/submission"
)

func TestReviewRejectsRepeatedTransition(t *testing.T) {
	createdAt := time.Now().UTC()
	item, err := submission.New(submission.CreateInput{
		ID:          "submission-1",
		SubmitterID: "user-1",
		Title:       "测试投稿",
		Content:     "测试正文",
		CreatedAt:   createdAt,
	})
	if err != nil {
		t.Fatalf("创建投稿失败: %v", err)
	}

	item, err = item.Review(submission.StatusApproved, "内容有效", createdAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("审核投稿失败: %v", err)
	}
	if _, err = item.Review(submission.StatusRejected, "重复审核", createdAt.Add(time.Hour)); err == nil {
		t.Fatal("已审核投稿不应再次发生状态变更")
	}
}
