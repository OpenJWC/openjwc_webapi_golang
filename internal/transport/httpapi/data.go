package httpapi

import "github.com/OpenJWC/openjwc_webapi_golang/internal/service/contribution"

// mottoData 是每日一言的显式响应数据。
type mottoData struct {
	Text   string  `json:"text"`
	Author *string `json:"author"`
}

// submissionListData 是当前用户投稿的响应数据。
type submissionListData struct {
	Total   int                    `json:"total"`
	Notices []contribution.Summary `json:"notices"`
}

// validationData 是请求校验失败的兼容响应结构。
type validationData struct {
	Detail []validationIssue `json:"detail"`
}
