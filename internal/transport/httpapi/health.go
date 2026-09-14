package httpapi

import (
	"log/slog"
	"net/http"
)

// handleHealth 返回不依赖外部网络的进程存活状态。
func handleHealth(logger *slog.Logger) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		logger.Debug("健康检查请求", "method", request.Method, "path", request.URL.Path)
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	}
}
