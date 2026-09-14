package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// tableView 按终端单元宽度分配列，复用 Bubbles 的表头、选择和滚动渲染。
func (model Model) tableView() string {
	if len(model.result.Columns) == 0 {
		return "暂无记录"
	}
	available := max(12, min(model.width-8, 152))
	count := min(len(model.result.Columns), max(1, available/9))
	columns := make([]table.Column, count)
	for index := range columns {
		width := max(4, lipgloss.Width(clean(model.result.Columns[index], 40)))
		for _, row := range model.result.Rows {
			if index < len(row) {
				width = max(width, min(32, lipgloss.Width(clean(row[index], 100))))
			}
		}
		columns[index] = table.Column{Title: clean(model.result.Columns[index], 40), Width: width}
	}
	for {
		total, largest := 0, 0
		for index, col := range columns {
			total += col.Width + 3
			if col.Width > columns[largest].Width {
				largest = index
			}
		}
		if total <= available || columns[largest].Width <= 3 {
			break
		}
		columns[largest].Width--
	}
	rows := make([]table.Row, 0, len(model.result.Rows))
	for _, index := range model.visibleIndices() {
		row := make(table.Row, count)
		for col := range row {
			if col < len(model.result.Rows[index]) {
				row[col] = clean(model.result.Rows[index][col], 200)
			}
		}
		rows = append(rows, row)
	}
	view := table.New(table.WithColumns(columns), table.WithRows(rows), table.WithHeight(max(1, model.height-14)), table.WithWidth(available), table.WithFocused(true))
	cell := lipgloss.NewStyle().Padding(0, 1).BorderStyle(lipgloss.NormalBorder()).BorderRight(true)
	view.SetStyles(table.Styles{Header: cell.Bold(true), Cell: cell, Selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))})
	view.SetCursor(model.row)
	return view.View()
}
