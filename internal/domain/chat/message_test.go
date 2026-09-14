package chat_test

import (
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/chat"
)

// TestNewMessageRejectsUnknownRole 验证未知对话角色不能进入领域对象。
func TestNewMessageRejectsUnknownRole(t *testing.T) {
	if _, err := chat.NewMessage(chat.Role("tool"), "测试内容"); err == nil {
		t.Fatal("未知消息角色应返回错误")
	}
}
