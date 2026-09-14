package httpapi

import (
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
	"net/http"
	"strings"
)

// KeyService 定义旧 API Key 设备处理器依赖。
type KeyService = identity.KeyRepository

// keyCredentials 校验旧接口要求的凭据及设备请求头。
func keyCredentials(writer http.ResponseWriter, request *http.Request) (string, string, bool) {
	device := request.Header.Get("X-Device-ID")
	if device == "" || len(device) > 128 {
		invalidInput(writer, "X-Device-ID 无效")
		return "", "", false
	}
	parts := strings.Fields(request.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		writeJSON(writer, 401, map[string]string{"detail": "未提供认证信息"})
		return "", "", false
	}
	return parts[1], device, true
}

// keyDevices 保留旧 API Key 设备列表行为。
func (client *Client) keyDevices(writer http.ResponseWriter, request *http.Request) {
	token, device, ok := keyCredentials(writer, request)
	if !ok {
		return
	}
	data, err := client.dependencies.Keys.KeyDevices(request.Context(), token, device)
	if err != nil {
		client.fail(writer, err)
		return
	}
	writeJSON(writer, 200, envelope[identity.KeyDevices]{Message: "请求成功", Data: data})
}

// keyUnbind 保留旧 API Key 解绑响应。
func (client *Client) keyUnbind(writer http.ResponseWriter, request *http.Request) {
	token, device, ok := keyCredentials(writer, request)
	if !ok {
		return
	}
	if _, err := client.dependencies.Keys.KeyDevices(request.Context(), token, device); err != nil {
		client.fail(writer, err)
		return
	}
	found, err := client.dependencies.Keys.KeyUnbind(request.Context(), token, device)
	if err != nil {
		client.fail(writer, err)
		return
	}
	if !found {
		writeJSON(writer, 404, map[string]string{"detail": "绑定关系不存在或Key无效"})
		return
	}
	writeJSON(writer, 200, map[string]string{"detail": "解绑成功，名额已释放。"})
}
