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
