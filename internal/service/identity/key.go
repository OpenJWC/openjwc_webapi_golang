package identity

import "context"

// KeyDevices 是旧 API Key 设备接口的数据投影。
type KeyDevices struct {
	Total   int      `json:"total"`
	Devices []string `json:"bound_devices"`
}

// KeyRepository 定义旧版设备查询与解绑能力。
type KeyRepository interface {
	KeyDevices(context.Context, string, string) (KeyDevices, error)
	KeyUnbind(context.Context, string, string) (bool, error)
}
