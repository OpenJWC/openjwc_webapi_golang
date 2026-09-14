package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
)

// envelope 是保留旧客户端字段名的泛型响应包装。
type envelope[T any] struct {
	Message string `json:"msg"`
	Data    T      `json:"data"`
}

// writeJSON 仅编码显式传输结构，不接收领域实体作为返回值。
func writeJSON[T any](writer http.ResponseWriter, status int, payload T) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

// decodeBody 拒绝空体、截断及多份 JSON，同时兼容旧客户端附加字段。
func decodeBody[T any](writer http.ResponseWriter, request *http.Request, payload *T) bool {
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(payload); err != nil {
		invalidInput(writer, "请求 JSON 无效或超出大小限制")
		return false
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		invalidInput(writer, "请求必须恰好包含一个 JSON 对象")
		return false
	}
	return true
}

// invalidInput 返回兼容校验错误数组，不回显请求密码或其他敏感输入。
func invalidInput(writer http.ResponseWriter, message string) {
	writeJSON(writer, http.StatusUnprocessableEntity, validationData{Detail: []validationIssue{{Type: "value_error", Location: []string{"body"}, Message: message}}})
}

// fail 将可预期错误映射到稳定协议信息，不向客户端输出内部错误。
func (client *Client) fail(writer http.ResponseWriter, err error) {
	status, detail := 500, "服务内部错误"
	switch {
	case errors.Is(err, access.ErrUnauthorized):
		status, detail = 401, "无效的Token或账号密码错误"
		writer.Header().Set("WWW-Authenticate", "Bearer")
	case errors.Is(err, access.ErrForbidden):
		status, detail = 403, "当前用户没有该操作权限"
	case errors.Is(err, access.ErrConflict):
		status, detail = 409, "用户名、邮箱、设备配额或记录状态冲突"
	case errors.Is(err, context.DeadlineExceeded):
		status, detail = 504, "请求处理超时"
	case errors.Is(err, context.Canceled):
		return
	default:
		client.logger.Error("客户端请求失败", "error", err)
	}
	writeJSON(writer, status, map[string]string{"detail": detail})
}
