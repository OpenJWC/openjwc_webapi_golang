package httpapi_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/httpapi"
)

func TestHealthEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	httpapi.NewRouter(logger).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("健康检查状态码错误: %d", response.Code)
	}
	if response.Body.String() != `{"status":"ok"}` {
		t.Fatalf("健康检查响应错误: %s", response.Body.String())
	}
}
