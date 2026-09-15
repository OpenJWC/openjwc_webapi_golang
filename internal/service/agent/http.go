package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// complete 执行工具可用的有界模型请求，瞬时失败可重试一次。
func (loop *Loop) complete(ctx context.Context, values map[string]string, messages []modelMessage, budget setting.AgentBudget) (modelMessage, error) {
	message, err := loop.modelRound(ctx, values, messages, budget)
	if err == nil {
		return message, nil
	}
	if retryErr := retryModelOnce(ctx, budget, err, false, func() error {
		var attemptErr error
		message, attemptErr = loop.modelRound(ctx, values, messages, budget)
		return attemptErr
	}); retryErr != nil {
		return modelMessage{}, retryErr
	}
	return message, nil
}

// modelRound 完成一次工具轮模型请求，不发布中间正文。
func (loop *Loop) modelRound(parent context.Context, values map[string]string, messages []modelMessage, budget setting.AgentBudget) (modelMessage, error) {
	ctx, cancel := context.WithTimeout(parent, budget.ModelTimeout)
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
		return modelMessage{}, &modelError{Kind: "body_read", Retry: true, Base: ErrUnavailable}
	}
	var result modelResponse
	if err = json.Unmarshal(content, &result); err != nil || len(result.Choices) != 1 {
		return modelMessage{}, &modelError{Kind: "response_invalid", Base: errModelProtocol}
	}
	return result.Choices[0].Message, nil
}

// finalize 执行一次禁用工具的最终回答，尚未输出任何文字前允许一次重试。
func (loop *Loop) finalize(ctx context.Context, values map[string]string, messages []modelMessage, budget setting.AgentBudget, emit func(string) error) (string, bool, error) {
	emitted := false
	publish := func(text string) error {
		emitted = true
		return emit(text)
	}
	buffered, streamed, err := loop.finalRound(ctx, values, messages, budget, publish)
	if err == nil {
		return buffered, streamed, nil
	}
	if retryErr := retryModelOnce(ctx, budget, err, emitted, func() error {
		var attemptErr error
		buffered, streamed, attemptErr = loop.finalRound(ctx, values, messages, budget, publish)
		return attemptErr
	}); retryErr != nil {
		return "", false, retryErr
	}
	return buffered, streamed, nil
}

// finalRound 完成一次收束请求，只有 SSE 内容才即时发布。
func (loop *Loop) finalRound(parent context.Context, values map[string]string, messages []modelMessage, budget setting.AgentBudget, emit func(string) error) (string, bool, error) {
	ctx, cancel := context.WithTimeout(parent, budget.ModelTimeout)
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
		return "", false, &modelError{Kind: "body_read", Retry: true, Base: ErrUnavailable}
	}
	var result modelResponse
	if err = json.Unmarshal(content, &result); err != nil || len(result.Choices) != 1 || len(result.Choices[0].Message.Calls) != 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", false, &modelError{Kind: "final_response_invalid", Base: errModelProtocol}
	}
	return result.Choices[0].Message.Content, false, nil
}

// requestModel 创建 OpenAI 兼容请求并返回分类后的脱敏失败。
func (loop *Loop) requestModel(ctx context.Context, values map[string]string, messages []modelMessage, choice *string) (*http.Response, error) {
	payload, err := json.Marshal(modelRequest{Model: values["llm_model"], Messages: messages, Tools: json.RawMessage(toolSchema), ToolChoice: choice, MaxTokens: 4096, Stream: true})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(values["llm_base_url"], "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, &modelError{Kind: "request_build", Base: errModelProtocol}
	}
	request.Header.Set("Authorization", "Bearer "+values["llm_api_key"])
	request.Header.Set("Content-Type", "application/json")
	response, err := loop.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &modelError{Kind: "network", Retry: true, Base: ErrUnavailable}
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		return nil, &modelError{Kind: fmt.Sprintf("upstream_%d", response.StatusCode), Status: response.StatusCode, Wait: retryAfter(response), Retry: response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500, Base: ErrUnavailable}
	}
	return response, nil
}

// retryAfter 解析有界的 Retry-After 秒数，不信任过大或非法提示。
func retryAfter(response *http.Response) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(response.Header.Get("Retry-After")))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// toolSchema 固定模型可见工具的 JSON Schema。
const toolSchema = `[{"type":"function","function":{"name":"bash","description":"读取资讯虚拟文件系统；支持 ls、find、grep、cat、head 与右侧为 head 的单管道。不是宿主机 Bash。","parameters":{"type":"object","properties":{"command":{"type":"string","maxLength":2048}},"required":["command"],"additionalProperties":false}}}]`
