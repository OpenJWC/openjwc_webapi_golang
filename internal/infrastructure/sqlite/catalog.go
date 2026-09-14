package sqlite

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// noticeQuery 从封闭排序与参数化条件构造查询，通用参数只存在于 SQL 边界。
func noticeQuery(filter notice.Filter) (string, string, string, []interface{}) {
	from := "FROM notices n "
	where := "WHERE 1=1"
	order := " ORDER BY n.published_at DESC,n.id DESC"
	var args []interface{}
	if filter.Label != "" {
		where += " AND n.label=?"
		args = append(args, filter.Label)
	}
	if !filter.Since.IsZero() {
		where += " AND n.published_at>=?"
		args = append(args, filter.Since.UTC().Format(time.RFC3339))
	}
	if !filter.Before.IsZero() {
		where += " AND n.published_at<?"
		args = append(args, filter.Before.UTC().Format(time.RFC3339))
	}
	query := strings.TrimSpace(filter.Query)
	if query != "" {
		if utf8.RuneCountInString(query) < 3 {
			where += " AND (instr(n.title,?)>0 OR instr(n.content,?)>0)"
			args = append(args, query, query)
		} else {
			from = "FROM notice_search JOIN notices n ON n.rowid=notice_search.rowid "
			where += " AND notice_search MATCH ?"
			args = append(args, `"`+strings.ReplaceAll(query, `"`, `""`)+`"`)
			if filter.Order == notice.Relevance {
				order = " ORDER BY bm25(notice_search,5.0,1.0),n.published_at DESC,n.id DESC"
			}
		}
	}
	return from, where, order, args
}

// Catalog 在单个查询快照中读取覆盖范围，不把抓取时间等同于业务有效期。
func (store *Store) Catalog(ctx context.Context) (notice.Catalog, error) {
	var result notice.Catalog
	var first, latest, last string
	err := store.reader.QueryRowContext(ctx, `SELECT count(*),coalesce(min(published_at),''),coalesce(max(published_at),''),
 coalesce((SELECT last_success FROM job_state WHERE name='crawler'),'') FROM notices`).Scan(&result.Total, &first, &latest, &last)
	if err != nil {
		return result, err
	}
	for _, entry := range []struct {
		value  string
		target *time.Time
	}{{value: first, target: &result.First}, {value: latest, target: &result.Latest}, {value: last, target: &result.LastCrawl}} {
		if entry.value == "" {
			continue
		}
		*entry.target, err = time.Parse(time.RFC3339Nano, entry.value)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
