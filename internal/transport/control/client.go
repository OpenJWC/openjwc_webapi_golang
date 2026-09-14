package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// Client 通过 Unix socket 调用后台进程，不直接打开数据库。
type Client struct{ http *http.Client }

// NewClient 创建绑定到单个本地套接字的管理客户端。
func NewClient(path string) *Client {
	transport := &http.Transport{DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", path)
	}}
	return &Client{http: &http.Client{Transport: transport, Timeout: 3 * time.Minute}}
}

// Execute 发送类型化命令并返回表格或明确错误。
func (client *Client) Execute(ctx context.Context, command admin.Request) (admin.Response, error) {
	if err := command.Validate(); err != nil {
		return admin.Response{}, err
	}
	payload, err := json.Marshal(command)
	if err != nil {
		return admin.Response{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost/command", bytes.NewReader(payload))
	if err != nil {
		return admin.Response{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return admin.Response{}, fmt.Errorf("无法连接本地服务，请先运行 openjwc start: %w", err)
	}
	defer response.Body.Close()
	var result admin.Response
	if err = json.NewDecoder(http.MaxBytesReader(nil, response.Body, 4<<20)).Decode(&result); err != nil {
		return admin.Response{}, fmt.Errorf("读取管理响应: %w", err)
	}
	if result.Error != "" {
		return admin.Response{}, fmt.Errorf("%s", result.Error)
	}
	if response.StatusCode != http.StatusOK {
		return admin.Response{}, fmt.Errorf("管理响应状态 %d", response.StatusCode)
	}
	return result, nil
}

// Close 释放本地管理连接，不影响后台服务。
func (client *Client) Close() { client.http.CloseIdleConnections() }

// healthResponse 是本地服务身份与就绪探测的最小响应。
type healthResponse struct {
	PID int `json:"pid"`
}

// Probe 在调用者限定的时间内确认服务进程身份。
func (client *Client) Probe(ctx context.Context) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/healthz", nil)
	if err != nil {
		return 0, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return 0, fmt.Errorf("就绪响应状态 %d", response.StatusCode)
	}
	var value healthResponse
	err = json.NewDecoder(io.LimitReader(response.Body, 1024)).Decode(&value)
	return value.PID, err
}
