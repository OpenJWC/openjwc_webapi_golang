package agent

import (
	"bufio"
	"encoding/json"
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
			return &modelError{Kind: "stream_oversize", Base: errModelProtocol}
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
				return &modelError{Kind: "missing_done", Retry: true, Base: ErrUnavailable}
			}
			return nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return &modelError{Kind: "chunk_invalid", Base: errModelProtocol}
		}
		choice, err := singleStreamChoice(chunk, finished)
		if err != nil {
			return err
		}
		if len(choice.Delta.Calls) > 0 {
			return &modelError{Kind: "final_tool_call", Base: errModelProtocol}
		}
		if choice.Delta.Content != "" {
			contentBytes += len(choice.Delta.Content)
			if contentBytes > maxModelContentBytes {
				return &modelError{Kind: "content_oversize", Base: errModelProtocol}
			}
			received = true
			if err = emit(choice.Delta.Content); err != nil {
				return err
			}
		}
		if choice.Finish != nil {
			if *choice.Finish == "length" {
				return &modelError{Kind: "finish_length", Base: errModelProtocol}
			}
			if *choice.Finish != "stop" {
				return &modelError{Kind: "finish_invalid", Base: errModelProtocol}
			}
			finished = true
		}
	}
	if scanner.Err() != nil {
		return &modelError{Kind: "stream_truncated", Retry: true, Base: ErrUnavailable}
	}
	return &modelError{Kind: "stream_truncated", Retry: true, Base: ErrUnavailable}
}

// singleStreamChoice 约束最终流只包含一个尚未结束的候选项。
func singleStreamChoice(chunk streamChunk, finished bool) (streamChoice, error) {
	if len(chunk.Choices) != 1 || chunk.Choices[0].Index != 0 || finished {
		return streamChoice{}, &modelError{Kind: "choice_invalid", Base: errModelProtocol}
	}
	return chunk.Choices[0], nil
}
