package cli

import (
	"fmt"
	"os"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/daemon"
)

// configureInstall 只在显式安装时应用给定的环境配置，不改变运行中服务的配置来源。
func configureInstall(manager *daemon.Manager, cfg config.Config) error {
	if _, set := os.LookupEnv("OPENJWC_DATA_DIR"); set {
		if manager.System && cfg.DataDir != "/var/lib/openjwc" {
			return fmt.Errorf("系统级部署固定使用 /var/lib/openjwc，不接受其他 OPENJWC_DATA_DIR")
		}
		manager.Config.DataDir = cfg.DataDir
		manager.Config.DatabasePath = cfg.DatabasePath
		manager.Config.SocketPath = cfg.SocketPath
	}
	if _, set := os.LookupEnv("OPENJWC_HTTP_ADDRESS"); set {
		manager.Config.Server.Address = cfg.Server.Address
	}
	_, certSet := os.LookupEnv("OPENJWC_TLS_CERT")
	_, keySet := os.LookupEnv("OPENJWC_TLS_KEY")
	if certSet || keySet {
		manager.Config.Server.TLSCert = cfg.Server.TLSCert
		manager.Config.Server.TLSKey = cfg.Server.TLSKey
	}
	if _, set := os.LookupEnv("OPENJWC_CRAWLER_PROGRAMS"); set {
		manager.Config.CrawlerPrograms = cfg.CrawlerPrograms
	}
	return nil
}
