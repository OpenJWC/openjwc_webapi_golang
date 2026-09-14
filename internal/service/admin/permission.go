package admin

import (
	"fmt"
	"strconv"
)

// UserPermissions 是本地管理界面的具名用户权限投影，不含认证材料。
type UserPermissions struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Active     bool   `json:"active"`
	Browse     bool   `json:"browse"`
	Chat       bool   `json:"chat"`
	Submit     bool   `json:"submit"`
	MaxDevices int    `json:"max_devices"`
}

// ParsePermissions 将表单字段转换为具名权限，拒绝缺失字段与未知布尔值。
func ParsePermissions(id string, values map[string]string) (UserPermissions, error) {
	result := UserPermissions{ID: id}
	fields := []struct {
		name   string
		target *bool
	}{
		{name: "active", target: &result.Active},
		{name: "browse", target: &result.Browse},
		{name: "chat", target: &result.Chat},
		{name: "submit", target: &result.Submit},
	}
	for _, field := range fields {
		value, err := strconv.ParseBool(values[field.name])
		if err != nil {
			return UserPermissions{}, fmt.Errorf("%s 必须为 true 或 false", field.name)
		}
		*field.target = value
	}
	quota, err := strconv.Atoi(values["max_devices"])
	if err != nil || quota < 1 || quota > 100 {
		return UserPermissions{}, fmt.Errorf("设备上限必须为 1～100")
	}
	result.MaxDevices = quota
	return result, nil
}
