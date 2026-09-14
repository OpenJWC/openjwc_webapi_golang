package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
)

// Backup 在线创建一致快照，验证后发布且绝不覆盖已有目标。
func (store *Store) Backup(ctx context.Context, destination string) error {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("解析备份路径: %w", err)
	}
	if absolute == store.path {
		return fmt.Errorf("备份不能覆盖当前数据库")
	}
	dir := filepath.Dir(absolute)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("备份目录必须是权限 0700 的真实目录")
	}
	file, err := os.CreateTemp(dir, ".openjwc-backup-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err = file.Close(); err != nil {
		return err
	}
	if err = store.backupTo(ctx, temporary); err != nil {
		return err
	}
	if err = os.Link(temporary, absolute); err != nil {
		return fmt.Errorf("发布备份失败，目标不能已存在: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

// backupTo 利用 SQLite 快照导出并在校验失败时清理不完整目标。
func (store *Store) backupTo(ctx context.Context, destination string) (err error) {
	defer func() {
		if err != nil {
			_ = os.Remove(destination)
		}
	}()
	if _, err = store.writer.ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return fmt.Errorf("导出备份快照: %w", err)
	}
	backup, err := sql.Open("sqlite", connectionURL(destination, true))
	if err != nil {
		return fmt.Errorf("打开备份检查: %w", err)
	}
	err = checkDatabase(ctx, backup, "PRAGMA integrity_check")
	closeErr := backup.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	file, err := os.OpenFile(destination, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = file.Sync()
	closeErr = file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	dir, err := os.Open(filepath.Dir(destination))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
