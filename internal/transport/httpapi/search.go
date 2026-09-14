package httpapi

import (
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"net/http"
)

// searchDTO 保留旧客户端使用的搜索结果字段。
type searchDTO struct {
	ID         string  `json:"id"`
	Label      *string `json:"label"`
	Title      string  `json:"title"`
	Date       string  `json:"date"`
	URL        string  `json:"detail_url"`
	IsPage     bool    `json:"is_page"`
	Similarity float64 `json:"similarity_score"`
	Distance   float64 `json:"distance"`
}

// searchResultDTO 聚合有界检索结果。
type searchResultDTO struct {
	Total   int         `json:"total_found"`
	Results []searchDTO `json:"results"`
}

// motto 返回管理员设置的每日一言，不依赖启动时的外部网络服务。
func (client *Client) motto(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	values, err := client.dependencies.Settings.Settings(request.Context())
	if err != nil {
		client.fail(writer, err)
		return
	}
	var author *string
	if value := values["motto_author"]; value != "" && value != "佚名" {
		author = &value
	}
	data := mottoData{Text: values["motto_text"], Author: author}
	writeJSON(writer, 200, envelope[mottoData]{Message: "每日一言获取成功", Data: data})
}
