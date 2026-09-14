package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
)

// deployment 保存部署配置与 unit 摘要，卸载时拒绝误删其他文件。
type deployment struct {
	Config   config.Config
	Binary   string
	UnitHash string
}

// profile 读取私有部署记录，拒绝无界输入与损坏记录。
func (manager *Manager) profile() (deployment, error) {
	var value deployment
	file, err := os.Open(manager.ProfilePath)
	if err != nil {
		return value, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 65537))
	if err != nil {
		return value, err
	}
	if len(data) > 65536 {
		return value, fmt.Errorf("部署记录过大")
	}
	err = json.Unmarshal(data, &value)
	if err == nil && (value.UnitHash == "" || !filepath.IsAbs(value.Binary) || !filepath.IsAbs(value.Config.DataDir)) {
		err = fmt.Errorf("部署记录无效")
	}
	return value, err
}

// Install 原子写入自有部署记录和 unit，并启用服务。
func (manager *Manager) Install(ctx context.Context, binary string) error {
	if err := manager.Preflight(ctx); err != nil {
		return err
	}
	lock, err := manager.lockDeployment()
	if err != nil {
		return err
	}
	defer lock.Close()
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	for _, path := range []*string{&manager.Config.Server.TLSCert, &manager.Config.Server.TLSKey} {
		if *path != "" {
			*path, err = filepath.Abs(*path)
			if err != nil {
				return fmt.Errorf("规范化 TLS 路径: %w", err)
			}
		}
	}
	unit, err := manager.render(binary)
	if err != nil {
		return err
	}
	if err := manager.checkFragment(ctx); err != nil {
		return err
	}
	existing, err := os.ReadFile(manager.UnitPath)
	if err == nil {
		saved, readErr := manager.profile()
		if readErr != nil || digest(existing) != saved.UnitHash {
			return fmt.Errorf("unit 已存在但不属于本程序或被人工修改，拒绝覆盖")
		}
		if string(existing) != unit {
			return fmt.Errorf("部署内容发生变化，请显式卸载旧部署后安装；数据库不会删除")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	value := deployment{Config: manager.Config, Binary: binary, UnitHash: digest([]byte(unit))}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicFile(manager.ProfilePath, data, 0600); err != nil {
		return err
	}
	if err = atomicFile(manager.UnitPath, []byte(unit), 0644); err != nil {
		return err
	}
	if _, err = manager.systemctl(ctx, "daemon-reload"); err != nil {
		return err
	}
	_, err = manager.systemctl(ctx, "enable", "openjwc.service")
	return err
}

// Uninstall 只删除校验通过的自有 unit，保留配置、账号、数据库和备份。
func (manager *Manager) Uninstall(ctx context.Context) error {
	if manager.System && os.Geteuid() != 0 {
		return fmt.Errorf("卸载系统服务需要显式 root 授权")
	}
	lock, err := manager.lockDeployment()
	if err != nil {
		return err
	}
	defer lock.Close()
	data, err := os.ReadFile(manager.UnitPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	saved, err := manager.profile()
	if err != nil || digest(data) != saved.UnitHash {
		return fmt.Errorf("unit 不属于本程序或已被修改，拒绝卸载")
	}
	if err = manager.checkFragment(ctx); err != nil {
		return err
	}
	if _, err = manager.systemctl(ctx, "disable", "--now", "openjwc.service"); err != nil {
		return err
	}
	if err = os.Remove(manager.UnitPath); err != nil {
		return err
	}
	_, err = manager.systemctl(ctx, "daemon-reload")
	return err
}

// render 生成正确引用路径和环境值的 unit，不依赖登录会话环境。
func (manager *Manager) render(binary string) (string, error) {
	programs := ""
	if len(manager.Config.CrawlerPrograms) > 0 {
		encoded, err := json.Marshal(manager.Config.CrawlerPrograms)
		if err != nil {
			return "", err
		}
		programs = string(encoded)
	}
	values := []string{binary, manager.Config.DataDir, manager.Config.Server.Address, manager.Config.Server.TLSCert, manager.Config.Server.TLSKey, programs}
	for _, value := range values {
		if strings.ContainsAny(value, "\x00\n\r") {
			return "", fmt.Errorf("部署路径或配置含禁止字符")
		}
	}
	identity := ""
	target := "default.target"
	if manager.System {
		identity = "User=openjwc\nGroup=openjwc\nStateDirectory=openjwc\nStateDirectoryMode=0700\nPrivateTmp=true\nProtectSystem=strict\nProtectHome=true\nReadWritePaths=" + unitQuote(manager.Config.DataDir) + "\n"
		target = "multi-user.target"
	}
	environment := map[string]string{"OPENJWC_DATA_DIR": manager.Config.DataDir, "OPENJWC_HTTP_ADDRESS": manager.Config.Server.Address, "OPENJWC_TLS_CERT": manager.Config.Server.TLSCert, "OPENJWC_TLS_KEY": manager.Config.Server.TLSKey, "OPENJWC_CRAWLER_PROGRAMS": programs}
	var settings strings.Builder
	for _, key := range []string{"OPENJWC_DATA_DIR", "OPENJWC_HTTP_ADDRESS", "OPENJWC_TLS_CERT", "OPENJWC_TLS_KEY", "OPENJWC_CRAWLER_PROGRAMS"} {
		fmt.Fprintf(&settings, "Environment=%s\n", unitQuote(key+"="+environment[key]))
	}
	return "[Unit]\nDescription=OpenJWC managed service\nAfter=network-online.target\n\n[Service]\nType=simple\n" + identity + "ExecStart=:" + unitQuote(binary) + " serve\n" + settings.String() + "UMask=0077\nRestart=on-failure\nRestartSec=5\nTimeoutStopSec=25\nNoNewPrivileges=true\n\n[Install]\nWantedBy=" + target + "\n", nil
}

// unitQuote 转义 systemd 字符串与说明符，防止环境插值改变路径。
func unitQuote(value string) string { return strconv.Quote(strings.ReplaceAll(value, "%", "%%")) }

// digest 计算文件所有权校验所需的内容摘要。
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// trim 去除 systemd 单值输出两侧空白。
func trim(value string) string { return strings.TrimSpace(value) }

// atomicFile 在目标目录内完成原子替换并同步目录项。
func atomicFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".openjwc-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err = file.Chmod(mode); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
