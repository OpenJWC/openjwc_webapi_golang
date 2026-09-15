package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// readFinalStream 校验禁用工具的最终 SSE，并在读取到正文分片时立即发布。
func readFinalStream(reader io.Reader, emit func(string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	data := ""
	finished, received := false, false
	total, contentBytes := 0, 0
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		total += len(line) + 1
		if total > 2<<20 {
			return fmt.Errorf("%w: 模型流过大", errModelProtocol)
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
			if !finished || !received {
				return fmt.Errorf("%w: 最终模型流不完整", errModelProtocol)
			}
			return nil
		}
		var chunk streamChunk
		if err := decodeStreamChunk(payload, &chunk); err != nil {
			return err
		}
		choice, err := singleStreamChoice(chunk, finished)
		if err != nil {
			return err
		}
		if len(choice.Delta.Calls) > 0 {
			return fmt.Errorf("%w: 最终回答不能调用工具", errModelProtocol)
		}
		if choice.Delta.Content != "" {
			contentBytes += len(choice.Delta.Content)
			if contentBytes > maxModelContentBytes {
				return fmt.Errorf("%w: 最终回答过长", errModelProtocol)
			}
			received = true
			if err = emit(choice.Delta.Content); err != nil {
				return err
			}
		}
		if choice.Finish != nil {
			if *choice.Finish != "stop" {
				return fmt.Errorf("%w: 最终回答未正常停止", errModelProtocol)
			}
			finished = true
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%w: 读取模型流失败", errModelProtocol)
	}
	return fmt.Errorf("%w: 模型流提前结束", errModelProtocol)
}

// decodeStreamChunk 将供应商帧解析为当前 Agent 所需的最小结构。
func decodeStreamChunk(payload string, chunk *streamChunk) error {
	if err := json.Unmarshal([]byte(payload), chunk); err != nil {
		return fmt.Errorf("%w: 模型帧无效", errModelProtocol)
	}
	return nil
}

// singleStreamChoice 约束最终流只包含一个尚未结束的候选项。
func singleStreamChoice(chunk streamChunk, finished bool) (streamChoice, error) {
	if len(chunk.Choices) != 1 || chunk.Choices[0].Index != 0 || finished {
		return streamChoice{}, fmt.Errorf("%w: 模型候选项无效", errModelProtocol)
	}
	return chunk.Choices[0], nil
}
