package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
)

// Config 表示进程启动时加载的不可变配置。
type Config struct {
	Server          ServerConfig
	DataDir         string
	DatabasePath    string
	SocketPath      string
	CrawlerPrograms []CrawlerProgram
}

// CrawlerProgram 是显式配置的一次性外部爬虫进程，不包含运行时凭据。
type CrawlerProgram struct {
	Name string   `json:"name"`
	Path string   `json:"path"`
	Args []string `json:"args,omitempty"`
}

// ServerConfig 表示公开客户端 HTTP 服务配置。
type ServerConfig struct {
	Address string
	TLSCert string
	TLSKey  string
}

// Load 使用环境变量或用户私有状态目录，不读取仓库中的生产数据。
func Load() (Config, error) {
	address := os.Getenv("OPENJWC_HTTP_ADDRESS")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return Config{}, fmt.Errorf("OPENJWC_HTTP_ADDRESS 必须为 host:port: %w", err)
	}
	dir := os.Getenv("OPENJWC_DATA_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("解析用户目录: %w", err)
		}
		dir = filepath.Join(home, ".local", "state", "openjwc")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Config{}, fmt.Errorf("解析数据目录: %w", err)
	}
	socket := filepath.Join(dir, "admin.sock")
	if len(socket) > 100 {
		return Config{}, fmt.Errorf("OPENJWC_DATA_DIR 过长，Unix socket 路径必须不超过 100 字节")
	}
	cert, key := os.Getenv("OPENJWC_TLS_CERT"), os.Getenv("OPENJWC_TLS_KEY")
	if (cert == "") != (key == "") {
		return Config{}, fmt.Errorf("OPENJWC_TLS_CERT 和 OPENJWC_TLS_KEY 必须同时配置")
	}
	programs, err := parseCrawlerPrograms(os.Getenv("OPENJWC_CRAWLER_PROGRAMS"))
	if err != nil {
		return Config{}, err
	}
	return Config{Server: ServerConfig{Address: address, TLSCert: cert, TLSKey: key}, DataDir: dir, DatabasePath: filepath.Join(dir, "openjwc.db"), SocketPath: socket, CrawlerPrograms: programs}, nil
}

// parseCrawlerPrograms 解析有界 JSON 数组并拒绝 shell 命令、相对路径和来源重名。
func parseCrawlerPrograms(value string) ([]CrawlerProgram, error) {
	if value == "" {
		return nil, nil
	}
	if len(value) > 32768 {
		return nil, fmt.Errorf("OPENJWC_CRAWLER_PROGRAMS 超过 32 KiB")
	}
	var programs []CrawlerProgram
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&programs); err != nil || len(programs) == 0 || len(programs) > 16 {
		return nil, fmt.Errorf("OPENJWC_CRAWLER_PROGRAMS 必须是包含 1～16 个程序的 JSON 数组")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("OPENJWC_CRAWLER_PROGRAMS 只能包含一个 JSON 数组")
	}
	seen := make(map[string]bool)
	for index := range programs {
		program := &programs[index]
		if !crawl.ValidSourceName(program.Name) || crawl.BuiltinSourceName(program.Name) || seen[program.Name] {
			return nil, fmt.Errorf("外部爬虫名称无效、重复或占用内置名称: %s", program.Name)
		}
		seen[program.Name] = true
		if !filepath.IsAbs(program.Path) || filepath.Clean(program.Path) != program.Path || len(program.Path) > 4096 || strings.ContainsAny(program.Path, "\x00\n\r") {
			return nil, fmt.Errorf("外部爬虫 %s 必须使用规范绝对路径", program.Name)
		}
		if len(program.Args) > 32 {
			return nil, fmt.Errorf("外部爬虫 %s 参数过多", program.Name)
		}
		for _, argument := range program.Args {
			if len(argument) > 4096 || strings.ContainsAny(argument, "\x00\n\r") {
				return nil, fmt.Errorf("外部爬虫 %s 参数无效", program.Name)
			}
		}
	}
	return programs, nil
}
