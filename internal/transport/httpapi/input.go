package httpapi

// unbindInput 是解绑请求的协议结构。
type unbindInput struct {
	UUID string `json:"device_uuid"`
}

// searchInput 保留旧搜索接口的可空阈值与默认结果数量。
type searchInput struct {
	Query   string   `json:"query"`
	TopK    *int     `json:"top_k"`
	Minimum *float64 `json:"min_similarity"`
}

// validationIssue 保留 FastAPI 校验错误的类型、位置与消息字段。
type validationIssue struct {
	Type     string   `json:"type"`
	Location []string `json:"loc"`
	Message  string   `json:"msg"`
}
