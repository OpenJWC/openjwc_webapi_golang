package recovery

import (
	"context"
	"fmt"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// Restore 定义已持有进程锁时调用的离线恢复能力。
type Restore func(context.Context, string) (string, error)

// Service 仅公开离线恢复操作，不创建数据库或暴露普通管理功能。
type Service struct {
	path    string
	restore Restore
}

// New 组装离线恢复界面需要的受限操作。
func New(path string, restore Restore) *Service { return &Service{path: path, restore: restore} }

// Execute 返回恢复状态或显式执行备份恢复。
func (service *Service) Execute(ctx context.Context, request admin.Request) (admin.Response, error) {
	switch request.Action {
	case admin.ActionStats:
		return admin.Response{Columns: []string{"目标数据库", "模式"}, Rows: [][]string{{service.path, "离线恢复：已锁定数据目录，服务不能同时启动"}}}, nil
	case admin.ActionRestore:
		directory, err := service.restore(ctx, request.ID)
		return admin.Response{Text: "恢复完成；旧数据库保留于 " + directory + "。退出后可启动 openjwc serve。"}, err
	default:
		return admin.Response{}, fmt.Errorf("离线模式仅支持恢复")
	}
}
