package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// complete 执行有界模型请求，不将响应正文或凭据写入错误。
func (loop *Loop) complete(ctx context.Context, settings map[string]string, messages []modelMessage) (modelMessage, error) {
	payload, err := json.Marshal(modelRequest{Model: settings["llm_model"], Messages: messages, Tools: json.RawMessage(toolSchema), MaxTokens: 4096, Stream: true})
	if err != nil {
		return modelMessage{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(settings["llm_base_url"], "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return modelMessage{}, ErrUnavailable
	}
	request.Header.Set("Authorization", "Bearer "+settings["llm_api_key"])
	request.Header.Set("Content-Type", "application/json")
	response, err := loop.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return modelMessage{}, ctx.Err()
		}
		return modelMessage{}, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return modelMessage{}, fmt.Errorf("模型服务状态 %d: %w", response.StatusCode, ErrUnavailable)
	}
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
		return modelMessage{}, ErrUnavailable
	}
	return result.Choices[0].Message, nil
}

// toolSchema 固定模型可见工具的 JSON Schema。
const toolSchema = `[{"type":"function","function":{"name":"bash","description":"读取资讯虚拟文件系统，先 ls / 查看类别、日期和近期目录；ls/grep 支持日期范围与排序；cat 支持字符偏移续读。不是宿主机 Bash。","parameters":{"type":"object","properties":{"command":{"type":"string","maxLength":2048}},"required":["command"],"additionalProperties":false}}}]`
