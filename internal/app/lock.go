package app

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockDirectory 使用操作系统锁确保同一数据目录只运行一个服务进程。
func lockDirectory(path string) (*os.File, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("数据目录必须为权限 0700 的真实目录: %s", path)
	}
	file, err := os.OpenFile(filepath.Join(path, "service.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, fmt.Errorf("打开服务进程锁: %w", err)
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("该数据目录已有服务进程运行: %w", err)
	}
	return file, nil
}
