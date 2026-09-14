package daemon

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// checkFragment 拒绝通过更高优先级的路径遮蔽其他部署来源的同名服务。
func (manager *Manager) checkFragment(ctx context.Context) error {
	text, err := manager.systemctl(ctx, "show", "openjwc.service", "--property=FragmentPath,LoadState")
	if strings.Contains(text, "LoadState=not-found") {
		return nil
	}
	if err != nil {
		return err
	}
	fragment := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "FragmentPath=") {
			fragment = strings.TrimPrefix(line, "FragmentPath=")
		}
	}
	if fragment == "" || filepath.Clean(fragment) != filepath.Clean(manager.UnitPath) {
		return fmt.Errorf("同名服务来自其他或无法确认的 unit 路径，拒绝覆盖: %s", fragment)
	}
	return nil
}
