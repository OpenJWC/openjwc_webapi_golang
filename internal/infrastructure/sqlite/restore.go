package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Restore 在服务进程锁已持有时离线恢复，旧数据库及侧文件留存在隔离目录中。
func Restore(ctx context.Context, source string, destination string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	input, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("打开恢复来源: %w", err)
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("恢复来源必须为普通备份文件")
	}
	if wal, err := os.Stat(source + "-wal"); err == nil && wal.Size() > 0 {
		return "", fmt.Errorf("恢复来源存在 WAL，请先从旧服务创建一致备份")
	}
	if target, err := os.Stat(destination); err == nil && os.SameFile(info, target) {
		return "", fmt.Errorf("恢复来源不能是当前数据库")
	}
	dir := filepath.Dir(destination)
	marker := filepath.Join(dir, "recovery-in-progress")
	if _, err := os.Lstat(marker); !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("存在未完成恢复标记，请检查隔离目录后人工处理")
	}
	staging, err := os.MkdirTemp(dir, "recovery-")
	if err != nil {
		return "", err
	}
	staged := filepath.Join(staging, "verified.db")
	output, err := os.OpenFile(staged, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if err = errors.Join(copyErr, syncErr, closeErr); err != nil {
		return "", err
	}
	verified, err := Open(ctx, staged)
	if err != nil {
		return "", fmt.Errorf("恢复副本验证失败，原数据库未改动: %w", err)
	}
	checkErr := verified.Check(ctx)
	closeErr = verified.Close()
	if err = errors.Join(checkErr, closeErr); err != nil {
		return "", fmt.Errorf("恢复副本校验失败，原数据库未改动: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	flag, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := flag.WriteString(staging)
	syncErr = flag.Sync()
	closeErr = flag.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return "", err
	}
	if err = syncDirectory(dir); err != nil {
		return "", err
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		path := destination + suffix
		if _, err = os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if err = os.Rename(path, filepath.Join(staging, "previous.db"+suffix)); err != nil {
			return "", fmt.Errorf("隔离旧数据库失败，保留恢复标记: %w", err)
		}
	}
	if err = syncDirectory(staging); err != nil {
		return "", err
	}
	if err = os.Rename(staged, destination); err != nil {
		return "", fmt.Errorf("发布恢复数据库失败，保留恢复标记: %w", err)
	}
	if err = syncDirectory(dir); err != nil {
		return "", err
	}
	if err = os.Remove(marker); err != nil {
		return "", err
	}
	return staging, syncDirectory(dir)
}

// syncDirectory 同步目录元数据，保证恢复步骤在崩溃后可追踪。
func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
