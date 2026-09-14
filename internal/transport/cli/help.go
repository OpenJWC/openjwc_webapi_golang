package cli

import (
	"fmt"
	"io"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/brand"
)

// Version 由发布构建注入版本号，开发构建明确显示 dev。
var Version = "dev"

// helpRow 将命令与说明分开保存，避免手工空格在新增长命令后失效。
type helpRow struct {
	command     string
	description string
}

// help 输出动态对齐的帮助，正常帮助不依赖服务和配置。
func help(out io.Writer, opts options) {
	colored := brand.ColorEnabled(out)
	accent := func(text string) string { return styledText(colored, "1;38;5;86", text) }
	strong := func(text string) string { return styledText(colored, "1", text) }
	commands := []helpRow{
		{command: "tui", description: "本地管理终端（默认）"},
		{command: "serve", description: "前台服务，由 systemd 托管时使用"},
		{command: "start | stop | restart", description: "启动、停止或重启已安装服务"},
		{command: "status [--json]", description: "查看状态；就绪为 0，未就绪为 3"},
		{command: "service install [--now]", description: "安装并启用服务，可立即启动"},
		{command: "service uninstall", description: "停止并移除自有 unit；保留数据和配置"},
		{command: "recover", description: "服务停止后的离线恢复终端"},
		{command: "licenses", description: "第三方许可证"},
	}
	flags := []helpRow{
		{command: "-h, --help", description: "帮助（含子命令帮助）"},
		{command: "--version", description: "构建版本"},
		{command: "--user", description: "用户级服务（默认，检查 linger）"},
		{command: "--system", description: "系统级服务；变更必须显式 root 授权"},
		{command: "--no-banner", description: "不显示帮助及退出时的块状字标"},
		{command: "--", description: "结束选项解析"},
	}
	width := 0
	for _, rows := range [][]helpRow{commands, flags} {
		for _, row := range rows {
			width = max(width, len(row.command))
		}
	}
	brand.Print(out, opts.noBanner)
	fmt.Fprintf(out, "%s %s\n\n%s\n", accent("Usage:"), strong("openjwc [COMMAND] [OPTIONS]"), accent("Commands:"))
	printHelpRows(out, commands, width, colored)
	fmt.Fprintf(out, "\n%s\n", accent("Options:"))
	printHelpRows(out, flags, width, colored)
	fmt.Fprintf(out, "\n%s 0 成功，1 操作失败，2 用法错误，3 服务未就绪。\n", accent("Exit status:"))
	fmt.Fprintf(out, "%s %s\n", accent("首次使用:"), strong("openjwc service install --now"))
	fmt.Fprintf(out, "%s 明确授权 %s。\n", accent("常驻前提:"), strong("loginctl enable-linger <用户名>"))
	fmt.Fprintf(out, "%s OPENJWC_DATA_DIR、OPENJWC_HTTP_ADDRESS、OPENJWC_TLS_CERT、OPENJWC_TLS_KEY、OPENJWC_CRAWLER_PROGRAMS。\n", accent("环境变量:"))
	fmt.Fprintln(out, "安装时保存启动配置；后续生命周期命令使用已保存的部署配置。")
	if opts.command != "tui" && opts.command != "help" {
		fmt.Fprintf(out, "\n%s %s %s\n", accent("当前命令:"), opts.command, opts.action)
	}
}

// printHelpRows 根据统一命令列宽输出说明，先填充再着色以保持终端列对齐。
func printHelpRows(out io.Writer, rows []helpRow, width int, colored bool) {
	for _, row := range rows {
		command := fmt.Sprintf("%-*s", width, row.command)
		fmt.Fprintf(out, "  %s  %s\n", styledText(colored, "38;5;245", command), row.description)
	}
}

// styledText 仅在已确认颜色能力时添加可自闭合的 SGR 样式。
func styledText(enabled bool, code, text string) string {
	if !enabled {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}
