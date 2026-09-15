package agent

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
)

// list 将虚拟目录及筛选参数转换为数据库查询，不扩大授权范围。
func (vfs *VFS) list(ctx context.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("用法: ls <目录> [页码]，grep '关键词' <目录> [筛选参数]")
	}
	directory := args[1]
	query := ""
	options := args[2:]
	if args[0] == "grep" {
		if len(args) < 3 {
			return "", fmt.Errorf("grep 缺少目录")
		}
		query = args[1]
		directory = args[2]
		options = args[3:]
	}
	if args[0] == "find" && len(options) >= 2 && options[0] == "-type" {
		if options[1] != "f" {
			return "", fmt.Errorf("find 仅支持 -type f")
		}
		options = options[2:]
	}
	filter, page, err := parseFilter(options)
	if err != nil {
		return "", err
	}
	filter.Query = query
	if directory != "/" {
		directory = strings.TrimSuffix(directory, "/")
	}
	if path.Clean(directory) != directory {
		return "", fmt.Errorf("目录路径无效")
	}
	if directory == "/" {
		if query != "" {
			return "", fmt.Errorf("请在具体目录内 grep")
		}
		return "/notices/ 稳定规范文件\n/recent/ 最近30个UTC日期（优先入口）\n/by-label/ 类别视图\n/by-date/ 日期视图\nls/find/grep 可用 --from YYYY-MM-DD --to YYYY-MM-DD --label 类别 --sort newest|relevance --page N；find 可选 -type f。cat 文件 [字符偏移] [字符数量] 支持续读；可用单管道 | head -n N。", nil
	}
	if directory == "/by-label" {
		return vfs.labelDirectories(ctx, page)
	}
	if directory == "/by-date" {
		return vfs.dateDirectories(ctx, page)
	}
	switch {
	case directory == "/notices":
	case directory == "/recent":
		today := vfs.clock().UTC().Truncate(24 * time.Hour)
		intersectDates(&filter, today.AddDate(0, 0, -29), today.AddDate(0, 0, 1))
	case strings.HasPrefix(directory, "/by-label/"):
		label, err := url.PathUnescape(strings.TrimPrefix(directory, "/by-label/"))
		if err != nil || label == "" || len(label) > 256 || (filter.Label != "" && filter.Label != label) {
			return "", fmt.Errorf("类别路径与筛选条件冲突或无效")
		}
		filter.Label = label
	case strings.HasPrefix(directory, "/by-date/"):
		month, err := time.Parse("2006/01", strings.TrimPrefix(directory, "/by-date/"))
		if err != nil {
			return "", fmt.Errorf("日期目录格式为 /by-date/YYYY/MM")
		}
		intersectDates(&filter, month, month.AddDate(0, 1, 0))
	default:
		return "", fmt.Errorf("未知虚拟目录")
	}
	items, total, err := vfs.library.List(ctx, filter)
	if err != nil {
		return "", err
	}
	output := listing(items)
	if args[0] == "find" {
		output = findListing(items)
	}
	return output + fmt.Sprintf("\n共 %d 条；第 %d/%d 页；排序 %s。没有结果时可扩大日期范围或换用关键词。", total, page, max(1, (total+19)/20), filter.Order), nil
}

// parseFilter 校验封闭选项及包含末日的日期范围，默认最新优先。
func parseFilter(args []string) (notice.Filter, int, error) {
	filter := notice.Filter{Limit: 20, Order: notice.Newest}
	page := 1
	if len(args) > 0 && !strings.HasPrefix(args[0], "--") {
		number, err := strconv.Atoi(args[0])
		if err != nil {
			return filter, 0, fmt.Errorf("页码无效")
		}
		page = number
		args = args[1:]
	}
	if len(args)%2 != 0 {
		return filter, 0, fmt.Errorf("筛选选项缺少值")
	}
	seen := make(map[string]bool)
	for index := 0; index < len(args); index += 2 {
		key, value := args[index], args[index+1]
		if seen[key] {
			return filter, 0, fmt.Errorf("重复筛选选项")
		}
		seen[key] = true
		switch key {
		case "--page":
			number, err := strconv.Atoi(value)
			if err != nil {
				return filter, 0, err
			}
			page = number
		case "--label":
			filter.Label = value
		case "--sort":
			filter.Order = notice.Order(value)
			if filter.Order != notice.Newest && filter.Order != notice.Relevance {
				return filter, 0, fmt.Errorf("排序只能为 newest 或 relevance")
			}
		case "--from", "--to":
			date, err := time.Parse("2006-01-02", value)
			if err != nil {
				return filter, 0, fmt.Errorf("日期格式无效")
			}
			if key == "--from" {
				filter.Since = date
			} else {
				filter.Before = date.AddDate(0, 0, 1)
			}
		default:
			return filter, 0, fmt.Errorf("未知筛选选项")
		}
	}
	if page < 1 || page > 10000 {
		return filter, 0, fmt.Errorf("页码超出范围")
	}
	filter.Offset = (page - 1) * 20
	return filter, page, nil
}

// intersectDates 将显式筛选与目录范围求交集，不能通过选项逃逸目录含义。
func intersectDates(filter *notice.Filter, since, before time.Time) {
	if filter.Since.IsZero() || filter.Since.Before(since) {
		filter.Since = since
	}
	if filter.Before.IsZero() || filter.Before.After(before) {
		filter.Before = before
	}
}

// dateDirectories 按覆盖范围展示月份视图，允许某个月份实际为空。
func (vfs *VFS) dateDirectories(ctx context.Context, page int) (string, error) {
	catalog, err := vfs.library.Catalog(ctx)
	if err != nil {
		return "", err
	}
	if catalog.Total == 0 {
		return "资讯库为空", nil
	}
	latest := time.Date(catalog.Latest.Year(), catalog.Latest.Month(), 1, 0, 0, 0, 0, time.UTC)
	first := time.Date(catalog.First.Year(), catalog.First.Month(), 1, 0, 0, 0, 0, time.UTC)
	start := latest.AddDate(0, -(page-1)*20, 0)
	var output strings.Builder
	for count := 0; count < 20 && !start.Before(first); count++ {
		fmt.Fprintf(&output, "/by-date/%s\n", start.Format("2006/01"))
		start = start.AddDate(0, -1, 0)
	}
	output.WriteString("月份视图按覆盖范围生成，可能为空；可使用页码继续查看更早月份。")
	return output.String(), nil
}
