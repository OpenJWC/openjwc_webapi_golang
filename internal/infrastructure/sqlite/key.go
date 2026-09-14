package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
)

// KeyDevices 查询已有绑定，不将未经授权的设备自动注册到旧密钥。
func (store *Store) KeyDevices(ctx context.Context, token string, device string) (identity.KeyDevices, error) {
	tx, err := store.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return identity.KeyDevices{}, err
	}
	defer tx.Rollback()
	var id int64
	var quota int
	err = tx.QueryRowContext(ctx, `SELECT k.id,k.max_devices FROM api_keys k JOIN api_key_devices d ON d.api_key_id=k.id
		WHERE k.key_hash=? AND k.is_active=1 AND d.device_id=?`, hashToken(token), device).Scan(&id, &quota)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.KeyDevices{}, access.ErrForbidden
	}
	if err != nil {
		return identity.KeyDevices{}, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT device_id FROM api_key_devices WHERE api_key_id=? ORDER BY device_id", id)
	if err != nil {
		return identity.KeyDevices{}, err
	}
	defer rows.Close()
	result := identity.KeyDevices{Total: quota, Devices: make([]string, 0)}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return result, err
		}
		result.Devices = append(result.Devices, value)
	}
	return result, rows.Err()
}

// KeyUnbind 在单条条件删除中复核密钥状态与设备归属。
func (store *Store) KeyUnbind(ctx context.Context, token string, device string) (bool, error) {
	result, err := store.writer.ExecContext(ctx, "DELETE FROM api_key_devices WHERE device_id=? AND api_key_id IN (SELECT id FROM api_keys WHERE key_hash=? AND is_active=1)", device, hashToken(token))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
