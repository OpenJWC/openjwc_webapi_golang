package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// ImportUsers 将旧库的账号、注册申请及 API Key 原子导入空身份库，不复制管理员或登录会话。
func (store *Store) ImportUsers(ctx context.Context, source string) (admin.Response, error) {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return admin.Response{}, err
	}
	if absolute == store.path {
		return admin.Response{}, fmt.Errorf("不能将目标数据库作为迁移来源")
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return admin.Response{}, fmt.Errorf("旧数据库必须为存在的普通文件")
	}
	legacy, err := sql.Open("sqlite", connectionURL(absolute, true))
	if err != nil {
		return admin.Response{}, err
	}
	defer legacy.Close()
	legacy.SetMaxOpenConns(1)
	read, err := legacy.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return admin.Response{}, err
	}
	defer read.Rollback()
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM import_runs WHERE source_hash=?", hashToken(absolute)).Scan(&exists); err != nil {
		return admin.Response{}, err
	}
	if exists > 0 {
		return admin.Response{Text: "该来源已迁移，无需重复导入"}, nil
	}
	if err = tx.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM users)+(SELECT count(*) FROM user_registrations)+(SELECT count(*) FROM api_keys)").Scan(&exists); err != nil {
		return admin.Response{}, err
	}
	if exists > 0 {
		return admin.Response{}, fmt.Errorf("用户态迁移要求目标身份表为空，请在首次上线前执行")
	}
	for _, copy := range []func(context.Context, *sql.Tx, *sql.Tx) error{importAccounts, importRegistrations, importKeys} {
		if err = copy(ctx, read, tx); err != nil {
			return admin.Response{}, fmt.Errorf("旧用户态迁移已回滚: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO import_runs(source_hash,imported_at) VALUES(?,?)", hashToken(absolute), now()); err != nil {
		return admin.Response{}, err
	}
	if _, err = commitAdmin(ctx, tx, admin.Request{Action: admin.ActionImportUsers, ID: "legacy-sqlite"}); err != nil {
		return admin.Response{}, err
	}
	return admin.Response{Text: "用户、注册申请和 API Key 已导入；用户须重新登录，问答权限默认关闭。资讯与旧管理员未导入。"}, nil
}

// legacyTable 检查已知旧表是否存在，允许早期部署缺少 v2 表。
func legacyTable(ctx context.Context, tx *sql.Tx, name string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&count)
	return count > 0, err
}
