package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

// importAccounts 复制已批准用户并保留 bcrypt 哈希，默认关闭新增的问答授权。
func importAccounts(ctx context.Context, source *sql.Tx, target *sql.Tx) error {
	exists, err := legacyTable(ctx, source, "users")
	if err != nil || !exists {
		return err
	}
	rows, err := source.QueryContext(ctx, "SELECT id,username,email,password_hash,is_active FROM users ORDER BY id")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var username, email, hash string
		var active bool
		if err = rows.Scan(&id, &username, &email, &hash, &active); err != nil {
			return err
		}
		if _, err = bcrypt.Cost([]byte(hash)); err != nil {
			return fmt.Errorf("旧用户 %d 密码哈希格式不受支持", id)
		}
		if _, err = target.ExecContext(ctx, "INSERT INTO users(id,username,email,password_hash,is_active,created_at) VALUES(?,?,?,?,?,?)", id, username, email, hash, active, now()); err != nil {
			return err
		}
	}
	return rows.Err()
}

// importRegistrations 保留审核申请的标识、状态和意见。
func importRegistrations(ctx context.Context, source *sql.Tx, target *sql.Tx) error {
	exists, err := legacyTable(ctx, source, "user_registrations")
	if err != nil || !exists {
		return err
	}
	rows, err := source.QueryContext(ctx, "SELECT id,username,email,password_hash,status,coalesce(review,'') FROM user_registrations ORDER BY id")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var username, email, hash, status, review string
		if err = rows.Scan(&id, &username, &email, &hash, &status, &review); err != nil {
			return err
		}
		if _, err = bcrypt.Cost([]byte(hash)); err != nil {
			return fmt.Errorf("旧申请 %d 密码哈希格式不受支持", id)
		}
		if _, err = target.ExecContext(ctx, "INSERT INTO user_registrations(id,username,email,password_hash,status,review,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)", id, username, email, hash, status, review, now(), now()); err != nil {
			return err
		}
	}
	return rows.Err()
}
