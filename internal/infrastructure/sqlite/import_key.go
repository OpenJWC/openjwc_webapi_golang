package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// importKeys 将旧明文密钥转换为摘要并保留设备绑定，任何无效记录都会导致整体回滚。
func importKeys(ctx context.Context, source *sql.Tx, target *sql.Tx) error {
	exists, err := legacyTable(ctx, source, "api_keys")
	if err != nil || !exists {
		return err
	}
	rows, err := source.QueryContext(ctx, "SELECT id,key_string,coalesce(owner_name,''),is_active,max_devices,coalesce(bound_devices,'[]'),coalesce(total_requests,0) FROM api_keys ORDER BY id")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, requests int64
		var token, owner, encoded string
		var active bool
		var quota int
		if err = rows.Scan(&id, &token, &owner, &active, &quota, &encoded, &requests); err != nil {
			return err
		}
		if token == "" || quota < 1 || quota > 100 || requests < 0 {
			return fmt.Errorf("旧 API Key %d 配额或统计无效", id)
		}
		var devices []string
		if err = json.Unmarshal([]byte(encoded), &devices); err != nil || len(devices) > quota {
			return fmt.Errorf("旧 API Key %d 设备列表无效", id)
		}
		if _, err = target.ExecContext(ctx, "INSERT INTO api_keys(id,key_hash,owner_name,is_active,max_devices,total_requests,created_at) VALUES(?,?,?,?,?,?,?)", id, hashToken(token), owner, active, quota, requests, now()); err != nil {
			return err
		}
		for _, device := range devices {
			if device == "" || len(device) > 128 {
				return fmt.Errorf("旧 API Key %d 包含无效设备", id)
			}
			if _, err = target.ExecContext(ctx, "INSERT INTO api_key_devices(api_key_id,device_id,bound_at) VALUES(?,?,?)", id, device, now()); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}
