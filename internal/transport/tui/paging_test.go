package tui

import (
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

// TestSinglePageAndNonPagedViewsIgnorePageKeys 验证单页和非分页页面不发送无效翻页请求。
func TestSinglePageAndNonPagedViewsIgnorePageKeys(t *testing.T) {
	for _, page := range []*admin.Pagination{nil, {Index: 0, Size: 200, Total: 1}, {Index: 0, Size: 200, Total: 0}} {
		calls := 0
		model := Model{navigation: menus, result: admin.Response{Pagination: page}, command: func(request admin.Request) tea.Cmd { calls++; return nil }}
		for _, value := range []rune{'[', ']'} {
			_, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{value}})
		}
		if calls != 0 {
			t.Fatalf("无效翻页仍请求后端: %d", calls)
		}
	}
}

// TestFailedPageRequestKeepsOriginalPage 验证翻页失败不改变原列表和页码。
func TestFailedPageRequestKeepsOriginalPage(t *testing.T) {
	model := Model{navigation: menus, result: admin.Response{Pagination: &admin.Pagination{Index: 0, Size: 200, Total: 201}}, command: func(request admin.Request) tea.Cmd { return nil }}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if next.(Model).page != 0 {
		t.Fatal("尚未成功便更改了页码")
	}
}
