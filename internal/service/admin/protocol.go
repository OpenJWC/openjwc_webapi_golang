package admin

import (
	"context"
	"fmt"
)

// Request 是仅在受保护本地套接字上传输的管理命令。
type Request struct {
	Action Action            `json:"action"`
	Page   int               `json:"page,omitempty"`
	ID     string            `json:"id,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

// Response 是 TUI 表格或消息的显式传输结构。
type Response struct {
	Pagination *Pagination       `json:"pagination,omitempty"`
	Users      []UserPermissions `json:"users,omitempty"`
	Columns    []string          `json:"columns,omitempty"`
	Rows       [][]string        `json:"rows,omitempty"`
	Text       string            `json:"text,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// Executor 定义 TUI 可用的本地管理入口。
type Executor interface {
	Execute(context.Context, Request) (Response, error)
}

// Validate 拒绝过大的命令参数及未知动作。
func (request Request) Validate() error {
	if len(request.ID) > 256 || len(request.Values) > 20 || request.Page < 0 || request.Page > 5000 {
		return fmt.Errorf("管理参数超出范围")
	}
	for key, value := range request.Values {
		if len(key) > 64 || len(value) > 32768 {
			return fmt.Errorf("管理字段超出范围")
		}
	}
	if !request.Action.Valid() {
		return fmt.Errorf("未知管理命令")
	}
	return nil
}
