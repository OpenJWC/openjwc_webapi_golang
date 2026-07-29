package httpapi

import (
	"log/slog"
	"net/http"
)

// NewRouter 创建应用的 HTTP 路由器。
func NewRouter(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth(logger))
	return mux
}

func handleHealth(logger *slog.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		logger.Debug("健康检查请求", "method", request.Method, "path", request.URL.Path)
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	}
}
