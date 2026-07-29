package access_test

import (
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
)

func TestBindDeviceHonorsQuota(t *testing.T) {
	key, err := access.NewAPIKey("hash", "测试用户", 1, time.Now())
	if err != nil {
		t.Fatalf("创建访问凭证失败: %v", err)
	}
	key, err = key.BindDevice("device-1")
	if err != nil {
		t.Fatalf("绑定第一个设备失败: %v", err)
	}
	if _, err = key.BindDevice("device-2"); err == nil {
		t.Fatal("绑定数量超过配额时应返回错误")
	}
}
