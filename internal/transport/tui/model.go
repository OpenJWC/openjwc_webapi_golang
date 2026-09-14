package tui

import (
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	tea "github.com/charmbracelet/bubbletea"
)

// Model 拥有单个终端的显示与选择状态，命令不保存到领域对象中。
type Model struct {
	command         commandBuilder
	menu, row, page int
	result          admin.Response
	status          string
	busy            bool
	form            *form
	width, height   int
	navigation      []menuSpec
	detail          bool
	detailOffset    int
	detailRestore   *detailState
	filter          string
	searching       bool
	pendingG        bool
	showHelp        bool
}

// resultMessage 携带原请求，使结果处理不依赖当前页面猜测操作类型。
type resultMessage struct {
	response admin.Response
	err      error
	request  admin.Request
}

// Init 异步读取首页状态。
func (model Model) Init() tea.Cmd {
	return model.command(admin.Request{Action: model.navigation[0].action})
}

// Update 在唯一界面循环中处理尺寸、异步响应和模式化输入。
func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		if model.form != nil && model.form.insert {
			model.form.input.Width = max(8, model.width-16)
			model.form.area.SetWidth(max(12, model.width-12))
			model.form.area.SetHeight(max(3, min(6, model.height-14)))
		}
	case resultMessage:
		model.busy = false
		if message.err != nil {
			model.status = message.err.Error()
			if model.detailRestore != nil {
				model.result = model.detailRestore.result
				model.row = model.detailRestore.row
				model.filter = model.detailRestore.filter
				model.detail = false
				model.detailRestore = nil
			}
			return model, nil
		}
		response := message.response
		if model.navigation[model.menu].action == admin.ActionUsers {
			response = userTable(response)
		}
		if response.Pagination != nil {
			model.page = response.Pagination.Index
		}
		model.status = response.Text
		model.form = nil
		if response.Columns == nil && response.Users == nil && response.Text != "" && message.request.Action != "" && message.request.Action != admin.ActionCreateKey {
			model.busy = true
			return model, model.command(admin.Request{Action: model.navigation[model.menu].action, Page: model.page})
		}
		model.result = response
		model.row = 0
	case tea.KeyMsg:
		if message.Type == tea.KeyRunes && len(message.Runes) > 1 && !message.Paste && !message.Alt {
			return model.updateKeyBatch(message)
		}
		if message.String() == "ctrl+c" || (message.String() == "q" && model.busy && model.form == nil) {
			return model, tea.Quit
		}
		if model.busy {
			return model, nil
		}
		if model.form != nil {
			return model.updateForm(message)
		}
		if model.detail {
			return model.updateDetail(message)
		}
		return model.updateNormal(message)
	default:
		if model.form != nil && model.form.insert {
			return model.updateEditor(message)
		}
	}
	return model, nil
}

// visibleIndices 将当前页过滤结果映射回原始具名记录，避免编辑错位。
func (model Model) visibleIndices() []int {
	indices := make([]int, 0, len(model.result.Rows))
	query := strings.ToLower(model.filter)
	for index, row := range model.result.Rows {
		if query == "" || strings.Contains(strings.ToLower(strings.Join(row, " ")), query) {
			indices = append(indices, index)
		}
	}
	return indices
}

// selectedIndex 返回真实记录索引，不使用过滤后的显示序号授权。
func (model Model) selectedIndex() int {
	indices := model.visibleIndices()
	if model.row < 0 || model.row >= len(indices) {
		return -1
	}
	return indices[model.row]
}

// selected 复制所选记录，详情与表单不修改原始响应。
func (model Model) selected() []string {
	index := model.selectedIndex()
	if index < 0 {
		return nil
	}
	return append([]string(nil), model.result.Rows[index]...)
}
