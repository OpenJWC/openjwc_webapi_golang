package sqlite

import (
	"context"
	"fmt"
	"strconv"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// reviewRegistration 在同一写事务中创建用户并完成一次性审核。
func (store *Store) reviewRegistration(ctx context.Context, request admin.Request) (admin.Response, error) {
	decision := request.Values["decision"]
	if decision != "approved" && decision != "rejected" {
		return admin.Response{}, fmt.Errorf("审核结果必须为 approved 或 rejected")
	}
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	var username, email, hash, status string
	if err = tx.QueryRowContext(ctx, "SELECT username,email,password_hash,status FROM user_registrations WHERE id=?", request.ID).Scan(&username, &email, &hash, &status); err != nil {
		return admin.Response{}, err
	}
	if status != "pending" {
		return admin.Response{}, access.ErrConflict
	}
	if decision == "approved" {
		if _, err = tx.ExecContext(ctx, "INSERT INTO users(username,email,password_hash,created_at) VALUES(?,?,?,?)", username, email, hash, now()); err != nil {
			return admin.Response{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE user_registrations SET status=?,review=?,updated_at=? WHERE id=? AND status='pending'", decision, request.Values["review"], now(), request.ID); err != nil {
		return admin.Response{}, err
	}
	return commitAdmin(ctx, tx, request)
}

// setPermissions 原子更新用户权限并检查设备配额范围。
func (store *Store) setPermissions(ctx context.Context, request admin.Request) (admin.Response, error) {
	permissions, err := admin.ParsePermissions(request.ID, request.Values)
	if err != nil {
		return admin.Response{}, err
	}
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM user_devices WHERE user_id=?", request.ID).Scan(&count); err != nil {
		return admin.Response{}, err
	}
	if permissions.MaxDevices < count {
		return admin.Response{}, fmt.Errorf("设备上限不能小于当前绑定数 %d", count)
	}
	result, err := tx.ExecContext(ctx, "UPDATE users SET is_active=?,can_browse=?,can_chat=?,can_submit=?,max_devices=? WHERE id=?", permissions.Active, permissions.Browse, permissions.Chat, permissions.Submit, permissions.MaxDevices, permissions.ID)
	if err != nil {
		return admin.Response{}, err
	}
	count64, err := result.RowsAffected()
	if err != nil {
		return admin.Response{}, err
	}
	if count64 == 0 {
		return admin.Response{}, fmt.Errorf("用户不存在")
	}
	return commitAdmin(ctx, tx, request)
}

// createKey 创建只显示一次的高熵 API Key，数据库仅保存摘要。
func (store *Store) createKey(ctx context.Context, request admin.Request) (admin.Response, error) {
	owner := request.Values["owner"]
	quota, err := strconv.Atoi(request.Values["max_devices"])
	if owner == "" || len(owner) > 128 || err != nil || quota < 1 || quota > 100 {
		return admin.Response{}, fmt.Errorf("所有者或设备上限无效")
	}
	token := "oj_" + newToken()
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO api_keys(key_hash,owner_name,max_devices,created_at) VALUES(?,?,?,?)", hashToken(token), owner, quota, now()); err != nil {
		return admin.Response{}, err
	}
	if _, err = commitAdmin(ctx, tx, request); err != nil {
		return admin.Response{}, err
	}
	return admin.Response{Text: "API Key 仅显示本次，请安全保存：\n" + token}, nil
}
