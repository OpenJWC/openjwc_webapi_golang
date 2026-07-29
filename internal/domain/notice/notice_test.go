package notice_test

import (
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

func TestNewCopiesAttachments(t *testing.T) {
	attachments := []notice.Attachment{{Name: "日程.pdf", URL: "https://example.com/calendar.pdf"}}
	item, err := notice.New(notice.CreateInput{
		ID:          "notice-1",
		Title:       "考试安排",
		PublishedAt: time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC),
		DetailURL:   "https://example.com/notices/1",
		Attachments: attachments,
	})
	if err != nil {
		t.Fatalf("创建通知失败: %v", err)
	}

	attachments[0].Name = "被篡改的名称"
	if item.Attachments()[0].Name != "日程.pdf" {
		t.Fatal("通知不应持有调用方附件切片的可变引用")
	}
}
