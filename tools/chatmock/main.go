package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

// modelMessage 是测试服务器判断工具结果是否已返回所需的最小消息。
type modelMessage struct {
	Role string `json:"role"`
}

// modelRequest 是测试服务器接受的 OpenAI 兼容请求子集。
type modelRequest struct {
	Messages []modelMessage `json:"messages"`
	Stream   bool           `json:"stream"`
}

// mockServer 为本地端到端测试固定执行一次 VFS 工具调用后返回答案。
type mockServer struct{}

// ServeHTTP 校验请求路径与流式开关并发送确定性 SSE。
func (mockServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/v1/chat/completions" {
		http.NotFound(writer, request)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	var input modelRequest
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil || !input.Stream {
		http.Error(writer, "invalid model request", http.StatusBadRequest)
		return
	}
	hasToolResult := false
	for _, message := range input.Messages {
		if message.Role == "tool" {
			hasToolResult = true
		}
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	if hasToolResult {
		writeFrame(writer, `{"choices":[{"index":0,"delta":{"content":"本地模拟模型已收到 VFS 工具结果，Agent Chat 链路工作正常。"},"finish_reason":null}]}`)
		writeFrame(writer, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
	} else {
		writeFrame(writer, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"local-call-1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"ls /recent\"}"}}]},"finish_reason":null}]}`)
		writeFrame(writer, `{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)
	}
	fmt.Fprint(writer, "data: [DONE]\n\n")
}

// writeFrame 写入单个符合模型端协议的 SSE 数据帧。
func writeFrame(writer http.ResponseWriter, payload string) {
	fmt.Fprintf(writer, "data: %s\n\n", payload)
}

// main 启动仅监听 loopback 的开发测试模型，生产二进制不包含该工具。
func main() {
	listen := flag.String("listen", "127.0.0.1:18080", "loopback listen address")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	address := net.ParseIP(host)
	if err != nil || address == nil || !address.IsLoopback() {
		log.Fatal("listen address must be a loopback IP and port")
	}
	server := &http.Server{Addr: *listen, Handler: mockServer{}, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second}
	log.Printf("OpenJWC chat mock listening on http://%s/v1", *listen)
	log.Fatal(server.ListenAndServe())
}
