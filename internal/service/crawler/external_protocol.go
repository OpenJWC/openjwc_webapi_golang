package crawler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// protocolRequest 是通过标准输入发送给外部爬虫的唯一请求。
type protocolRequest struct {
	Version  int    `json:"version"`
	Type     string `json:"type"`
	RunID    string `json:"run_id"`
	Source   string `json:"source"`
	Since    string `json:"since"`
	MaxPages int    `json:"max_pages"`
}

// protocolEvent 是外部爬虫逐行输出的进度、资讯或终止事件。
type protocolEvent struct {
	Version int             `json:"version"`
	Type    string          `json:"type"`
	Scanned *int            `json:"scanned,omitempty"`
	Skipped *int            `json:"skipped,omitempty"`
	Failed  *int            `json:"failed,omitempty"`
	Detail  string          `json:"detail,omitempty"`
	Notice  *protocolNotice `json:"notice,omitempty"`
}

// protocolNotice 是不信任的外部资讯输入，主程序负责生成 ID 和校验。
type protocolNotice struct {
	Label       string               `json:"label"`
	Title       string               `json:"title"`
	PublishedAt string               `json:"published_at"`
	DetailURL   string               `json:"detail_url"`
	IsPage      bool                 `json:"is_page"`
	Content     string               `json:"content"`
	Attachments []protocolAttachment `json:"attachments,omitempty"`
}

// protocolAttachment 是语言无关的小写 JSON 附件结构。
type protocolAttachment struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// createInput 将外部输入转换为有界领域构造参数并保持 URL 身份规则。
func (input protocolNotice) createInput() (notice.CreateInput, error) {
	if len(input.Label) > 256 || len(input.Title) > 1024 || len(input.Content) > 2<<20 || len(input.DetailURL) > 2048 || len(input.Attachments) > 50 {
		return notice.CreateInput{}, fmt.Errorf("外部资讯字段超出范围")
	}
	published, err := time.Parse(time.RFC3339, input.PublishedAt)
	if err != nil {
		return notice.CreateInput{}, fmt.Errorf("外部资讯发布时间必须为 RFC3339")
	}
	address, err := validExternalURL(input.DetailURL)
	if err != nil {
		return notice.CreateInput{}, err
	}
	attachments := make([]notice.Attachment, 0, len(input.Attachments))
	for _, attachment := range input.Attachments {
		if len(attachment.Name) > 512 || len(attachment.URL) > 2048 {
			return notice.CreateInput{}, fmt.Errorf("外部资讯附件超出范围")
		}
		if _, err = validExternalURL(attachment.URL); err != nil {
			return notice.CreateInput{}, fmt.Errorf("外部资讯附件地址无效")
		}
		attachments = append(attachments, notice.Attachment{Name: attachment.Name, URL: attachment.URL})
	}
	return notice.CreateInput{ID: noticeID(address), Label: input.Label, Title: input.Title, PublishedAt: published, DetailURL: address.String(), IsPage: input.IsPage, Content: input.Content, Attachments: attachments}, nil
}

// noticeID 保持按规范详情 URL 计算 SHA-256 身份的兼容规则。
func noticeID(address *url.URL) string {
	digest := sha256.Sum256([]byte(address.String()))
	return hex.EncodeToString(digest[:])
}

// validExternalURL 接受无凭据的 HTTP(S) 绝对地址并移除片段。
func validExternalURL(value string) (*url.URL, error) {
	address, err := url.Parse(strings.TrimSpace(value))
	if err != nil || address.Host == "" || address.User != nil || address.Scheme != "http" && address.Scheme != "https" {
		return nil, fmt.Errorf("外部资讯地址无效")
	}
	return address, nil
}
