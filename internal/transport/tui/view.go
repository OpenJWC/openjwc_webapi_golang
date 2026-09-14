package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View 根据当前模式组合有界组件，不把整个数据库正文拼进列表。
func (model Model) View() string {
	if model.width > 0 && (model.width < 30 || model.height < 12) {
		return lipgloss.NewStyle().MaxWidth(max(1, model.width)).MaxHeight(max(1, model.height)).Render("OpenJWC\n终端过小，请放大窗口。\nq / Ctrl+c 退出")
	}
	width := max(16, min(model.width-4, 160))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	var output strings.Builder
	output.WriteString(accent.Render("OpenJWC") + "  本机管理 · NORMAL\n\n")
	if len(model.navigation) > 0 {
		fmt.Fprintf(&output, "‹ %s ›\n\n", model.navigation[model.menu].title)
	}
	switch {
	case model.detail:
		output.WriteString(model.detailViewport().View())
	case model.form != nil:
		output.WriteString(model.formView())
	default:
		if len(model.navigation) > 0 {
			output.WriteString(clean(model.navigation[model.menu].hint, width) + "\n\n")
		}
		if model.searching || model.filter != "" {
			output.WriteString("/ 当前页过滤: " + clean(model.filter, width-16) + "\n")
		}
		output.WriteString(model.tableView())
		if page := model.result.Pagination; page != nil {
			fmt.Fprintf(&output, "\n第 %d/%d 页 · 共 %d 条", page.Index+1, page.Pages(), page.Total)
		} else {
			output.WriteString("\n非分页视图")
		}
	}
	if model.busy {
		output.WriteString("\n处理中…")
	}
	if model.status != "" {
		output.WriteString("\n" + clean(model.status, width*2))
	}
	output.WriteString("\n\n" + model.keyHelp())
	return lipgloss.NewStyle().Padding(1, 2).Width(width).Render(output.String())
}

// detailViewport 将完整详情交给 Bubbles 处理显示宽度和滚动边界。
func (model Model) detailViewport() viewport.Model {
	view := viewport.New(max(12, min(model.width-8, 152)), max(1, model.height-10))
	view.SetContent(strings.Join(model.detailLines(), "\n"))
	view.SetYOffset(model.detailOffset)
	return view
}

// clean 去除终端控制序列并按显示宽度截断，兼容中文和组合字符。
func clean(text string, limit int) string {
	return ansi.Truncate(safeText(text), max(0, limit), "…")
}

// safeText 只清除控制字符，正文详情由 viewport 负责换行而不截断内容。
func safeText(text string) string {
	text = ansi.Strip(text)
	text = strings.Map(func(char rune) rune {
		if unicode.IsControl(char) || unicode.Is(unicode.Cf, char) {
			return ' '
		}
		return char
	}, text)
	return text
}
