package crawljob

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
)

// Progress 组合当前任务状态和持久化来源摘要，不要求客户端维持长连接。
func (manager *Manager) Progress(ctx context.Context) (admin.Response, error) {
	manager.mutex.Lock()
	id, state, detail := manager.id, manager.state, manager.detail
	manager.mutex.Unlock()
	sources, err := manager.store.CrawlSources(ctx)
	if err != nil {
		return admin.Response{}, err
	}
	response := admin.Response{Text: detail, Columns: []string{"当前任务ID", "来源", "状态", "扫描", "保存", "跳过", "失败", "最近成功", "说明"}, Rows: [][]string{{id, "总任务", state, "", "", "", "", "", "来源行显示各来源最近一次记录"}}}
	for _, source := range sources {
		success := "未成功"
		if !source.LastSuccess.IsZero() {
			success = source.LastSuccess.UTC().Format(time.RFC3339)
		}
		response.Rows = append(response.Rows, []string{id, source.Name, string(source.State), fmt.Sprint(source.Scanned), fmt.Sprint(source.Saved), fmt.Sprint(source.Skipped), fmt.Sprint(source.Failed), success, source.Detail})
	}
	return response, nil
}
