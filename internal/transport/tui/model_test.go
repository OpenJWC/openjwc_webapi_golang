package tui

import (
	"strings"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	tea "github.com/charmbracelet/bubbletea"
)

// TestPermissionFormPreservesExistingValues 验证权限编辑不会将现有授权重置为默认值。
func TestPermissionFormPreservesExistingValues(t *testing.T) {
	spec := testAction(t, "permissions")
	form := newForm(spec, nil)
	form.fillPermissions(admin.UserPermissions{ID: "7", Username: "student", Email: "mail@example.com", Active: false, Browse: true, Chat: true, Submit: false, MaxDevices: 5})
	request := form.request()
	if request.ID != "7" || request.Values["active"] != "false" || request.Values["chat"] != "true" || request.Values["max_devices"] != "5" {
		t.Fatalf("表单未保留现有值: %#v", request)
	}
}

// TestDestructiveFormRequiresExplicitConfirmation 验证单次回车不会误触发删除。
func TestDestructiveFormRequiresExplicitConfirmation(t *testing.T) {
	form := newForm(testAction(t, "delete-user"), []string{"7"})
	model := Model{form: form}
	model.form.cursor = len(form.fields) - 1
	updated, command := model.updateForm(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || updated.(Model).busy {
		t.Fatal("未确认就执行了删除")
	}
	updated, command = model.updateForm(tea.KeyMsg{Type: tea.KeyEsc})
	if command != nil || updated.(Model).form != nil {
		t.Fatal("Esc 未取消表单")
	}
}

// TestTerminalContentIsSanitizedAndSecretsMasked 验证数据库内容不能注入终端控制序列或泄露设置凭据。
func TestTerminalContentIsSanitizedAndSecretsMasked(t *testing.T) {
	if value := clean("标题\x1b[2J\u202e注入", 100); strings.ContainsAny(value, "\x1b\u202e") {
		t.Fatal("控制字符未清理")
	}
	form := newForm(testAction(t, "set-setting"), []string{"llm_api_key", "[已配置]"})
	for index := range form.fields {
		if form.fields[index].key == "value" {
			form.fields[index].value = "secret-test-value"
		}
	}
	model := Model{form: form, navigation: menus, width: 100, height: 30}
	if strings.Contains(model.View(), "secret-test-value") {
		t.Fatal("密钥输入未遮蔽")
	}
}

// testAction 按动作语义查找测试表单，不依赖菜单排列位置。
func testAction(t *testing.T, action admin.Action) actionSpec {
	t.Helper()
	for _, menu := range menus {
		for _, spec := range menu.actions {
			if spec.action == action {
				return spec
			}
		}
	}
	t.Fatalf("找不到管理动作: %s", action)
	return actionSpec{}
}
