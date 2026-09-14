package tui

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/brand"
	tea "github.com/charmbracelet/bubbletea"
)

// commandBuilder 构造单次管理调用，界面模型无需持有会话上下文。
type commandBuilder func(admin.Request) tea.Cmd

// ExitOptions 定义退出后的展示开关和重新进入命令，不保存用户数据。
type ExitOptions struct {
	NoBanner      bool
	ReturnCommand string
}

// Run 拥有在线 TUI 会话的取消边界，退出时取消尚未完成的管理请求。
func Run(parent context.Context, executor admin.Executor, out io.Writer, display ExitOptions) error {
	started := time.Now()
	err := runProgram(parent, executor, menus)
	brand.Print(out, display.NoBanner)
	fmt.Fprintf(out, "  OpenJWC · 本地终端已退出 · 会话 %s\n", time.Since(started).Round(time.Second))
	probe, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	state, probeErr := executor.Execute(probe, admin.Request{Action: admin.ActionServiceStatus})
	if probeErr == nil && len(state.Rows) > 0 && len(state.Rows[0]) > 1 {
		fmt.Fprintln(out, "  服务状态:", clean(state.Rows[0][1], 40))
	} else {
		fmt.Fprintln(out, "  使用 openjwc status 查看服务状态")
	}
	if display.ReturnCommand == "" {
		display.ReturnCommand = "openjwc"
	}
	fmt.Fprintln(out, "  重新进入:", clean(display.ReturnCommand, 120))
	fmt.Fprintln(out)
	return err
}

// runProgram 统一拥有在线和离线终端的取消、请求等待与界面生命周期。
func runProgram(parent context.Context, executor admin.Executor, navigation []menuSpec) error {
	ctx, cancel := context.WithCancel(parent)
	scope := &requestScope{next: executor}
	defer func() {
		cancel()
		scope.closeAndWait()
	}()
	model := Model{
		command: newCommandBuilder(ctx, scope),
		width:   100, height: 30, busy: true, navigation: navigation,
	}
	_, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx)).Run()
	return err
}

// newCommandBuilder 将会话取消范围绑定到命令构造能力，而不是模型字段。
func newCommandBuilder(ctx context.Context, executor admin.Executor) commandBuilder {
	return func(request admin.Request) tea.Cmd {
		return func() tea.Msg {
			response, err := executor.Execute(ctx, request)
			return resultMessage{response: response, err: err, request: request}
		}
	}
}
