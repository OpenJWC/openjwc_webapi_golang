package admin

// Pagination 是同一查询快照中的分页元数据，索引从零开始。
type Pagination struct {
	Index int `json:"index"`
	Size  int `json:"size"`
	Total int `json:"total"`
}

// Pages 返回可展示页数，空结果也保留一个空页面。
func (page Pagination) Pages() int {
	if page.Size < 1 {
		return 1
	}
	return max(1, (page.Total+page.Size-1)/page.Size)
}
