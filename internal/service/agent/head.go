package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// head 读取规范资讯的开头，支持常见的 -n 行数形式。
func (vfs *VFS) head(ctx context.Context, args []string) (string, error) {
	if len(args) == 2 {
		return vfs.read(ctx, []string{"cat", args[1], "0", "2000"})
	}
	file, lines, err := headFileArgs(args)
	if err != nil {
		return "", err
	}
	text, err := vfs.read(ctx, []string{"cat", file, "0", "12000"})
	if err != nil {
		return "", err
	}
	return headLines(text, lines), nil
}

// headFileArgs 解析带文件的受限 head 行数参数。
func headFileArgs(args []string) (string, int, error) {
	if len(args) == 4 && args[1] == "-n" {
		lines, err := parseHeadLines(args[2])
		return args[3], lines, err
	}
	if len(args) == 3 && strings.HasPrefix(args[1], "-n") && len(args[1]) > 2 {
		lines, err := parseHeadLines(strings.TrimPrefix(args[1], "-n"))
		return args[2], lines, err
	}
	if len(args) == 3 && strings.HasPrefix(args[1], "-") {
		lines, err := parseHeadLines(strings.TrimPrefix(args[1], "-"))
		return args[2], lines, err
	}
	return "", 0, fmt.Errorf("用法: head <文件> 或 head -n <行数> <文件>")
}

// pipelineHeadArgs 解析管道末端的 head 参数，不接受文件名。
func pipelineHeadArgs(args []string) (int, error) {
	if len(args) == 2 && strings.HasPrefix(args[1], "-n") && len(args[1]) > 2 {
		return parseHeadLines(strings.TrimPrefix(args[1], "-n"))
	}
	if len(args) == 2 && strings.HasPrefix(args[1], "-") && len(args[1]) > 1 {
		return parseHeadLines(strings.TrimPrefix(args[1], "-"))
	}
	if len(args) == 3 && args[1] == "-n" {
		return parseHeadLines(args[2])
	}
	return 0, fmt.Errorf("管道只支持 head -n <行数> 或 head -<行数>")
}

// parseHeadLines 拒绝零、负数和无界的行数。
func parseHeadLines(value string) (int, error) {
	lines, err := strconv.Atoi(value)
	if err != nil || lines < 1 || lines > 200 {
		return 0, fmt.Errorf("head 行数必须为 1～200")
	}
	return lines, nil
}

// headLines 截取文本前若干行，保留换行而不拆分 UTF-8。
func headLines(text string, lines int) string {
	parts := strings.SplitAfter(text, "\n")
	if len(parts) <= lines {
		return text
	}
	return strings.Join(parts[:lines], "") + "[输出已按 head 截断]\n"
}
