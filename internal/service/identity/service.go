package identity

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Service 限制昂贵密码运算的并发，避免匿名登录耗尽 CPU。
type Service struct {
	repository Repository
	slots      chan struct{}
}

// New 创建有界身份服务，由应用共享实例。
func New(repository Repository) *Service {
	return &Service{repository: repository, slots: make(chan struct{}, 4)}
}

// Register 校验资料并在进入数据库前计算密码哈希。
func (service *Service) Register(ctx context.Context, input RegistrationInput) error {
	if err := input.Validate(); err != nil {
		return err
	}
	select {
	case service.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-service.slots }()
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("计算注册密码摘要: %w", err)
	}
	return service.repository.Register(ctx, input, string(hash))
}

// Login 限制登录并发并校验设备及字段长度。
func (service *Service) Login(ctx context.Context, input LoginInput, device string) (LoginResult, error) {
	if input.Account == "" || len(input.Account) > 254 || len(input.Password) < 1 || len(input.Password) > 72 || device == "" || len(device) > 128 || input.DeviceName == "" || len(input.DeviceName) > 256 {
		return LoginResult{}, fmt.Errorf("登录参数无效")
	}
	select {
	case service.slots <- struct{}{}:
	case <-ctx.Done():
		return LoginResult{}, ctx.Err()
	}
	defer func() { <-service.slots }()
	return service.repository.Login(ctx, input, device)
}
