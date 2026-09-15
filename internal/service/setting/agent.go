package setting

import (
	"fmt"
	"strconv"
	"time"
)

const (
	agentRoundsKey     = "agent_max_model_rounds"
	agentToolsKey      = "agent_max_tool_calls"
	agentToolsRoundKey = "agent_max_tools_per_round"
	agentResultKey     = "agent_max_tool_result_bytes"
	agentTotalKey      = "agent_max_total_tool_bytes"
	agentModelTimeKey  = "agent_model_timeout_seconds"
	agentRunTimeKey    = "agent_run_timeout_seconds"
)

// AgentBudget 表示每次 Agent 运行读取的有界资源预算快照。
type AgentBudget struct {
	MaxModelRounds     int
	MaxToolCalls       int
	MaxToolsPerRound   int
	MaxToolResultBytes int
	MaxTotalToolBytes  int
	ModelTimeout       time.Duration
	RunTimeout         time.Duration
}

// AgentBudgetKeys 返回管理员可调整的 Agent 预算设置名。
func AgentBudgetKeys() []string {
	return []string{agentRoundsKey, agentToolsKey, agentToolsRoundKey, agentResultKey, agentTotalKey, agentModelTimeKey, agentRunTimeKey}
}

// IsAgentBudgetKey 判断设置名是否属于 Agent 资源预算。
func IsAgentBudgetKey(key string) bool {
	for _, candidate := range AgentBudgetKeys() {
		if key == candidate {
			return true
		}
	}
	return false
}

// ParseAgentBudget 从完整设置快照读取预算，并再次拒绝损坏持久化值。
func ParseAgentBudget(values map[string]string) (AgentBudget, error) {
	rounds, err := parseAgentSetting(values, agentRoundsKey, 1, 12)
	if err != nil {
		return AgentBudget{}, err
	}
	tools, err := parseAgentSetting(values, agentToolsKey, 1, 24)
	if err != nil {
		return AgentBudget{}, err
	}
	perRound, err := parseAgentSetting(values, agentToolsRoundKey, 1, 6)
	if err != nil {
		return AgentBudget{}, err
	}
	result, err := parseAgentSetting(values, agentResultKey, 1024, 32768)
	if err != nil {
		return AgentBudget{}, err
	}
	total, err := parseAgentSetting(values, agentTotalKey, 4096, 196608)
	if err != nil {
		return AgentBudget{}, err
	}
	modelTimeout, err := parseAgentSetting(values, agentModelTimeKey, 5, 90)
	if err != nil {
		return AgentBudget{}, err
	}
	runTimeout, err := parseAgentSetting(values, agentRunTimeKey, 20, 300)
	if err != nil {
		return AgentBudget{}, err
	}
	if total < result {
		return AgentBudget{}, fmt.Errorf("Agent 累计工具结果预算不能小于单次结果预算")
	}
	if runTimeout < modelTimeout {
		return AgentBudget{}, fmt.Errorf("Agent 总超时不能小于单次模型超时")
	}
	return AgentBudget{MaxModelRounds: rounds, MaxToolCalls: tools, MaxToolsPerRound: perRound, MaxToolResultBytes: result, MaxTotalToolBytes: total, ModelTimeout: time.Duration(modelTimeout) * time.Second, RunTimeout: time.Duration(runTimeout) * time.Second}, nil
}

// ValidateAgentBudgetValue 校验单个预算设置的硬边界。
func ValidateAgentBudgetValue(key string, value string) error {
	switch key {
	case agentRoundsKey:
		_, err := parseAgentValue(key, value, 1, 12)
		return err
	case agentToolsKey:
		_, err := parseAgentValue(key, value, 1, 24)
		return err
	case agentToolsRoundKey:
		_, err := parseAgentValue(key, value, 1, 6)
		return err
	case agentResultKey:
		_, err := parseAgentValue(key, value, 1024, 32768)
		return err
	case agentTotalKey:
		_, err := parseAgentValue(key, value, 4096, 196608)
		return err
	case agentModelTimeKey:
		_, err := parseAgentValue(key, value, 5, 90)
		return err
	case agentRunTimeKey:
		_, err := parseAgentValue(key, value, 20, 300)
		return err
	default:
		return fmt.Errorf("未知 Agent 预算设置: %s", key)
	}
}

// parseAgentSetting 读取已经包含默认值的单个预算字段。
func parseAgentSetting(values map[string]string, key string, low int, high int) (int, error) {
	return parseAgentValue(key, values[key], low, high)
}

// parseAgentValue 将管理员输入转换为受硬上限约束的整数。
func parseAgentValue(key string, value string, low int, high int) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < low || number > high {
		return 0, fmt.Errorf("%s 必须为 %d～%d", key, low, high)
	}
	return number, nil
}
