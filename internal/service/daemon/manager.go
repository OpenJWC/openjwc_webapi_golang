package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
)

// Runner 定义可替换的封闭进程调用边界。
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// Manager 管理一个明确作用域的 systemd 部署，不直接管理子进程或 PID 文件。
type Manager struct {
	System      bool
	UnitPath    string
	ProfilePath string
	Runner      Runner
	Config      config.Config
}

// New 选择明确的用户级或系统级路径，并恢复已部署的配置。
func New(system bool, cfg config.Config) (*Manager, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	unit := filepath.Join(base, "systemd", "user", "openjwc.service")
	if system {
		base = "/etc"
		unit = "/etc/systemd/system/openjwc.service"
		cfg.DataDir = "/var/lib/openjwc"
		cfg.DatabasePath = filepath.Join(cfg.DataDir, "openjwc.db")
		cfg.SocketPath = filepath.Join(cfg.DataDir, "admin.sock")
	}
	manager := &Manager{System: system, UnitPath: unit, ProfilePath: filepath.Join(base, "openjwc", "service.json"), Runner: processRunner{}, Config: cfg}
	saved, err := manager.profile()
	if err == nil {
		manager.Config = saved.Config
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return manager, nil
}

// systemctl 为每次 systemd 操作显式指定作用域并禁止交互提权。
func (manager *Manager) systemctl(ctx context.Context, args ...string) (string, error) {
	scope := "--user"
	if manager.System {
		scope = "--system"
	}
	return manager.Runner.Run(ctx, "systemctl", append([]string{scope, "--no-pager", "--no-ask-password"}, args...)...)
}

// Preflight 检查修改权限、用户管理器和退出 SSH 后的常驻条件。
func (manager *Manager) Preflight(ctx context.Context) error {
	if manager.System {
		if os.Geteuid() != 0 {
			return fmt.Errorf("系统级管理需要显式 root 授权，请使用 sudo openjwc ... --system")
		}
		if _, err := user.Lookup("openjwc"); err != nil {
			return fmt.Errorf("系统服务账号 openjwc 不存在，请先创建无登录权限的专用账号: %w", err)
		}
		return nil
	}
	if _, err := manager.systemctl(ctx, "show", "--property=Version"); err != nil {
		return fmt.Errorf("用户 systemd 管理器不可用: %w", err)
	}
	current, err := user.Current()
	if err != nil {
		return err
	}
	text, err := manager.Runner.Run(ctx, "loginctl", "show-user", current.Uid, "--property=Linger", "--value")
	if err != nil || trim(text) != "yes" {
		return fmt.Errorf("用户 linger 未启用或无法确认；SSH 退出后不能保证常驻。请授权执行 loginctl enable-linger %s 后重试", current.Username)
	}
	return nil
}

// processRunner 使用有界标准库进程调用，不经过 shell。
type processRunner struct{}

// Run 限制进程运行时间并保留诊断信息。
func (processRunner) Run(parent context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.WaitDelay = time.Second
	data, err := command.CombinedOutput()
	if err != nil {
		return string(data), fmt.Errorf("%s 调用失败: %w: %s", name, err, data)
	}
	return string(data), nil
}
