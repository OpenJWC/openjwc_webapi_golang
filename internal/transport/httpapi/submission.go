package httpapi

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/submission"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/contribution"
)

// SubmissionService 定义客户端投稿依赖的最小端口。
type SubmissionService = contribution.Repository

// submissionContentDTO 保留旧客户端的嵌套投稿正文结构。
type submissionContentDTO struct {
	Text string   `json:"text"`
	URLs []string `json:"attachment_urls"`
}

// submissionInputDTO 是投稿请求的显式协议结构。
type submissionInputDTO struct {
	Title   string               `json:"title"`
	Label   string               `json:"label"`
	Date    string               `json:"date"`
	URL     string               `json:"detail_url"`
	IsPage  bool                 `json:"is_page"`
	Content submissionContentDTO `json:"content"`
}

// submit 校验大小和日期后创建待审核领域对象。
func (client *Client) submit(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	var input submissionInputDTO
	if !decodeBody(writer, request, &input) {
		return
	}
	settings, err := client.dependencies.Settings.Settings(request.Context())
	if err != nil {
		client.fail(writer, err)
		return
	}
	limit, err := strconv.Atoi(settings["submission_max_length"])
	if err != nil {
		client.fail(writer, err)
		return
	}
	published, err := time.Parse("2006-01-02", input.Date)
	if err != nil || len(input.Title) > 1024 || len(input.Label) > 256 || len(input.URL) > 2048 || utf8.RuneCountInString(input.Content.Text) > limit || len(input.Content.URLs) > 50 {
		invalidInput(writer, "投稿日期或字段长度无效")
		return
	}
	for _, url := range input.Content.URLs {
		if len(url) > 2048 {
			invalidInput(writer, "附件地址超出长度限制")
			return
		}
	}
	item, err := submission.New(submission.CreateInput{
		ID:          fmt.Sprintf("%x", sha256.Sum256([]byte(input.Title+input.Date+input.URL))),
		SubmitterID: strconv.FormatInt(principal.ID(), 10),
		Title:       input.Title,
		Label:       input.Label,
		PublishedAt: published,
		DetailURL:   input.URL,
		IsPage:      input.IsPage,
		Content:     input.Content.Text,
		Attachments: input.Content.URLs,
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		invalidInput(writer, err.Error())
		return
	}
	if err = client.dependencies.Submissions.Submit(request.Context(), item); err != nil {
		if errors.Is(err, access.ErrConflict) {
			writeJSON(writer, 422, envelope[struct{}]{Message: "投稿已存在或提交失败"})
			return
		}
		client.fail(writer, err)
		return
	}
	writeJSON(writer, 200, envelope[struct{}]{Message: "提交成功"})
}

// mySubmissions 仅返回当前用户的投稿，不允许指定其他用户标识。
func (client *Client) mySubmissions(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	items, err := client.dependencies.Submissions.Submissions(request.Context(), principal.ID())
	if err != nil {
		client.fail(writer, err)
		return
	}
	data := submissionListData{Total: len(items), Notices: items}
	writeJSON(writer, 200, envelope[submissionListData]{Message: "提交成功", Data: data})
}
