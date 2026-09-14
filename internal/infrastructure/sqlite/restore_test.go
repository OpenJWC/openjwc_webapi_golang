package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestOfflineRestoreRetainsOldDataAndPublishesVerifiedBackup 验证离线恢复保留原库且新库通过完整检查。
func TestOfflineRestoreRetainsOldDataAndPublishesVerifiedBackup(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.Save(ctx, testNotice(t, "restore-one", "备份里的资讯")); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "backups", "snapshot.db")
	if err := store.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "state.db")
	if err := os.WriteFile(target, []byte("broken-original"), 0600); err != nil {
		t.Fatal(err)
	}
	quarantine, err := Restore(ctx, backup, target)
	if err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(filepath.Join(quarantine, "previous.db"))
	if err != nil || string(old) != "broken-original" {
		t.Fatalf("原库未保留: %v", err)
	}
	restored, err := Open(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if _, found, err := restored.FindByID(ctx, "restore-one"); err != nil || !found {
		t.Fatalf("恢复资讯失败: %v %v", found, err)
	}
	if err := restored.Check(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestIncompleteRecoveryMarkerBlocksEmptyDatabaseCreation 验证中断恢复时不能误启动空数据库。
func TestIncompleteRecoveryMarkerBlocksEmptyDatabaseCreation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "recovery-in-progress"), []byte("quarantine"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.db")
	if store, err := Open(context.Background(), path); err == nil {
		store.Close()
		t.Fatal("恢复中断后自动创建了空库")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("不应创建目标数据库")
	}
}
