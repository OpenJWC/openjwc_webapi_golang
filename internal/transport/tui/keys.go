package tui

import (
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// bindings 提供当前模式对应的 Bubbles 帮助协议。
type bindings struct{ items []key.Binding }

// ShortHelp 返回精简键位帮助。
func (keys bindings) ShortHelp() []key.Binding { return keys.items }

// FullHelp 返回按模式整理的完整键位帮助。
func (keys bindings) FullHelp() [][]key.Binding { return [][]key.Binding{keys.items} }

// keyHelp 为导航、详情和表单生成对应帮助，不隐藏 Vim 模式。
func (model Model) keyHelp() string {
	labels := [][2]string{{"h/l", "页面"}, {"j/k", "选择"}, {"gg/G", "首尾"}, {"ctrl+d/u", "半页"}, {"[/]", "数据页"}, {"/", "本页过滤"}, {"?", "帮助"}, {"q", "退出"}}
	if model.detail {
		labels = [][2]string{{"j/k", "滚动"}, {"gg/G", "首尾"}, {"ctrl+d/u", "半页"}, {"esc", "返回"}}
	}
	if model.form != nil {
		labels = [][2]string{{"j/k,tab", "字段"}, {"i", "插入模式"}, {"enter", "下一项/确认"}, {"esc", "取消"}}
		if model.form.insert {
			labels = [][2]string{{"esc", "普通模式"}, {"ctrl+s", "完成编辑"}, {"ctrl+u", "清空"}}
		}
	}
	items := make([]key.Binding, 0, len(labels))
	for _, label := range labels {
		items = append(items, key.NewBinding(key.WithKeys(label[0]), key.WithHelp(label[0], label[1])))
	}
	view := help.New()
	view.Width = max(20, model.width-8)
	view.ShowAll = model.showHelp
	return view.View(bindings{items: items})
}

// updateNormal 处理默认 Vim 普通模式，页面边界不会发起请求。
func (model Model) updateNormal(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	pressed := message.String()
	if model.searching {
		switch pressed {
		case "esc", "enter":
			model.searching = false
		case "backspace":
			runes := []rune(model.filter)
			if len(runes) > 0 {
				model.filter = string(runes[:len(runes)-1])
			}
		default:
			if message.Type == tea.KeyRunes && len(model.filter) < 256 {
				model.filter += string(message.Runes)
			}
		}
		model.row = 0
		return model, nil
	}
	if pressed == "g" {
		if model.pendingG {
			model.row = 0
			model.pendingG = false
		} else {
			model.pendingG = true
		}
		return model, nil
	}
	model.pendingG = false
	count := len(model.visibleIndices())
	switch pressed {
	case "?":
		model.showHelp = !model.showHelp
	case "/":
		model.searching = true
		model.filter = ""
		model.row = 0
	case "esc":
		if model.filter != "" {
			model.filter = ""
			model.row = 0
			return model, nil
		}
		return model, tea.Quit
	case "q":
		return model, tea.Quit
	case "enter":
		return model.openDetail()
	case "left", "h", "right", "l", "tab":
		delta := 1
		if pressed == "left" || pressed == "h" {
			delta = -1
		}
		model.menu = (model.menu + delta + len(model.navigation)) % len(model.navigation)
		model.row = 0
		model.page = 0
		model.filter = ""
		model.result = admin.Response{}
		model.busy = true
		return model, model.command(admin.Request{Action: model.navigation[model.menu].action})
	case "up", "k":
		model.row = max(0, model.row-1)
	case "down", "j":
		model.row = min(max(0, count-1), model.row+1)
	case "G":
		model.row = max(0, count-1)
	case "ctrl+u", "pgup":
		model.row = max(0, model.row-max(1, (model.height-12)/2))
	case "ctrl+d", "pgdown":
		model.row = min(max(0, count-1), model.row+max(1, (model.height-12)/2))
	case "[", "]":
		page := model.result.Pagination
		if page == nil {
			return model, nil
		}
		next := model.page + 1
		if pressed == "[" {
			next = model.page - 1
		}
		if next < 0 || next >= page.Pages() {
			return model, nil
		}
		model.busy = true
		return model, model.command(admin.Request{Action: model.navigation[model.menu].action, Page: next})
	case "r":
		model.busy = true
		return model, model.command(admin.Request{Action: model.navigation[model.menu].action, Page: model.page})
	default:
		if spec, ok := model.navigation[model.menu].actions[pressed]; ok {
			index := model.selectedIndex()
			if spec.action == admin.ActionPermissions && (index < 0 || index >= len(model.result.Users)) {
				model.status = "没有可编辑的用户权限记录"
				return model, nil
			}
			model.form = newForm(spec, model.selected())
			if spec.action == admin.ActionPermissions {
				model.form.fillPermissions(model.result.Users[index])
			}
			model.status = ""
		}
	}
	return model, nil
}
