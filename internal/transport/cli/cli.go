package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/app"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/license"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/daemon"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/brand"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/control"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/tui"
)

// options 保存命令解析结果，不承担业务状态。
type options struct {
	system, user, json, now, help, version, noBanner bool
	command, action                                  string
}

// parse 解析无歧义的布尔选项与子命令，未知参数返回用法错误。
func parse(args []string) (options, error) {
	var result options
	flags := flag.NewFlagSet("openjwc", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&result.system, "system", false, "")
	flags.BoolVar(&result.user, "user", false, "")
	flags.BoolVar(&result.json, "json", false, "")
	flags.BoolVar(&result.now, "now", false, "")
	flags.BoolVar(&result.help, "h", false, "")
	flags.BoolVar(&result.help, "help", false, "")
	flags.BoolVar(&result.version, "version", false, "")
	flags.BoolVar(&result.noBanner, "no-banner", false, "")
	var positional, switches []string
	ended := false
	for _, arg := range args {
		if arg == "--" && !ended {
			ended = true
			continue
		}
		if strings.HasPrefix(arg, "-") && !ended {
			switches = append(switches, arg)
		} else {
			positional = append(positional, arg)
		}
	}
	if err := flags.Parse(switches); err != nil {
		return result, err
	}
	if result.system && result.user {
		return result, fmt.Errorf("--user 与 --system 不能同时使用")
	}
	if len(positional) > 2 {
		return result, fmt.Errorf("命令参数过多")
	}
	result.command = "tui"
	if len(positional) > 0 {
		result.command = positional[0]
	}
	if len(positional) > 1 {
		result.action = positional[1]
	}
	if result.command != "service" && result.action != "" {
		return result, fmt.Errorf("此命令不接受位置参数")
	}
	switch result.command {
	case "help", "tui", "serve", "recover", "licenses", "start", "stop", "restart", "status", "service":
	default:
		return result, fmt.Errorf("未知命令 %q", result.command)
	}
	if result.command == "service" && result.action != "install" && result.action != "uninstall" && !result.help {
		return result, fmt.Errorf("用法: openjwc service install|uninstall [--user|--system]")
	}
	if result.now && (result.command != "service" || result.action != "install") {
		return result, fmt.Errorf("--now 仅用于 service install")
	}
	if (result.system || result.user) && result.command == "serve" {
		return result, fmt.Errorf("serve 是前台运行模式，请通过环境变量配置；作用域选项用于部署管理或 TUI")
	}
	if result.json && result.command != "status" {
		return result, fmt.Errorf("--json 仅用于 status")
	}
	return result, nil
}

// Run 执行命令并返回稳定退出码，帮助不依赖服务或配置。
func Run(ctx context.Context, args []string, out io.Writer) (int, error) {
	opts, err := parse(args)
	if err != nil {
		return 2, err
	}
	if opts.help || opts.command == "help" {
		help(out, opts)
		return 0, nil
	}
	if opts.version {
		fmt.Fprintln(out, "openjwc", Version)
		return 0, nil
	}
	if opts.command == "licenses" {
		fmt.Fprintln(out, license.ThirdParty, license.Crawler)
		return 0, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return 1, err
	}
	if opts.command == "serve" {
		err = app.Run(ctx, cfg, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
		if err != nil {
			return 1, err
		}
		return 0, nil
	}
	manager, err := daemon.New(opts.system, cfg)
	if err != nil {
		return 1, err
	}
	switch opts.command {
	case "start", "stop", "restart":
		err = manager.Change(ctx, opts.command)
	case "status":
		status, statusErr := manager.Status(ctx)
		if statusErr != nil {
			return 1, statusErr
		}
		if opts.json {
			err = json.NewEncoder(out).Encode(status)
		} else {
			fmt.Fprintf(out, "OpenJWC: %s\nscope: %s\npid: %d\ndata: %s\n%s\n", status.State, status.Scope, status.PID, status.DataDir, status.Detail)
		}
		if err == nil && !status.Ready {
			return 3, nil
		}
	case "service":
		if opts.action == "uninstall" {
			err = manager.Uninstall(ctx)
		} else {
			if err = configureInstall(manager, cfg); err != nil {
				return 1, err
			}
			binary, binaryErr := os.Executable()
			if binaryErr != nil {
				return 1, binaryErr
			}
			err = manager.Install(ctx, binary)
			if err == nil && opts.now {
				err = manager.Change(ctx, "start")
			}
		}
	case "recover":
		err = app.Recover(ctx, manager.Config)
		if err == nil {
			brand.Print(out, opts.noBanner)
			fmt.Fprintln(out, "  OpenJWC · 恢复终端已退出；服务不会自动启动")
		}
	case "tui":
		client := control.NewClient(manager.Config.SocketPath)
		defer client.Close()
		returnCommand := "openjwc"
		if opts.system {
			returnCommand += " --system"
		}
		err = tui.Run(ctx, newLocalAdmin(manager, client), out, tui.ExitOptions{NoBanner: opts.noBanner, ReturnCommand: returnCommand})
	}
	if err != nil {
		return 1, err
	}
	if opts.command == "serve" {
		return 0, nil
	}
	return 0, nil
}
