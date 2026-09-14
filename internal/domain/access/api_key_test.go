package access_test

import (
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
)

// TestBindDeviceHonorsQuota 验证绑定配额、重复绑定与原值不可变性。
func TestBindDeviceHonorsQuota(t *testing.T) {
	key, err := access.NewAPIKey("hash", "测试用户", 1, time.Now())
	if err != nil {
		t.Fatalf("创建访问凭证失败: %v", err)
	}
	original := key
	key, err = key.BindDevice("device-1")
	if err != nil {
		t.Fatalf("绑定第一个设备失败: %v", err)
	}
	if len(original.BoundDeviceIDs()) != 0 {
		t.Fatal("绑定设备修改了原凭据")
	}
	rebound, err := key.BindDevice("device-1")
	if err != nil || len(rebound.BoundDeviceIDs()) != 1 {
		t.Fatal("重复绑定不应消耗额外配额")
	}
	if _, err = key.BindDevice("device-2"); err == nil {
		t.Fatal("绑定数量超过配额时应返回错误")
	}
}
