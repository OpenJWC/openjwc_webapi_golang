package access

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// APIKey 表示客户端访问凭证及其设备配额。
type APIKey struct {
	id            int64
	keyHash       string
	ownerName     string
	active        bool
	maxDevices    int
	boundDevices  map[string]struct{}
	totalRequests uint64
	createdAt     time.Time
}

// NewAPIKey 创建尚未绑定设备的访问凭证。
func NewAPIKey(keyHash string, ownerName string, maxDevices int, createdAt time.Time) (APIKey, error) {
	if strings.TrimSpace(keyHash) == "" {
		return APIKey{}, fmt.Errorf("密钥摘要不能为空")
	}
	if strings.TrimSpace(ownerName) == "" {
		return APIKey{}, fmt.Errorf("密钥所有者不能为空")
	}
	if maxDevices < 1 {
		return APIKey{}, fmt.Errorf("设备配额必须大于零")
	}
	if createdAt.IsZero() {
		return APIKey{}, fmt.Errorf("创建时间不能为空")
	}

	return APIKey{
		keyHash:      strings.TrimSpace(keyHash),
		ownerName:    strings.TrimSpace(ownerName),
		active:       true,
		maxDevices:   maxDevices,
		boundDevices: make(map[string]struct{}),
		createdAt:    createdAt.UTC(),
	}, nil
}

// BindDevice 返回绑定指定设备后的凭证副本。
func (key APIKey) BindDevice(deviceID string) (APIKey, error) {
	deviceID = strings.TrimSpace(deviceID)
	if !key.active {
		return APIKey{}, fmt.Errorf("访问凭证已停用")
	}
	if deviceID == "" {
		return APIKey{}, fmt.Errorf("设备 ID 不能为空")
	}
	if _, exists := key.boundDevices[deviceID]; exists {
		return key.clone(), nil
	}
	if len(key.boundDevices) >= key.maxDevices {
		return APIKey{}, fmt.Errorf("设备绑定数量已达到上限")
	}

	bound := key.clone()
	bound.boundDevices[deviceID] = struct{}{}
	return bound, nil
}

// OwnerName 返回访问凭证所有者名称。
func (key APIKey) OwnerName() string {
	return key.ownerName
}

// IsActive 返回访问凭证是否可用。
func (key APIKey) IsActive() bool {
	return key.active
}

// MaxDevices 返回设备绑定上限。
func (key APIKey) MaxDevices() int {
	return key.maxDevices
}

// BoundDeviceIDs 返回按字典序排列的已绑定设备标识副本。
func (key APIKey) BoundDeviceIDs() []string {
	devices := make([]string, 0, len(key.boundDevices))
	for deviceID := range key.boundDevices {
		devices = append(devices, deviceID)
	}
	sort.Strings(devices)
	return devices
}

func (key APIKey) clone() APIKey {
	cloned := key
	cloned.boundDevices = make(map[string]struct{}, len(key.boundDevices))
	for deviceID := range key.boundDevices {
		cloned.boundDevices[deviceID] = struct{}{}
	}
	return cloned
}
