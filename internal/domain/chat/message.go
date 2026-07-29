package chat

import (
	"fmt"
	"strings"
)

// Role 表示对话消息的发送方。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message 表示发送给大语言模型的一条对话消息。
type Message struct {
	role    Role
	content string
}

// NewMessage 创建并校验一条对话消息。
func NewMessage(role Role, content string) (Message, error) {
	if role != RoleSystem && role != RoleUser && role != RoleAssistant {
		return Message{}, fmt.Errorf("不支持的消息角色: %q", role)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return Message{}, fmt.Errorf("消息内容不能为空")
	}
	return Message{role: role, content: content}, nil
}

// Role 返回消息发送方。
func (message Message) Role() Role {
	return message.role
}

// Content 返回消息正文。
func (message Message) Content() string {
	return message.content
}
