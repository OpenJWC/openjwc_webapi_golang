package daemon

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/control"
)

// Status 是脚本与 TUI 共用的显式服务状态投影。
type Status struct {
	Scope   string `json:"scope"`
	State   string `json:"state"`
	Active  string `json:"active"`
	PID     int    `json:"pid"`
	Ready   bool   `json:"ready"`
	DataDir string `json:"data_dir"`
	Detail  string `json:"detail,omitempty"`
}

// Status 同时检查 systemd 状态和对应进程的私有就绪接口。
func (manager *Manager) Status(parent context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	result := Status{Scope: "user", DataDir: manager.Config.DataDir}
	if manager.System {
		result.Scope = "system"
	}
	text, err := manager.systemctl(ctx, "show", "openjwc.service", "--property=LoadState,ActiveState,SubState,MainPID")
	values := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[key] = value
		}
	}
	if values["LoadState"] == "not-found" {
		result.State = "not-installed"
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if values["LoadState"] != "loaded" {
		return result, fmt.Errorf("服务 unit 无法加载: %s", values["LoadState"])
	}
	result.Active = values["ActiveState"]
	result.PID, _ = strconv.Atoi(values["MainPID"])
	switch result.Active {
	case "inactive":
		result.State = "stopped"
	case "failed":
		result.State = "failed"
	case "activating", "deactivating":
		result.State = result.Active
	case "active":
		result.State = "not-ready"
		client := control.NewClient(manager.Config.SocketPath)
		defer client.Close()
		probe, cancelProbe := context.WithTimeout(ctx, time.Second)
		defer cancelProbe()
		pid, probeErr := client.Probe(probe)
		result.Ready = probeErr == nil && pid == result.PID && pid > 0
		if result.Ready {
			result.State = "running"
		} else {
			result.Detail = "systemd 进程与就绪接口尚未共同确认"
		}
	default:
		result.State = "unknown"
	}
	return result, nil
}

// Change 执行封闭生命周期动作，并等待停止或就绪状态。
func (manager *Manager) Change(parent context.Context, action string) error {
	if action != "start" && action != "stop" && action != "restart" {
		return fmt.Errorf("未知生命周期动作")
	}
	if manager.System && os.Geteuid() != 0 {
		return fmt.Errorf("系统级服务控制需要显式 root 授权")
	}
	if action != "stop" {
		if err := manager.Preflight(parent); err != nil {
			return err
		}
	}
	lock, err := manager.lockDeployment()
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = manager.checkFragment(parent); err != nil {
		return err
	}
	if _, err := os.Stat(manager.UnitPath); os.IsNotExist(err) {
		return fmt.Errorf("服务未安装，请先运行 openjwc service install")
	} else if err != nil {
		return err
	}
	data, err := os.ReadFile(manager.UnitPath)
	if err != nil {
		return err
	}
	saved, err := manager.profile()
	if err != nil || digest(data) != saved.UnitHash {
		return fmt.Errorf("unit 未由本 CLI 管理或已被修改，拒绝生命周期变更")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	if _, err := manager.systemctl(ctx, action, "openjwc.service"); err != nil {
		return err
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err := manager.Status(ctx)
		if err != nil {
			return err
		}
		if action == "stop" && (status.State == "stopped" || status.State == "failed") {
			return nil
		}
		if action != "stop" && status.Ready {
			return nil
		}
		if status.State == "failed" {
			return fmt.Errorf("服务启动失败，请查看 journalctl 日志")
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待服务状态超时: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
