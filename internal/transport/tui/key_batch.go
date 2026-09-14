package tui

import tea "github.com/charmbracelet/bubbletea"

// updateKeyBatch 顺序处理终端合并的快速键入，不把 gg 当成无法识别的单键。
func (model Model) updateKeyBatch(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd
	for _, char := range message.Runes {
		quitting := char == 'q' && model.form == nil && !model.detail && !model.searching
		updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}})
		model = updated.(Model)
		if command != nil {
			commands = append(commands, command)
		}
		if model.busy || quitting {
			break
		}
	}
	return model, tea.Batch(commands...)
}
