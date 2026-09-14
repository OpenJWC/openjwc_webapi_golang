package sqlite

import (
	"context"
	"database/sql"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// adminUsers 读取具名权限投影，表格列布局不再决定授权语义。
func (store *Store) adminUsers(ctx context.Context, page int) (admin.Response, error) {
	tx, pagination, err := store.adminPage(ctx, page, "SELECT count(*) FROM users")
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id,username,email,is_active,can_browse,can_chat,can_submit,max_devices
		FROM users ORDER BY id DESC LIMIT 200 OFFSET ?`, pagination.Index*pagination.Size)
	if err != nil {
		return admin.Response{}, err
	}
	defer rows.Close()
	users := make([]admin.UserPermissions, 0)
	for rows.Next() {
		var user admin.UserPermissions
		if err = rows.Scan(&user.ID, &user.Username, &user.Email, &user.Active, &user.Browse, &user.Chat, &user.Submit, &user.MaxDevices); err != nil {
			return admin.Response{}, err
		}
		users = append(users, user)
	}
	return admin.Response{Users: users, Pagination: &pagination}, rows.Err()
}

// adminStats 单独读取非分页概览，不依赖查询文本的排版形式。
func (store *Store) adminStats(ctx context.Context) (admin.Response, error) {
	var notices, users, registrations, submissions string
	err := store.reader.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM notices),(SELECT count(*) FROM users),
		(SELECT count(*) FROM user_registrations WHERE status='pending'),(SELECT count(*) FROM user_submissions WHERE status='pending')`).Scan(&notices, &users, &registrations, &submissions)
	return admin.Response{Columns: []string{"资讯", "用户", "待审注册", "待审投稿"}, Rows: [][]string{{notices, users, registrations, submissions}}}, err
}

// adminPage 在同一只读快照内读取总数，并将越界请求收敛到有效末页。
func (store *Store) adminPage(ctx context.Context, index int, countQuery string) (*sql.Tx, admin.Pagination, error) {
	page := admin.Pagination{Size: 200}
	tx, err := store.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, page, err
	}
	if err = tx.QueryRowContext(ctx, countQuery).Scan(&page.Total); err != nil {
		tx.Rollback()
		return nil, page, err
	}
	page.Index = min(max(0, index), page.Pages()-1)
	return tx, page, nil
}
