package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// read 按字符偏移读取完整资讯投影，使长正文可以在预算内分段访问。
func (vfs *VFS) read(ctx context.Context, args []string) (string, error) {
	if len(args) < 2 || len(args) > 4 || (args[0] == "head" && len(args) != 2) {
		return "", fmt.Errorf("用法: cat <文件> [字符偏移] [字符数量]；head <文件>")
	}
	file := args[1]
	if !strings.HasPrefix(file, "/notices/") || path.Clean(file) != file || path.Ext(file) != ".md" {
		return "", fmt.Errorf("只能读取规范资讯路径")
	}
	id, err := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(file, "/notices/"), ".md"))
	if err != nil || len(id) == 0 || len(id) > 256 || FilePath(string(id)) != file {
		return "", fmt.Errorf("资讯文件名无效")
	}
	offset, limit := 0, 12000
	if args[0] == "head" {
		limit = 2000
	}
	if len(args) >= 3 {
		offset, err = strconv.Atoi(args[2])
		if err != nil || offset < 0 || offset > 2000000 {
			return "", fmt.Errorf("字符偏移无效")
		}
	}
	if len(args) == 4 {
		limit, err = strconv.Atoi(args[3])
		if err != nil || limit < 1 || limit > 12000 {
			return "", fmt.Errorf("读取数量必须为 1～12000 字符")
		}
	}
	item, found, err := vfs.library.FindByID(ctx, string(id))
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("资讯文件不存在")
	}
	var output strings.Builder
	fmt.Fprintf(&output, "ID: %s\n标题: %s\n类别: %s\n发布日期: %s\n官方链接: %s\n\n%s\n\n附件链接（未下载或解析内容）：\n", item.ID(), item.Title(), item.Label(), item.PublishedAt().Format("2006-01-02"), item.DetailURL(), item.Content())
	for _, attachment := range item.Attachments() {
		fmt.Fprintf(&output, "%s: %s\n", attachment.Name, attachment.URL)
	}
	text := []rune(output.String())
	if offset > len(text) {
		return "", fmt.Errorf("字符偏移超过总长度 %d", len(text))
	}
	end := min(len(text), offset+limit)
	suffix := fmt.Sprintf("\n[字符 %d:%d / %d]", offset, end, len(text))
	if end < len(text) {
		suffix += fmt.Sprintf("\n续读: cat %s %d %d", file, end, limit)
	} else {
		suffix += " EOF"
	}
	return string(text[offset:end]) + suffix, nil
}
