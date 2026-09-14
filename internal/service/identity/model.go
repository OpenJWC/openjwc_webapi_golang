package identity

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
)

// RegistrationInput 聚合通过客户端提交的待审核注册资料。
type RegistrationInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password_hash"`
}

// Validate 检查注册边界，密码上限与旧版 bcrypt 行为保持一致。
func (input RegistrationInput) Validate() error {
	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email || len(input.Email) > 254 {
		return fmt.Errorf("邮箱格式无效")
	}
	if strings.TrimSpace(input.Username) != input.Username || input.Username == "" || len(input.Username) > 128 || strings.ContainsAny(input.Username, "@ \r\n\t\x00") {
		return fmt.Errorf("用户名格式无效")
	}
	if len(input.Password) < 1 || len(input.Password) > 72 {
		return fmt.Errorf("password_hash 长度必须为 1～72 字节")
	}
	return nil
}

// LoginInput 聚合登录接口的请求参数。
type LoginInput struct {
	Account    string `json:"account"`
	Password   string `json:"password_hash"`
	DeviceName string `json:"device_name"`
}

// LoginResult 是登录成功后返回的显式响应数据。
type LoginResult struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Device 是客户端可见的设备列表投影，不包含凭据摘要。
type Device struct {
	UUID      string `json:"device_uuid"`
	Name      string `json:"device_name"`
	LastLogin string `json:"last_login"`
}

// Repository 是客户端认证服务消费的最小存储能力。
type Repository interface {
	Register(context.Context, RegistrationInput, string) error
	Login(context.Context, LoginInput, string) (LoginResult, error)
	Authenticate(context.Context, string, string) (access.Principal, error)
	Devices(context.Context, int64) ([]Device, error)
	Unbind(context.Context, int64, string) (bool, error)
}
