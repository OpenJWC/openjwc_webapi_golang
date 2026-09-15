package setting

import "testing"

// TestParseAgentBudgetRejectsInvalidPersistedRelationships 验证运行时再次拒绝损坏或互相矛盾的持久化预算。
func TestParseAgentBudgetRejectsInvalidPersistedRelationships(t *testing.T) {
	values := Defaults()
	budget, err := ParseAgentBudget(values)
	if err != nil || budget.MaxModelRounds != 8 || budget.MaxToolResultBytes != 16000 {
		t.Fatalf("默认预算解析错误: %#v %v", budget, err)
	}
	values[agentTotalKey] = "12000"
	if _, err = ParseAgentBudget(values); err == nil {
		t.Fatal("累计预算小于单次预算仍被接受")
	}
	values = Defaults()
	values[agentModelTimeKey] = "90"
	values[agentRunTimeKey] = "20"
	if _, err = ParseAgentBudget(values); err == nil {
		t.Fatal("总超时小于模型超时仍被接受")
	}
}

// TestValidateAgentBudgetValueEnforcesHardBounds 验证管理员设置不能突破编译期硬边界。
func TestValidateAgentBudgetValueEnforcesHardBounds(t *testing.T) {
	for _, entry := range []struct {
		key   string
		value string
		valid bool
	}{{agentRoundsKey, "12", true}, {agentRoundsKey, "13", false}, {agentToolsKey, "24", true}, {agentToolsKey, "25", false}, {agentResultKey, "1023", false}, {agentTotalKey, "196608", true}, {agentModelTimeKey, "4", false}, {agentRunTimeKey, "301", false}} {
		err := ValidateAgentBudgetValue(entry.key, entry.value)
		if (err == nil) != entry.valid {
			t.Errorf("%s=%s 校验错误: %v", entry.key, entry.value, err)
		}
	}
}
