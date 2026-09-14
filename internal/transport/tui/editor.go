package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// startEdit 为当前字段建立单行或多行编辑器，秘密字段始终使用掩码。
func (model Model) startEdit() (tea.Model, tea.Cmd) {
	form := model.form
	field := form.fields[form.cursor]
	form.insert = true
	form.multiline = !field.secret && field.key == "value"
	if form.multiline {
		form.area = textarea.New()
		form.area.CharLimit = 32768
		form.area.SetWidth(max(12, model.width-12))
		form.area.SetHeight(max(3, min(6, model.height-14)))
		form.area.SetValue(field.value)
		return model, form.area.Focus()
	}
	form.input = textinput.New()
	form.input.CharLimit = 32768
	form.input.Width = max(8, model.width-16)
	form.input.SetValue(field.value)
	if field.secret {
		form.input.EchoMode = textinput.EchoPassword
		form.input.EchoCharacter = '•'
	}
	return model, form.input.Focus()
}

// updateEditor 将组件输入同步回当前字段，模型消息不得越过编辑模式。
func (model Model) updateEditor(message tea.Msg) (tea.Model, tea.Cmd) {
	form := model.form
	var command tea.Cmd
	if form.multiline {
		form.area, command = form.area.Update(message)
		form.fields[form.cursor].value = form.area.Value()
	} else {
		form.input, command = form.input.Update(message)
		form.fields[form.cursor].value = form.input.Value()
	}
	return model, command
}

// updateForm 区分普通与插入模式，只有确认字段为 yes 才发送管理命令。
func (model Model) updateForm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	form := model.form
	pressed := key.String()
	if form.insert {
		if pressed == "esc" || pressed == "ctrl+s" || (pressed == "enter" && !form.multiline) {
			form.insert = false
			form.input.Blur()
			form.area.Blur()
			return model, nil
		}
		if pressed == "ctrl+u" {
			form.fields[form.cursor].value = ""
			if form.multiline {
				form.area.SetValue("")
			} else {
				form.input.SetValue("")
			}
			return model, nil
		}
		return model.updateEditor(key)
	}
	switch pressed {
	case "esc":
		model.form = nil
	case "i":
		return model.startEdit()
	case "up", "k", "shift+tab":
		form.cursor = max(0, form.cursor-1)
	case "down", "j", "tab":
		form.cursor = min(len(form.fields)-1, form.cursor+1)
	case "enter":
		if form.cursor < len(form.fields)-1 {
			form.cursor++
			return model, nil
		}
		if strings.TrimSpace(form.fields[form.cursor].value) != "yes" {
			model.status = "按 i 编辑确认字段并输入 yes；Esc 回普通模式后 Enter 提交"
			return model, nil
		}
		model.busy = true
		return model, model.command(form.request())
	}
	return model, nil
}
