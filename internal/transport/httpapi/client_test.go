package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/report"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
)

// fixedChat 提供不依赖真实供应商的确定性问答结果。
type fixedChat struct{}

// Answer 返回包含换行的文本以验证旧 Android 流式解析。
func (fixedChat) Answer(ctx context.Context, request agent.Request) (string, error) {
	return "第一行\n第二行", nil
}

// testClient 创建真实 SQLite 和 HTTP 适配器的隔离测试环境。
func testClient(t *testing.T) (http.Handler, *sqlite.Store, string) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "db", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	router := NewClientRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), ClientDependencies{Notices: store, Identity: store, Settings: store, Chat: fixedChat{}, Events: fixedChat{}, Submissions: store, Keys: store, Reports: store})
	response := callClient(router, "POST", "/api/v2/client/auth/register", `{"username":"student","email":"student@example.com","password_hash":"password-hash"}`, "")
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	if _, err := store.Execute(context.Background(), admin.Request{Action: "review-registration", ID: "1", Values: map[string]string{"decision": "approved"}}); err != nil {
		t.Fatal(err)
	}
	response = callClient(router, "POST", "/api/v2/client/auth/login", `{"account":"student","password_hash":"password-hash","device_name":"phone"}`, "")
	var result envelope[identity.LoginResult]
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 {
		t.Fatalf("登录失败: %s %v", response.Body, err)
	}
	return router, store, result.Data.Token
}

// callClient 构造旧客户端实际使用的认证头和请求体。
func callClient(router http.Handler, method string, path string, body string, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("X-Device-ID", "device")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

// TestClientNoticeResponseMatchesLegacyShape 验证旧客户端资讯字段、空集合和分页校验。
func TestClientNoticeResponseMatchesLegacyShape(t *testing.T) {
	router, store, token := testClient(t)
	item, err := notice.New(notice.CreateInput{ID: "notice-1", Label: "教务", Title: "考试安排", PublishedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), DetailURL: "https://example.com/n1", IsPage: true, Content: "考试时间为周一"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	response := callClient(router, "GET", "/api/v1/client/notices", "", token)
	want := `{"msg":"获取成功","data":{"total_returned":1,"total_label":1,"notices":[{"id":"notice-1","label":"教务","title":"考试安排","date":"2026-04-01","detail_url":"https://example.com/n1","is_page":true,"content_text":"考试时间为周一","attachments":[]}]}}`
	if response.Code != 200 || strings.TrimSpace(response.Body.String()) != want {
		t.Fatalf("旧协议响应不符: %d %s", response.Code, response.Body)
	}
	for _, path := range []string{"/api/v1/client/notices?page=0", "/api/v1/client/notices?size=51"} {
		if response := callClient(router, "GET", path, "", token); response.Code != 422 {
			t.Fatalf("未拒绝无效分页: %s", path)
		}
	}
}

// TestClientRejectsAdminRoutesAndRevokedPermissions 验证公开管理接口移除及权限拒绝。
func TestClientRejectsAdminRoutesAndRevokedPermissions(t *testing.T) {
	router, store, token := testClient(t)
	for _, path := range []string{"/api/v1/admin/settings", "/api/v2/admin/users", "/docs"} {
		if response := callClient(router, "GET", path, "", token); response.Code != 404 {
			t.Fatalf("公开管理路由存在: %s", path)
		}
	}
	response := callClient(router, "POST", "/api/v1/client/chat", `{"user_query":"问题"}`, token)
	if response.Code != 403 {
		t.Fatalf("未授权聊天: %d", response.Code)
	}
	if _, err := store.Execute(context.Background(), admin.Request{Action: "permissions", ID: "1", Values: map[string]string{"active": "false", "browse": "true", "chat": "true", "submit": "true", "max_devices": "3"}}); err != nil {
		t.Fatal(err)
	}
	if response := callClient(router, "GET", "/api/v1/client/notices", "", token); response.Code != 401 {
		t.Fatalf("停用仍可访问: %d", response.Code)
	}
}

// TestLegacySSEPreservesRawTextFrames 验证旧 Android 按双换行 data 分帧时不会插入额外前缀。
func TestLegacySSEPreservesRawTextFrames(t *testing.T) {
	router, store, token := testClient(t)
	if _, err := store.Execute(context.Background(), admin.Request{Action: "permissions", ID: "1", Values: map[string]string{"active": "true", "browse": "true", "chat": "true", "submit": "true", "max_devices": "3"}}); err != nil {
		t.Fatal(err)
	}
	response := callClient(router, "POST", "/api/v1/client/chat", `{"user_query":"问题","stream":true}`, token)
	if response.Code != 200 || response.Body.String() != "data: 第一行\n第二行\n\ndata: [DONE]\n\n" {
		t.Fatalf("SSE 响应不符: %d %q", response.Code, response.Body.String())
	}
}

// TestDailyReportTodayReturnsLatestPublished 验证今日接口由服务器解析最近一份已完成日报。
func TestDailyReportTodayReturnsLatestPublished(t *testing.T) {
	router, store, token := testClient(t)
	if err := store.SaveDaily(context.Background(), "2000-01-01", report.Completed, "历史日报正文", 1); err != nil {
		t.Fatal(err)
	}
	response := callClient(router, "GET", "/api/v2/client/daily-reports/today", "", token)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "2000-01-01") || !strings.Contains(response.Body.String(), "历史日报正文") {
		t.Fatalf("今日接口响应错误: %d %s", response.Code, response.Body)
	}
	response = callClient(router, "GET", "/api/v2/client/daily-reports/2000-01-01", "", token)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "历史日报正文") {
		t.Fatalf("按日期接口响应错误: %d %s", response.Code, response.Body)
	}
	if response = callClient(router, "GET", "/api/v2/client/daily-reports/2999-01-01", "", token); response.Code != 404 {
		t.Fatalf("未发布日期应返回 404: %d", response.Code)
	}
}
