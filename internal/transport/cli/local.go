package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/daemon"
)

// localAdmin 将本地服务管理与在线业务 RPC 分离，停止服务后仍可管理部署。
type localAdmin struct {
	manager *daemon.Manager
	remote  admin.Executor
}

// newLocalAdmin 组装显式的本地操作依赖，不向服务端暴露提权入口。
func newLocalAdmin(manager *daemon.Manager, remote admin.Executor) *localAdmin {
	return &localAdmin{manager: manager, remote: remote}
}

// Execute 在本地处理服务生命周期，其余动作仍通过私有 RPC。
func (local *localAdmin) Execute(ctx context.Context, request admin.Request) (admin.Response, error) {
	switch request.Action {
	case admin.ActionServiceStatus:
		status, err := local.manager.Status(ctx)
		if err != nil {
			return admin.Response{Text: "服务管理器不可用；仍可查看帮助或退出"}, err
		}
		return admin.Response{Columns: []string{"作用域", "状态", "PID", "数据目录"}, Rows: [][]string{{status.Scope, status.State, fmt.Sprint(status.PID), status.DataDir}}}, nil
	case admin.ActionServiceStart, admin.ActionServiceStop, admin.ActionServiceRestart, admin.ActionServiceInstall, admin.ActionServiceUninstall:
		if local.manager.System {
			return admin.Response{}, fmt.Errorf("系统级变更请退出 TUI 后使用显式授权的 CLI 命令")
		}
		var err error
		switch request.Action {
		case admin.ActionServiceInstall:
			binary, binaryErr := os.Executable()
			if binaryErr != nil {
				return admin.Response{}, binaryErr
			}
			err = local.manager.Install(ctx, binary)
		case admin.ActionServiceUninstall:
			err = local.manager.Uninstall(ctx)
		case admin.ActionServiceStart:
			err = local.manager.Change(ctx, "start")
		case admin.ActionServiceStop:
			err = local.manager.Change(ctx, "stop")
		case admin.ActionServiceRestart:
			err = local.manager.Change(ctx, "restart")
		}
		if err != nil {
			return admin.Response{}, err
		}
		return local.Execute(ctx, admin.Request{Action: admin.ActionServiceStatus})
	default:
		return local.remote.Execute(ctx, request)
	}
}
