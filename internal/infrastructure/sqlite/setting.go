package sqlite

import (
	"context"
	"sort"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// Settings 返回默认值与持久化值合并后的独立快照。
func (store *Store) Settings(ctx context.Context) (map[string]string, error) {
	result := setting.Defaults()
	rows, err := store.reader.QueryContext(ctx, "SELECT key,value FROM system_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		if _, ok := result[key]; ok {
			result[key] = value
		}
	}
	return result, rows.Err()
}

// settingsTable 遮蔽模型服务凭据后生成设置列表。
func (store *Store) settingsTable(ctx context.Context) (admin.Response, error) {
	values, err := store.Settings(ctx)
	if err != nil {
		return admin.Response{}, err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := admin.Response{Columns: []string{"设置", "当前值"}, Rows: make([][]string, 0, len(keys))}
	for _, key := range keys {
		value := values[key]
		if key == "llm_api_key" && value != "" {
			value = "[已配置]"
		}
		result.Rows = append(result.Rows, []string{key, value})
	}
	return result, nil
}

// changeSettings 校验后保存单个设置或重置全部设置，并记录审计。
func (store *Store) changeSettings(ctx context.Context, request admin.Request) (admin.Response, error) {
	if request.Action == admin.ActionSetSetting {
		if err := setting.Validate(request.ID, request.Values["value"]); err != nil {
			return admin.Response{}, err
		}
	}
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return admin.Response{}, err
	}
	defer tx.Rollback()
	if request.Action == admin.ActionResetSettings {
		_, err = tx.ExecContext(ctx, "DELETE FROM system_settings")
	} else {
		_, err = tx.ExecContext(ctx, "INSERT INTO system_settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at", request.ID, request.Values["value"], now())
	}
	if err != nil {
		return admin.Response{}, err
	}
	return commitAdmin(ctx, tx, request)
}
