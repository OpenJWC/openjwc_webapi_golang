package agent

import (
	"fmt"
	"strings"
)

// splitCommand 仅处理引号分组，拒绝 shell 元字符及不闭合引号。
func splitCommand(command string) ([]string, error) {
	if len(command) > 2048 || strings.ContainsAny(command, "\x00\r\n;|&<>`$\\") {
		return nil, fmt.Errorf("命令包含禁止的 shell 语法")
	}
	var args []string
	var token strings.Builder
	var quote rune
	for _, char := range command {
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				token.WriteRune(char)
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == ' ' || char == '\t' {
			if token.Len() > 0 {
				args = append(args, token.String())
				token.Reset()
			}
			continue
		}
		token.WriteRune(char)
	}
	if quote != 0 {
		return nil, fmt.Errorf("命令引号未闭合")
	}
	if token.Len() > 0 {
		args = append(args, token.String())
	}
	if len(args) == 0 || len(args) > 18 {
		return nil, fmt.Errorf("命令参数数量无效")
	}
	return args, nil
}
