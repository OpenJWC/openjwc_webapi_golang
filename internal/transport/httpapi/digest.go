package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
)

// ReportReader 定义已发布日报的读取能力。
type ReportReader interface {
	DailyContent(context.Context, string) (string, bool, error)
	LatestDaily(context.Context, string) (string, string, bool, error)
}

// dailyReport 校验日期及资讯权限后返回完成的日报。
func (client *Client) dailyReport(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	day := request.PathValue("date")
	if _, err := time.Parse("2006-01-02", day); err != nil {
		invalidInput(writer, "日期必须为 YYYY-MM-DD")
		return
	}
	content, found, err := client.dependencies.Reports.DailyContent(request.Context(), day)
	if err != nil {
		client.fail(writer, err)
		return
	}
	if !found {
		writeJSON(writer, 404, map[string]string{"detail": "该日期日报尚未发布"})
		return
	}
	writeJSON(writer, 200, envelope[map[string]string]{Message: "获取成功", Data: map[string]string{"date": day, "content": content}})
}

// dailyReportToday 按服务器本地时区返回最近一份已发布日报，避免客户端时钟偏差。
func (client *Client) dailyReportToday(writer http.ResponseWriter, request *http.Request, principal access.Principal) {
	settings, err := client.dependencies.Settings.Settings(request.Context())
	if err != nil {
		client.fail(writer, err)
		return
	}
	location, err := time.LoadLocation(settings["timezone"])
	if err != nil {
		client.fail(writer, err)
		return
	}
	today := time.Now().In(location).Format("2006-01-02")
	day, content, found, err := client.dependencies.Reports.LatestDaily(request.Context(), today)
	if err != nil {
		client.fail(writer, err)
		return
	}
	if !found {
		writeJSON(writer, 404, map[string]string{"detail": "尚无已发布日报"})
		return
	}
	writeJSON(writer, 200, envelope[map[string]string]{Message: "获取成功", Data: map[string]string{"date": day, "content": content}})
}
