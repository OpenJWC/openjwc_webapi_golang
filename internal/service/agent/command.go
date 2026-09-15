package agent

import (
	"fmt"
	"strings"
)

// splitCommand 解析单个受限 VFS 命令，不接受未引用的管道。
func splitCommand(command string) ([]string, error) {
	parts, err := splitPipeline(command)
	if err != nil {
		return nil, err
	}
	if len(parts) != 1 {
		return nil, fmt.Errorf("单个命令不接受管道")
	}
	return parts[0], nil
}

// splitPipeline 仅支持一个受限 VFS 管道，绝不进入宿主 Shell。
func splitPipeline(command string) ([][]string, error) {
	if len(command) > 2048 || strings.ContainsAny(command, "\x00\r\n;&<>`$\\") {
		return nil, fmt.Errorf("命令包含禁止的 Shell 语法")
	}
	segments := make([]string, 0, 2)
	start := 0
	var quote rune
	for index, char := range command {
		if quote != 0 {
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '|' {
			if len(segments) > 0 {
				return nil, fmt.Errorf("最多支持一个管道")
			}
			segments = append(segments, command[start:index])
			start = index + 1
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("命令引号未闭合")
	}
	segments = append(segments, command[start:])
	if len(segments) > 2 {
		return nil, fmt.Errorf("最多支持一个管道")
	}
	result := make([][]string, 0, len(segments))
	for _, segment := range segments {
		args, err := splitWords(segment)
		if err != nil {
			return nil, err
		}
		result = append(result, args)
	}
	return result, nil
}

// splitWords 仅处理引号分组，不进行变量、转义或通配符展开。
func splitWords(command string) ([]string, error) {
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
