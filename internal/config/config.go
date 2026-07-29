package config

import (
	"fmt"
	"os"
)

const defaultServerAddress = ":8080"

// Config 表示应用启动所需的不可变配置。
type Config struct {
	Server ServerConfig
}

// ServerConfig 表示 HTTP 服务的网络配置。
type ServerConfig struct {
	Address string
}

// Load 从环境变量加载配置，并为本地开发提供安全默认值。
func Load() (Config, error) {
	address := os.Getenv("OPENJWC_HTTP_ADDRESS")
	if address == "" {
		address = defaultServerAddress
	}
	if len(address) > 255 {
		return Config{}, fmt.Errorf("OPENJWC_HTTP_ADDRESS 长度不能超过 255")
	}

	return Config{Server: ServerConfig{Address: address}}, nil
}
