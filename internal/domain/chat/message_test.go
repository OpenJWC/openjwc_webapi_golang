package chat_test

import (
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/chat"
)

func TestNewMessageRejectsUnknownRole(t *testing.T) {
	if _, err := chat.NewMessage(chat.Role("tool"), "测试内容"); err == nil {
		t.Fatal("未知消息角色应返回错误")
	}
}
