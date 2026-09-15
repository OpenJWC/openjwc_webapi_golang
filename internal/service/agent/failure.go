package agent

import (
	"context"
	"errors"
	"fmt"
)

// FailureCode 是不含请求内容与供应商细节的稳定运行失败分类。
type FailureCode string

const (
	FailureBusy          FailureCode = "agent_busy"
	FailureUnavailable   FailureCode = "model_unavailable"
	FailureTimeout       FailureCode = "agent_timeout"
	FailureProtocol      FailureCode = "model_protocol_error"
	FailureConfiguration FailureCode = "agent_configuration_error"
	FailureInternal      FailureCode = "agent_failed"
	ToolInvalidCommand   FailureCode = "tool_invalid_command"
	ToolUnsupported      FailureCode = "tool_unsupported_command"
	ToolReadFailed       FailureCode = "tool_read_failed"
)

// Failure 保存可公开和可记录的脱敏运行状态。
type Failure struct {
	RunID           string
	Code            FailureCode
	ModelRounds     int
	ToolCalls       int
	ToolResultBytes int
	Cause           error
}

// Error 返回固定失败码，避免错误文本进入日志和响应。
func (failure *Failure) Error() string { return string(failure.Code) }

// Unwrap 保留内部错误链，仅供进程内部分类使用。
func (failure *Failure) Unwrap() error { return failure.Cause }

// classifyFailure 将内部错误映射为稳定且不泄露细节的公开码。
func classifyFailure(err error, usage runUsage) *Failure {
	code := FailureInternal
	switch {
	case errors.Is(err, ErrBusy):
		code = FailureBusy
	case errors.Is(err, ErrUnavailable):
		code = FailureUnavailable
	case errors.Is(err, context.DeadlineExceeded):
		code = FailureTimeout
	case errors.Is(err, errModelProtocol):
		code = FailureProtocol
	case errors.Is(err, errBudgetConfiguration):
		code = FailureConfiguration
	}
	return &Failure{Code: code, ModelRounds: usage.modelRounds, ToolCalls: usage.toolCalls, ToolResultBytes: usage.toolResultBytes, Cause: err}
}

// toolFailureCode 将工具错误归类为移动端可展示的固定码。
func toolFailureCode(err error) FailureCode {
	if errors.Is(err, errUnsupportedCommand) {
		return ToolUnsupported
	}
	if errors.Is(err, errInvalidCommand) {
		return ToolInvalidCommand
	}
	return ToolReadFailed
}

// safeToolObservation 返回可交给模型的固定失败提示，不转交内部错误文本。
func safeToolObservation(code FailureCode) string {
	return fmt.Sprintf("工具未完成（%s）。请改用受限 VFS 命令、规范路径或更小范围继续检索。", code)
}

// failureSummary 返回可展示且不包含底层错误的固定提示。
func failureSummary(code FailureCode) string {
	switch code {
	case FailureBusy:
		return "智能问答繁忙，请稍后重试。"
	case FailureUnavailable:
		return "模型服务暂时不可用，请稍后重试。"
	case FailureTimeout:
		return "问答处理超时，请缩小查询范围后重试。"
	case FailureProtocol:
		return "模型响应未完成，请稍后重试。"
	case FailureConfiguration:
		return "智能问答预算配置无效，请联系管理员。"
	default:
		return "问答未完成，请重试或缩小查询范围。"
	}
}
