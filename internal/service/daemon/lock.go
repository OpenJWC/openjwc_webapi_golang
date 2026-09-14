package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockDeployment 串行化同一作用域的安装和卸载，避免 unit 与部署记录交错发布。
func (manager *Manager) lockDeployment() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(manager.ProfilePath), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(filepath.Dir(manager.ProfilePath), "service.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("已有部署操作正在执行: %w", err)
	}
	return file, nil
}
