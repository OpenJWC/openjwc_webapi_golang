package sqlite

import (
	"context"
	"database/sql"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// adminDetail 读取单条完整记录，避免列表一次加载数百篇正文。
func (store *Store) adminDetail(ctx context.Context, request admin.Request) (admin.Response, error) {
	var columns []string
	var query string
	switch request.Action {
	case admin.ActionNoticeDetail:
		columns = []string{"ID", "标签", "标题", "日期", "链接", "正文"}
		query = "SELECT id,label,title,published_at,detail_url,content FROM notices WHERE id=?"
	case admin.ActionSubmissionDetail:
		columns = []string{"ID", "用户", "标签", "标题", "日期", "链接", "正文", "附件", "状态", "审核意见"}
		query = "SELECT id,user_id,label,title,published_at,detail_url,content,attachments,status,review FROM user_submissions WHERE id=?"
	case admin.ActionReportDetail:
		columns = []string{"日期", "状态", "来源数量", "正文"}
		query = "SELECT day,status,source_count,content FROM daily_reports WHERE day=?"
	}
	values := make([]sql.NullString, len(columns))
	targets := make([]interface{}, len(columns))
	for index := range values {
		targets[index] = &values[index]
	}
	if err := store.reader.QueryRowContext(ctx, query, request.ID).Scan(targets...); err != nil {
		return admin.Response{}, err
	}
	row := make([]string, len(columns))
	for index, value := range values {
		row[index] = value.String
	}
	return admin.Response{Columns: columns, Rows: [][]string{row}}, nil
}
