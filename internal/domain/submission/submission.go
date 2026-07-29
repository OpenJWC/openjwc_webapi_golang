package submission

import (
	"fmt"
	"strings"
	"time"
)

// Status 表示投稿的审核状态。
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// CreateInput 表示创建投稿所需的数据。
type CreateInput struct {
	ID          string
	SubmitterID string
	Label       string
	Title       string
	PublishedAt time.Time
	DetailURL   string
	IsPage      bool
	Content     string
	Attachments []string
	CreatedAt   time.Time
}

// Submission 表示等待管理员审核的用户投稿。
type Submission struct {
	id          string
	submitterID string
	label       string
	title       string
	publishedAt time.Time
	detailURL   string
	isPage      bool
	content     string
	attachments []string
	status      Status
	review      string
	createdAt   time.Time
	updatedAt   time.Time
}

// New 创建一条待审核投稿。
func New(input CreateInput) (Submission, error) {
	if strings.TrimSpace(input.ID) == "" {
		return Submission{}, fmt.Errorf("投稿 ID 不能为空")
	}
	if strings.TrimSpace(input.SubmitterID) == "" {
		return Submission{}, fmt.Errorf("投稿者 ID 不能为空")
	}
	if strings.TrimSpace(input.Title) == "" {
		return Submission{}, fmt.Errorf("投稿标题不能为空")
	}
	if strings.TrimSpace(input.Content) == "" {
		return Submission{}, fmt.Errorf("投稿正文不能为空")
	}
	if input.CreatedAt.IsZero() {
		return Submission{}, fmt.Errorf("投稿创建时间不能为空")
	}

	return Submission{
		id:          strings.TrimSpace(input.ID),
		submitterID: strings.TrimSpace(input.SubmitterID),
		label:       strings.TrimSpace(input.Label),
		title:       strings.TrimSpace(input.Title),
		publishedAt: input.PublishedAt.UTC(),
		detailURL:   strings.TrimSpace(input.DetailURL),
		isPage:      input.IsPage,
		content:     strings.TrimSpace(input.Content),
		attachments: append([]string(nil), input.Attachments...),
		status:      StatusPending,
		createdAt:   input.CreatedAt.UTC(),
		updatedAt:   input.CreatedAt.UTC(),
	}, nil
}

// Review 返回完成审核后的投稿副本。
func (submission Submission) Review(status Status, review string, reviewedAt time.Time) (Submission, error) {
	if submission.status != StatusPending {
		return Submission{}, fmt.Errorf("仅待审核投稿可以变更审核状态")
	}
	if status != StatusApproved && status != StatusRejected {
		return Submission{}, fmt.Errorf("审核结果必须是通过或拒绝")
	}
	if reviewedAt.Before(submission.createdAt) {
		return Submission{}, fmt.Errorf("审核时间不能早于投稿时间")
	}

	reviewed := submission
	reviewed.status = status
	reviewed.review = strings.TrimSpace(review)
	reviewed.updatedAt = reviewedAt.UTC()
	reviewed.attachments = append([]string(nil), submission.attachments...)
	return reviewed, nil
}

// ID 返回投稿标识。
func (submission Submission) ID() string {
	return submission.id
}

// SubmitterID 返回投稿者标识。
func (submission Submission) SubmitterID() string {
	return submission.submitterID
}

// Label 返回投稿标签。
func (submission Submission) Label() string {
	return submission.label
}

// Title 返回投稿标题。
func (submission Submission) Title() string {
	return submission.title
}

// PublishedAt 返回资讯发布日期。
func (submission Submission) PublishedAt() time.Time {
	return submission.publishedAt
}

// DetailURL 返回投稿详情链接。
func (submission Submission) DetailURL() string {
	return submission.detailURL
}

// IsPage 返回详情链接是否指向网页。
func (submission Submission) IsPage() bool {
	return submission.isPage
}

// Content 返回投稿正文。
func (submission Submission) Content() string {
	return submission.content
}

// Attachments 返回附件链接副本。
func (submission Submission) Attachments() []string {
	return append([]string(nil), submission.attachments...)
}

// Status 返回当前审核状态。
func (submission Submission) Status() Status {
	return submission.status
}

// ReviewNote 返回审核意见。
func (submission Submission) ReviewNote() string {
	return submission.review
}

// CreatedAt 返回投稿创建时间。
func (submission Submission) CreatedAt() time.Time {
	return submission.createdAt
}

// UpdatedAt 返回投稿最后更新时间。
func (submission Submission) UpdatedAt() time.Time {
	return submission.updatedAt
}
