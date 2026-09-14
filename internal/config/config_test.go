package config

import (
	"path/filepath"
	"testing"
)

// TestCrawlerProgramsRequireExplicitBoundedExecutables 验证多个程序可配置且相对路径、重名和未知字段被拒绝。
func TestCrawlerProgramsRequireExplicitBoundedExecutables(t *testing.T) {
	t.Setenv("OPENJWC_DATA_DIR", t.TempDir())
	valid := `[{"name":"college-a","path":"/opt/openjwc/crawler-a","args":["--campus","main"]},{"name":"college-b","path":"/opt/openjwc/crawler-b"}]`
	t.Setenv("OPENJWC_CRAWLER_PROGRAMS", valid)
	cfg, err := Load()
	if err != nil || len(cfg.CrawlerPrograms) != 2 || cfg.CrawlerPrograms[0].Args[1] != "main" {
		t.Fatalf("合法程序未加载: %+v %v", cfg.CrawlerPrograms, err)
	}
	invalid := []string{
		`[{"name":"college","path":"relative"}]`,
		`[{"name":"jwc","path":"/opt/crawler"}]`,
		`[{"name":"same","path":"/opt/a"},{"name":"same","path":"/opt/b"}]`,
		`[{"name":"college","path":"/opt/a","environment":{"TOKEN":"secret"}}]`,
	}
	for _, value := range invalid {
		t.Setenv("OPENJWC_CRAWLER_PROGRAMS", value)
		if _, err = Load(); err == nil {
			t.Errorf("无效程序配置未拒绝: %s", value)
		}
	}
}

// TestDefaultConfigHasNoExternalCrawler 验证未配置环境变量时不启动任意外部程序。
func TestDefaultConfigHasNoExternalCrawler(t *testing.T) {
	t.Setenv("OPENJWC_DATA_DIR", filepath.Join(t.TempDir(), "state"))
	t.Setenv("OPENJWC_CRAWLER_PROGRAMS", "")
	cfg, err := Load()
	if err != nil || len(cfg.CrawlerPrograms) != 0 {
		t.Fatalf("默认外部程序不为空: %+v %v", cfg.CrawlerPrograms, err)
	}
}
