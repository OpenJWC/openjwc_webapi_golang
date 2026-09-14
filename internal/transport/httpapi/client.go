package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// NoticeReader 定义客户端资讯读取所需的存储端口。
type NoticeReader interface {
	List(ctx context.Context, filter notice.Filter) ([]notice.Notice, int, error)
	Labels(context.Context) ([]string, error)
	Search(context.Context, string, int) ([]notice.Notice, error)
}

// ClientDependencies 聚合客户端 HTTP 适配器的显式依赖。
type ClientDependencies struct {
	Notices     NoticeReader
	Identity    identity.Repository
	Settings    setting.Reader
	Events      agent.EventChat
	Chat        ChatService
	Submissions SubmissionService
	Keys        KeyService
	Reports     ReportReader
}

// Client 保存共享依赖，chatMutex 保护每用户问答在途状态。
type Client struct {
	dependencies ClientDependencies
	identity     *identity.Service
	logger       *slog.Logger
	chatMutex    sync.Mutex
	chatUsers    map[int64]bool
}

// NewClientRouter 注册客户端兼容接口，不注册任何公开管理接口。
func NewClientRouter(logger *slog.Logger, dependencies ClientDependencies) http.Handler {
	client := &Client{dependencies: dependencies, identity: identity.New(dependencies.Identity), logger: logger, chatUsers: make(map[int64]bool)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth(logger))
	mux.HandleFunc("GET /{$}", func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, 200, map[string]string{"message": "Server is running!"})
	})
	mux.HandleFunc("POST /api/v2/client/auth/register", client.register)
	mux.HandleFunc("POST /api/v2/client/auth/login", client.login)
	mux.HandleFunc("GET /api/v2/client/device", client.withIdentity("", client.devices))
	mux.HandleFunc("POST /api/v2/client/device/unbind", client.withIdentity("", client.unbind))
	mux.HandleFunc("GET /api/v1/client/notices", client.withIdentity(access.Browse, client.notices))
	mux.HandleFunc("GET /api/v1/client/notices/labels", client.withIdentity(access.Browse, client.labels))
	mux.HandleFunc("POST /api/v1/client/notices/search", client.withIdentity(access.Browse, client.search))
	mux.HandleFunc("POST /api/v1/client/register", client.withIdentity("", client.registerDevice))
	mux.HandleFunc("GET /api/v1/client/motto", client.withIdentity(access.Browse, client.motto))
	mux.HandleFunc("POST /api/v2/client/chat", client.withIdentity(access.Chat, client.chatEvents))
	mux.HandleFunc("POST /api/v1/client/chat", client.withIdentity(access.Chat, client.chat))
	mux.HandleFunc("POST /api/v1/client/submissions", client.withIdentity(access.Submit, client.submit))
	mux.HandleFunc("GET /api/v1/client/submissions/my", client.withIdentity("", client.mySubmissions))
	mux.HandleFunc("GET /api/v2/client/daily-reports/{date}", client.withIdentity(access.Browse, client.dailyReport))
	mux.HandleFunc("GET /api/v1/client/device", client.keyDevices)
	mux.HandleFunc("POST /api/v1/client/device/unbind", client.keyUnbind)
	return bounded(limitLogins(mux), 128)
}

// withIdentity 在每次请求时验证身份与当前权限，不信任客户端声明。
func (client *Client) withIdentity(permission access.Permission, next func(http.ResponseWriter, *http.Request, access.Principal)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		parts := strings.Fields(request.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			client.fail(writer, access.ErrUnauthorized)
			return
		}
		principal, err := client.dependencies.Identity.Authenticate(request.Context(), parts[1], request.Header.Get("X-Device-ID"))
		if err != nil {
			client.fail(writer, err)
			return
		}
		if permission != "" && !principal.Allows(permission) {
			client.fail(writer, access.ErrForbidden)
			return
		}
		next(writer, request, principal)
	}
}

// bounded 限制在途请求和处理时长，容量耗尽时拒绝排队增长。
func bounded(next http.Handler, capacity int) http.Handler {
	slots := make(chan struct{}, capacity)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case slots <- struct{}{}:
		default:
			writer.Header().Set("Retry-After", "1")
			writeJSON(writer, 503, map[string]string{"detail": "服务繁忙，请稍后重试"})
			return
		}
		defer func() { <-slots }()
		duration := 15 * time.Second
		if request.URL.Path == "/api/v1/client/chat" || request.URL.Path == "/api/v2/client/chat" {
			duration = 2 * time.Minute
		}
		ctx, cancel := context.WithTimeout(request.Context(), duration)
		defer cancel()
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}
