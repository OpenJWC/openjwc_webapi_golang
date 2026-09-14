package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	"strings"
)

// formView 让短终端中的长表单随当前字段滚动，秘密字段始终遮蔽。
func (model Model) formView() string {
	width := max(12, min(model.width-8, 152))
	var output strings.Builder
	output.WriteString(clean(model.form.spec.title, width) + "\n\n")
	activeLine := 0
	for index, field := range model.form.fields {
		prefix := "  "
		if index == model.form.cursor {
			prefix = "> "
			activeLine = strings.Count(output.String(), "\n")
		}
		value := field.value
		if field.secret {
			value = strings.Repeat("•", min(len([]rune(value)), 40))
		}
		if index == model.form.cursor && model.form.insert {
			output.WriteString(clean(prefix+field.label+" [INSERT]", width) + "\n")
			if model.form.multiline {
				output.WriteString(model.form.area.View())
			} else {
				output.WriteString(model.form.input.View())
			}
			output.WriteString("\n")
		} else {
			output.WriteString(clean(prefix+field.label+": "+value, width) + "\n")
		}
	}
	view := viewport.New(width, max(3, model.height-10))
	view.SetContent(output.String())
	view.SetYOffset(max(0, activeLine-view.Height/2))
	return view.View()
}
