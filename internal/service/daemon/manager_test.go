package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/config"
)

// fakeRunner 记录服务命令且不改变宿主部署。
type fakeRunner struct {
	calls  []string
	linger string
	state  string
	fail   bool
}

// Run 返回可控的管理器状态。
func (runner *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	runner.calls = append(runner.calls, name+" "+strings.Join(args, " "))
	if runner.fail {
		return "", os.ErrPermission
	}
	if name == "loginctl" {
		return runner.linger, nil
	}
	if strings.Contains(strings.Join(args, " "), "--property=FragmentPath") {
		return "LoadState=not-found\n", nil
	}
	if strings.Contains(strings.Join(args, " "), "--property=LoadState") {
		return runner.state, nil
	}
	return "", nil
}

// testManager 创建仅使用临时目录和假命令的部署管理器。
func testManager(t *testing.T) (*Manager, *fakeRunner) {
	t.Helper()
	dir := t.TempDir()
	runner := &fakeRunner{linger: "yes", state: "LoadState=loaded\nActiveState=inactive\nMainPID=0\n"}
	return &Manager{UnitPath: filepath.Join(dir, "units", "openjwc.service"), ProfilePath: filepath.Join(dir, "config", "service.json"), Runner: runner, Config: config.Config{DataDir: filepath.Join(dir, "data"), SocketPath: filepath.Join(dir, "data", "admin.sock"), Server: config.ServerConfig{Address: "127.0.0.1:8080"}}}, runner
}

// TestInstallUninstallPreservesDataAndRejectsModifiedUnit 验证重复部署、数据保留和拒绝覆盖冲突文件。
func TestInstallUninstallPreservesDataAndRejectsModifiedUnit(t *testing.T) {
	manager, _ := testManager(t)
	ctx := context.Background()
	for index := 0; index < 2; index++ {
		if err := manager.Install(ctx, "/opt/open jwc%/openjwc"); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(manager.UnitPath)
	if err != nil || !strings.Contains(string(data), `ExecStart=:"/opt/open jwc%%/openjwc" serve`) {
		t.Fatalf("unit 转义错误: %s %v", data, err)
	}
	if err = os.MkdirAll(manager.Config.DataDir, 0700); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(manager.Config.DataDir, "keep.db")
	if err = os.WriteFile(database, []byte("保留"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(manager.UnitPath, []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = manager.Uninstall(ctx); err == nil {
		t.Fatal("不应删除人工修改的 unit")
	}
	if err = os.WriteFile(manager.UnitPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		if err = manager.Uninstall(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = os.Stat(database); err != nil {
		t.Fatal("卸载删除了数据库")
	}
	if _, err = os.Stat(manager.ProfilePath); err != nil {
		t.Fatal("卸载删除了配置")
	}
}

// TestPreflightAndStatusFailuresAreExplicit 验证 linger、权限和服务缺失不会被报告为已就绪。
func TestPreflightAndStatusFailuresAreExplicit(t *testing.T) {
	manager, runner := testManager(t)
	runner.linger = "no"
	if err := manager.Install(context.Background(), "/opt/openjwc"); err == nil {
		t.Fatal("缺少 linger 不应宣称常驻安装成功")
	}
	runner.fail = true
	if _, err := manager.Status(context.Background()); err == nil {
		t.Fatal("管理器失败不应被吞掉")
	}
	runner.fail = false
	runner.state = "LoadState=not-found\n"
	state, err := manager.Status(context.Background())
	if err != nil || state.State != "not-installed" || state.Ready {
		t.Fatalf("未安装状态错误: %+v %v", state, err)
	}
}

// TestInstallPersistsAbsoluteTLSPaths 验证相对证书路径安装后不会随 systemd 工作目录改变含义。
func TestInstallPersistsAbsoluteTLSPaths(t *testing.T) {
	manager, _ := testManager(t)
	base := t.TempDir()
	t.Chdir(base)
	manager.Config.Server.TLSCert = "certs/server.crt"
	manager.Config.Server.TLSKey = "certs/server.key"
	if err := manager.Install(context.Background(), "/opt/openjwc"); err != nil {
		t.Fatal(err)
	}
	saved, err := manager.profile()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Config.Server.TLSCert != filepath.Join(base, "certs/server.crt") || saved.Config.Server.TLSKey != filepath.Join(base, "certs/server.key") {
		t.Fatalf("证书路径未规范化: %+v", saved.Config.Server)
	}
	unit, err := os.ReadFile(manager.UnitPath)
	if err != nil || !strings.Contains(string(unit), "OPENJWC_TLS_CERT="+saved.Config.Server.TLSCert) {
		t.Fatalf("unit 证书路径不正确: %v", err)
	}
}

// TestInstallPersistsExternalCrawlerPrograms 验证外部程序配置按 JSON 写入 unit 和私有部署记录。
func TestInstallPersistsExternalCrawlerPrograms(t *testing.T) {
	manager, _ := testManager(t)
	manager.Config.CrawlerPrograms = []config.CrawlerProgram{{Name: "college", Path: "/opt/openjwc/crawler", Args: []string{"--site", "main"}}}
	if err := manager.Install(context.Background(), "/opt/openjwc"); err != nil {
		t.Fatal(err)
	}
	saved, err := manager.profile()
	if err != nil || len(saved.Config.CrawlerPrograms) != 1 || saved.Config.CrawlerPrograms[0].Args[1] != "main" {
		t.Fatalf("部署记录未保存爬虫: %+v %v", saved.Config.CrawlerPrograms, err)
	}
	unit, err := os.ReadFile(manager.UnitPath)
	if err != nil || !strings.Contains(string(unit), `OPENJWC_CRAWLER_PROGRAMS=[{\"name\":\"college\"`) {
		t.Fatalf("unit 未保存爬虫 JSON: %s %v", unit, err)
	}
}

// TestLifecycleRejectsMissingOrFailedService 验证未安装及启动失败不会被报告为启动成功。
func TestLifecycleRejectsMissingOrFailedService(t *testing.T) {
	manager, runner := testManager(t)
	ctx := context.Background()
	if err := manager.Change(ctx, "start"); err == nil {
		t.Fatal("未安装服务也被启动")
	}
	if err := manager.Install(ctx, "/opt/openjwc"); err != nil {
		t.Fatal(err)
	}
	runner.state = "LoadState=loaded\nActiveState=failed\nMainPID=0\n"
	if err := manager.Change(ctx, "start"); err == nil {
		t.Fatal("服务启动失败被报告为成功")
	}
	runner.state = "LoadState=loaded\nActiveState=inactive\nMainPID=0\n"
	if err := manager.Change(ctx, "stop"); err != nil {
		t.Fatal(err)
	}
}

// TestDeploymentMutationIsExclusive 验证部署文件不能被两个同时进行的操作交错写入。
func TestDeploymentMutationIsExclusive(t *testing.T) {
	manager, _ := testManager(t)
	lock, err := manager.lockDeployment()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = manager.Install(context.Background(), "/opt/openjwc"); err == nil {
		t.Fatal("并发部署锁未生效")
	}
	if _, err = os.Stat(manager.UnitPath); !os.IsNotExist(err) {
		t.Fatal("被拒绝的部署仍写入了 unit")
	}
}
