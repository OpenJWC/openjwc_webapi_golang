package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// noticeDTO 保留旧接口日期、正文和附件字段。
type noticeDTO struct {
	ID          string   `json:"id"`
	Label       *string  `json:"label"`
	Title       string   `json:"title"`
	Date        string   `json:"date"`
	URL         string   `json:"detail_url"`
	IsPage      bool     `json:"is_page"`
	Content     string   `json:"content_text"`
	Attachments []string `json:"attachments"`
}

// noticeListDTO 定义资讯列表响应的数据部分。
type noticeListDTO struct {
	Total   int         `json:"total_returned"`
	Labels  int         `json:"total_label"`
	Notices []noticeDTO `json:"notices"`
}

// toNoticeDTO 复制领域数据并规范空数组与可空标签。
func toNoticeDTO(item notice.Notice) noticeDTO {
	result := noticeDTO{ID: item.ID(), Title: item.Title(), Date: item.PublishedAt().Format("2006-01-02"), URL: item.DetailURL(), IsPage: item.IsPage(), Content: item.Content(), Attachments: make([]string, 0)}
	if strings.HasPrefix(result.URL, "urn:openjwc:submission:") {
		result.URL = ""
	}
	if label := item.Label(); label != "" {
		result.Label = &label
	}
	for _, attachment := range item.Attachments() {
		result.Attachments = append(result.Attachments, attachment.URL)
	}
	return result
}

// notices 校验查询参数后返回稳定分页结果。
func (client *Client) notices(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	page, size := 1, 20
	var err error
	if text := request.URL.Query().Get("page"); text != "" {
		page, err = strconv.Atoi(text)
	}
	if err != nil || page < 1 || page > 20001 {
		invalidInput(writer, "page 超出范围")
		return
	}
	if text := request.URL.Query().Get("size"); text != "" {
		size, err = strconv.Atoi(text)
	}
	label := request.URL.Query().Get("label")
	if err != nil || size < 1 || size > 50 || len(label) > 256 {
		invalidInput(writer, "size 或 label 超出范围")
		return
	}
	items, total, err := client.dependencies.Notices.List(request.Context(), notice.Filter{Label: label, Limit: size, Offset: (page - 1) * size})
	if err != nil {
		client.fail(writer, err)
		return
	}
	labels, err := client.dependencies.Notices.Labels(request.Context())
	if err != nil {
		client.fail(writer, err)
		return
	}
	data := noticeListDTO{Total: total, Labels: len(labels), Notices: make([]noticeDTO, 0, len(items))}
	for _, item := range items {
		data.Notices = append(data.Notices, toNoticeDTO(item))
	}
	writeJSON(writer, 200, envelope[noticeListDTO]{Message: "获取成功", Data: data})
}

// labels 返回不含空值的标签列表。
func (client *Client) labels(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	items, err := client.dependencies.Notices.Labels(request.Context())
	if err != nil {
		client.fail(writer, err)
		return
	}
	writeJSON(writer, 200, envelope[map[string][]string]{Message: "获取成功", Data: map[string][]string{"labels": items}})
}

// search 保留旧搜索响应形状，匹配分数表示字面命中而非向量概率。
func (client *Client) search(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	writer.Header().Set("X-OpenJWC-Search-Mode", "lexical")
	writer.Header().Set("X-OpenJWC-Similarity", "deprecated-placeholder")
	var input searchInput
	if !decodeBody(writer, request, &input) {
		return
	}
	if strings.TrimSpace(input.Query) == "" || utf8.RuneCountInString(input.Query) > 200 {
		invalidInput(writer, "query 长度必须为 1～200 字符")
		return
	}
	limit := 5
	if input.TopK != nil {
		limit = max(1, min(*input.TopK, 20))
	}
	items, err := client.dependencies.Notices.Search(request.Context(), input.Query, limit)
	if err != nil {
		client.fail(writer, err)
		return
	}
	results := make([]searchDTO, 0, len(items))
	if input.Minimum == nil || *input.Minimum <= 1 {
		for _, item := range items {
			dto := toNoticeDTO(item)
			results = append(results, searchDTO{ID: dto.ID, Label: dto.Label, Title: dto.Title, Date: dto.Date, URL: dto.URL, IsPage: dto.IsPage, Similarity: 1})
		}
	}
	writeJSON(writer, 200, envelope[searchResultDTO]{Message: "搜索成功", Data: searchResultDTO{Total: len(results), Results: results}})
}
