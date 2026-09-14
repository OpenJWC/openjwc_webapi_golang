package access

import (
	"errors"
	"fmt"
)

// ErrUnauthorized 表示客户端凭据不存在、过期或已撤销。
var ErrUnauthorized = errors.New("客户端凭据无效")

// ErrForbidden 表示当前身份不具备操作权限。
var ErrForbidden = errors.New("当前身份无权执行操作")

// ErrConflict 表示唯一值、设备配额或状态迁移发生冲突。
var ErrConflict = errors.New("操作与现有状态冲突")

// Permission 定义用户可独立授予的业务能力。
type Permission string

// Browse 允许读取资讯及其派生检索结果。
const Browse Permission = "browse"

// Chat 允许调用智能问答。
const Chat Permission = "chat"

// Submit 允许提交资讯。
const Submit Permission = "submit"

// Principal 是经过存储层复核的不可变客户端身份。
type Principal struct {
	id       int64
	username string
	browse   bool
	chat     bool
	submit   bool
}

// RestorePrincipal 从已验证记录恢复身份，拒绝无效标识。
func RestorePrincipal(id int64, username string, browse bool, chat bool, submit bool) (Principal, error) {
	if id < 1 || username == "" {
		return Principal{}, fmt.Errorf("身份标识无效")
	}
	return Principal{id: id, username: username, browse: browse, chat: chat, submit: submit}, nil
}

// ID 返回身份关联的用户标识。
func (principal Principal) ID() int64 { return principal.id }

// Username 返回已验证的用户名。
func (principal Principal) Username() string { return principal.username }

// Allows 判断当前身份是否拥有指定能力。
func (principal Principal) Allows(permission Permission) bool {
	switch permission {
	case Browse:
		return principal.browse
	case Chat:
		return principal.chat && principal.browse
	case Submit:
		return principal.submit
	default:
		return false
	}
}
