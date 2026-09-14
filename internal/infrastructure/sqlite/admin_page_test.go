package sqlite

import (
	"context"
	"fmt"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"testing"
)

// TestAdminPaginationReturnsDifferentRecords 验证真实管理查询的第二页不会重复第一页。
func TestAdminPaginationReturnsDifferentRecords(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	for index := 0; index < 201; index++ {
		if err := store.Save(ctx, testNotice(t, fmt.Sprintf("id-%03d", index), "分页资讯")); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.Execute(ctx, admin.Request{Action: admin.ActionNotices, Page: 0})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Execute(ctx, admin.Request{Action: admin.ActionNotices, Page: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Rows) != 200 || len(second.Rows) != 1 || first.Rows[0][0] == second.Rows[0][0] {
		t.Fatalf("分页结果不符: first=%d second=%d", len(first.Rows), len(second.Rows))
	}
}

// TestAdminOutOfRangePageFallsBackToLastPage 验证删空末页或过期页码回退到同一快照中的有效页。
func TestAdminOutOfRangePageFallsBackToLastPage(t *testing.T) {
	store := openTestStore(t)
	response, err := store.Execute(context.Background(), admin.Request{Action: admin.ActionNotices, Page: 99})
	if err != nil || response.Pagination == nil || response.Pagination.Index != 0 || response.Pagination.Total != 0 {
		t.Fatalf("页码未回退: %+v %v", response.Pagination, err)
	}
}
