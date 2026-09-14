package control

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// Listen 创建仅服务账号与 root 可访问的本地管理套接字。
func Listen(path string) (net.Listener, error) {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("管理套接字路径被其他文件占用")
		}
		if err = os.Remove(path); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

// Handler 只接受有界本地管理命令，不与公开 HTTP 路由共享监听器。
func Handler(executor admin.Executor) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, request *http.Request) {
		_ = json.NewEncoder(writer).Encode(healthResponse{PID: os.Getpid()})
	})
	mux.HandleFunc("POST /command", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		request.Body = http.MaxBytesReader(writer, request.Body, 128<<10)
		var command admin.Request
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&command); err != nil {
			writer.WriteHeader(400)
			_ = json.NewEncoder(writer).Encode(admin.Response{Error: "管理请求无效"})
			return
		}
		var extra json.RawMessage
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			writer.WriteHeader(400)
			_ = json.NewEncoder(writer).Encode(admin.Response{Error: "请求只能包含一个 JSON 对象"})
			return
		}
		result, err := executor.Execute(request.Context(), command)
		if err != nil {
			writer.WriteHeader(400)
			result = admin.Response{Error: err.Error()}
		}
		_ = json.NewEncoder(writer).Encode(result)
	})
	return http.TimeoutHandler(mux, 3*time.Minute, `{"error":"管理操作超时"}`)
}
