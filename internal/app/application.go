package app

import (
	"log/slog"
	"net/http"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/httpapi"
)

// Application 保存应用生命周期内共享的依赖。
type Application struct {
	router http.Handler
}

// New 组装应用依赖并创建实例。
func New(logger *slog.Logger) *Application {
	return &Application{router: httpapi.NewRouter(logger)}
}

// Router 返回应用的 HTTP 路由器。
func (application *Application) Router() http.Handler {
	return application.router
}
