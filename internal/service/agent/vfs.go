package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// Library 定义虚拟资讯视图唯一允许读取的存储能力。
type Library interface {
	FindByID(context.Context, string) (notice.Notice, bool, error)
	List(context.Context, notice.Filter) ([]notice.Notice, int, error)
	Labels(context.Context) ([]string, error)
	Catalog(context.Context) (notice.Catalog, error)
}

// VFS 是数据库的只读投影，时钟可替换且不访问宿主文件系统。
type VFS struct {
	library Library
	clock   func() time.Time
}

// timezoneContextKey 在单次运行内传递时区，date 指令据此返回本地日期时间。
type timezoneContextKey struct{}

// NewVFS 创建无会话状态的共享读取器。
func NewVFS(library Library) *VFS { return &VFS{library: library, clock: time.Now} }

// FilePath 为任意资讯 ID 提供稳定且不能穿越路径的规范文件名。
func FilePath(id string) string {
	return "/notices/" + base64.RawURLEncoding.EncodeToString([]byte(id)) + ".md"
}

// Execute 解释受限的只读 VFS 命令，目录仅映射查询而非宿主路径。
func (vfs *VFS) Execute(ctx context.Context, command string) (string, error) {
	parts, err := splitPipeline(command)
	if err != nil {
		return "", err
	}
	if len(parts) == 1 {
		return vfs.executeCommand(ctx, parts[0])
	}
	if parts[1][0] != "head" {
		return "", fmt.Errorf("管道末端只能使用 head")
	}
	lines, err := pipelineHeadArgs(parts[1])
	if err != nil {
		return "", err
	}
	if parts[0][0] == "head" || parts[0][0] == "date" {
		return "", fmt.Errorf("该命令不能作为管道左侧命令")
	}
	output, err := vfs.executeCommand(ctx, parts[0])
	if err != nil {
		return "", err
	}
	return headLines(output, lines), nil
}

// executeCommand 执行一个已经完成管道语法校验的单独 VFS 命令。
func (vfs *VFS) executeCommand(ctx context.Context, args []string) (string, error) {
	switch args[0] {
	case "ls", "grep", "find":
		return vfs.list(ctx, args)
	case "cat":
		return vfs.read(ctx, args)
	case "head":
		return vfs.head(ctx, args)
	case "date":
		return vfs.date(ctx, args)
	default:
		return "", fmt.Errorf("仅支持 ls、find、grep、cat、head、date")
	}
}

// date 返回配置时区的当前日期时间，帮助模型确定当下日期。
func (vfs *VFS) date(ctx context.Context, args []string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("date 不接受参数")
	}
	timezone, _ := ctx.Value(timezoneContextKey{}).(string)
	if timezone == "" {
		timezone = "UTC"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return "", fmt.Errorf("时区无效: %w", err)
	}
	return vfs.clock().In(location).Format(time.RFC3339), nil
}

// listing 返回有界元数据与规范路径，类别目录不改变文件身份。
func listing(items []notice.Notice) string {
	var output strings.Builder
	for _, item := range items {
		fmt.Fprintf(&output, "%s\tID=%s\t%s\t%s\t%s\n", FilePath(item.ID()), item.ID(), item.PublishedAt().Format("2006-01-02"), clipRunes(item.Label(), 60), clipRunes(item.Title(), 200))
	}
	return clipRunes(output.String(), maxListingRunes)
}

// findListing 仅输出可被 cat 使用的规范文件路径，模拟受限 find -type f 视图。
func findListing(items []notice.Notice) string {
	var output strings.Builder
	for _, item := range items {
		output.WriteString(FilePath(item.ID()) + "\n")
	}
	return clipRunes(output.String(), maxListingRunes)
}

// Metadata 返回真实覆盖范围、抓取时间和当前时间，不承诺资讯业务上仍有效。
func (vfs *VFS) Metadata(ctx context.Context, timezone string) (string, error) {
	catalog, err := vfs.library.Catalog(ctx)
	if err != nil {
		return "", err
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return "", fmt.Errorf("检索时区无效: %w", err)
	}
	last := "尚无完整成功抓取记录"
	if !catalog.LastCrawl.IsZero() {
		last = catalog.LastCrawl.UTC().Format(time.RFC3339)
	}
	interval := "空库"
	if catalog.Total > 0 {
		interval = catalog.First.Format("2006-01-02") + " 至 " + catalog.Latest.Format("2006-01-02")
	}
	return fmt.Sprintf("当前时间：%s；时区：%s。资讯 %d 条，发布日期范围：%s。最近完整抓取成功：%s。日期目录按存储的发布日期（UTC 日期）组织。发布时间、抓取时间不代表报名截止时间或当前有效性。", vfs.clock().In(location).Format(time.RFC3339), timezone, catalog.Total, interval, last), nil
}

// labelDirectories 以分页形式展示安全编码的类别路径。
func (vfs *VFS) labelDirectories(ctx context.Context, page int) (string, error) {
	labels, err := vfs.library.Labels(ctx)
	if err != nil {
		return "", err
	}
	start := min(len(labels), (page-1)*20)
	var output strings.Builder
	for _, label := range labels[start:min(len(labels), start+20)] {
		fmt.Fprintf(&output, "/by-label/%s\t%s\n", url.PathEscape(label), clipRunes(label, 100))
	}
	fmt.Fprintf(&output, "共 %d 类；第 %d 页", len(labels), page)
	return output.String(), nil
}

// clipRunes 按 Unicode 字符截断目录元数据，不破坏 UTF-8 编码。
func clipRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) > limit {
		return string(runes[:limit]) + "\n[内容已截断]"
	}
	return text
}

// clipUTF8 按 UTF-8 字节截断模型工具观察，并为截断说明预留空间。
func clipUTF8(text string, limit int) string {
	const suffix = "\n[内容已截断]"
	if len(text) <= limit {
		return text
	}
	if limit <= len(suffix) {
		return suffix[:limit]
	}
	cut := limit - len(suffix)
	for cut > 0 && (text[cut]&0xc0) == 0x80 {
		cut--
	}
	return text[:cut] + suffix
}
