package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
	"golang.org/x/crypto/bcrypt"
)

// legacyFixture 创建仅含虚构凭据的旧版数据库夹具。
func legacyFixture(t *testing.T, badDevices bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE users(id INTEGER,username TEXT,email TEXT,password_hash TEXT,is_active INTEGER);
		CREATE TABLE api_keys(id INTEGER,key_string TEXT,owner_name TEXT,is_active INTEGER,max_devices INTEGER,bound_devices TEXT,total_requests INTEGER);`)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("legacy-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO users VALUES(7,'legacy','legacy@example.com',?,1)", string(hash)); err != nil {
		t.Fatal(err)
	}
	devices := `["legacy-device"]`
	if badDevices {
		devices = `["one","two"]`
	}
	if _, err = db.Exec("INSERT INTO api_keys VALUES(9,'legacy-test-token','legacy owner',1,1,?,12)", devices); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLegacyImportPreservesIdentityAndHashesKeys 验证迁移凭据摘要、密码验证及重复导入幂等性。
func TestLegacyImportPreservesIdentityAndHashesKeys(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	path := legacyFixture(t, false)
	if _, err := store.ImportUsers(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportUsers(ctx, path); err != nil {
		t.Fatal(err)
	}
	login, err := identity.New(store).Login(ctx, identity.LoginInput{Account: "legacy", Password: "legacy-password", DeviceName: "new phone"}, "new-device")
	if err != nil || login.Token == "" {
		t.Fatalf("旧密码不可登录: %v", err)
	}
	devices, err := store.KeyDevices(ctx, "legacy-test-token", "legacy-device")
	if err != nil || len(devices.Devices) != 1 {
		t.Fatalf("旧绑定未保留: %v", err)
	}
	var hash string
	if err = store.reader.QueryRowContext(ctx, "SELECT key_hash FROM api_keys WHERE id=9").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if hash == "legacy-test-token" || hash != hashToken("legacy-test-token") {
		t.Fatal("API Key 未按摘要保存")
	}
}

// TestInvalidLegacyRecordRollsBackEntireImport 验证坏记录不会留下部分迁移用户。
func TestInvalidLegacyRecordRollsBackEntireImport(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.ImportUsers(context.Background(), legacyFixture(t, true)); err == nil {
		t.Fatal("错误配额被接受")
	}
	var count int
	if err := store.reader.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("迁移失败后留下部分用户")
	}
}
