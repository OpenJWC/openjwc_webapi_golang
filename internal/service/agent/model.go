package agent

import "encoding/json"

// modelMessage 是 OpenAI 兼容协议的消息结构。
type modelMessage struct {
	Role    string     `json:"role"`
	Content string     `json:"content"`
	Calls   []toolCall `json:"tool_calls,omitempty"`
	CallID  string     `json:"tool_call_id,omitempty"`
}

// toolCall 描述模型请求的单次工具调用。
type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

// toolFunction 保存工具名称及待校验 JSON 参数。
type toolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// modelRequest 聚合模型调用参数与工具定义。
type modelRequest struct {
	Stream     bool            `json:"stream"`
	Model      string          `json:"model"`
	Messages   []modelMessage  `json:"messages"`
	Tools      json.RawMessage `json:"tools"`
	ToolChoice *string         `json:"tool_choice,omitempty"`
	MaxTokens  int             `json:"max_tokens"`
}

// modelResponse 只解析当前 Agent 使用的模型响应字段。
type modelResponse struct {
	Choices []modelChoice `json:"choices"`
}

// modelChoice 包含一次模型生成的消息。
type modelChoice struct {
	Message modelMessage `json:"message"`
	Finish  *string      `json:"finish_reason"`
}
