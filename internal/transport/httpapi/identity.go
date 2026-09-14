package httpapi

import (
	"net/http"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
)

// register 接收用户申请，审核通过前不创建可登录账号。
func (client *Client) register(writer http.ResponseWriter, request *http.Request) {
	var input identity.RegistrationInput
	if !decodeBody(writer, request, &input) {
		return
	}
	if err := input.Validate(); err != nil {
		invalidInput(writer, err.Error())
		return
	}
	if err := client.identity.Register(request.Context(), input); err != nil {
		client.fail(writer, err)
		return
	}
	writeJSON(writer, 200, envelope[struct{}]{Message: "注册申请已提交，等待管理员审核"})
}

// login 将客户端传入的 password_hash 按旧版协议视为密码材料。
func (client *Client) login(writer http.ResponseWriter, request *http.Request) {
	var input identity.LoginInput
	if !decodeBody(writer, request, &input) {
		return
	}
	device := request.Header.Get("X-Device-ID")
	if device == "" || len(device) > 128 || input.Account == "" || len(input.Account) > 254 || input.Password == "" || len(input.Password) > 72 || input.DeviceName == "" || len(input.DeviceName) > 256 {
		invalidInput(writer, "登录字段或 X-Device-ID 无效")
		return
	}
	result, err := client.identity.Login(request.Context(), input, device)
	if err != nil {
		client.fail(writer, err)
		return
	}
	writeJSON(writer, 200, envelope[identity.LoginResult]{Message: "登录成功", Data: result})
}

// devices 返回当前身份拥有的设备列表。
func (client *Client) devices(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	devices, err := client.dependencies.Identity.Devices(request.Context(), principal.ID())
	if err != nil {
		client.fail(writer, err)
		return
	}
	writeJSON(writer, 200, envelope[map[string][]identity.Device]{Message: "请求成功", Data: map[string][]identity.Device{"devices": devices}})
}

// unbind 允许用户撤销自己任意设备的会话。
func (client *Client) unbind(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	var input unbindInput
	if !decodeBody(writer, request, &input) {
		return
	}
	if input.UUID == "" || len(input.UUID) > 128 {
		invalidInput(writer, "device_uuid 无效")
		return
	}
	found, err := client.dependencies.Identity.Unbind(request.Context(), principal.ID(), input.UUID)
	if err != nil {
		client.fail(writer, err)
		return
	}
	if !found {
		writeJSON(writer, 404, map[string]string{"detail": "设备不存在或未绑定"})
		return
	}
	writeJSON(writer, 200, map[string]string{"detail": "解绑成功"})
}

// registerDevice 保留旧版注册设备成功响应，实际绑定发生在登录事务中。
func (client *Client) registerDevice(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	writeJSON(writer, 200, envelope[struct{}]{Message: "设备注册成功"})
}
