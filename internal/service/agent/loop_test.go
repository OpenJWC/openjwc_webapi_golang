package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/infrastructure/sqlite"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// testSettings 是每次返回独立副本的模型设置夹具。
type testSettings struct{ url string }

// Settings 返回本地模拟模型服务地址与虚构密钥。
func (settings testSettings) Settings(ctx context.Context) (map[string]string, error) {
	values := setting.Defaults()
	values["llm_base_url"] = settings.url
	values["llm_api_key"] = "test-only"
	return values, nil
}

// agentLibrary 创建包含虚构资讯的真实数据库。
func agentLibrary(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "db", "agent.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	item, err := notice.New(notice.CreateInput{ID: "notice", Title: "考试安排", PublishedAt: time.Now(), DetailURL: "https://example.com/notice", Content: "考试在星期一举行"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	return store
}

// TestVFSRejectsShellEscapeAndHostPaths 验证虚拟只读工具不能穿越或启动宿主命令。
func TestVFSRejectsShellEscapeAndHostPaths(t *testing.T) {
	vfs := NewVFS(agentLibrary(t))
	for _, command := range []string{"cat /etc/passwd", "cat /notices/../etc/passwd", "ls /notices; whoami", "grep $(env) /notices", "cat /notices/a.md | sh", "rm -rf /notices", "grep 'unterminated /notices"} {
		if _, err := vfs.Execute(context.Background(), command); err == nil {
			t.Errorf("接受不安全命令: %s", command)
		}
	}
	text, err := vfs.Execute(context.Background(), "cat "+FilePath("notice"))
	if err != nil || !strings.Contains(text, "星期一") {
		t.Fatalf("资讯读取失败: %s %v", text, err)
	}
}

// TestAgentUsesToolObservationsAndIsolatesConcurrentHistories 验证真正的工具回环及不同请求之间不共享消息。
func TestAgentUsesToolObservationsAndIsolatesConcurrentHistories(t *testing.T) {
	store := agentLibrary(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input modelRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if request.Header.Get("Authorization") != "Bearer test-only" {
			t.Error("模型凭据缺失")
		}
		query := ""
		for _, message := range input.Messages {
			if message.Role == "user" {
				if query != "" {
					t.Error("不同会话混入同一历史")
				}
				query = message.Content
			}
		}
		last := input.Messages[len(input.Messages)-1]
		if last.Role != "tool" {
			call := toolCall{ID: "read", Type: "function", Function: toolFunction{Name: "bash", Arguments: `{"command":"cat ` + FilePath("notice") + `"}`}}
			message := modelMessage{Role: "assistant", Calls: []toolCall{call}}
			_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: message}}})
			return
		}
		if !strings.Contains(last.Content, "星期一") {
			t.Error("模型未收到工具观察")
		}
		_ = json.NewEncoder(writer).Encode(modelResponse{Choices: []modelChoice{{Message: modelMessage{Role: "assistant", Content: query + "：星期一"}}}})
	}))
	defer server.Close()
	loop := New(testSettings{url: server.URL}, store)
	var workers sync.WaitGroup
	for _, query := range []string{"用户甲", "用户乙", "用户丙", "用户丁"} {
		workers.Go(func() {
			answer, err := loop.Answer(context.Background(), Request{Query: query})
			if err != nil || answer != query+"：星期一" {
				t.Errorf("会话隔离失败: %q %v", answer, err)
			}
		})
	}
	workers.Wait()
}

// TestModelRedirectDoesNotLeakCredentials 验证供应商重定向不会将凭据转交其他地址。
func TestModelRedirectDoesNotLeakCredentials(t *testing.T) {
	store := agentLibrary(t)
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Error("凭据被发送到重定向目标")
		io.WriteString(writer, "{}")
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, 302)
	}))
	defer server.Close()
	if _, err := New(testSettings{url: server.URL}, store).Answer(context.Background(), Request{Query: "考试"}); err == nil {
		t.Fatal("重定向未被拒绝")
	}
}
