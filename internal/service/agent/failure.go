package agent

import (
	"context"
	"errors"
	"fmt"
	"time"
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

// modelError 保存一次模型调用的脱敏失败细节，Kind 与状态码不包含响应内容。
type modelError struct {
	Kind   string
	Status int
	Wait   time.Duration
	Retry  bool
	Base   error
}

// Error 返回固定细节标识，仅供日志与排障使用。
func (err *modelError) Error() string { return err.Kind }

// Unwrap 暴露稳定错误链，使公开失败分类可以继续使用哨兵判断。
func (err *modelError) Unwrap() error { return err.Base }

// Failure 保存可公开和可记录的脱敏运行状态。
type Failure struct {
	RunID           string
	Code            FailureCode
	Detail          string
	UpstreamStatus  int
	ModelRounds     int
	ToolCalls       int
	ToolResultBytes int
	Cause           error
}

// Error 返回固定失败码，避免错误文本进入日志和响应。
func (failure *Failure) Error() string { return string(failure.Code) }

// Unwrap 保留内部错误链，仅供进程内部分类使用。
func (failure *Failure) Unwrap() error { return failure.Cause }

// classifyFailure 将内部错误映射为稳定且不泄露细节的公开码与日志细节。
func classifyFailure(err error, usage runUsage) *Failure {
	failure := &Failure{Code: FailureInternal, ModelRounds: usage.modelRounds, ToolCalls: usage.toolCalls, ToolResultBytes: usage.toolResultBytes, Cause: err}
	var model *modelError
	if errors.As(err, &model) {
		failure.Detail = model.Kind
		failure.UpstreamStatus = model.Status
	}
	switch {
	case errors.Is(err, ErrBusy):
		failure.Code = FailureBusy
	case errors.Is(err, ErrUnavailable):
		failure.Code = FailureUnavailable
	case errors.Is(err, context.DeadlineExceeded):
		failure.Code = FailureTimeout
	case errors.Is(err, errModelProtocol), errors.Is(err, errFinishLength):
		failure.Code = FailureProtocol
	case errors.Is(err, errBudgetConfiguration):
		failure.Code = FailureConfiguration
	}
	return failure
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
