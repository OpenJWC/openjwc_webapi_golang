package sqlite

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
)

// registerTestUser 创建待审核申请并通过本地用例批准。
func registerTestUser(t *testing.T, store *Store) *identity.Service {
	t.Helper()
	service := identity.New(store)
	ctx := context.Background()
	if err := service.Register(ctx, identity.RegistrationInput{Username: "student", Email: "student@example.com", Password: "client-password-hash"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Execute(ctx, admin.Request{Action: "review-registration", ID: "1", Values: map[string]string{"decision": "approved"}}); err != nil {
		t.Fatal(err)
	}
	return service
}

// TestIdentityRevocationAndDeviceIsolation 验证权限撤销、换设备及解绑不能绕过认证。
func TestIdentityRevocationAndDeviceIsolation(t *testing.T) {
	store := openTestStore(t)
	service := registerTestUser(t, store)
	ctx := context.Background()
	login, err := service.Login(ctx, identity.LoginInput{Account: "student", Password: "client-password-hash", DeviceName: "phone"}, "device-one")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := store.Authenticate(ctx, login.Token, "device-one")
	if err != nil || !principal.Allows(access.Browse) || principal.Allows(access.Chat) {
		t.Fatalf("默认权限无效: %v", err)
	}
	if _, err := store.Authenticate(ctx, login.Token, "device-two"); !errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("设备隔离失败: %v", err)
	}
	if _, err := store.Execute(ctx, admin.Request{Action: "permissions", ID: "1", Values: map[string]string{"active": "true", "browse": "false", "chat": "true", "submit": "false", "max_devices": "3"}}); err != nil {
		t.Fatal(err)
	}
	principal, err = store.Authenticate(ctx, login.Token, "device-one")
	if err != nil || principal.Allows(access.Browse) || principal.Allows(access.Chat) {
		t.Fatalf("权限撤销无效: %v", err)
	}
	if _, err := store.Unbind(ctx, principal.ID(), "device-one"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Authenticate(ctx, login.Token, "device-one"); !errors.Is(err, access.ErrUnauthorized) {
		t.Fatalf("解绑后凭据仍有效: %v", err)
	}
}

// TestConcurrentLoginHonorsDeviceQuota 验证并发绑定不能超过事务内设备配额。
func TestConcurrentLoginHonorsDeviceQuota(t *testing.T) {
	store := openTestStore(t)
	service := registerTestUser(t, store)
	ctx := context.Background()
	var workers sync.WaitGroup
	var successes atomic.Int64
	for index := 0; index < 8; index++ {
		workers.Go(func() {
			_, err := service.Login(ctx, identity.LoginInput{Account: "student", Password: "client-password-hash", DeviceName: "phone"}, "device-"+strconv.Itoa(index))
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, access.ErrConflict) {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	devices, err := store.Devices(ctx, 1)
	if err != nil || len(devices) != 3 || successes.Load() != 3 {
		t.Fatalf("设备配额被绕过: devices=%d success=%d err=%v", len(devices), successes.Load(), err)
	}
}

// TestConcurrentRegistrationReviewCreatesOneUser 验证审核状态迁移只能成功一次。
func TestConcurrentRegistrationReviewCreatesOneUser(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	service := identity.New(store)
	if err := service.Register(ctx, identity.RegistrationInput{Username: "student", Email: "student@example.com", Password: "hash"}); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	var successes atomic.Int64
	for range 8 {
		workers.Go(func() {
			_, err := store.Execute(ctx, admin.Request{Action: "review-registration", ID: "1", Values: map[string]string{"decision": "approved"}})
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, access.ErrConflict) {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	if successes.Load() != 1 {
		t.Fatalf("审核成功次数=%d", successes.Load())
	}
}
