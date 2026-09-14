package httpapi

import (
	"errors"
	"net/http"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
)

// ChatService 定义可取消的单次 Agent 问答能力。
type ChatService = agent.Chat

// chat 保留文本和旧版原始文本 SSE 帧，工具过程不暴露到客户端。
func (client *Client) chat(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	var input agent.Request
	if !decodeBody(writer, request, &input) {
		return
	}
	if err := input.Validate(); err != nil {
		invalidInput(writer, err.Error())
		return
	}
	client.chatMutex.Lock()
	if client.chatUsers[principal.ID()] {
		client.chatMutex.Unlock()
		client.chatError(writer, agent.ErrBusy)
		return
	}
	client.chatUsers[principal.ID()] = true
	client.chatMutex.Unlock()
	defer func() {
		client.chatMutex.Lock()
		delete(client.chatUsers, principal.ID())
		client.chatMutex.Unlock()
	}()
	if input.Stream {
		client.chatStream(writer, request, input)
		return
	}
	answer, err := client.dependencies.Chat.Answer(request.Context(), input)
	if err != nil {
		client.chatError(writer, err)
		return
	}
	writeJSON(writer, 200, map[string]string{"status": "success", "reply": answer})
}

// chatError 映射模型容量及可用性错误，不泄露供应商响应。
func (client *Client) chatError(writer http.ResponseWriter, err error) {
	if errors.Is(err, agent.ErrBusy) {
		writer.Header().Set("Retry-After", "3")
		writeJSON(writer, 429, map[string]string{"detail": "智能问答繁忙，请稍后重试"})
		return
	}
	if errors.Is(err, agent.ErrUnavailable) {
		writeJSON(writer, 503, map[string]string{"detail": "AI 服务暂时不可用"})
		return
	}
	client.fail(writer, err)
}
