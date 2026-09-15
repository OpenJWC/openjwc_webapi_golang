package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// complete 执行工具可用的有界模型请求，不发布中间模型正文。
func (loop *Loop) complete(ctx context.Context, values map[string]string, messages []modelMessage, budget setting.AgentBudget) (modelMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, budget.ModelTimeout)
	defer cancel()
	response, err := loop.requestModel(ctx, values, messages, nil)
	if err != nil {
		return modelMessage{}, err
	}
	defer response.Body.Close()
	if strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		message, streamErr := readModelStream(io.LimitReader(response.Body, (2<<20)+1))
		if ctx.Err() != nil {
			return modelMessage{}, ctx.Err()
		}
		return message, streamErr
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(content) > 2<<20 {
		return modelMessage{}, ErrUnavailable
	}
	var result modelResponse
	if err = json.Unmarshal(content, &result); err != nil || len(result.Choices) != 1 {
		return modelMessage{}, fmt.Errorf("%w: 非流式模型响应无效", errModelProtocol)
	}
	return result.Choices[0].Message, nil
}

// finalize 执行一次明确禁用工具的最终回答请求，只有 SSE 内容才即时发布。
func (loop *Loop) finalize(ctx context.Context, values map[string]string, messages []modelMessage, budget setting.AgentBudget, emit func(string) error) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, budget.ModelTimeout)
	defer cancel()
	choice := "none"
	response, err := loop.requestModel(ctx, values, messages, &choice)
	if err != nil {
		return "", false, err
	}
	defer response.Body.Close()
	if strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		if err = readFinalStream(io.LimitReader(response.Body, (2<<20)+1), emit); err != nil {
			return "", false, err
		}
		return "", true, nil
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(content) > 2<<20 {
		return "", false, ErrUnavailable
	}
	var result modelResponse
	if err = json.Unmarshal(content, &result); err != nil || len(result.Choices) != 1 || len(result.Choices[0].Message.Calls) != 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", false, fmt.Errorf("%w: 最终模型响应无效", errModelProtocol)
	}
	return result.Choices[0].Message.Content, false, nil
}

// requestModel 创建带单次超时的 OpenAI 兼容请求，拒绝响应内容和凭据进入错误。
func (loop *Loop) requestModel(ctx context.Context, values map[string]string, messages []modelMessage, choice *string) (*http.Response, error) {
	payload, err := json.Marshal(modelRequest{Model: values["llm_model"], Messages: messages, Tools: json.RawMessage(toolSchema), ToolChoice: choice, MaxTokens: 4096, Stream: true})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(values["llm_base_url"], "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+values["llm_api_key"])
	request.Header.Set("Content-Type", "application/json")
	response, err := loop.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrUnavailable
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, ErrUnavailable
	}
	return response, nil
}

// toolSchema 固定模型可见工具的 JSON Schema。
const toolSchema = `[{"type":"function","function":{"name":"bash","description":"读取资讯虚拟文件系统；支持 ls、find、grep、cat、head 与右侧为 head 的单管道。不是宿主机 Bash。","parameters":{"type":"object","properties":{"command":{"type":"string","maxLength":2048}},"required":["command"],"additionalProperties":false}}}]`
