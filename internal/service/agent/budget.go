package agent

import "errors"

const (
	maxListingRunes      = 16000
	maxModelContentBytes = 32768
)

var (
	errBudgetConfiguration = errors.New("Agent 预算配置无效")
	errRoundLimit          = errors.New("Agent 模型轮数达到上限")
	errToolLimit           = errors.New("Agent 工具调用次数达到上限")
	errToolOutputLimit     = errors.New("Agent 工具结果达到累计上限")
	errModelProtocol       = errors.New("模型响应协议无效")
	errFinishLength        = errors.New("模型回答达到长度上限")
	errInvalidCommand      = errors.New("VFS 命令无效")
	errUnsupportedCommand  = errors.New("VFS 命令不受支持")
)

// runUsage 记录单次运行的脱敏资源用量。
type runUsage struct {
	modelRounds     int
	toolCalls       int
	toolResultBytes int
}

// finalizationReason 表示可由最后一次无工具模型调用收束的预算终止原因。
type finalizationReason string

const (
	finalizeRoundLimit  finalizationReason = "round_limit"
	finalizeToolLimit   finalizationReason = "tool_limit"
	finalizeOutputLimit finalizationReason = "tool_output_limit"
	finalizeAnswerCut   finalizationReason = "answer_length_limit"
)
