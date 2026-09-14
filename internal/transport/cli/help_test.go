package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestHelpDescriptionsShareOneColumn 验证长命令不会使说明列错位，选项沿用同一列宽。
func TestHelpDescriptionsShareOneColumn(t *testing.T) {
	var output bytes.Buffer
	help(&output, options{noBanner: true})
	descriptions := []string{"本地管理终端（默认）", "前台服务，由 systemd 托管时使用", "启动、停止或重启已安装服务", "查看状态；就绪为 0，未就绪为 3", "安装并启用服务，可立即启动", "停止并移除自有 unit；保留数据和配置", "帮助（含子命令帮助）", "构建版本"}
	column := -1
	for _, description := range descriptions {
		found := false
		for _, line := range strings.Split(output.String(), "\n") {
			position := strings.Index(line, description)
			if position < 0 {
				continue
			}
			found = true
			if column < 0 {
				column = position
			} else if position != column {
				t.Errorf("说明 %q 位于第 %d 列，应为 %d", description, position, column)
			}
		}
		if !found {
			t.Errorf("缺少帮助说明: %s", description)
		}
	}
}

// TestStyledTextKeepsVisibleWidth 验证帮助配色不破坏终端对齐且可完全禁用。
func TestStyledTextKeepsVisibleWidth(t *testing.T) {
	text := "service install [--now]  "
	colored := styledText(true, "38;5;245", text)
	if !strings.Contains(colored, "\x1b[38;5;245m") || ansi.StringWidth(colored) != ansi.StringWidth(text) {
		t.Fatal("帮助配色破坏了可见列宽")
	}
	if plain := styledText(false, "38;5;245", text); plain != text {
		t.Fatal("禁用配色后仍改变文本")
	}
}
