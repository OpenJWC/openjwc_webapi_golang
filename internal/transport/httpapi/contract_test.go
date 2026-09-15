package httpapi

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestDocumentedClientRoutesAreRegistered 验证契约中的每个公开路径实际注册，且未重新引入管理 API。
func TestDocumentedClientRoutesAreRegistered(t *testing.T) {
	router, _, _ := testClient(t)
	content, err := os.ReadFile("../../../api/openapi/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`(?m)^  (/[^\n]*):\n    (get|post):`)
	matches := pattern.FindAllStringSubmatch(string(content), -1)
	if len(matches) != 19 {
		t.Fatalf("契约路径数量意外变化: %d", len(matches))
	}
	for _, match := range matches {
		path := strings.ReplaceAll(match[1], "{date}", "2026-09-01")
		if strings.Contains(path, "/admin/") {
			t.Fatalf("契约公开了管理接口: %s", path)
		}
		response := callClient(router, strings.ToUpper(match[2]), path, "", "")
		if response.Code == 404 || response.Code == 405 {
			t.Errorf("契约路由未注册: %s %s", match[2], path)
		}
	}
}
