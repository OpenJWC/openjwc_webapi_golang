package notice

import (
	"fmt"
	"strings"
	"time"
)

// Attachment 表示通知正文关联的附件。
type Attachment struct {
	Name string
	URL  string
}

// CreateInput 表示从爬虫或投稿审核结果创建通知所需的数据。
type CreateInput struct {
	ID          string
	Label       string
	Title       string
	PublishedAt time.Time
	DetailURL   string
	IsPage      bool
	Content     string
	Attachments []Attachment
}

// Notice 表示一条已发布的教务通知。
type Notice struct {
	id          string
	label       string
	title       string
	publishedAt time.Time
	detailURL   string
	isPage      bool
	content     string
	attachments []Attachment
}

// New 创建并校验一条通知。
func New(input CreateInput) (Notice, error) {
	if strings.TrimSpace(input.ID) == "" {
		return Notice{}, fmt.Errorf("通知 ID 不能为空")
	}
	if strings.TrimSpace(input.Title) == "" {
		return Notice{}, fmt.Errorf("通知标题不能为空")
	}
	if input.PublishedAt.IsZero() {
		return Notice{}, fmt.Errorf("通知发布日期不能为空")
	}
	if strings.TrimSpace(input.DetailURL) == "" {
		return Notice{}, fmt.Errorf("通知详情链接不能为空")
	}

	attachments := append([]Attachment(nil), input.Attachments...)
	return Notice{
		id:          strings.TrimSpace(input.ID),
		label:       strings.TrimSpace(input.Label),
		title:       strings.TrimSpace(input.Title),
		publishedAt: input.PublishedAt.UTC(),
		detailURL:   strings.TrimSpace(input.DetailURL),
		isPage:      input.IsPage,
		content:     strings.TrimSpace(input.Content),
		attachments: attachments,
	}, nil
}

// ID 返回通知的稳定标识。
func (notice Notice) ID() string {
	return notice.id
}

// Label 返回通知标签。
func (notice Notice) Label() string {
	return notice.label
}

// Title 返回通知标题。
func (notice Notice) Title() string {
	return notice.title
}

// PublishedAt 返回通知发布日期。
func (notice Notice) PublishedAt() time.Time {
	return notice.publishedAt
}

// DetailURL 返回通知详情链接。
func (notice Notice) DetailURL() string {
	return notice.detailURL
}

// IsPage 返回详情链接是否指向网页。
func (notice Notice) IsPage() bool {
	return notice.isPage
}

// Content 返回通知正文。
func (notice Notice) Content() string {
	return notice.content
}

// Attachments 返回附件副本，避免调用方修改领域对象状态。
func (notice Notice) Attachments() []Attachment {
	return append([]Attachment(nil), notice.attachments...)
}
