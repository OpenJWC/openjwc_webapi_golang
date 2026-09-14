package tui

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
)

// TestActualUserProjectionRoundTripsWithoutPermissionChanges 验证实际用户查询到表单再提交不会因展示列变化改写权限。
func TestActualUserProjectionRoundTripsWithoutPermissionChanges(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "db", "permissions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = identity.New(store).Register(ctx, identity.RegistrationInput{Username: "student", Email: "student@example.com", Password: "test-password"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Execute(ctx, admin.Request{Action: admin.ActionReviewRegistration, ID: "1", Values: map[string]string{"decision": "approved"}}); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"active": "false", "browse": "true", "chat": "true", "submit": "false", "max_devices": "5"}
	if _, err = store.Execute(ctx, admin.Request{Action: admin.ActionPermissions, ID: "1", Values: values}); err != nil {
		t.Fatal(err)
	}
	before, err := store.Execute(ctx, admin.Request{Action: admin.ActionUsers})
	if err != nil || len(before.Users) != 1 {
		t.Fatalf("用户投影读取失败: %v", err)
	}
	form := newForm(testAction(t, admin.ActionPermissions), []string{"不再依赖的展示列"})
	form.fillPermissions(before.Users[0])
	if _, err = store.Execute(ctx, form.request()); err != nil {
		t.Fatal(err)
	}
	after, err := store.Execute(ctx, admin.Request{Action: admin.ActionUsers})
	if err != nil || !reflect.DeepEqual(before.Users, after.Users) {
		t.Fatalf("无修改提交改变了权限: %#v -> %#v，%v", before.Users, after.Users, err)
	}
}
