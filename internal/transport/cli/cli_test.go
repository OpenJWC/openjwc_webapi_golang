package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestHelpDoesNotRequireConfiguration 验证帮助无需有效运行配置且管道输出没有终端装饰。
func TestHelpDoesNotRequireConfiguration(t *testing.T) {
	t.Setenv("OPENJWC_HTTP_ADDRESS", "invalid")
	for _, args := range [][]string{{"-h"}, {"service", "install", "--help"}, {"status", "-h"}, {"--version"}} {
		var output bytes.Buffer
		code, err := Run(context.Background(), args, &output)
		if code != 0 || err != nil || output.Len() == 0 || strings.Contains(output.String(), "\x1b") {
			t.Fatalf("帮助不符合约定: %v %d %v", args, code, err)
		}
	}
}

// TestInvalidCLIArgumentsReturnUsageCode 验证未知命令、冲突作用域和无效选项返回用法错误。
func TestInvalidCLIArgumentsReturnUsageCode(t *testing.T) {
	for _, args := range [][]string{{"missing"}, {"start", "--user", "--system"}, {"stop", "--now"}, {"serve", "--json"}, {"--bad"}} {
		var output bytes.Buffer
		code, err := Run(context.Background(), args, &output)
		if code != 2 || err == nil {
			t.Fatalf("无效参数未拒绝: %v %d %v", args, code, err)
		}
	}
}
