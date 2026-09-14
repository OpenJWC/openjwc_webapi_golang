package sqlite

import (
	"context"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
)

// Devices 返回用户设备的公开投影，不暴露认证摘要。
func (store *Store) Devices(ctx context.Context, id int64) ([]identity.Device, error) {
	rows, err := store.reader.QueryContext(ctx, "SELECT device_uuid,device_name,last_login FROM user_devices WHERE user_id=? ORDER BY last_login DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]identity.Device, 0)
	for rows.Next() {
		var item identity.Device
		if err = rows.Scan(&item.UUID, &item.Name, &item.LastLogin); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// Unbind 仅删除当前用户拥有的指定设备。
func (store *Store) Unbind(ctx context.Context, id int64, device string) (bool, error) {
	result, err := store.writer.ExecContext(ctx, "DELETE FROM user_devices WHERE user_id=? AND device_uuid=?", id, device)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
