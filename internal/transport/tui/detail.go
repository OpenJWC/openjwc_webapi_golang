package tui

import (
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// detailState 保存进入详情前的列表快照与选中位置。
type detailState struct {
	result admin.Response
	row    int
	filter string
}

// openDetail 对正文记录单独取回全文，其他表格直接展开当前投影。
func (model Model) openDetail() (tea.Model, tea.Cmd) {
	row := model.selected()
	if len(row) == 0 {
		return model, nil
	}
	model.detail, model.detailOffset = true, 0
	var action admin.Action
	switch model.navigation[model.menu].action {
	case admin.ActionNotices:
		action = admin.ActionNoticeDetail
	case admin.ActionSubmissions:
		action = admin.ActionSubmissionDetail
	case admin.ActionReports:
		action = admin.ActionReportDetail
	}
	if action == "" {
		return model, nil
	}
	model.detailRestore = &detailState{result: model.result, row: model.row, filter: model.filter}
	model.filter = ""
	model.busy = true
	return model, model.command(admin.Request{Action: action, ID: row[0]})
}

// detailLines 将选中记录全部字段折行为可滚动文本，不截断审核正文或提示词。
func (model Model) detailLines() []string {
	row := model.selected()
	var lines []string
	width := max(20, min(model.width-8, 150))
	for index, value := range row {
		label := "字段"
		if index < len(model.result.Columns) {
			label = model.result.Columns[index]
		}
		lines = append(lines, clean(label, 100)+":")
		for _, line := range strings.Split(value, "\n") {
			wrapped := lipgloss.NewStyle().Width(width).Render(safeText(line))
			lines = append(lines, strings.Split(wrapped, "\n")...)
		}
		lines = append(lines, "")
	}
	if len(lines) == 0 {
		return []string{"没有可查看的记录"}
	}
	return lines
}

// updateDetail 处理详情阅读键位，返回列表不会触发任何修改。
func (model Model) updateDetail(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	view := model.detailViewport()
	switch key.String() {
	case "esc", "q", "enter":
		model.detail = false
		if model.detailRestore != nil {
			model.result = model.detailRestore.result
			model.row = model.detailRestore.row
			model.filter = model.detailRestore.filter
			model.detailRestore = nil
		}
	case "g":
		if model.pendingG {
			view.GotoTop()
			model.pendingG = false
		} else {
			model.pendingG = true
		}
		model.detailOffset = view.YOffset
		return model, nil
	case "G":
		view.GotoBottom()
	case "up", "k":
		view.ScrollUp(1)
	case "down", "j":
		view.ScrollDown(1)
	case "ctrl+u", "pgup":
		view.HalfPageUp()
	case "ctrl+d", "pgdown":
		view.HalfPageDown()
	}
	model.pendingG = false
	model.detailOffset = view.YOffset
	return model, nil
}
