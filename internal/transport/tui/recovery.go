package tui

import (
	"context"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// RunRecovery 提供只含离线恢复动作的终端，共享明确的会话取消与请求等待边界。
func RunRecovery(parent context.Context, executor admin.Executor) error {
	navigation := []menuSpec{{
		title: "离线恢复", action: admin.ActionStats,
		hint: "b 从一致备份恢复 · 原库保留在隔离目录",
		actions: map[string]actionSpec{
			"b": {action: admin.ActionRestore, title: "从备份恢复（目标填写备份来源路径）", target: true},
		},
	}}
	return runProgram(parent, executor, navigation)
}
