package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// toolArguments 是模型允许传入的唯一工具参数结构。
type toolArguments struct {
	Command string `json:"command"`
}

// toolCommand 拒绝未知字段、尾随 JSON 和 shell 语法，不把未经校验的参数发给客户端。
func toolCommand(call toolCall) (string, error) {
	if call.Type != "function" || call.Function.Name != "bash" {
		return "", fmt.Errorf("%w: 未知工具", errUnsupportedCommand)
	}
	var args toolArguments
	decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return "", fmt.Errorf("%w: 工具参数无效", errInvalidCommand)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("%w: 工具参数包含尾随内容", errInvalidCommand)
	}
	parts, err := splitPipeline(args.Command)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errInvalidCommand, err)
	}
	for index, part := range parts {
		switch part[0] {
		case "ls", "find", "grep", "cat", "head":
		default:
			return "", fmt.Errorf("%w: 只允许受限资讯读取工具", errUnsupportedCommand)
		}
		if index == 1 && part[0] != "head" {
			return "", fmt.Errorf("%w: 管道末端只能使用 head", errInvalidCommand)
		}
	}
	if len(parts) == 2 && parts[0][0] == "head" {
		return "", fmt.Errorf("%w: head 不能作为管道左侧命令", errInvalidCommand)
	}
	return args.Command, nil
}

// observe 对每次工具尝试发送配对事件，完整结果只进入服务端模型历史。
func (loop *Loop) observe(ctx context.Context, call toolCall, index int, resultBytes int, writer *eventWriter) (string, error) {
	command, toolErr := toolCommand(call)
	summary := "参数校验未通过"
	if toolErr == nil {
		summary = clipRunes(command, 200)
	}
	toolID := fmt.Sprintf("tool-%d", index)
	if err := writer.send(ctx, Event{Type: ToolStarted, ToolID: toolID, Name: "bash", Summary: summary}); err != nil {
		return "", err
	}
	started := time.Now()
	output := ""
	if toolErr == nil {
		output, toolErr = loop.vfs.Execute(ctx, command)
	}
	status, code := "completed", FailureCode("")
	if toolErr != nil {
		status = "failed"
		code = toolFailureCode(toolErr)
		output = safeToolObservation(code)
	} else {
		output = clipUTF8(output, resultBytes)
	}
	if err := writer.send(ctx, Event{Type: ToolCompleted, ToolID: toolID, Name: "bash", Status: status, DurationMS: time.Since(started).Milliseconds(), Code: string(code)}); err != nil {
		return "", err
	}
	return output, nil
}

// runTool 保留直接工具校验入口，统一使用严格参数解析。
func (loop *Loop) runTool(ctx context.Context, call toolCall) (string, error) {
	command, err := toolCommand(call)
	if err != nil {
		return "", err
	}
	return loop.vfs.Execute(ctx, command)
}
