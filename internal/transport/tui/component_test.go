package tui

import (
	"strings"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// TestTableSeparatorsAlignWithWideCharacters 验证中文和 emoji 的列分隔符按显示宽度对齐。
func TestTableSeparatorsAlignWithWideCharacters(t *testing.T) {
	model := Model{width: 100, height: 30, result: admin.Response{Columns: []string{"标题", "状态"}, Rows: [][]string{{"中文🎓资讯", "通过"}, {"ASCII", "待审"}}}}
	expected := -1
	count := 0
	for _, line := range strings.Split(model.tableView(), "\n") {
		prefix, _, found := strings.Cut(line, "│")
		if !found {
			continue
		}
		width := ansi.StringWidth(prefix)
		if expected < 0 {
			expected = width
		} else if width != expected {
			t.Fatalf("列分隔符不对齐: %d != %d", width, expected)
		}
		count++
	}
	if count < 3 {
		t.Fatalf("未检查到表头和两条记录: %d", count)
	}
}

// TestVimNavigationDoesNotTriggerDigest 验证 gg 只移动光标，普通模式不会输入表单内容。
func TestVimNavigationDoesNotTriggerDigest(t *testing.T) {
	model := Model{navigation: menus, row: 1, result: admin.Response{Rows: [][]string{{"first"}, {"last"}}}}
	for _, pressed := range []rune{'g', 'g'} {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{pressed}})
		model = next.(Model)
	}
	if model.row != 0 || model.form != nil {
		t.Fatal("gg 触发了业务动作")
	}
	model.form = newForm(testAction(t, admin.ActionDeleteUser), []string{"7"})
	before := model.form.fields[0].value
	next, _ := model.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if next.(Model).form.fields[0].value != before {
		t.Fatal("普通模式误输入文字")
	}
}

// TestBatchedVimKeysAndBusyQuitRemainResponsive 验证快速连续 gg 与忙碌时退出都可响应。
func TestBatchedVimKeysAndBusyQuitRemainResponsive(t *testing.T) {
	model := Model{navigation: menus, row: 1, result: admin.Response{Rows: [][]string{{"a"}, {"b"}}}}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("gg")})
	if next.(Model).row != 0 {
		t.Fatal("合并键入的 gg 未响应")
	}
	model.busy = true
	_, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if command == nil {
		t.Fatal("忙碌时 q 无法退出")
	}
}

// TestChineseDetailPreservesItsTailBeforeScrolling 验证中文详情在进入滚动组件前没有被按错误单位截断。
func TestChineseDetailPreservesItsTailBeforeScrolling(t *testing.T) {
	content := strings.Repeat("中文正文", 100) + "重要截止时间在这里"
	model := Model{width: 40, height: 12, result: admin.Response{Columns: []string{"正文"}, Rows: [][]string{{content}}}}
	lines := strings.Join(model.detailLines(), "")
	if !strings.Contains(lines, "重要截止时间在这里") {
		t.Fatal("中文详情尾部丢失")
	}
}
