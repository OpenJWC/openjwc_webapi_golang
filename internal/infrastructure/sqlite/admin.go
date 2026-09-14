package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// Execute 分派本地管理动作，所有表名和 SQL 均来自封闭分支。
func (store *Store) Execute(ctx context.Context, request admin.Request) (admin.Response, error) {
	if err := request.Validate(); err != nil {
		return admin.Response{}, err
	}
	switch request.Action {
	case admin.ActionUsers:
		return store.adminUsers(ctx, request.Page)
	case admin.ActionRegistrations:
		return store.adminTable(ctx, request.Page, []string{"ID", "用户名", "邮箱", "状态", "审核意见"}, "SELECT id,username,email,status,review FROM user_registrations ORDER BY id DESC LIMIT 200 OFFSET ?", "SELECT count(*) FROM user_registrations")
	case admin.ActionKeys:
		return store.adminTable(ctx, request.Page, []string{"ID", "所有者", "启用", "设备上限", "请求数", "关联用户"}, "SELECT id,owner_name,is_active,max_devices,total_requests,coalesce(user_id,'未关联') FROM api_keys ORDER BY id DESC LIMIT 200 OFFSET ?", "SELECT count(*) FROM api_keys")
	case admin.ActionNotices:
		return store.adminTable(ctx, request.Page, []string{"ID", "标题", "发布日期"}, "SELECT id,title,published_at FROM notices ORDER BY published_at DESC,id DESC LIMIT 200 OFFSET ?", "SELECT count(*) FROM notices")
	case admin.ActionSubmissions:
		return store.adminTable(ctx, request.Page, []string{"ID", "用户", "标题", "正文", "状态", "审核意见"}, "SELECT id,user_id,title,substr(content,1,120),status,review FROM user_submissions ORDER BY created_at DESC,id DESC LIMIT 200 OFFSET ?", "SELECT count(*) FROM user_submissions")
	case admin.ActionAudit:
		return store.adminTable(ctx, request.Page, []string{"时间", "操作", "目标"}, "SELECT created_at,action,target FROM audit_events ORDER BY id DESC LIMIT 200 OFFSET ?", "SELECT count(*) FROM audit_events")
	case admin.ActionStats:
		return store.adminStats(ctx)
	case admin.ActionReports:
		return store.adminTable(ctx, request.Page, []string{"日期", "状态", "来源数量", "正文"}, "SELECT day,status,source_count,substr(content,1,120) FROM daily_reports ORDER BY day DESC LIMIT 200 OFFSET ?", "SELECT count(*) FROM daily_reports")
	case admin.ActionNoticeDetail, admin.ActionSubmissionDetail, admin.ActionReportDetail:
		return store.adminDetail(ctx, request)
	case admin.ActionSettings:
		return store.settingsTable(ctx)
	case admin.ActionReviewRegistration:
		return store.reviewRegistration(ctx, request)
	case admin.ActionReviewSubmission:
		return store.reviewSubmission(ctx, request)
	case admin.ActionPermissions:
		return store.setPermissions(ctx, request)
	case admin.ActionLinkKey:
		return store.linkKey(ctx, request)
	case admin.ActionCreateKey:
		return store.createKey(ctx, request)
	case admin.ActionSetSetting, admin.ActionResetSettings:
		return store.changeSettings(ctx, request)
	case admin.ActionDeleteUser, admin.ActionDeleteKey, admin.ActionKeyStatus, admin.ActionDeleteNotice:
		return store.adminMutation(ctx, request)
	case admin.ActionImportUsers:
		return store.ImportUsers(ctx, request.ID)
	case admin.ActionRebuildIndex:
		return store.rebuildIndex(ctx, request)
	case admin.ActionCheck:
		return admin.Response{Text: "数据库完整性检查通过"}, store.Check(ctx)
	case admin.ActionBackup:
		return admin.Response{Text: "备份已创建并验证"}, store.Backup(ctx, request.ID)
	default:
		return admin.Response{}, fmt.Errorf("管理动作尚未实现: %s", request.Action)
	}
}

// adminTable 将只读查询转换成有界 TUI 表格，通用扫描仅存在于 SQL 边界。
func (store *Store) adminTable(ctx context.Context, page int, columns []string, query string, countQuery string) (admin.Response, error) {
	tx, pagination, err := store.adminPage(ctx, page, countQuery)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, query, pagination.Index*pagination.Size)
	if err != nil {
		return admin.Response{}, err
	}
	defer rows.Close()
	result := admin.Response{Columns: columns, Rows: make([][]string, 0), Pagination: &pagination}
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		targets := make([]interface{}, len(columns))
		for index := range values {
			targets[index] = &values[index]
		}
		if err = rows.Scan(targets...); err != nil {
			return result, err
		}
		line := make([]string, len(columns))
		for index, value := range values {
			line[index] = value.String
		}
		result.Rows = append(result.Rows, line)
	}
	return result, rows.Err()
}

// adminMutation 对破坏性管理动作执行事务并记录不含敏感字段的审计。
func (store *Store) adminMutation(ctx context.Context, request admin.Request) (admin.Response, error) {
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	var result sql.Result
	switch request.Action {
	case admin.ActionDeleteUser:
		result, err = tx.ExecContext(ctx, "DELETE FROM users WHERE id=?", request.ID)
	case admin.ActionDeleteKey:
		result, err = tx.ExecContext(ctx, "DELETE FROM api_keys WHERE id=?", request.ID)
	case admin.ActionDeleteNotice:
		result, err = tx.ExecContext(ctx, "DELETE FROM notices WHERE id=?", request.ID)
	case admin.ActionKeyStatus:
		active, parseErr := strconv.ParseBool(request.Values["active"])
		if parseErr != nil {
			return admin.Response{}, fmt.Errorf("启用状态必须为 true 或 false")
		}
		result, err = tx.ExecContext(ctx, "UPDATE api_keys SET is_active=? WHERE id=?", active, request.ID)
	}
	if err != nil {
		return admin.Response{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return admin.Response{}, err
	}
	if count == 0 {
		return admin.Response{}, fmt.Errorf("目标记录不存在")
	}
	return commitAdmin(ctx, tx, request)
}

// commitAdmin 将审计与业务变更共同提交，不记录值或凭据。
func commitAdmin(ctx context.Context, tx *sql.Tx, request admin.Request) (admin.Response, error) {
	if _, err := tx.ExecContext(ctx, "INSERT INTO audit_events(action,target,created_at) VALUES(?,?,?)", request.Action, strings.TrimSpace(request.ID), now()); err != nil {
		return admin.Response{}, err
	}
	return admin.Response{Text: "操作成功"}, tx.Commit()
}
