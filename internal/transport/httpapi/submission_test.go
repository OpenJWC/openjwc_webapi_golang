package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/admin"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/contribution"
)

// TestSubmissionCompatibilityAndAtomicPublication 验证投稿响应、去重和审核发布的旧版完整流程。
func TestSubmissionCompatibilityAndAtomicPublication(t *testing.T) {
	router, store, token := testClient(t)
	body := `{"label":"教务","title":"实验课调课通知","date":"2026-04-10","detail_url":"https://example.com/submission","is_page":true,"content":{"text":"实验课改到周四","attachment_urls":["https://example.com/a.pdf"]}}`
	response := callClient(router, "POST", "/api/v1/client/submissions", body, token)
	if response.Code != 200 || strings.TrimSpace(response.Body.String()) != `{"msg":"提交成功","data":{}}` {
		t.Fatalf("投稿协议不符: %d %s", response.Code, response.Body)
	}
	if response := callClient(router, "POST", "/api/v1/client/submissions", body, token); response.Code != 422 {
		t.Fatalf("重复投稿未拒绝: %d", response.Code)
	}
	response = callClient(router, "GET", "/api/v1/client/submissions/my", "", token)
	var result envelope[submissionListData]
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Total != 1 {
		t.Fatalf("投稿数量不符: %s", response.Body)
	}
	item := result.Data.Notices[0]
	if item.Status != "pending" {
		t.Fatal("投稿不在待审核状态")
	}
	if _, err := store.Execute(context.Background(), admin.Request{Action: "review-submission", ID: item.ID, Values: map[string]string{"decision": "approved", "review": "内容属实"}}); err != nil {
		t.Fatal(err)
	}
	if _, found, err := store.FindByID(context.Background(), item.ID); err != nil || !found {
		t.Fatalf("审核后未原子发布: %v %v", found, err)
	}
	var summaries []contribution.Summary
	summaries, err := store.Submissions(context.Background(), 1)
	if err != nil || len(summaries) != 1 || summaries[0].Status != "approved" {
		t.Fatalf("审核状态未更新: %v %v", summaries, err)
	}
	if _, err := store.Execute(context.Background(), admin.Request{Action: "review-submission", ID: item.ID, Values: map[string]string{"decision": "approved"}}); err == nil {
		t.Fatal("重复审核被接受")
	}
	if _, err := store.Execute(context.Background(), admin.Request{Action: "delete-user", ID: "1"}); err != nil {
		t.Fatalf("已有投稿的用户无法删除: %v", err)
	}
	if _, found, err := store.FindByID(context.Background(), item.ID); err != nil || !found {
		t.Fatal("删除用户不应删除已发布资讯")
	}
}
