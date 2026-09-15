package agent

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// streamChunk 只解析可展示文本与工具增量，忽略 reasoning_content 等隐藏字段。
type streamChunk struct {
	Choices []streamChoice `json:"choices"`
}

// streamChoice 保存一个候选项的增量和结束原因。
type streamChoice struct {
	Index int `json:"index"`
	Delta struct {
		Content string       `json:"content"`
		Calls   []streamCall `json:"tool_calls"`
	} `json:"delta"`
	Finish *string `json:"finish_reason"`
}

// streamCall 通过索引关联可能跨多个帧传输的工具名称与参数。
type streamCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

// readModelStream 有界组装模型轮次，确认没有工具调用后才允许正文成为最终答案。
func readModelStream(reader io.Reader) (modelMessage, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	message := modelMessage{Role: "assistant"}
	calls := make(map[int]toolCall)
	data := ""
	finished := false
	total := 0
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		total += len(line) + 1
		if total > 2<<20 {
			return modelMessage{}, &modelError{Kind: "stream_oversize", Base: errModelProtocol}
		}
		if line != "" {
			if strings.HasPrefix(line, "data:") {
				if data != "" {
					data += "\n"
				}
				data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			}
			continue
		}
		if data == "" {
			continue
		}
		payload := data
		data = ""
		if payload == "[DONE]" {
			if !finished {
				return modelMessage{}, &modelError{Kind: "missing_done", Retry: true, Base: ErrUnavailable}
			}
			for index := 0; index < len(calls); index++ {
				call, ok := calls[index]
				if !ok {
					return modelMessage{}, &modelError{Kind: "tool_gap", Base: errModelProtocol}
				}
				message.Calls = append(message.Calls, call)
			}
			return message, nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return modelMessage{}, &modelError{Kind: "chunk_invalid", Base: errModelProtocol}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if finished || len(chunk.Choices) != 1 || chunk.Choices[0].Index != 0 {
			return modelMessage{}, &modelError{Kind: "choice_invalid", Base: errModelProtocol}
		}
		choice := chunk.Choices[0]
		message.Content += choice.Delta.Content
		if len(message.Content) > maxModelContentBytes {
			return modelMessage{}, &modelError{Kind: "content_oversize", Base: errModelProtocol}
		}
		for _, delta := range choice.Delta.Calls {
			if delta.Index < 0 || delta.Index >= 4 {
				return modelMessage{}, &modelError{Kind: "tool_index_invalid", Base: errModelProtocol}
			}
			call := calls[delta.Index]
			call.ID += delta.ID
			call.Type += delta.Type
			call.Function.Name += delta.Function.Name
			call.Function.Arguments += delta.Function.Arguments
			if len(call.ID) > 128 || len(call.Type) > 32 || len(call.Function.Name) > 32 || len(call.Function.Arguments) > 4096 {
				return modelMessage{}, &modelError{Kind: "tool_field_oversize", Base: errModelProtocol}
			}
			calls[delta.Index] = call
		}
		if choice.Finish != nil {
			if *choice.Finish != "stop" && *choice.Finish != "tool_calls" {
				if *choice.Finish == "length" {
					return modelMessage{}, &modelError{Kind: "finish_length", Base: errFinishLength}
				}
				return modelMessage{}, &modelError{Kind: "finish_invalid", Base: errModelProtocol}
			}
			if (*choice.Finish == "tool_calls") != (len(calls) > 0) {
				return modelMessage{}, &modelError{Kind: "finish_mismatch", Base: errModelProtocol}
			}
			finished = true
		}
	}
	if scanner.Err() != nil {
		return modelMessage{}, &modelError{Kind: "stream_truncated", Retry: true, Base: ErrUnavailable}
	}
	return modelMessage{}, &modelError{Kind: "stream_truncated", Retry: true, Base: ErrUnavailable}
}
