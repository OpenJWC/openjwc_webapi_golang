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
		return "", fmt.Errorf("未知工具")
	}
	var args toolArguments
	decoder := json.NewDecoder(strings.NewReader(call.Function.Arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return "", fmt.Errorf("工具参数无效")
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("工具参数包含尾随内容")
	}
	parts, err := splitCommand(args.Command)
	if err != nil {
		return "", err
	}
	switch parts[0] {
	case "ls", "grep", "cat", "head":
	default:
		return "", fmt.Errorf("只允许资讯读取工具")
	}
	return args.Command, nil
}

// observe 对每次工具尝试发送配对事件，完整结果只进入服务端模型历史。
func (loop *Loop) observe(ctx context.Context, call toolCall, index int, writer *eventWriter) (string, error) {
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
	status := "completed"
	if toolErr != nil {
		status = "failed"
		output = "工具拒绝或失败：" + toolErr.Error()
	}
	if err := writer.send(ctx, Event{Type: ToolCompleted, ToolID: toolID, Name: "bash", Status: status, DurationMS: time.Since(started).Milliseconds()}); err != nil {
		return "", err
	}
	return clipRunes(output, maxToolResultRunes), nil
}

// runTool 保留直接工具校验入口，统一使用严格参数解析。
func (loop *Loop) runTool(ctx context.Context, call toolCall) (string, error) {
	command, err := toolCommand(call)
	if err != nil {
		return "", err
	}
	return loop.vfs.Execute(ctx, command)
}
