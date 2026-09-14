package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// authenticateKey 对显式关联用户的 API Key 执行设备配额事务并继承用户权限。
func (store *Store) authenticateKey(ctx context.Context, token string, device string) (access.Principal, error) {
	if device == "" {
		return access.Principal{}, access.ErrUnauthorized
	}
	var keyID, userID int64
	var username string
	var browse, chat, submit bool
	var quota int
	err := store.reader.QueryRowContext(ctx, `SELECT k.id,u.id,u.username,u.can_browse,u.can_chat,u.can_submit,k.max_devices FROM api_keys k
		JOIN users u ON u.id=k.user_id WHERE k.key_hash=? AND k.is_active=1 AND u.is_active=1`, hashToken(token)).Scan(&keyID, &userID, &username, &browse, &chat, &submit, &quota)
	if errors.Is(err, sql.ErrNoRows) {
		return access.Principal{}, access.ErrUnauthorized
	}
	if err != nil {
		return access.Principal{}, err
	}
	var bound int
	if err = store.reader.QueryRowContext(ctx, "SELECT count(*) FROM api_key_devices WHERE api_key_id=? AND device_id=?", keyID, device).Scan(&bound); err != nil {
		return access.Principal{}, err
	}
	if bound > 0 {
		return access.RestorePrincipal(userID, username, browse, chat, submit)
	}
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return access.Principal{}, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT u.id,u.username,u.can_browse,u.can_chat,u.can_submit,k.max_devices FROM api_keys k JOIN users u ON u.id=k.user_id
		WHERE k.id=? AND k.is_active=1 AND u.is_active=1`, keyID).Scan(&userID, &username, &browse, &chat, &submit, &quota)
	if errors.Is(err, sql.ErrNoRows) {
		return access.Principal{}, access.ErrUnauthorized
	}
	if err != nil {
		return access.Principal{}, err
	}
	var count int
	if err = tx.QueryRowContext(ctx, "SELECT count(*),coalesce(sum(device_id=?),0) FROM api_key_devices WHERE api_key_id=?", device, keyID).Scan(&count, &bound); err != nil {
		return access.Principal{}, err
	}
	if bound == 0 && count >= quota {
		return access.Principal{}, access.ErrForbidden
	}
	if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO api_key_devices(api_key_id,device_id,bound_at) VALUES(?,?,?)", keyID, device, now()); err != nil {
		return access.Principal{}, err
	}
	if err = tx.Commit(); err != nil {
		return access.Principal{}, err
	}
	return access.RestorePrincipal(userID, username, browse, chat, submit)
}

// linkKey 显式关联用户且撤销旧设备绑定，避免所有权变更保留旧授权。
func (store *Store) linkKey(ctx context.Context, request admin.Request) (admin.Response, error) {
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE api_keys SET user_id=? WHERE id=?", request.Values["user_id"], request.ID)
	if err != nil {
		return admin.Response{}, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return admin.Response{}, err
	}
	if count != 1 {
		return admin.Response{}, fmt.Errorf("API Key 不存在")
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM api_key_devices WHERE api_key_id=?", request.ID); err != nil {
		return admin.Response{}, err
	}
	return commitAdmin(ctx, tx, request)
}
