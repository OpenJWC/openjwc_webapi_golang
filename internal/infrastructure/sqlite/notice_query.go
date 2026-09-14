package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// List 返回同一只读快照中的总数和分页资讯。
func (store *Store) List(ctx context.Context, filter notice.Filter) ([]notice.Notice, int, error) {
	if filter.Limit < 1 || filter.Limit > 100 || filter.Offset < 0 || filter.Offset > 1000000 || len(filter.Label) > 256 || utf8.RuneCountInString(filter.Query) > 200 {
		return nil, 0, fmt.Errorf("资讯查询范围无效")
	}
	if filter.Order != "" && filter.Order != notice.Newest && filter.Order != notice.Relevance {
		return nil, 0, fmt.Errorf("资讯排序无效")
	}
	if !filter.Before.IsZero() && !filter.Since.IsZero() && !filter.Before.After(filter.Since) {
		return nil, 0, fmt.Errorf("资讯日期范围无效")
	}
	tx, err := store.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	from, where, order, args := noticeQuery(filter)
	var total int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) "+from+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, filter.Limit, filter.Offset)
	rows, err := tx.QueryContext(ctx, "SELECT n.id,n.label,n.title,n.published_at,n.detail_url,n.is_page,n.content "+from+where+order+" LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, 0, err
	}
	items, err := collectNotices(ctx, tx, rows)
	return items, total, err
}

// collectNotices 先释放列表游标，再在同一快照内读取关联附件。
func collectNotices(ctx context.Context, tx *sql.Tx, rows *sql.Rows) ([]notice.Notice, error) {
	defer rows.Close()
	inputs := make([]notice.CreateInput, 0)
	for rows.Next() {
		input, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	items := make([]notice.Notice, 0, len(inputs))
	for _, input := range inputs {
		var err error
		input.Attachments, err = readAttachments(ctx, tx, input.ID)
		if err != nil {
			return nil, err
		}
		item, err := notice.New(input)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// Labels 返回稳定排序且去除空值的标签集合。
func (store *Store) Labels(ctx context.Context) ([]string, error) {
	rows, err := store.reader.QueryContext(ctx, "SELECT DISTINCT label FROM notices WHERE label<>'' ORDER BY label")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	labels := make([]string, 0)
	for rows.Next() {
		var label string
		if err = rows.Scan(&label); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

// Search 按全文相关性检索，短于三个字符时用字面子串匹配补足中文短词。
func (store *Store) Search(ctx context.Context, query string, limit int) ([]notice.Notice, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > 200 || limit < 1 || limit > 50 {
		return nil, fmt.Errorf("检索词或结果数量超出范围")
	}
	tx, err := store.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var rows *sql.Rows
	if utf8.RuneCountInString(query) < 3 {
		rows, err = tx.QueryContext(ctx, `SELECT id,label,title,published_at,detail_url,is_page,content FROM notices
			WHERE instr(title,?)>0 OR instr(content,?)>0 ORDER BY published_at DESC,id DESC LIMIT ?`, query, query, limit)
	} else {
		phrase := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
		rows, err = tx.QueryContext(ctx, `SELECT n.id,n.label,n.title,n.published_at,n.detail_url,n.is_page,n.content
			FROM notice_search JOIN notices n ON n.rowid=notice_search.rowid
			WHERE notice_search MATCH ? ORDER BY bm25(notice_search,5.0,1.0),n.published_at DESC,n.id DESC LIMIT ?`, phrase, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("检索资讯: %w", err)
	}
	return collectNotices(ctx, tx, rows)
}
