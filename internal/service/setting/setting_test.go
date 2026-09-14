package setting

import "testing"

// TestModelURLAllowsOnlySecureOrLoopbackEndpoints 验证本地模型无需 TLS，但远程明文和带凭据地址仍被拒绝。
func TestModelURLAllowsOnlySecureOrLoopbackEndpoints(t *testing.T) {
	cases := []struct {
		value string
		valid bool
	}{
		{value: "https://api.example.com/v1", valid: true},
		{value: "http://127.0.0.1:18080/v1", valid: true},
		{value: "http://[::1]:18080/v1", valid: true},
		{value: "http://localhost:18080/v1", valid: true},
		{value: "http://localhost.example.com/v1", valid: false},
		{value: "http://192.168.1.2/v1", valid: false},
		{value: "https://user:secret@example.com/v1", valid: false},
		{value: "https://api.example.com/v1?key=secret", valid: false},
	}
	for _, test := range cases {
		err := Validate("llm_base_url", test.value)
		if (err == nil) != test.valid {
			t.Errorf("地址 %q 校验结果错误: %v", test.value, err)
		}
	}
}
